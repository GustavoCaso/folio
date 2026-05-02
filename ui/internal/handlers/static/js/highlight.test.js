import { describe, it, expect, beforeEach } from "vitest";
import {
  applyHighlight,
  applyHighlights,
  findBlockAncestor,
  formatHighlight,
  offsetWithinBlock,
  wrapRangeTextNodes,
} from "./highlight.js";

function makeReader(html) {
  document.body.innerHTML = `<div id="reader">${html}</div>`;
  return document.getElementById("reader");
}

describe("findBlockAncestor", () => {
  it("returns the nearest element with data-block-id", () => {
    const reader = makeReader(`<p data-block-id="paragraph-1"><span>hi</span></p>`);
    const span = reader.querySelector("span");
    const found = findBlockAncestor(span.firstChild, reader);
    expect(found.dataset.blockId).toBe("paragraph-1");
  });

  it("returns null when no ancestor has data-block-id", () => {
    const reader = makeReader(`<p>hi</p>`);
    const text = reader.querySelector("p").firstChild;
    expect(findBlockAncestor(text, reader)).toBe(null);
  });
});

describe("offsetWithinBlock", () => {
  it("counts characters across multiple text nodes", () => {
    const reader = makeReader(`<p data-block-id="p-1">hello <em>world</em>!</p>`);
    const block = reader.querySelector("p");
    const emText = block.querySelector("em").firstChild;
    expect(offsetWithinBlock(block, emText, 3)).toBe("hello ".length + 3);
  });

  it("counts each <img> as 1 character", () => {
    const reader = makeReader(
      `<p data-block-id="p-1">before<img src="x"/>after</p>`,
    );
    const block = reader.querySelector("p");
    const afterText = block.lastChild;
    // "before" = 6, img = 1, target = start of "after" → 7
    expect(offsetWithinBlock(block, afterText, 0)).toBe(7);
  });

  it("returns offset when target is the block element with child index", () => {
    const reader = makeReader(
      `<p data-block-id="p-1">a<img src="x"/>b</p>`,
    );
    const block = reader.querySelector("p");
    // child index 2 = after the img → "a"(1) + img(1) = 2
    expect(offsetWithinBlock(block, block, 2)).toBe(2);
  });
});

describe("applyHighlight with images in the block", () => {
  let reader;
  beforeEach(() => {
    reader = makeReader(
      `<p data-block-id="p-1">before<img src="x"/>after</p>`,
    );
  });

  it("highlights text + image + text producing 3 marks", () => {
    // "before"(6) + img(1) + "after"(5) = 11
    // chars 3..10 → "ore" + img + "aft"
    applyHighlight(reader, {
      ID: "h1",
      StartBlockID: "p-1",
      EndBlockID: "p-1",
      StartPos: 3,
      EndPos: 10,
    });

    const marks = reader.querySelectorAll("mark.highlight");
    expect(marks.length).toBe(3);
    expect(marks[0].textContent).toBe("ore");
    expect(marks[1].querySelector("img")).toBeTruthy();
    expect(marks[2].textContent).toBe("aft");
    marks.forEach((m) => expect(m.dataset.highlightId).toBe("h1"));
  });

  it("highlighting just the image wraps only the image", () => {
    // img at offsets [6, 7]
    applyHighlight(reader, {
      ID: "h1",
      StartBlockID: "p-1",
      EndBlockID: "p-1",
      StartPos: 6,
      EndPos: 7,
    });
    const marks = reader.querySelectorAll("mark.highlight");
    expect(marks.length).toBe(1);
    expect(marks[0].querySelector("img")).toBeTruthy();
    expect(marks[0].textContent).toBe("");
  });

  it("preserves the image element in the DOM", () => {
    applyHighlight(reader, {
      ID: "h1",
      StartBlockID: "p-1",
      EndBlockID: "p-1",
      StartPos: 0,
      EndPos: 12,
    });
    expect(reader.querySelectorAll("img").length).toBe(1);
  });
});

describe("applyHighlight with simple text", () => {
  it("wraps a contiguous range in a single mark", () => {
    const reader = makeReader(`<p data-block-id="p-1">hello world</p>`);
    applyHighlight(reader, {
      ID: "h1",
      StartBlockID: "p-1",
      EndBlockID: "p-1",
      StartPos: 0,
      EndPos: 5,
    });
    const marks = reader.querySelectorAll("mark.highlight");
    expect(marks.length).toBe(1);
    expect(marks[0].textContent).toBe("hello");
  });

  it("does not set title attribute on mark (tooltip is handled via JS click)", () => {
    const reader = makeReader(`<p data-block-id="p-1">hello</p>`);
    applyHighlight(reader, {
      ID: "h1",
      StartBlockID: "p-1",
      EndBlockID: "p-1",
      StartPos: 0,
      EndPos: 5,
      Tag: "important",
      Note: "remember this",
    });
    const mark = reader.querySelector("mark.highlight");
    expect(mark.title).toBe("");
  });

  it("does nothing when block-id is missing", () => {
    const reader = makeReader(`<p data-block-id="p-1">hi</p>`);
    applyHighlight(reader, {
      ID: "h1",
      StartBlockID: "missing",
      EndBlockID: "missing",
      StartPos: 0,
      EndPos: 1,
    });
    expect(reader.querySelectorAll("mark").length).toBe(0);
  });

  it("supports legacy BlockID field for backward-compat callers", () => {
    const reader = makeReader(`<p data-block-id="p-1">hello</p>`);
    applyHighlight(reader, { ID: "h1", BlockID: "p-1", StartPos: 0, EndPos: 5 });
    expect(reader.querySelectorAll("mark.highlight").length).toBe(1);
  });
});

describe("applyHighlight across multiple blocks", () => {
  it("wraps text in start, middle, and end blocks", () => {
    const reader = makeReader(
      `<p data-block-id="p-1">hello world</p>` +
        `<p data-block-id="p-2">middle block</p>` +
        `<p data-block-id="p-3">end here</p>`,
    );
    // start at "world" (offset 6 in p-1), end at "end" (offset 3 in p-3)
    applyHighlight(reader, {
      ID: "h1",
      StartBlockID: "p-1",
      EndBlockID: "p-3",
      StartPos: 6,
      EndPos: 3,
    });
    const marks = reader.querySelectorAll("mark.highlight");
    expect(marks.length).toBe(3);
    expect(marks[0].textContent).toBe("world");
    expect(marks[1].textContent).toBe("middle block");
    expect(marks[2].textContent).toBe("end");
  });

  it("includes image in middle block of a multi-block highlight", () => {
    const reader = makeReader(
      `<p data-block-id="p-1">hello</p>` +
        `<p data-block-id="p-2">a<img src="x"/>b</p>` +
        `<p data-block-id="p-3">end</p>`,
    );
    applyHighlight(reader, {
      ID: "h1",
      StartBlockID: "p-1",
      EndBlockID: "p-3",
      StartPos: 0,
      EndPos: 3,
    });
    const marks = reader.querySelectorAll("mark.highlight");
    // p-1: "hello" (1) + p-2: "a"(1) + img(1) + "b"(1) = 3 + p-3: "end"(1) = 5
    expect(marks.length).toBe(5);
    expect(reader.querySelectorAll("mark.highlight img").length).toBe(1);
  });
});

describe("applyHighlights", () => {
  it("applies multiple highlights", () => {
    const reader = makeReader(
      `<p data-block-id="p-1">hello world</p><p data-block-id="p-2">foo bar</p>`,
    );
    applyHighlights(reader, [
      { ID: "h1", StartBlockID: "p-1", EndBlockID: "p-1", StartPos: 0, EndPos: 5 },
      { ID: "h2", StartBlockID: "p-2", EndBlockID: "p-2", StartPos: 4, EndPos: 7 },
    ]);
    expect(reader.querySelectorAll("mark.highlight").length).toBe(2);
  });
});

describe("formatHighlight", () => {
  it("returns tag and note joined when both present", () => {
    expect(formatHighlight({ Tag: "important", Note: "remember this" })).toBe("important: remember this");
  });

  it("returns tag alone when note is absent", () => {
    expect(formatHighlight({ Tag: "important", Note: "" })).toBe("important");
  });

  it("returns note alone when tag is absent", () => {
    expect(formatHighlight({ Tag: "", Note: "remember this" })).toBe("remember this");
  });

  it("returns fallback when both are absent", () => {
    expect(formatHighlight({ Tag: "", Note: "" })).toBe("(no annotation)");
  });

  it("returns fallback for null/undefined highlight", () => {
    expect(formatHighlight(null)).toBe("(no annotation)");
    expect(formatHighlight(undefined)).toBe("(no annotation)");
  });
});

describe("wrapRangeTextNodes", () => {
  it("wraps text in a single text-node range", () => {
    const reader = makeReader(`<p data-block-id="p-1">hello</p>`);
    const text = reader.querySelector("p").firstChild;
    const range = document.createRange();
    range.setStart(text, 1);
    range.setEnd(text, 4);
    wrapRangeTextNodes(range, "highlight", { highlightId: "x" });
    expect(reader.querySelector("mark.highlight").textContent).toBe("ell");
  });

  it("wraps an <img> element inside the range", () => {
    const reader = makeReader(`<p data-block-id="p-1">a<img src="x"/>b</p>`);
    const block = reader.querySelector("p");
    const range = document.createRange();
    range.selectNodeContents(block);
    wrapRangeTextNodes(range, "highlight", { highlightId: "x" });
    expect(reader.querySelectorAll("mark.highlight img").length).toBe(1);
  });
});
