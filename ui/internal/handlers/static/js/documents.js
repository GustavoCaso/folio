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
    const errorText = card.querySelector("[data-error-text]");
    const actions = card.querySelector("[data-actions]");
    const filename = card.querySelector("[data-filename]")?.textContent || "";

    if (d.Status && statusBadge) {
      statusBadge.innerHTML = badgeHTML(d.Status);
    }

    if (d.Status === "DONE") {
      if (errorText) errorText.textContent = "";
      if (actions && !actions.querySelector("a[href]")) {
        actions.innerHTML = readButtonHTML(jobID) + deleteFormButton(jobID, filename);
      }
      es.close();
    } else if (d.Status === "FAILED") {
      if (errorText) errorText.textContent = d.Error || "";
      if (actions && !actions.querySelector("form.retry-form")) {
        actions.innerHTML = retryFormHTML(jobID) + deleteFormButton(jobID, filename);
      }
      es.close();
    } else if (d.Stage && errorText) {
      let text = d.Stage;
      if (d.Message) text += " — " + d.Message;
      if (d.PagesTotal) text += " (" + d.PagesDone + "/" + d.PagesTotal + ")";
      errorText.textContent = text;
      if ((actions) && !actions.querySelector("[data-cancel-job]")) {
        actions.innerHTML = cancelButtonHTML(jobID);
      }
    }
  });
}

function bootstrap() {
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
