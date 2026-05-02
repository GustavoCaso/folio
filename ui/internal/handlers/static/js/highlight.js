// Highlight model: a highlight stores (start_block_id, start_pos, end_block_id, end_pos).
// Positions are character offsets within a block, where each <img> counts as 1.
// Capture (mouseup) computes positions from a Selection. Apply (on load) reconstructs
// DOM ranges from positions and wraps text + images in <mark>.
//
// Pure helpers exported for tests.

// Walks up from a DOM node to find the nearest [data-block-id] element.
// Used to map a Selection endpoint back to its anchor block.
export function findBlockAncestor(node, root) {
  let el = node.nodeType === Node.TEXT_NODE ? node.parentElement : node;
  while (el && el !== root) {
    if (el.dataset && el.dataset.blockId) return el;
    el = el.parentElement;
  }
  return null;
}

// Single source of truth for "what counts in a block offset": text nodes and <img>.
// Used by both capture (offsetWithinBlock) and apply (locateInBlock) so positions
// round-trip exactly.
function blockOffsetWalker(block) {
  return document.createTreeWalker(
    block,
    NodeFilter.SHOW_TEXT | NodeFilter.SHOW_ELEMENT,
    {
      acceptNode(node) {
        if (node.nodeType === Node.TEXT_NODE) return NodeFilter.FILTER_ACCEPT;
        if (node.nodeType === Node.ELEMENT_NODE && node.tagName === "IMG") {
          return NodeFilter.FILTER_ACCEPT;
        }
        return NodeFilter.FILTER_SKIP;
      },
    }
  );
}

function nodeLen(node) {
  return node.nodeType === Node.TEXT_NODE ? node.textContent.length : 1;
}

// Convert a DOM Range endpoint (node + nodeOffset) into a character position.
// DOM endpoints come in two flavors:
//   - text node: nodeOffset is a char index within the text
//   - element node: nodeOffset is a child index (used when selection sits at an
//     img boundary, so endContainer is <p> with offset = childIndex)
export function offsetWithinBlock(block, targetNode, targetOffset) {
  // Element-target case: sum lengths of preceding children of targetNode.
  if (targetNode.nodeType === Node.ELEMENT_NODE) {
    const walker = blockOffsetWalker(block);
    let count = 0;
    while (walker.nextNode()) {
      const cur = walker.currentNode;
      if (cur.parentNode === targetNode) {
        const idx = Array.prototype.indexOf.call(targetNode.childNodes, cur);
        if (idx >= targetOffset) return count;
      }
      count += nodeLen(cur);
    }
    return count;
  }

  // Text-target case: walk until we reach the target text node, then add its char offset.
  const walker = blockOffsetWalker(block);
  let count = 0;
  while (walker.nextNode()) {
    const cur = walker.currentNode;
    if (cur === targetNode) return count + targetOffset;
    count += nodeLen(cur);
  }
  return count;
}

function makeMark(className, dataset) {
  const mark = document.createElement("mark");
  mark.className = className;
  Object.assign(mark.dataset, dataset);
  return mark;
}

// Wrap every text fragment and <img> inside the range with a <mark>.
// We can't use range.surroundContents() — it throws when the range crosses
// element boundaries (true for any non-trivial highlight). Instead we wrap
// each text node / img individually with its own sub-range.
export function wrapRangeTextNodes(range, className, dataset) {
  const root = range.commonAncestorContainer;
  const nodes = [];
  // TreeWalker doesn't include its root, so a single-text-node range needs special handling.
  if (root.nodeType === Node.TEXT_NODE) {
    nodes.push(root);
  } else {
    const walker = document.createTreeWalker(
      root,
      NodeFilter.SHOW_TEXT | NodeFilter.SHOW_ELEMENT,
      {
        acceptNode(n) {
          if (n.nodeType === Node.TEXT_NODE) return NodeFilter.FILTER_ACCEPT;
          if (n.nodeType === Node.ELEMENT_NODE && n.tagName === "IMG") {
            return NodeFilter.FILTER_ACCEPT;
          }
          return NodeFilter.FILTER_SKIP;
        },
      }
    );
    while (walker.nextNode()) {
      if (range.intersectsNode(walker.currentNode)) {
        nodes.push(walker.currentNode);
      }
    }
  }

  nodes.forEach((node) => {
    // Image: splice a <mark> in manually (surroundContents requires text endpoints).
    // Skip if already wrapped — happens when overlapping highlights apply on load.
    if (node.nodeType === Node.ELEMENT_NODE && node.tagName === "IMG") {
      if (node.parentElement && node.parentElement.tagName === "MARK") return;
      const mark = makeMark(className, dataset);
      node.parentNode.insertBefore(mark, node);
      mark.appendChild(node);
      return;
    }

    // Text: build a sub-range covering the part of this node that's inside `range`.
    // Only the boundary nodes get clipped; middle nodes are wrapped in full.
    const nodeRange = document.createRange();
    nodeRange.selectNodeContents(node);
    if (node === range.startContainer) nodeRange.setStart(node, range.startOffset);
    if (node === range.endContainer) nodeRange.setEnd(node, range.endOffset);
    if (nodeRange.collapsed) return;

    const mark = makeMark(className, dataset);
    try {
      nodeRange.surroundContents(mark);
    } catch {
      // text node already inside another <mark>; safe to skip
    }
  });
}

// Inverse of offsetWithinBlock: turn a stored char position back into a
// DOM Range endpoint. `which` disambiguates exact boundaries:
//   - "start": pos at end-of-text-node belongs to the NEXT node (so we don't
//     start a range at a zero-width tail)
//   - "end":   pos at end-of-text-node belongs to THIS node (so we don't
//     end a range at the zero-width head of the next)
// Image positions resolve to {parent, childIndex} or {parent, childIndex+1}
// — Range endpoints can't go inside an <img>, only around it.
function locateInBlock(block, pos, which) {
  const walker = blockOffsetWalker(block);
  let count = 0;
  while (walker.nextNode()) {
    const cur = walker.currentNode;
    const len = nodeLen(cur);

    if (cur.nodeType === Node.TEXT_NODE) {
      const cmp = which === "start" ? count + len > pos : count + len >= pos;
      if (cmp) {
        return { node: cur, offset: pos - count, kind: "text" };
      }
    } else {
      // <img> occupies positions [count, count+1].
      if (which === "start" && pos <= count) {
        const parent = cur.parentNode;
        const idx = Array.prototype.indexOf.call(parent.childNodes, cur);
        return { node: parent, offset: idx, kind: "element" }; // before img
      }
      if (which === "end" && pos === count + 1) {
        const parent = cur.parentNode;
        const idx = Array.prototype.indexOf.call(parent.childNodes, cur);
        return { node: parent, offset: idx + 1, kind: "element" }; // after img
      }
    }
    count += len;
  }
  // pos beyond block content — clamp to block end.
  return { node: block, offset: block.childNodes.length, kind: "element" };
}

// Slice of [data-block-id] elements between start and end (inclusive), in document order.
// Used to walk every block touched by a multi-block highlight.
function collectBlocksBetween(reader, startBlock, endBlock) {
  if (startBlock === endBlock) return [startBlock];
  const all = reader.querySelectorAll("[data-block-id]");
  const out = [];
  let inRange = false;
  for (const el of all) {
    if (el === startBlock) inRange = true;
    if (inRange) out.push(el);
    if (el === endBlock) break;
  }
  return out;
}

function blockTotalLen(block) {
  const walker = blockOffsetWalker(block);
  let n = 0;
  while (walker.nextNode()) n += nodeLen(walker.currentNode);
  return n;
}

// Render a stored highlight onto the DOM. Walks every block from start to end
// and applies a per-block sub-range; never builds one giant cross-block Range
// (those would crash in surroundContents on element boundaries).
export function applyHighlight(reader, h) {
  // BlockID fallback supports legacy single-block test fixtures.
  const startBlockID = h.StartBlockID || h.BlockID;
  const endBlockID = h.EndBlockID || startBlockID;
  const startBlock = reader.querySelector(`[data-block-id="${startBlockID}"]`);
  const endBlock = reader.querySelector(`[data-block-id="${endBlockID}"]`);
  if (!startBlock || !endBlock) return;

  const blocks = collectBlocksBetween(reader, startBlock, endBlock);

  blocks.forEach((block) => {
    // Start block: [StartPos … blockEnd]. End block: [0 … EndPos]. Middle: full block.
    const fromPos = block === startBlock ? h.StartPos : 0;
    const toPos = block === endBlock ? h.EndPos : blockTotalLen(block);
    if (fromPos >= toPos) return;

    const start = locateInBlock(block, fromPos, "start");
    const end = locateInBlock(block, toPos, "end");
    if (!start || !end) return;

    const range = document.createRange();
    try {
      range.setStart(start.node, start.offset);
      range.setEnd(end.node, end.offset);
    } catch {
      return;
    }
    wrapRangeTextNodes(range, "highlight", { highlightId: h.ID });
  });
}

export function applyHighlights(reader, highlights) {
  highlights.forEach((h) => applyHighlight(reader, h));
}

// --- Tooltip helpers ---

export function formatHighlight(h) {
  if (!h) return "(no annotation)";
  if (h.Tag && h.Note) return h.Tag + ": " + h.Note;
  return h.Tag || h.Note || "(no annotation)";
}

function showTooltip(rect, text) {
  const tooltip = document.getElementById("hl-tooltip");
  const content = document.getElementById("hl-tooltip-content");
  if (!tooltip || !content) return;
  content.textContent = text;
  tooltip.style.top = (rect.bottom + 6) + "px";
  tooltip.style.left = rect.left + "px";
  tooltip.classList.remove("hidden");
}

function hideTooltip() {
  const tooltip = document.getElementById("hl-tooltip");
  if (tooltip) tooltip.classList.add("hidden");
}

// --- Bootstrap (browser only) ---

function bootstrap() {
  const reader = document.getElementById("reader");
  if (!reader) return;
  const popoverContent = document.getElementById("hl-popover-content");
  const saveBtn = document.getElementById("hl-save");
  const cancelBtn = document.getElementById("hl-cancel");
  if (!popoverContent || !saveBtn || !cancelBtn) return;
  const jobID = reader.dataset.jobId;

  let pendingSelection = null;
  let popoverOpen = false;

  applyHighlights(reader, window.__highlights || []);

  const tooltip = document.getElementById("hl-tooltip");
  if (tooltip) {
    reader.addEventListener("click", (e) => {
      const mark = e.target.closest("mark.highlight");
      if (!mark) return;
      const h = (window.__highlights || []).find(x => String(x.ID) === mark.dataset.highlightId);
      showTooltip(mark.getBoundingClientRect(), formatHighlight(h));
    });

    document.addEventListener("click", (e) => {
      if (!tooltip.classList.contains("hidden") &&
        !tooltip.contains(e.target) &&
        !e.target.closest("mark.highlight")) {
        hideTooltip();
      }
    });
  }

  popoverContent.addEventListener("toggle", (e) => {
    if (e.newState === "closed") {
      popoverContent.setAttribute("data-tui-popover-open", "false");
      popoverOpen = false;
      pendingSelection = null;
      window.getSelection()?.removeAllRanges();
    }
  });

  function openPopover(rect) {
    if (popoverOpen) return;
    popoverContent.style.top = (rect.bottom + 6) + "px";
    popoverContent.style.left = rect.left + "px";
    popoverContent.showPopover();
    popoverContent.setAttribute("data-tui-popover-open", "true");
    popoverOpen = true;
  }

  function closePopover() {
    if (!popoverOpen) return;
    popoverContent.hidePopover();
  }

  document.addEventListener("click", (e) => {
    if (popoverOpen && !popoverContent.contains(e.target)) {
      closePopover();
    }
  });

  document.addEventListener("mouseup", () => {
    setTimeout(() => {
      const sel = window.getSelection();
      if (!sel || sel.isCollapsed || sel.rangeCount === 0) return;

      const range = sel.getRangeAt(0);
      if (!reader.contains(range.startContainer) || !reader.contains(range.endContainer)) return;

      const startBlock = findBlockAncestor(range.startContainer, reader);
      const endBlock = findBlockAncestor(range.endContainer, reader);
      if (!startBlock || !endBlock) {
        sel.removeAllRanges();
        return;
      }

      pendingSelection = {
        job_id: jobID,
        start_block_id: startBlock.dataset.blockId,
        end_block_id: endBlock.dataset.blockId,
        start_pos: offsetWithinBlock(startBlock, range.startContainer, range.startOffset),
        end_pos: offsetWithinBlock(endBlock, range.endContainer, range.endOffset),
        text: sel.toString(),
      };

      openPopover(range.getBoundingClientRect());
    }, 0);
  });

  cancelBtn.addEventListener("click", () => {
    pendingSelection = null;
    closePopover();
    window.getSelection()?.removeAllRanges();
  });

  saveBtn.addEventListener("click", async () => {
    if (!pendingSelection) return;

    const tag = document.getElementById("hl-tag").value;
    const note = document.getElementById("hl-note").value;

    const resp = await fetch("/highlights", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ...pendingSelection, tag, note }),
    });

    if (resp.ok) {
      closePopover();
      pendingSelection = null;
      window.location.reload();
    }
  });
}

if (typeof document !== "undefined" && document.getElementById("reader")) {
  bootstrap();
}
