import { describe, it, expect, beforeEach, vi } from "vitest";
import { watchJob } from "./documents.js";

function makeJobCard(id, { filename = "test.pdf", status = "PENDING", error = "" } = {}) {
  const card = document.createElement("div");
  card.id = "job-" + id;

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
      const id = url.match(/^\/jobs\/(.+)\/watch$/)[1];
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

describe("watchJob", () => {
  it("no-ops when card not found", () => {
    let constructed = false;
    vi.stubGlobal("EventSource", class { constructor() { constructed = true; } });
    watchJob("missing");
    expect(constructed).toBe(false);
  });

  it("opens SSE for the job", () => {
    makeJobCard("w1");
    const es = makeEventSource();
    watchJob("w1");
    expect(es.source("w1").addEventListener).toHaveBeenCalledWith("status", expect.any(Function));
  });

  it("DONE: updates badge, removes skeleton, injects read link and delete button, closes", () => {
    const card = makeJobCard("w2");
    const es = makeEventSource();
    watchJob("w2");

    es.fire("w2", { Status: "DONE" });

    expect(card.querySelector("[data-status-badge]").textContent).toContain("Done");
    expect(card.querySelector("[data-actions] a[href]")).not.toBeNull();
    expect(card.querySelector("[data-actions] a[href]").href).toContain("/read/w2");
    expect(card.querySelector("[data-delete-job]")).not.toBeNull();
    expect(es.source("w2").close).toHaveBeenCalled();
  });

  it("DONE: read link injection is idempotent", () => {
    const card = makeJobCard("w2b");
    const es = makeEventSource();
    watchJob("w2b");

    es.fire("w2b", { Status: "DONE" });
    es.fire("w2b", { Status: "DONE" });

    // one link for read and one link for edits
    expect(card.querySelectorAll("a[href]").length).toBe(2);
    expect(card.querySelectorAll("[data-delete-job]").length).toBe(1);
  });

  it("FAILED: updates badge, removes skeleton, sets error text, injects retry form and delete button, closes", () => {
    const card = makeJobCard("w3");
    const es = makeEventSource();
    watchJob("w3");

    es.fire("w3", { Status: "FAILED", Error: "timeout" });

    expect(card.querySelector("[data-status-badge]").textContent).toContain("Failed");
    expect(card.querySelector("[data-status-text]").textContent).toBe("timeout");
    expect(card.querySelector("form.retry-form")).not.toBeNull();
    expect(card.querySelector("[data-delete-job]")).not.toBeNull();
    expect(es.source("w3").close).toHaveBeenCalled();
  });

  it("FAILED: retry injection is idempotent", () => {
    const card = makeJobCard("w3b");
    const es = makeEventSource();
    watchJob("w3b");

    es.fire("w3b", { Status: "FAILED", Error: "err" });
    es.fire("w3b", { Status: "FAILED", Error: "err" });

    expect(card.querySelectorAll("form.retry-form").length).toBe(1);
  });

  it("PROCESSING: sets error-text to message", () => {
    const card = makeJobCard("w4");
    const es = makeEventSource();
    watchJob("w4");

    es.fire("w4", { Status: "PROCESSING", Message: "converted page 1/10" });

    expect(card.querySelector("[data-status-text]").textContent).toBe("converted page 1/10");
  });

  it("routes events to the matching job card when watching several jobs", () => {
    const a = makeJobCard("m1");
    const b = makeJobCard("m2");
    const es = makeEventSource();
    watchJob("m1");
    watchJob("m2");

    es.fire("m2", { Status: "DONE" });

    expect(b.querySelector("[data-status-badge]").textContent).toContain("Done");
    expect(a.querySelector("[data-status-badge]").textContent).toContain("PENDING");
    expect(es.source("m2").close).toHaveBeenCalled();
    expect(es.source("m1").close).not.toHaveBeenCalled();
  });

  it("delete confirm cancel: does not call fetch", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(false);
    const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue({ ok: true, text: async () => "" });
    const card = makeJobCard("w5");
    const es = makeEventSource();
    watchJob("w5");

    es.fire("w5", { Status: "DONE" });

    const btn = card.querySelector("[data-delete-job]");
    btn.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await Promise.resolve();

    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it("delete confirm ok: calls fetch DELETE", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue({ ok: true, text: async () => "" });
    const card = makeJobCard("w6");
    const es = makeEventSource();
    watchJob("w6");

    es.fire("w6", { Status: "DONE" });

    const btn = card.querySelector("[data-delete-job]");
    btn.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await new Promise(r => setTimeout(r, 0));

    expect(fetchSpy).toHaveBeenCalledWith("/documents/w6", expect.objectContaining({ method: "DELETE" }));
  });
});
