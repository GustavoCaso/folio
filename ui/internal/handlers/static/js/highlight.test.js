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
