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

function blockWalker(block) {
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

function makeMark(className, dataset) {
  const mark = document.createElement("mark");
  mark.className = className;
  Object.assign(mark.dataset, dataset);
  return mark;
}

// Resolves a block-level offset to a specific child node and a local offset
// within that node. Walks text nodes and <img> children in document order,
// each img counting as 1. Accumulates lengths to find which node owns pos:
//
//   <p>before<img/>after</p>  pos=7
//   node      len  accumulated
//   "before"   6    0 → 6   (7 <= 6? no)
//   <img>      1    6 → 7   (7 <= 7? no)
//   "after"    5    7 → 12  (7 <= 12? yes) → offset = 7-7 = 0
function getAnchorNode(block, pos) {
  const walker = blockWalker(block);
  let accumulated = 0;
  while (walker.nextNode()) {
    const node = walker.currentNode;
    const len = nodeLen(node);
    if (pos <= accumulated + len) {
      if (node.nodeType === Node.ELEMENT_NODE) return { node, offset: 0 };
      return { node, offset: pos - accumulated };
    }
    accumulated += len;
  }
  return null;
}

// Wrap every text fragment and <img> inside the range with a <mark>.
// We can't use range.surroundContents() — it throws when the range crosses
// element boundaries (true for any non-trivial highlight). Instead we wrap
// each text node / img individually with its own sub-range.
export function wrapRangeTextNodes(range, className, dataset) {
  const root = range.commonAncestorContainer;
  const nodes = [];
  // Single image selection creates a collpased range
  // intersectsNode returns false. We need to handle manually
  if (root.nodeType === Node.ELEMENT_NODE && root.tagName === "IMG") {
    nodes.push(root);
    // TreeWalker doesn't include its root, so a single-text-node range needs special handling.
  } else if (root.nodeType === Node.TEXT_NODE) {
    nodes.push(root);
  } else {
    const walker = blockWalker(root);
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

// Render a stored highlight onto the DOM.
export function applyHighlight(reader, h) {
  const startBlockID = h.StartBlockID;
  const endBlockID = h.EndBlockID;
  const startBlock = reader.querySelector(`[data-block-id="${startBlockID}"]`);
  const endBlock = reader.querySelector(`[data-block-id="${endBlockID}"]`);
  if (!startBlock || !endBlock) {
    console.log('[highlight.js] skipping highlight. no start or end block');
    return;
  }

  const startAnchor = getAnchorNode(startBlock, h.StartPos);
  const endAnchor = getAnchorNode(endBlock, h.EndPos);

  if (!startAnchor || !endAnchor) {
    console.log('[highlight.js] skipping highlight. no node to anchor at');
    return;
  }

  const range = document.createRange();
  try {
    range.setStart(startAnchor.node, startAnchor.offset);
    range.setEnd(endAnchor.node, endAnchor.offset);
  } catch (error) {
    console.error(`Failed to create range. ${error}`)
    return;
  }

  wrapRangeTextNodes(range, "highlight", { highlightId: h.ID });
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

  const highlightsData = document.getElementById("highlights-data")
  if (!highlightsData) return;

  let highlights;
  try {
    highlights = JSON.parse(highlightsData.textContent)
  } catch (error) {
    console.error(error);
    return;
  }


  const popoverContent = document.getElementById("hl-popover-content");
  const saveBtn = document.getElementById("hl-save");
  const cancelBtn = document.getElementById("hl-cancel");
  if (!popoverContent || !saveBtn || !cancelBtn) return;
  const jobID = reader.dataset.jobId;

  let pendingSelection = null;
  let popoverOpen = false;
  if (highlights != null && highlights.length > 0) {
    applyHighlights(reader, highlights);
    console.log('[hightlight.js] Highlights applied')
  }


  const tooltip = document.getElementById("hl-tooltip");
  if (tooltip) {
    reader.addEventListener("click", (e) => {
      const mark = e.target.closest("mark.highlight");
      if (!mark) return;
      const h = highlights.find(x => String(x.ID) === mark.dataset.highlightId);
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
        start_pos: range.startOffset,
        end_pos: range.endOffset,
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
