import { describe, it, expect, beforeEach, vi } from "vitest";
import { injectRetryForm, injectDeleteForm, watchJob } from "./documents.js";

function makeJobLi(id, { filename = "test.pdf", error = "" } = {}) {
	const li = document.createElement("li");
	li.id = "job-" + id;

	const strong = document.createElement("strong");
	strong.className = "filename";
	strong.textContent = filename;
	li.appendChild(strong);

	const statusSpan = document.createElement("span");
	statusSpan.className = "status";
	li.appendChild(statusSpan);

	const detail = document.createElement("span");
	detail.className = "detail";
	if (error) detail.textContent = error;
	li.appendChild(detail);

	const readLink = document.createElement("span");
	readLink.className = "read-link";
	li.appendChild(readLink);

	const deleteSlot = document.createElement("span");
	deleteSlot.className = "delete-action";
	li.appendChild(deleteSlot);

	document.body.appendChild(li);
	return li;
}

beforeEach(() => {
	document.body.innerHTML = "";
	vi.restoreAllMocks();
});

describe("injectRetryForm", () => {
	it("adds retry form with error text to detail", () => {
		const li = makeJobLi("1");
		injectRetryForm(li, "1", "parse failed");

		const form = li.querySelector("form.retry-form");
		expect(form).not.toBeNull();
		expect(form.method).toBe("post");
		expect(form.action).toContain("/documents/1/retry");
		expect(li.querySelector(".detail").textContent).toContain("parse failed");
	});

	it("is idempotent — does not add a second form", () => {
		const li = makeJobLi("2");
		injectRetryForm(li, "2", "err");
		injectRetryForm(li, "2", "err");

		expect(li.querySelectorAll("form.retry-form").length).toBe(1);
	});

	it("uses empty string when errorText is falsy", () => {
		const li = makeJobLi("3");
		injectRetryForm(li, "3", null);

		const detail = li.querySelector(".detail");
		expect(detail.querySelector("form.retry-form")).not.toBeNull();
		// no error text before the separator
		const textNodes = [...detail.childNodes].filter(n => n.nodeType === Node.TEXT_NODE).map(n => n.textContent);
		expect(textNodes).not.toContain(expect.stringMatching(/\S/));
	});
});

describe("injectDeleteForm", () => {
	it("adds delete form to delete-action slot", () => {
		const li = makeJobLi("4", { filename: "book.pdf" });
		injectDeleteForm(li, "4");

		const form = li.querySelector("form.delete-form");
		expect(form).not.toBeNull();
		expect(form.method).toBe("post");
		expect(form.action).toContain("/documents/4/delete");
		expect(form.dataset.filename).toBe("book.pdf");
	});

	it("is idempotent — does not add a second form", () => {
		const li = makeJobLi("5");
		injectDeleteForm(li, "5");
		injectDeleteForm(li, "5");

		expect(li.querySelectorAll("form.delete-form").length).toBe(1);
	});

	it("confirm cancel prevents submit", () => {
		vi.spyOn(window, "confirm").mockReturnValue(false);
		const li = makeJobLi("6", { filename: "doc.pdf" });
		injectDeleteForm(li, "6");

		const form = li.querySelector("form.delete-form");
		const e = new Event("submit", { cancelable: true });
		form.dispatchEvent(e);

		expect(e.defaultPrevented).toBe(true);
	});

	it("confirm ok allows submit", () => {
		vi.spyOn(window, "confirm").mockReturnValue(true);
		const li = makeJobLi("7", { filename: "doc.pdf" });
		injectDeleteForm(li, "7");

		const form = li.querySelector("form.delete-form");
		const e = new Event("submit", { cancelable: true });
		form.dispatchEvent(e);

		expect(e.defaultPrevented).toBe(false);
	});
});

describe("watchJob", () => {
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

	it("no-ops when li not found", () => {
		let constructed = false;
		vi.stubGlobal("EventSource", class { constructor() { constructed = true; } });
		watchJob("missing");
		expect(constructed).toBe(false);
	});

	it("opens SSE for the job", () => {
		const li = makeJobLi("w1");
		const es = makeEventSource();
		watchJob("w1");
		expect(es.addEventListener).toHaveBeenCalledWith("status", expect.any(Function));
	});

	it("DONE: clears detail, injects read link and delete form, closes", () => {
		const li = makeJobLi("w2");
		const es = makeEventSource();
		watchJob("w2");

		es.fire({ Status: "DONE" });

		expect(li.querySelector(".detail").textContent).toBe("");
		expect(li.querySelector(".read-link").innerHTML).toContain("/read/w2");
		expect(li.querySelector("form.delete-form")).not.toBeNull();
		expect(es.close).toHaveBeenCalled();
	});

	it("FAILED: injects retry form and delete form, closes", () => {
		const li = makeJobLi("w3");
		const es = makeEventSource();
		watchJob("w3");

		es.fire({ Status: "FAILED", Error: "timeout" });

		expect(li.querySelector("form.retry-form")).not.toBeNull();
		expect(li.querySelector(".detail").textContent).toContain("timeout");
		expect(li.querySelector("form.delete-form")).not.toBeNull();
		expect(es.close).toHaveBeenCalled();
	});

	it("stage update: sets detail text", () => {
		const li = makeJobLi("w4");
		const es = makeEventSource();
		watchJob("w4");

		es.fire({ Status: "PROCESSING", Stage: "OCR", Message: "page 1", PagesDone: 1, PagesTotal: 10 });

		expect(li.querySelector(".detail").textContent).toBe("OCR — page 1 (1/10)");
	});

	it("status update: sets status class and text", () => {
		const li = makeJobLi("w5");
		const es = makeEventSource();
		watchJob("w5");

		es.fire({ Status: "PROCESSING" });

		const span = li.querySelector(".status");
		expect(span.textContent).toBe("PROCESSING");
		expect(span.className).toBe("status status-PROCESSING");
	});
});
