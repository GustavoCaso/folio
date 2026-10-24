import { describe, it, expect, beforeEach, vi } from "vitest";
import { watchDocument, setView } from "./documents.js";

function makeDocumentCard(id, { filename = "test.pdf", status = "PENDING", error = "" } = {}) {
  const card = document.createElement("div");
  card.id = "document-" + id;

  card.innerHTML = `
		<div>
			<p data-filename>${filename}</p>
			<span data-status-badge><span>${status}</span></span>
		</div>
		<p data-status-text>${error}</p>
		<div data-actions></div>
	`;

  document.body.appendChild(card);
  return card;
}

function makeEventSource() {
  const sources = {};
  vi.stubGlobal("EventSource", class {
    constructor(url) {
      const id = url.match(/^\/documents\/(.+)\/watch$/)[1];
      const handlers = {};
      const es = {
        url,
        addEventListener: vi.fn((name, fn) => { handlers[name] = fn; }),
        close: vi.fn(),
        handlers,
      };
      sources[id] = es;
      return es;
    }
  });
  return {
    source: (id) => sources[id],
    fire: (id, data) => sources[id].handlers.status({ data: JSON.stringify(data) }),
  };
}

beforeEach(() => {
  document.body.innerHTML = "";
  vi.restoreAllMocks();
});

describe("watchDocument", () => {
  it("no-ops when card not found", () => {
    let constructed = false;
    vi.stubGlobal("EventSource", class { constructor() { constructed = true; } });
    watchDocument("missing");
    expect(constructed).toBe(false);
  });

  it("opens SSE for the document", () => {
    makeDocumentCard("w1");
    const es = makeEventSource();
    watchDocument("w1");
    expect(es.source("w1").addEventListener).toHaveBeenCalledWith("status", expect.any(Function));
  });

  it("DONE: updates badge, removes skeleton, injects read link and delete button, closes", () => {
    const card = makeDocumentCard("w2");
    const es = makeEventSource();
    watchDocument("w2");

    es.fire("w2", { Status: "DONE" });

    expect(card.querySelector("[data-status-badge]").textContent).toContain("Done");
    expect(card.querySelector("[data-actions] a[href]")).not.toBeNull();
    expect(card.querySelector("[data-actions] a[href]").href).toContain("/read/w2");
    expect(card.querySelector("[data-delete-document]")).not.toBeNull();
    expect(es.source("w2").close).toHaveBeenCalled();
  });

  it("DONE: read link injection is idempotent", () => {
    const card = makeDocumentCard("w2b");
    const es = makeEventSource();
    watchDocument("w2b");

    es.fire("w2b", { Status: "DONE" });
    es.fire("w2b", { Status: "DONE" });

    // one link for read and one link for edits
    expect(card.querySelectorAll("a[href]").length).toBe(2);
    expect(card.querySelectorAll("[data-delete-document]").length).toBe(1);
  });

  it("FAILED: updates badge, removes skeleton, sets error text, injects retry form and delete button, closes", () => {
    const card = makeDocumentCard("w3");
    const es = makeEventSource();
    watchDocument("w3");

    es.fire("w3", { Status: "FAILED", Error: "timeout" });

    expect(card.querySelector("[data-status-badge]").textContent).toContain("Failed");
    expect(card.querySelector("[data-status-text]").textContent).toBe("timeout");
    expect(card.querySelector("form.retry-form")).not.toBeNull();
    expect(card.querySelector("[data-delete-document]")).not.toBeNull();
    expect(es.source("w3").close).toHaveBeenCalled();
  });

  it("FAILED: retry injection is idempotent", () => {
    const card = makeDocumentCard("w3b");
    const es = makeEventSource();
    watchDocument("w3b");

    es.fire("w3b", { Status: "FAILED", Error: "err" });
    es.fire("w3b", { Status: "FAILED", Error: "err" });

    expect(card.querySelectorAll("form.retry-form").length).toBe(1);
  });

  it("PROCESSING: sets error-text to message", () => {
    const card = makeDocumentCard("w4");
    const es = makeEventSource();
    watchDocument("w4");

    es.fire("w4", { Status: "PROCESSING", Message: "converted page 1/10" });

    expect(card.querySelector("[data-status-text]").textContent).toBe("converted page 1/10");
  });

  it("routes events to the matching document card when watching several documents", () => {
    const a = makeDocumentCard("m1");
    const b = makeDocumentCard("m2");
    const es = makeEventSource();
    watchDocument("m1");
    watchDocument("m2");

    es.fire("m2", { Status: "DONE" });

    expect(b.querySelector("[data-status-badge]").textContent).toContain("Done");
    expect(a.querySelector("[data-status-badge]").textContent).toContain("PENDING");
    expect(es.source("m2").close).toHaveBeenCalled();
    expect(es.source("m1").close).not.toHaveBeenCalled();
  });

  it("delete confirm cancel: does not call fetch", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(false);
    const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue({ ok: true, text: async () => "" });
    const card = makeDocumentCard("w5");
    const es = makeEventSource();
    watchDocument("w5");

    es.fire("w5", { Status: "DONE" });

    const btn = card.querySelector("[data-delete-document]");
    btn.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await Promise.resolve();

    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it("delete confirm ok: calls fetch DELETE", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue({ ok: true, text: async () => "" });
    const card = makeDocumentCard("w6");
    const es = makeEventSource();
    watchDocument("w6");

    es.fire("w6", { Status: "DONE" });

    const btn = card.querySelector("[data-delete-document]");
    btn.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await new Promise(r => setTimeout(r, 0));

    expect(fetchSpy).toHaveBeenCalledWith("/documents/w6", expect.objectContaining({ method: "DELETE" }));
  });
});

describe("watchDocument DONE with library card markup", () => {
  function makeLibraryCard(id) {
    const card = document.createElement("li");
    card.id = "document-" + id;
    card.innerHTML = `
      <span data-filename hidden>book.epub</span>
      <a data-read-link class="book-cover"><span data-cover-placeholder></span></a>
      <div class="book-meta">
        <h3 class="book-title"><a data-read-link data-title>book.epub</a></h3>
        <div class="book-status"><span data-status-badge></span><p data-status-text></p></div>
      </div>
      <div data-actions></div>`;
    document.body.appendChild(card);
    return card;
  }

  it("links cover and title to the reader, swaps in cover image, title and author", () => {
    const card = makeLibraryCard("lib1");
    const es = makeEventSource();
    watchDocument("lib1");

    es.fire("lib1", { Status: "DONE", Cover: "AAAA", Title: "Dune", Author: "Frank Herbert" });

    card.querySelectorAll("[data-read-link]").forEach((a) => {
      expect(a.getAttribute("href")).toBe("/read/lib1");
    });
    expect(card.querySelector("[data-cover-placeholder]")).toBeNull();
    expect(card.querySelector("img[data-cover]").src).toContain("data:image/png;base64,AAAA");
    expect(card.querySelector("[data-title]").textContent).toBe("Dune");
    const author = card.querySelector("[data-author]");
    expect(author.textContent).toBe("Frank Herbert");
    expect(author.getAttribute("href")).toBe("/?author=Frank+Herbert");
    expect(card.querySelector(".book-title").nextElementSibling).toBe(author);
  });
});

describe("setView", () => {
  beforeEach(() => {
    document.body.innerHTML = `
      <button data-view-toggle="grid" aria-pressed="true"></button>
      <button data-view-toggle="list" aria-pressed="false"></button>
      <section id="library" data-view="grid"></section>`;
    document.cookie = "folio_view=; Path=/; Max-Age=0";
  });

  it("switches data-view, pressed state and writes the cookie", () => {
    setView("list");

    expect(document.getElementById("library").dataset.view).toBe("list");
    expect(document.querySelector('[data-view-toggle="list"]').getAttribute("aria-pressed")).toBe("true");
    expect(document.querySelector('[data-view-toggle="grid"]').getAttribute("aria-pressed")).toBe("false");
    expect(document.cookie).toContain("folio_view=list");
  });

  it("ignores unknown views", () => {
    setView("table");

    expect(document.getElementById("library").dataset.view).toBe("grid");
    expect(document.cookie).not.toContain("folio_view=table");
  });

  it("toggle button click applies the view", () => {
    document.querySelector('[data-view-toggle="list"]').dispatchEvent(new MouseEvent("click", { bubbles: true }));

    expect(document.getElementById("library").dataset.view).toBe("list");
  });
});
