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

function deleteFormHTML(jobID, filename) {
  return `<form class="delete-form" method="POST" action="/documents/${jobID}/delete" data-filename="${filename}">` +
    `<button type="submit" class="inline-flex items-center justify-center rounded-md px-3 py-1.5 text-xs font-medium text-destructive hover:bg-accent">Delete</button>` +
    `</form>`;
}

function retryFormHTML(jobID) {
  return `<form class="retry-form" method="POST" action="/documents/${jobID}/retry">` +
    `<button type="submit" class="inline-flex items-center justify-center rounded-md px-3 py-1.5 text-xs font-medium hover:bg-accent">Retry</button>` +
    `</form>`;
}

export function watchJob(jobID) {
  const card = document.getElementById("job-" + jobID);
  if (!card) return;

  const es = new EventSource("/jobs/" + jobID + "/watch");
  es.addEventListener("status", function (e) {
    const d = JSON.parse(e.data);

    const statusBadge = card.querySelector("[data-status-badge]");
    const skeleton = card.querySelector("[data-skeleton]");
    const errorText = card.querySelector("[data-error-text]");
    const actions = card.querySelector("[data-actions]");
    const filename = card.querySelector("[data-filename]")?.textContent || "";

    if (d.Status && statusBadge) {
      statusBadge.innerHTML = badgeHTML(d.Status);
    }

    if (d.Status === "DONE") {
      if (skeleton) skeleton.remove();
      if (errorText) errorText.textContent = "";
      if (actions && !actions.querySelector("a[href]")) {
        actions.innerHTML = readButtonHTML(jobID) + deleteFormHTML(jobID, filename);
        wireDeleteConfirm(actions, filename);
      }
      es.close();
    } else if (d.Status === "FAILED") {
      if (skeleton) skeleton.remove();
      if (errorText) errorText.textContent = d.Error || "";
      if (actions && !actions.querySelector("form.retry-form")) {
        actions.innerHTML = retryFormHTML(jobID) + deleteFormHTML(jobID, filename);
        wireDeleteConfirm(actions, filename);
      }
      es.close();
    } else if (d.Stage && errorText) {
      let text = d.Stage;
      if (d.Message) text += " — " + d.Message;
      if (d.PagesTotal) text += " (" + d.PagesDone + "/" + d.PagesTotal + ")";
      errorText.textContent = text;
    }
  });
}

function wireDeleteConfirm(container, filename) {
  container.querySelectorAll("form.delete-form").forEach(function (form) {
    form.addEventListener("submit", function (e) {
      if (!confirm("Delete " + filename + " ?")) e.preventDefault();
    });
  });
}

function bootstrap() {
  document.querySelectorAll("form.delete-form").forEach(function (form) {
    form.addEventListener("submit", function (e) {
      if (!confirm("Delete " + form.dataset.filename + " ?")) e.preventDefault();
    });
  });

  const cfg = document.getElementById("watch-config");
  if (!cfg) return;
  cfg.dataset.jobIds.split(",").forEach(watchJob);
}

if (typeof document !== "undefined") {
  bootstrap();
}
