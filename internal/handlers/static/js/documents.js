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

// Menu item markup mirrors documentActions in documents.templ; keep in sync.
// Styling lives in library.css because Tailwind doesn't scan this file.
const itemAttrs = `data-tui-dropdown-item class="book-menu-item"`;
const dangerAttrs = `data-tui-dropdown-item class="book-menu-item book-menu-item-danger"`;

function escapeHTML(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);
}

function readButtonHTML(documentID) {
  return `<a ${itemAttrs} href="/read/${documentID}">Read</a>` +
    `<a ${itemAttrs} href="/documents/${documentID}/highlights">Highlights</a>`;
}

function editButtonHTML(documentID) {
  return `<a ${itemAttrs} href="/documents/${documentID}/edit">Edit</a>`;
}

function deleteFormButton(documentID, filename) {
  return `<button type="button" ${dangerAttrs} data-delete-document="${documentID}" data-document-name="${escapeHTML(filename)}">Delete</button>`;
}

function cancelButtonHTML(documentID, filename) {
  return `<button type="button" ${dangerAttrs} data-cancel-document="${documentID}" data-document-name="${escapeHTML(filename)}">Cancel</button>`;
}

function retryFormHTML(documentID, filename) {
  return `<form class="retry-form" method="POST" action="/documents/${documentID}/retry">` +
    `<button type="submit" ${itemAttrs} data-retry-document="${documentID}" data-document-name="${escapeHTML(filename)}">Retry</button>` +
    `</form>`;
}

// authorURL keeps the page's other filters, matching toggleAuthorURL in Go.
function authorURL(author) {
  const params = new URLSearchParams(window.location.search);
  params.set("author", author);
  return "/?" + params.toString();
}

export function watchDocument(documentID) {
  const card = document.getElementById("document-" + documentID);
  if (!card) return;

  const es = new EventSource("/documents/" + documentID + "/watch");
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
      if (actions) {
        actions.innerHTML = readButtonHTML(documentID) + editButtonHTML(documentID) + deleteFormButton(documentID, filename);
      }
      card.querySelectorAll("[data-read-link]").forEach((a) => { a.href = "/read/" + documentID; });
      if (d.Cover) {
        const placeholder = card.querySelector("[data-cover-placeholder]");
        if (placeholder) {
          const img = document.createElement("img");
          img.dataset.cover = "";
          img.src = "data:image/png;base64," + d.Cover;
          img.alt = "";
          img.className = "book-cover-img";
          placeholder.replaceWith(img);
        }
      }
      if (d.Title) {
        const title = card.querySelector("[data-title]");
        if (title) title.textContent = d.Title;
      }
      if (d.Author && !card.querySelector("[data-author]")) {
        const meta = card.querySelector(".book-meta");
        if (meta) {
          const a = document.createElement("a");
          a.dataset.author = "";
          a.className = "book-author";
          a.href = authorURL(d.Author);
          a.textContent = d.Author;
          const title = meta.querySelector(".book-title");
          if (title) title.after(a);
          else meta.prepend(a);
        }
      }
      es.close();
    } else if (d.Status === "FAILED") {
      if (statusText) statusText.textContent = d.Error || "";
      if (actions) {
        actions.innerHTML = editButtonHTML(documentID) + retryFormHTML(documentID, filename) + deleteFormButton(documentID, filename);
      }
      es.close();
    } else if (d.Message && statusText) {
      statusText.textContent = d.Message;
      if (actions && !actions.querySelector("[data-cancel-document]")) {
        actions.innerHTML = cancelButtonHTML(documentID, filename) + editButtonHTML(documentID);
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

  const busy = () => zone.getAttribute("aria-disabled") === "true";

  function selectFile(file) {
    const isEPUB = file && (
      file.type === "application/epub+zip" ||
      file.name?.toLowerCase().endsWith(".epub")
    );
    if (!isEPUB) return;
    const dt = new DataTransfer();
    dt.items.add(file);
    fileInput.files = dt.files;
    label.textContent = "EPUB ready";
    filenameSpan.textContent = file.name;
    actions.classList.remove("hidden");
    actions.classList.add("flex");
    zone.classList.add("border-foreground/40", "bg-accent/30");
  }

  zone.addEventListener("click", () => { if (!busy()) fileInput.click(); });
  zone.addEventListener("keydown", (e) => { if (!busy() && (e.key === "Enter" || e.key === " ")) fileInput.click(); });

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
    if (!busy()) selectFile(e.dataTransfer.files[0]);
  });
}

// initUploadBusy shows a spinner while the upload request runs. Sync formats
// convert before the server responds, so this can take a few seconds. The
// button stays disabled to block a duplicate upload; pageshow undoes it when
// the browser restores this page from the back/forward cache.
export function initUploadBusy() {
  const form = document.getElementById("upload-form");
  const btn = document.getElementById("upload-btn");
  const zone = document.getElementById("drop-zone");
  if (!form || !btn) return;
  const spinner = btn.querySelector("[data-upload-spinner]");
  const label = btn.querySelector("[data-upload-label]");

  function setBusy(isBusy) {
    btn.disabled = isBusy;
    btn.setAttribute("aria-busy", String(isBusy));
    if (spinner) spinner.hidden = !isBusy;
    if (label) label.textContent = isBusy ? "Uploading…" : "Upload";
    if (zone) zone.setAttribute("aria-disabled", String(isBusy));
  }

  form.addEventListener("submit", () => setBusy(true));
  window.addEventListener("pageshow", (e) => { if (e.persisted) setBusy(false); });
}

export const VIEW_COOKIE = "folio_view";

// setView swaps the library layout and remembers it for a year. The server
// reads the cookie on the next load so the first paint is already correct.
export function setView(view) {
  if (view !== "grid" && view !== "list") return;
  const library = document.getElementById("library");
  if (library) library.dataset.view = view;
  document.querySelectorAll("[data-view-toggle]").forEach((btn) => {
    btn.setAttribute("aria-pressed", String(btn.dataset.viewToggle === view));
  });
  document.cookie = `${VIEW_COOKIE}=${view}; Path=/; Max-Age=31536000; SameSite=Lax`;
}

function bootstrap() {
  initDropZone();
  initUploadBusy();
  const cfg = document.getElementById("watch-config");
  if (cfg) cfg.dataset.documentIds.split(",").forEach(watchDocument);

  document.addEventListener("click", async (e) => {
    const toggle = e.target.closest("[data-view-toggle]");
    if (toggle) {
      setView(toggle.dataset.viewToggle);
      return;
    }

    // Cancel button
    const cancelBtn = e.target.closest("[data-cancel-document]");
    if (cancelBtn) {
      if (confirm(`Cancel document?`)) {
        const id = cancelBtn.dataset.cancelDocument;
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
    const deleteBtn = e.target.closest("[data-delete-document]");
    if (deleteBtn) {
      const id = deleteBtn.dataset.deleteDocument;
      const documentName = deleteBtn.dataset.documentName;
      if (confirm(`Delete ${documentName} ?`)) {
        const resp = await fetch(`/documents/${id}`, { method: "DELETE", headers: { "Accept": "text/html" } });
        const html = await resp.text();
        document.body.insertAdjacentHTML("beforeend", html);
        if (resp.ok) {
          const documentCard = document.getElementById(`document-${id}`)
          if (documentCard) documentCard.remove();
        }
        return;
      }
    }
  })
}

if (typeof document !== "undefined") {
  bootstrap();
}
