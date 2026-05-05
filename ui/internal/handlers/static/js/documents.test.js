import { describe, it, expect, beforeEach, vi } from "vitest";
import { watchJob } from "./documents.js";

function makeJobCard(id, { filename = "test.pdf", status = "PROCESSING", error = "" } = {}) {
	const card = document.createElement("div");
	card.id = "job-" + id;

	card.innerHTML = `
		<div>
			<p data-filename>${filename}</p>
			<span data-status-badge><span>${status}</span></span>
		</div>
		${status === "PROCESSING" ? '<div data-skeleton></div>' : ""}
		<p data-error-text>${error}</p>
		<div data-actions></div>
	`;

	document.body.appendChild(card);
	return card;
}

function makeEventSource() {
	let handler = null;
	const es = {
		addEventListener: vi.fn((name, fn) => { if (name === "status") handler = fn; }),
		close: vi.fn(),
		fire: (data) => handler({ data: JSON.stringify(data) }),
	};
	vi.stubGlobal("EventSource", class {
		constructor() { return es; }
	});
	return es;
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
		expect(es.addEventListener).toHaveBeenCalledWith("status", expect.any(Function));
	});

	it("DONE: updates badge, removes skeleton, injects read link and delete button, closes", () => {
		const card = makeJobCard("w2");
		const es = makeEventSource();
		watchJob("w2");

		es.fire({ Status: "DONE" });

		expect(card.querySelector("[data-status-badge]").textContent).toContain("Done");
		expect(card.querySelector("[data-skeleton]")).toBeNull();
		expect(card.querySelector("[data-actions] a[href]")).not.toBeNull();
		expect(card.querySelector("[data-actions] a[href]").href).toContain("/read/w2");
		expect(card.querySelector("[data-delete-job]")).not.toBeNull();
		expect(es.close).toHaveBeenCalled();
	});

	it("DONE: read link injection is idempotent", () => {
		const card = makeJobCard("w2b");
		const es = makeEventSource();
		watchJob("w2b");

		es.fire({ Status: "DONE" });
		es.fire({ Status: "DONE" });

		expect(card.querySelectorAll("a[href]").length).toBe(1);
	});

	it("FAILED: updates badge, removes skeleton, sets error text, injects retry form and delete button, closes", () => {
		const card = makeJobCard("w3");
		const es = makeEventSource();
		watchJob("w3");

		es.fire({ Status: "FAILED", Error: "timeout" });

		expect(card.querySelector("[data-status-badge]").textContent).toContain("Failed");
		expect(card.querySelector("[data-skeleton]")).toBeNull();
		expect(card.querySelector("[data-error-text]").textContent).toBe("timeout");
		expect(card.querySelector("form.retry-form")).not.toBeNull();
		expect(card.querySelector("[data-delete-job]")).not.toBeNull();
		expect(es.close).toHaveBeenCalled();
	});

	it("FAILED: retry injection is idempotent", () => {
		const card = makeJobCard("w3b");
		const es = makeEventSource();
		watchJob("w3b");

		es.fire({ Status: "FAILED", Error: "err" });
		es.fire({ Status: "FAILED", Error: "err" });

		expect(card.querySelectorAll("form.retry-form").length).toBe(1);
	});

	it("stage update: sets error-text to stage info", () => {
		const card = makeJobCard("w4");
		const es = makeEventSource();
		watchJob("w4");

		es.fire({ Status: "PROCESSING", Stage: "OCR", Message: "page 1", PagesDone: 1, PagesTotal: 10 });

		expect(card.querySelector("[data-error-text]").textContent).toBe("OCR — page 1 (1/10)");
	});

	it("delete confirm cancel: does not call fetch", async () => {
		vi.spyOn(window, "confirm").mockReturnValue(false);
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue({ ok: true, text: async () => "" });
		const card = makeJobCard("w5");
		const es = makeEventSource();
		watchJob("w5");

		es.fire({ Status: "DONE" });

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

		es.fire({ Status: "DONE" });

		const btn = card.querySelector("[data-delete-job]");
		btn.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		await new Promise(r => setTimeout(r, 0));

		expect(fetchSpy).toHaveBeenCalledWith("/documents/w6", expect.objectContaining({ method: "DELETE" }));
	});
});
