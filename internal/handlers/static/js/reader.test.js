import { describe, it, expect, beforeEach, vi } from "vitest";
import {
  applyHighlight,
  applyHighlights,
  buildPendingSelection,
  calcPopoverPosition,
  computeProgress,
  filterHighlightsByChapter,
  findBlockAncestor,
  firstVisibleBlockID,
  formatHighlight,
  mirrorSidebarState,
  offsetWithinBlock,
  removeHighlightCard,
  removeHighlightMarks,
  activeTOCEntry,
  buildTOCScrollEntries,
  setupSidebarStateMirror,
  setupTOCActiveOnClick,
  setupTOCScrollTracking,
  wrapRangeTextNodes,
} from "./reader.js";

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
    expect(marks[0].outerHTML).toBe(`<mark class="highlight" data-highlight-id="h1">ore</mark>`);
    expect(marks[1].outerHTML).toBe(`<mark class="highlight" data-highlight-id="h1"><img src="x"></mark>`);
    expect(marks[2].outerHTML).toBe(`<mark class="highlight" data-highlight-id="h1">aft</mark>`);
    expect(reader.querySelector("p").innerHTML).toBe(
      `bef<mark class="highlight" data-highlight-id="h1">ore</mark>` +
      `<mark class="highlight" data-highlight-id="h1"><img src="x"></mark>` +
      `<mark class="highlight" data-highlight-id="h1">aft</mark>er`
    );
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

describe("offsetWithinBlock", () => {
  // Helper: build a block element and return it
  function makeBlock(html) {
    const p = document.createElement("p");
    p.setAttribute("data-block-id", "b-1");
    p.innerHTML = html;
    document.body.appendChild(p);
    return p;
  }

  // Text-node target: offset within a plain text block
  // <p>hello world</p>
  //  01234 56789...
  it("returns 0 for start of a plain text node", () => {
    const block = makeBlock("hello world");
    const textNode = block.firstChild;
    expect(offsetWithinBlock(block, textNode, 0)).toBe(0);
  });

  it("returns correct offset mid-word in a plain text node", () => {
    const block = makeBlock("hello world");
    const textNode = block.firstChild;
    expect(offsetWithinBlock(block, textNode, 5)).toBe(5);
  });

  it("returns end offset for end of a plain text node", () => {
    const block = makeBlock("hello world");
    const textNode = block.firstChild;
    expect(offsetWithinBlock(block, textNode, 11)).toBe(11);
  });

  // Text-node target: block with two text nodes separated by a <span>
  // <p>foo<span>bar</span>baz</p>
  //  position: 0 1 2 | 3 4 5 | 6 7 8
  //  node:      f o o   b a r   b a z
  // "baz" starts at position 6 because "foo"(3) + "bar"(3) come before it.
  it("accumulates lengths of preceding text nodes to find position of target node", () => {
    const block = makeBlock("foo<span>bar</span>baz");
    const bazNode = block.lastChild; // text "baz"
    expect(offsetWithinBlock(block, bazNode, 0)).toBe(6); // start of "baz"
    expect(offsetWithinBlock(block, bazNode, 2)).toBe(8); // exclusive end after "ba" (selects "b","a")
  });

  // Image counts as 1 character
  // <p>before<img/>after</p>
  //  position: 0 1 2 3 4 5 | 6 | 7 8 9 10 11
  //  node:      b e f o r e  img  a f t e  r
  // "after" starts at position 7 because "before"(6) + img(1) come before it.
  it("counts <img> as 1 character when accumulating position", () => {
    const block = makeBlock('before<img src="x"/>after');
    const afterNode = block.lastChild; // text "after"
    expect(offsetWithinBlock(block, afterNode, 0)).toBe(7);  // start of "after"
    expect(offsetWithinBlock(block, afterNode, 3)).toBe(10); // exclusive end after "aft" (selects "a","f","t")
  });

  // endOffset on the "before" text node is exclusive:
  // offset=6 means "all 6 chars selected" i.e. the selection ends just before the img.
  it("endOffset=6 on 'before' means selection ends just before the img (position 6)", () => {
    const block = makeBlock('before<img src="x"/>after');
    const beforeNode = block.firstChild; // text "before"
    expect(offsetWithinBlock(block, beforeNode, 6)).toBe(6);
  });

  // Element-node target: browser sets endContainer=<p> with endOffset=childIndex
  // when selection ends at an img boundary.
  // <p>before<img/>after</p>  childNodes: [0:"before", 1:img, 2:"after"]
  // childIndex is exclusive — it points to the first child NOT included.
  // childIndex 0 → no children included            → position 0
  // childIndex 1 → "before" included (6 chars)     → position 6
  // childIndex 2 → "before" + img included (6+1=7) → position 7
  it("element-node target: childIndex 0 means nothing included yet, position 0", () => {
    const block = makeBlock('before<img src="x"/>after');
    expect(offsetWithinBlock(block, block, 0)).toBe(0);
  });

  it("element-node target: childIndex 1 means 'before' is included, position 6", () => {
    const block = makeBlock('before<img src="x"/>after');
    expect(offsetWithinBlock(block, block, 1)).toBe(6);
  });

  it("element-node target: childIndex 2 means 'before' + img included, position 7", () => {
    const block = makeBlock('before<img src="x"/>after');
    expect(offsetWithinBlock(block, block, 2)).toBe(7);
  });

  // Simulates a real selection that spans multiple nodes and stops mid-word in the last one.
  // <p>one <img/> two three</p>
  //  position: 0 1 2 3 | 4 | 5 6 7 8 9 10 11 12 13 14 15 16
  //  node:      o n e      img  " " t  w  o  " " t  h  r  e  e
  // User selects from start of "one" to "thr" in "three" (endOffset=3, exclusive).
  // That means "one "(4) + img(1) + " two "(5) + "thr"(3) = position 13.
  it("selection spanning text, img, text, stops mid-word in last text node", () => {
    const block = makeBlock('one <img src="x"/> two three');
    const lastTextNode = block.lastChild; // text " two three"
    // " two three" starts at position 5 ("one "(4) + img(1))
    // endOffset=8 means selection stops just before "hree" — "thr" selected (exclusive)
    // " two three": positions 5..14, " two t"=6 chars → "thr" ends at offset 9 (exclusive)
    // " "(5) "t"(6) "w"(7) "o"(8) " "(9) "t"(10) "h"(11) "r"(12) "e"(13) "e"(14)
    // to select up to and including "r" at position 12: endOffset=8 within lastTextNode
    expect(offsetWithinBlock(block, lastTextNode, 8)).toBe(13); // "one " + img + " two thr"
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
    expect(reader.querySelector("p").innerHTML).toBe(
      `h<mark class="highlight" data-highlight-id="x">ell</mark>o`
    );
  });

  it("wraps an <img> element inside the range", () => {
    const reader = makeReader(`<p data-block-id="p-1">a<img src="x"/>b</p>`);
    const block = reader.querySelector("p");
    const range = document.createRange();
    range.selectNodeContents(block);
    wrapRangeTextNodes(range, "highlight", { highlightId: "x" });
    expect(block.innerHTML).toBe(
      `<mark class="highlight" data-highlight-id="x">a</mark>` +
      `<mark class="highlight" data-highlight-id="x"><img src="x"></mark>` +
      `<mark class="highlight" data-highlight-id="x">b</mark>`
    );
  });
});

describe("removeHighlightMarks", () => {
  it("unwraps all marks for a given highlight ID", () => {
    const reader = makeReader(
      `<p data-block-id="p-1">` +
      `bef<mark class="highlight" data-highlight-id="h1">ore</mark>` +
      `<mark class="highlight" data-highlight-id="h1">end</mark>` +
      `</p>`
    );
    removeHighlightMarks(reader, "h1");
    expect(reader.querySelectorAll("mark").length).toBe(0);
    expect(reader.querySelector("p").textContent).toBe("beforeend");
  });

  it("does not remove marks for a different highlight ID", () => {
    const reader = makeReader(
      `<p data-block-id="p-1">` +
      `<mark class="highlight" data-highlight-id="h1">keep</mark>` +
      `<mark class="highlight" data-highlight-id="h2">remove</mark>` +
      `</p>`
    );
    removeHighlightMarks(reader, "h2");
    const remaining = reader.querySelectorAll("mark");
    expect(remaining.length).toBe(1);
    expect(remaining[0].dataset.highlightId).toBe("h1");
  });

  it("is a no-op when no marks match", () => {
    const reader = makeReader(`<p data-block-id="p-1">no marks here</p>`);
    expect(() => removeHighlightMarks(reader, "h1")).not.toThrow();
    expect(reader.querySelectorAll("mark").length).toBe(0);
  });
});

describe("computeProgress", () => {
  it("returns 0 when no blocks", () => {
    expect(computeProgress([])).toBe(0);
  });

  it("returns 0 when first block visible (jsdom top=0 default)", () => {
    const reader = makeReader(
      `<p data-block-id="p-1">a</p><p data-block-id="p-2">b</p><p data-block-id="p-3">c</p>`
    );
    const blocks = Array.from(reader.querySelectorAll("[data-block-id]"));
    expect(computeProgress(blocks)).toBe(0);
  });

  it("returns 0 for single block", () => {
    const reader = makeReader(`<p data-block-id="p-1">only</p>`);
    const blocks = Array.from(reader.querySelectorAll("[data-block-id]"));
    expect(computeProgress(blocks)).toBe(0);
  });

  it("returns 100 when all blocks above viewport (top < 0)", () => {
    const reader = makeReader(
      `<p data-block-id="p-1">a</p><p data-block-id="p-2">b</p><p data-block-id="p-3">c</p>`
    );
    const blocks = Array.from(reader.querySelectorAll("[data-block-id]"));
    blocks.forEach(b => {
      b.getBoundingClientRect = () => ({ top: -100, bottom: -90, left: 0, right: 0, width: 0, height: 10 });
    });
    expect(computeProgress(blocks)).toBe(100);
  });

  it("returns 50 when middle block is first visible", () => {
    const reader = makeReader(
      `<p data-block-id="p-1">a</p><p data-block-id="p-2">b</p><p data-block-id="p-3">c</p>`
    );
    const blocks = Array.from(reader.querySelectorAll("[data-block-id]"));
    blocks[0].getBoundingClientRect = () => ({ top: -100, bottom: -90, left: 0, right: 0, width: 0, height: 10 });
    blocks[1].getBoundingClientRect = () => ({ top: 10, bottom: 20, left: 0, right: 0, width: 0, height: 10 });
    blocks[2].getBoundingClientRect = () => ({ top: 30, bottom: 40, left: 0, right: 0, width: 0, height: 10 });
    expect(computeProgress(blocks)).toBe(50);
  });

  it("returns 100 for two blocks when last is first visible after first is above viewport", () => {
    const reader = makeReader(
      `<p data-block-id="p-1">a</p><p data-block-id="p-2">b</p>`
    );
    const blocks = Array.from(reader.querySelectorAll("[data-block-id]"));
    blocks[0].getBoundingClientRect = () => ({ top: -50, bottom: -40, left: 0, right: 0, width: 0, height: 10 });
    blocks[1].getBoundingClientRect = () => ({ top: 5, bottom: 15, left: 0, right: 0, width: 0, height: 10 });
    expect(computeProgress(blocks)).toBe(100);
  });
});

describe("firstVisibleBlockID", () => {
  it("returns empty string when no blocks", () => {
    expect(firstVisibleBlockID([])).toBe("");
  });

  it("returns first block id when all have top=0 (jsdom default)", () => {
    const reader = makeReader(
      `<p data-block-id="p-1">a</p><p data-block-id="p-2">b</p>`
    );
    const blocks = Array.from(reader.querySelectorAll("[data-block-id]"));
    expect(firstVisibleBlockID(blocks)).toBe("p-1");
  });

  it("returns last block id when no block has top >= 0", () => {
    const reader = makeReader(`<p data-block-id="p-1">a</p><p data-block-id="p-2">b</p>`);
    const blocks = Array.from(reader.querySelectorAll("[data-block-id]"));
    blocks.forEach(b => {
      b.getBoundingClientRect = () => ({ top: -100, bottom: -90, left: 0, right: 0, width: 0, height: 10 });
    });
    expect(firstVisibleBlockID(blocks)).toBe("p-2");
  });
});

describe("calcPopoverPosition", () => {
  it("places popover to the right when there is enough space", () => {
    const rect = { top: 100, bottom: 120, left: 50, right: 200 };
    const pos = calcPopoverPosition(rect, 280, 1200);
    expect(pos).toEqual({ top: 100, left: 208 });
  });

  it("falls back below the selection when not enough space to the right", () => {
    // space right = 1000 - 800 - 8 = 192, less than popoverWidth 280 → fallback
    // left = Math.max(4, Math.min(50, 1000-280-4=716)) = 50
    const rect = { top: 100, bottom: 120, left: 50, right: 800 };
    const pos = calcPopoverPosition(rect, 280, 1000);
    expect(pos).toEqual({ top: 126, left: 50 });
  });

  it("clamps left to 4px minimum when selection is near left edge and fallback triggers", () => {
    // space right = 300 - 290 - 8 = 2, less than 280 → fallback
    // left = Math.max(4, Math.min(0, 300-280-4=16)) = 4
    const rect = { top: 100, bottom: 120, left: 0, right: 290 };
    const pos = calcPopoverPosition(rect, 280, 300);
    expect(pos.left).toBe(4);
  });

  it("clamps left so popover does not overflow right edge", () => {
    // space right = 400 - 250 - 8 = 142, less than 280 → fallback
    // left = Math.max(4, Math.min(200, 400-280-4=116)) = 116
    const rect = { top: 100, bottom: 120, left: 200, right: 250 };
    const pos = calcPopoverPosition(rect, 280, 400);
    expect(pos.left).toBe(116);
  });
});

describe("buildPendingSelection", () => {
  function fakeSelection(range, text = "") {
    return {
      isCollapsed: false,
      rangeCount: 1,
      getRangeAt: () => range,
      toString: () => text,
      removeAllRanges: () => {},
    };
  }

  it("returns null for collapsed selection", () => {
    const reader = makeReader(`<p data-block-id="p-1">hello</p>`);
    expect(buildPendingSelection(reader, "job1", { isCollapsed: true, rangeCount: 0 })).toBeNull();
  });

  it("returns null when selection is outside reader", () => {
    const reader = makeReader(`<p data-block-id="p-1">hello</p>`);
    const outside = document.createElement("p");
    outside.textContent = "outside";
    document.body.appendChild(outside);
    const range = document.createRange();
    range.selectNodeContents(outside);
    expect(buildPendingSelection(reader, "job1", fakeSelection(range, "outside"))).toBeNull();
  });

  it("returns null when selection has no block ancestor", () => {
    const reader = makeReader(`<p>no block id here</p>`);
    const text = reader.querySelector("p").firstChild;
    const range = document.createRange();
    range.setStart(text, 0);
    range.setEnd(text, 2);
    expect(buildPendingSelection(reader, "job1", fakeSelection(range, "no"))).toBeNull();
  });

  it("clears the selection when there is no block ancestor", () => {
    const reader = makeReader(`<p>no block id here</p>`);
    const text = reader.querySelector("p").firstChild;
    const range = document.createRange();
    range.setStart(text, 0);
    range.setEnd(text, 2);
    const sel = { ...fakeSelection(range, "no"), removeAllRanges: vi.fn() };
    buildPendingSelection(reader, "job1", sel);
    expect(sel.removeAllRanges).toHaveBeenCalledOnce();
  });

  it("builds correct result for a simple text selection", () => {
    const reader = makeReader(`<p data-block-id="p-1">hello world</p>`);
    const text = reader.querySelector("p").firstChild;
    const range = document.createRange();
    range.setStart(text, 0);
    range.setEnd(text, 5);
    const result = buildPendingSelection(reader, "job42", fakeSelection(range, "hello"));
    expect(result).toEqual({
      job_id: "job42",
      start_block_id: "p-1",
      end_block_id: "p-1",
      start_pos: 0,
      end_pos: 5,
      text: "hello",
    });
  });

  it("builds correct result for a cross-block selection", () => {
    const reader = makeReader(
      `<p data-block-id="p-1">hello</p><p data-block-id="p-2">world</p>`
    );
    const startText = reader.querySelector("[data-block-id='p-1']").firstChild;
    const endText = reader.querySelector("[data-block-id='p-2']").firstChild;
    const range = document.createRange();
    range.setStart(startText, 2);
    range.setEnd(endText, 3);
    const result = buildPendingSelection(reader, "job1", fakeSelection(range, "llo\nwor"));
    expect(result).toEqual({
      job_id: "job1",
      start_block_id: "p-1",
      end_block_id: "p-2",
      start_pos: 2,
      end_pos: 3,
      text: "llo\nwor",
    });
  });
});

describe("filterHighlightsByChapter", () => {
  const highlights = [
    { ID: "h1", start_block_id: "ch0-p-1" },
    { ID: "h2", start_block_id: "ch1-p-2" },
    { ID: "h3", start_block_id: "ch1-h1-1" },
  ];

  it("returns only highlights whose start_block_id matches the current chapter prefix", () => {
    const reader = makeReader(`<p data-block-id="ch1-p-2">hi</p>`);
    reader.dataset.currentChapter = "1";
    const filtered = filterHighlightsByChapter(reader, highlights);
    expect(filtered.map((h) => h.ID)).toEqual(["h2", "h3"]);
  });

  it("returns all highlights unfiltered when data-current-chapter is -1 (full view)", () => {
    const reader = makeReader(``);
    reader.dataset.currentChapter = "-1";
    const filtered = filterHighlightsByChapter(reader, highlights);
    expect(filtered).toEqual(highlights);
  });

  it("returns all highlights unfiltered when no data-current-chapter attribute is present (pdf/markdown)", () => {
    const reader = makeReader(`<p data-block-id="paragraph-1">hi</p>`);
    const filtered = filterHighlightsByChapter(reader, highlights);
    expect(filtered).toEqual(highlights);
  });

  it("excludes highlights whose start_block_id doesn't match the ch{N}- prefix pattern", () => {
    const reader = makeReader(``);
    reader.dataset.currentChapter = "0";
    const mixed = [
      { ID: "h1", start_block_id: "ch0-p-1" },
      { ID: "h2", start_block_id: "paragraph-3" },
    ];
    const filtered = filterHighlightsByChapter(reader, mixed);
    expect(filtered.map((h) => h.ID)).toEqual(["h1"]);
  });

  it("returns null unchanged when highlights is null", () => {
    const reader = makeReader(``);
    reader.dataset.currentChapter = "1";
    const filtered = filterHighlightsByChapter(reader, null);
    expect(filtered).toBeNull();
  });

  it("returns undefined unchanged when highlights is undefined", () => {
    const reader = makeReader(``);
    reader.dataset.currentChapter = "1";
    const filtered = filterHighlightsByChapter(reader, undefined);
    expect(filtered).toBeUndefined();
  });
});

describe("removeHighlightCard", () => {
  function makePanel(html) {
    const panel = document.createElement("div");
    panel.id = "highlights-panel";
    panel.innerHTML = html;
    document.body.appendChild(panel);
    return panel;
  }

  it("removes the card matching the highlight ID", () => {
    const panel = makePanel(
      `<div data-scroll-to-highlight="h1"><p>card one</p></div>` +
      `<div data-scroll-to-highlight="h2"><p>card two</p></div>`
    );
    removeHighlightCard(panel, "h1");
    expect(panel.querySelector(`[data-scroll-to-highlight="h1"]`)).toBeNull();
    expect(panel.querySelector(`[data-scroll-to-highlight="h2"]`)).not.toBeNull();
  });

  it("is a no-op when the card is not found", () => {
    const panel = makePanel(`<div data-scroll-to-highlight="h2"><p>card</p></div>`);
    expect(() => removeHighlightCard(panel, "h1")).not.toThrow();
    expect(panel.children.length).toBe(1);
  });
});

describe("mirrorSidebarState", () => {
  beforeEach(() => {
    document.body.innerHTML = "";
    document.body.removeAttribute("data-sidebar-state");
  });

  it("copies the wrapper's state attribute onto document.body", () => {
    const wrapper = document.createElement("div");
    wrapper.setAttribute("data-tui-sidebar-state", "expanded");
    document.body.appendChild(wrapper);

    mirrorSidebarState(wrapper);
    expect(document.body.getAttribute("data-sidebar-state")).toBe("expanded");

    wrapper.setAttribute("data-tui-sidebar-state", "collapsed");
    mirrorSidebarState(wrapper);
    expect(document.body.getAttribute("data-sidebar-state")).toBe("collapsed");
  });

  it("removes the body attribute when the wrapper has no state", () => {
    const wrapper = document.createElement("div");
    document.body.appendChild(wrapper);
    document.body.setAttribute("data-sidebar-state", "expanded");

    mirrorSidebarState(wrapper);
    expect(document.body.hasAttribute("data-sidebar-state")).toBe(false);
  });
});

describe("setupSidebarStateMirror", () => {
  beforeEach(() => {
    document.body.innerHTML = "";
    document.body.removeAttribute("data-sidebar-state");
  });

  function makeWrapper(state) {
    const wrapper = document.createElement("div");
    wrapper.setAttribute("data-tui-sidebar-wrapper", "");
    wrapper.setAttribute("data-tui-sidebar-id", "epub-toc-sidebar");
    wrapper.setAttribute("data-tui-sidebar-state", state);
    document.body.appendChild(wrapper);
    return wrapper;
  }

  it("is a no-op when no epub sidebar wrapper is present", () => {
    const observer = setupSidebarStateMirror();
    expect(observer).toBeUndefined();
    expect(document.body.hasAttribute("data-sidebar-state")).toBe(false);
  });

  it("does not react to wrappers with a different sidebar id", () => {
    const wrapper = document.createElement("div");
    wrapper.setAttribute("data-tui-sidebar-wrapper", "");
    wrapper.setAttribute("data-tui-sidebar-id", "some-other-sidebar");
    wrapper.setAttribute("data-tui-sidebar-state", "expanded");
    document.body.appendChild(wrapper);

    setupSidebarStateMirror();
    expect(document.body.hasAttribute("data-sidebar-state")).toBe(false);
  });

  it("mirrors the initial state on setup", () => {
    makeWrapper("expanded");
    setupSidebarStateMirror();
    expect(document.body.getAttribute("data-sidebar-state")).toBe("expanded");
  });

  it("mirrors subsequent state changes via MutationObserver", async () => {
    const wrapper = makeWrapper("expanded");
    setupSidebarStateMirror();
    expect(document.body.getAttribute("data-sidebar-state")).toBe("expanded");

    wrapper.setAttribute("data-tui-sidebar-state", "collapsed");
    // MutationObserver callbacks run as a microtask.
    await Promise.resolve();
    await Promise.resolve();

    expect(document.body.getAttribute("data-sidebar-state")).toBe("collapsed");
  });
});

describe("setupTOCActiveOnClick", () => {
  beforeEach(() => {
    document.body.innerHTML = "";
  });

  function makeSidebar() {
    const sidebar = document.createElement("div");
    sidebar.innerHTML = `
      <a data-tui-sidebar="menu-button" data-tui-sidebar-active="true" href="?chapter=0">Chapter 1</a>
      <a data-tui-sidebar="menu-sub-button" href="#anchor-a">Section A</a>
      <a data-tui-sidebar="menu-sub-button" href="#anchor-b">Section B</a>
    `;
    document.body.appendChild(sidebar);
    return sidebar;
  }

  it("is a no-op when sidebar is null", () => {
    expect(() => setupTOCActiveOnClick(null)).not.toThrow();
  });

  it("activates the clicked subsection link and deactivates the previous one", () => {
    const sidebar = makeSidebar();
    setupTOCActiveOnClick(sidebar);

    const [chapterLink, sectionA] = sidebar.querySelectorAll("a");
    sectionA.click();

    expect(chapterLink.hasAttribute("data-tui-sidebar-active")).toBe(false);
    expect(sectionA.getAttribute("data-tui-sidebar-active")).toBe("true");
  });

  it("moves the active state between subsection links on repeated clicks", () => {
    const sidebar = makeSidebar();
    setupTOCActiveOnClick(sidebar);

    const [, sectionA, sectionB] = sidebar.querySelectorAll("a");
    sectionA.click();
    sectionB.click();

    expect(sectionA.hasAttribute("data-tui-sidebar-active")).toBe(false);
    expect(sectionB.getAttribute("data-tui-sidebar-active")).toBe("true");
  });

  it("ignores clicks outside any TOC link", () => {
    const sidebar = makeSidebar();
    setupTOCActiveOnClick(sidebar);

    sidebar.click();

    const [chapterLink] = sidebar.querySelectorAll("a");
    expect(chapterLink.getAttribute("data-tui-sidebar-active")).toBe("true");
  });
});

describe("buildTOCScrollEntries / activeTOCEntry", () => {
  beforeEach(() => {
    document.body.innerHTML = "";
  });

  function makePage() {
    const reader = document.createElement("div");
    reader.id = "reader";
    reader.innerHTML = `
      <h1 id="intro">Intro</h1>
      <h2 id="section-a">Section A</h2>
      <h2 id="section-b">Section B</h2>
    `;
    document.body.appendChild(reader);

    const sidebar = document.createElement("div");
    sidebar.innerHTML = `
      <a data-toc-chapter="0" data-toc-anchor="">Chapter 1</a>
      <a data-toc-chapter="0" data-toc-anchor="section-a">Section A</a>
      <a data-toc-chapter="0" data-toc-anchor="section-b">Section B</a>
      <a data-toc-chapter="1" data-toc-anchor="other">Other chapter</a>
    `;
    document.body.appendChild(sidebar);

    return { reader, sidebar };
  }

  it("only includes links tagged with the current chapter", () => {
    const { reader, sidebar } = makePage();
    const entries = buildTOCScrollEntries(reader, sidebar, "0");
    expect(entries).toHaveLength(3);
    expect(entries.every((e) => e.link.dataset.tocChapter === "0")).toBe(true);
  });

  it("gives the chapter-root link -Infinity so it's always the fallback", () => {
    const { reader, sidebar } = makePage();
    const entries = buildTOCScrollEntries(reader, sidebar, "0");
    const rootEntry = entries.find((e) => e.link.dataset.tocAnchor === "");
    expect(rootEntry.top).toBe(-Infinity);
  });

  it("activeTOCEntry picks the last entry at or above scrollY", () => {
    const entries = [
      { link: "root", top: -Infinity },
      { link: "a", top: 100 },
      { link: "b", top: 500 },
    ];
    expect(activeTOCEntry(entries, 0).link).toBe("root");
    expect(activeTOCEntry(entries, 150).link).toBe("a");
    expect(activeTOCEntry(entries, 600).link).toBe("b");
  });

  it("returns null for an empty entry list", () => {
    expect(activeTOCEntry([], 0)).toBe(null);
  });
});

describe("setupTOCScrollTracking", () => {
  beforeEach(() => {
    document.body.innerHTML = "";
  });

  it("is a no-op when reader or sidebar is missing", () => {
    expect(() => setupTOCScrollTracking(null, null, "0")).not.toThrow();
  });

  it("marks the chapter-root link active when there are no headings in the DOM", () => {
    const reader = document.createElement("div");
    reader.id = "reader";
    document.body.appendChild(reader);

    const sidebar = document.createElement("div");
    sidebar.innerHTML = `<a data-toc-chapter="0" data-toc-anchor="">Chapter 1</a>`;
    document.body.appendChild(sidebar);

    setupTOCScrollTracking(reader, sidebar, "0");

    const link = sidebar.querySelector("a");
    expect(link.getAttribute("data-tui-sidebar-active")).toBe("true");
  });
});
