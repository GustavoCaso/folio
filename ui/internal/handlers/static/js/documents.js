function badgeHTML(status) {
  const variants = {
    PENDING: "bg-secondary text-secondary-foreground",
    PROCESSING: "border border-input bg-background text-foreground",
    FAILED: "bg-destructive text-white",
  };
  const cls = variants[status] || variants.PENDING;
  const label = status.charAt(0) + status.slice(1).toLowerCase();
  return `<span class="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-semibold ${cls}">${label}</span>`;
}

function readButtonHTML(jobID) {
  return `<a href="/read/${jobID}" class="inline-flex items-center justify-center rounded-md border border-input bg-background px-3 py-1.5 text-xs font-medium shadow-xs hover:bg-accent hover:text-accent-foreground">Read</a>`;
}

function deleteFormButton(jobID, filename) {
  return `<button type="button" data-delete-job="${jobID}" data-job-name="${filename}" class="inline-flex items-center justify-center rounded-md px-3 py-1.5 text-xs font-medium text-destructive hover:bg-accent">Delete</button>`
}

function cancelButtonHTML(jobID, filename) {
  return `<button type="button" data-cancel-job="${jobID}" data-job-name="${filename}" class="inline-flex items-center justify-center rounded-md px-3 py-1.5 text-xs font-medium text-destructive hover:bg-accent">Cancel</button>`;
}

function retryFormHTML(jobID) {
  return `<form class="retry-form" method="POST" action="/documents/${jobID}/retry">` +
    `<button type="submit" data-retry-job="${jobID}" class="inline-flex items-center justify-center rounded-md px-3 py-1.5 text-xs font-medium hover:bg-accent">Retry</button>` +
    `</form>`;
}

export function watchJob(jobID) {
  const card = document.getElementById("job-" + jobID);
  if (!card) return;

  const es = new EventSource("/jobs/" + jobID + "/watch");
  es.addEventListener("status", function (e) {
    const d = JSON.parse(e.data);

    const statusBadge = card.querySelector("[data-status-badge]");
    const statusText = card.querySelector("[data-status-text]");
    const actions = card.querySelector("[data-actions]");
    const filename = card.querySelector("[data-filename]")?.textContent || "";

    if (d.Status && statusBadge) {
      statusBadge.innerHTML = badgeHTML(d.Status);
    }

    if (d.Status === "DONE") {
      if (statusText) statusText.textContent = "";
      if (actions && !actions.querySelector("a[href]")) {
        actions.innerHTML = readButtonHTML(jobID) + deleteFormButton(jobID, filename);
      }
      const meta = card.querySelector("[data-metadata]");
      if (meta && (d.Cover || d.Title || d.Author)) {
        if (d.Cover && !meta.querySelector("[data-cover]")) {
          const img = document.createElement("img");
          img.dataset.cover = "";
          img.src = "data:image/png;base64," + d.Cover;
          img.alt = "cover";
          img.className = "w-full rounded object-cover aspect-[3/4] bg-muted";
          meta.prepend(img);
        }
        if (d.Title && !meta.querySelector("[data-title]")) {
          const p = document.createElement("p");
          p.dataset.title = "";
          p.className = "text-xs font-medium truncate";
          p.textContent = "Title: " + d.Title;
          meta.append(p);
        }
        if (d.Author && !meta.querySelector("[data-author]")) {
          const p = document.createElement("p");
          p.dataset.author = "";
          p.className = "text-xs text-muted-foreground truncate";
          p.textContent = "Author: " + d.Author;
          meta.append(p);
        }
      }
      es.close();
    } else if (d.Status === "FAILED") {
      if (statusText) statusText.textContent = d.Error || "";
      if (actions && !actions.querySelector("form.retry-form")) {
        actions.innerHTML = retryFormHTML(jobID) + deleteFormButton(jobID, filename);
      }
      es.close();
    } else if (d.Message && statusText) {
      statusText.textContent = d.Message;
      if (actions && !actions.querySelector("[data-cancel-job]")) {
        actions.innerHTML = cancelButtonHTML(jobID);
      }
    }
  });
}

function initDropZone() {
  const zone = document.getElementById("drop-zone");
  const fileInput = document.getElementById("file-input");
  const actions = document.getElementById("upload-actions");
  const label = document.getElementById("drop-label");
  const filenameSpan = document.getElementById("selected-filename");
  if (!zone || !fileInput) return;

  function selectFile(file) {
    if (!file || file.type !== "application/pdf") return;
    const dt = new DataTransfer();
    dt.items.add(file);
    fileInput.files = dt.files;
    label.textContent = "PDF ready";
    filenameSpan.textContent = file.name;
    actions.classList.remove("hidden");
    actions.classList.add("flex");
    zone.classList.add("border-foreground/40", "bg-accent/30");
  }

  zone.addEventListener("click", () => fileInput.click());
  zone.addEventListener("keydown", (e) => { if (e.key === "Enter" || e.key === " ") fileInput.click(); });

  fileInput.addEventListener("change", () => {
    if (fileInput.files[0]) selectFile(fileInput.files[0]);
  });

  zone.addEventListener("dragover", (e) => {
    e.preventDefault();
    zone.classList.add("border-foreground/40", "bg-accent/30");
  });
  zone.addEventListener("dragleave", () => {
    if (!fileInput.files[0]) zone.classList.remove("border-foreground/40", "bg-accent/30");
  });
  zone.addEventListener("drop", (e) => {
    e.preventDefault();
    selectFile(e.dataTransfer.files[0]);
  });
}

function bootstrap() {
  initDropZone();
  const cfg = document.getElementById("watch-config");
  if (cfg) cfg.dataset.jobIds.split(",").forEach(watchJob);

  document.addEventListener("click", async (e) => {
    // Cancel button
    const cancelBtn = e.target.closest("[data-cancel-job]");
    if (cancelBtn) {
      if (confirm(`Cancel Job ?`)) {
        const id = cancelBtn.dataset.cancelJob;
        const resp = await fetch(`/documents/${id}/cancel`, { method: "POST", headers: { "Accept": "text/html" } });

        if (!resp.ok) {
          const html = await resp.text();
          document.body.insertAdjacentHTML("beforeend", html);
        }

        // SSE FAILED event drives the card transition; no DOM change here
        return;
      }
    }

    // Delete button
    const deleteBtn = e.target.closest("[data-delete-job]");
    if (deleteBtn) {
      const id = deleteBtn.dataset.deleteJob;
      const jobName = deleteBtn.dataset.jobName;
      if (confirm(`Delete ${jobName} ?`)) {
        const resp = await fetch(`/documents/${id}`, { method: "DELETE", headers: { "Accept": "text/html" } });
        const html = await resp.text();
        document.body.insertAdjacentHTML("beforeend", html);
        if (resp.ok) {
          const jobCard = document.getElementById(`job-${id}`)
          if (jobCard) jobCard.remove();
        }
        return;
      }
    }
  })
}

if (typeof document !== "undefined") {
  bootstrap();
}
