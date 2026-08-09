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
  // Single image selection creates a collapsed range
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
    // Skip empty nodes
    if (node.textContent === '\n') return;
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

// Filters highlights down to those belonging to the currently viewed epub
// chapter. Reads #reader's own data-current-chapter attribute (set by
// reader.templ's currentChapterAttr):
//   - absent entirely: not an epub document (pdf/markdown) — pass through all highlights.
//   - "-1": full-book view — no single-chapter restriction, pass through all highlights.
//   - "N": single-chapter view — keep only highlights whose start_block_id starts
//     with "chN-" (epubrender's chapter-prefixed block ID scheme).
export function filterHighlightsByChapter(reader, highlights) {
  if (!highlights) return highlights;

  const chapterIdx = reader.dataset.currentChapter;
  if (chapterIdx === undefined || chapterIdx === "-1") return highlights;

  const prefix = `ch${chapterIdx}-`;
  return highlights.filter((h) => h.start_block_id && h.start_block_id.startsWith(prefix));
}

// Convert a DOM Range endpoint (node + nodeOffset) into a character position.
// DOM endpoints come in two flavors:
//   - text node: nodeOffset is a char index within the text
//   - element node: nodeOffset is a child index (used when selection sits at an
//     img boundary, so endContainer is <p> with offset = childIndex)
export function offsetWithinBlock(block, targetNode, targetOffset) {
  const walker = blockWalker(block);
  let count = 0;

  // Element-target case: sum lengths of preceding children of targetNode.
  if (targetNode.nodeType === Node.ELEMENT_NODE) {
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
  while (walker.nextNode()) {
    const cur = walker.currentNode;
    if (cur === targetNode) return count + targetOffset;
    count += nodeLen(cur);
  }
  return count;
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

// Remove all <mark> wrappers for a highlight ID, unwrapping their children in place.
export function removeHighlightMarks(reader, id) {
  reader.querySelectorAll(`mark[data-highlight-id="${id}"]`).forEach((mark) => {
    mark.replaceWith(...mark.childNodes);
  });
}

// Remove the highlight card from the panel by highlight ID.
export function removeHighlightCard(panel, id) {
  const card = panel.querySelector(`div[data-scroll-to-highlight="${id}"]`);
  if (card) card.remove();
}

// Returns 0-100 based on which block is first visible in the viewport.
export function computeProgress(blocks) {
  if (!blocks.length) return 0;
  let firstVisible = 0;
  for (let i = 0; i < blocks.length; i++) {
    if (blocks[i].getBoundingClientRect().top >= 0) {
      firstVisible = i;
      break;
    }
    firstVisible = i;
  }
  return Math.round((firstVisible / Math.max(blocks.length - 1, 1)) * 100);
}

// Returns the data-block-id of the first block whose top >= 0.
export function firstVisibleBlockID(blocks) {
  for (const block of blocks) {
    if (block.getBoundingClientRect().top >= 0) return block.dataset.blockId;
  }
  return blocks.length ? blocks[blocks.length - 1].dataset.blockId : "";
}

export function calcPopoverPosition(rect, popoverWidth, viewportWidth) {
  const spaceRight = viewportWidth - rect.right - 8;
  if (spaceRight >= popoverWidth) {
    return { top: rect.top, left: rect.right + 8 };
  }
  return {
    top: rect.bottom + 6,
    left: Math.max(4, Math.min(rect.left, viewportWidth - popoverWidth - 4)),
  };
}

export function buildPendingSelection(reader, jobID, sel) {
  if (!sel || sel.isCollapsed || sel.rangeCount === 0) return null;
  const range = sel.getRangeAt(0);
  if (!reader.contains(range.startContainer) || !reader.contains(range.endContainer)) return null;
  const startBlock = findBlockAncestor(range.startContainer, reader);
  const endBlock = findBlockAncestor(range.endContainer, reader);
  if (!startBlock || !endBlock) {
    sel.removeAllRanges();
    return null;
  }
  return {
    job_id: jobID,
    start_block_id: startBlock.dataset.blockId,
    end_block_id: endBlock.dataset.blockId,
    start_pos: offsetWithinBlock(startBlock, range.startContainer, range.startOffset),
    end_pos: offsetWithinBlock(endBlock, range.endContainer, range.endOffset),
    text: sel.toString(),
  };
}

// --- Sidebar state mirroring ---
//
// TemplUI's sidebar toggle mutates a `data-tui-sidebar-state` attribute on
// the sidebar's own wrapper element (see sidebar.js's setSidebarState) —
// there's no custom event, and TemplUI's built-in `peer`/`group-data-*` CSS
// mechanism only reaches DOM siblings positioned after that wrapper. The
// site header (#site-header in layout.templ) lives outside the sidebar's
// component tree entirely, so it can't use that mechanism to react to the
// sidebar opening/closing. Instead we mirror the wrapper's state onto
// document.body as `data-sidebar-state`, which #site-header's CSS can key
// off regardless of DOM position.
const EPUB_SIDEBAR_ID = "epub-toc-sidebar";

// Reads the current data-tui-sidebar-state off the given sidebar wrapper and
// copies it onto document.body as data-sidebar-state. Exported for tests.
export function mirrorSidebarState(wrapper) {
  const state = wrapper.getAttribute("data-tui-sidebar-state");
  if (state) {
    document.body.setAttribute("data-sidebar-state", state);
  } else {
    document.body.removeAttribute("data-sidebar-state");
  }
}

// Finds the epub TOC sidebar's wrapper element (if present on this page),
// mirrors its initial state onto document.body, and keeps it in sync via a
// MutationObserver watching for attribute changes. No-ops entirely when no
// sidebar is present (pdf/markdown reader pages, or any non-reader page),
// so document.body never gets a data-sidebar-state attribute there.
export function setupSidebarStateMirror() {
  const wrapper = document.querySelector(
    `[data-tui-sidebar-wrapper][data-tui-sidebar-id="${EPUB_SIDEBAR_ID}"]`
  );
  if (!wrapper) return;

  mirrorSidebarState(wrapper);

  const observer = new MutationObserver(() => mirrorSidebarState(wrapper));
  observer.observe(wrapper, {
    attributes: true,
    attributeFilter: ["data-tui-sidebar-state"],
  });
  return observer;
}

// --- TOC subsection active-state tracking ---
//
// Chapter links (menu-button) get their `data-tui-sidebar-active` state from
// the server (Reader's IsActive prop), since clicking one always does a full
// page navigation. Subsection links (menu-sub-button) that jump to an anchor
// within the *current* chapter don't navigate at all — the server has no way
// to know which one was clicked. This click handler fills that gap: it
// deactivates any TOC link inside the sidebar's menu and activates whichever
// menu-button or menu-sub-button was clicked, giving same-page anchor jumps
// the same "selected" styling cross-chapter links already get for free.
export function setupTOCActiveOnClick(sidebar) {
  if (!sidebar) return;

  sidebar.addEventListener("click", (e) => {
    const link = e.target.closest(
      '[data-tui-sidebar="menu-button"], [data-tui-sidebar="menu-sub-button"]'
    );
    if (!link || !sidebar.contains(link)) return;

    sidebar
      .querySelectorAll('[data-tui-sidebar-active="true"]')
      .forEach((el) => el.removeAttribute("data-tui-sidebar-active"));
    link.setAttribute("data-tui-sidebar-active", "true");
  });
}

// --- TOC scroll tracking ---
//
// As the reader scrolls through the current chapter, highlight whichever TOC
// link (chapter root or subsection) corresponds to the heading nearest the
// top of the viewport, and keep that link scrolled into view within the
// sidebar's own scroll container. Only links tagged with the chapter
// currently being read are considered — cross-chapter subsection links
// (rendered with `?chapter=N#anchor` hrefs) point at headings that aren't in
// this page's DOM at all, so they're excluded up front.
//
// reader.templ tags every TOC link with data-toc-chapter/data-toc-anchor;
// the chapter-root link carries data-toc-anchor="" and represents the top of
// the chapter, before its first subsection heading.
export function buildTOCScrollEntries(reader, sidebar, currentChapterIdx) {
  if (!reader || !sidebar) return [];

  const links = Array.from(
    sidebar.querySelectorAll(`[data-toc-chapter="${currentChapterIdx}"]`)
  );

  const entries = [];
  for (const link of links) {
    const anchor = link.dataset.tocAnchor;
    if (!anchor) {
      entries.push({ link, top: -Infinity });
      continue;
    }
    const heading = document.getElementById(anchor);
    if (!heading) continue;
    entries.push({ link, top: heading.getBoundingClientRect().top + window.scrollY });
  }

  entries.sort((a, b) => a.top - b.top);
  return entries;
}

// Picks the last entry whose heading is at or above the current scroll
// position (i.e. the section the reader is currently inside), given entries
// already sorted by document position (see buildTOCScrollEntries).
export function activeTOCEntry(entries, scrollY) {
  let active = entries[0] ?? null;
  for (const entry of entries) {
    if (entry.top <= scrollY) {
      active = entry;
    } else {
      break;
    }
  }
  return active;
}

export function setupTOCScrollTracking(reader, sidebar, currentChapterIdx) {
  if (!reader || !sidebar || currentChapterIdx == null) return;

  const entries = buildTOCScrollEntries(reader, sidebar, currentChapterIdx);
  if (entries.length === 0) return;

  let lastActiveLink = null;

  function update() {
    // scroll-padding-top (see reader.css) offsets anchor jumps by the
    // header height; add the same offset here so the section that just
    // scrolled to the top of the visible area is the one marked active.
    const offset = parseFloat(getComputedStyle(document.documentElement).scrollPaddingTop) || 0;
    const active = activeTOCEntry(entries, window.scrollY + offset + 1);
    if (!active || active.link === lastActiveLink) return;

    lastActiveLink = active.link;
    sidebar
      .querySelectorAll('[data-tui-sidebar-active="true"]')
      .forEach((el) => el.removeAttribute("data-tui-sidebar-active"));
    active.link.setAttribute("data-tui-sidebar-active", "true");
    if (typeof active.link.scrollIntoView === "function") {
      active.link.scrollIntoView({ block: "start", behavior: "smooth" });
    }
  }

  window.addEventListener("scroll", update, { passive: true });
  update();
}

// --- Bootstrap (browser only) ---

function bootstrap() {
  const reader = document.getElementById("reader");
  if (!reader) return;

  const progressBar = document.getElementById("progress-bar");

  const allBlocks = Array.from(reader.querySelectorAll("[data-block-id]"));

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
  const errorEl = document.getElementById("hl-error");
  if (!popoverContent || !saveBtn || !cancelBtn) return;
  const jobID = reader.dataset.jobId;

  let pendingSelection = null;
  let popoverOpen = false;
  let suppressNextClick = false;
  const filteredHighlights = filterHighlightsByChapter(reader, highlights);
  if (filteredHighlights != null && filteredHighlights.length > 0) {
    applyHighlights(reader, filteredHighlights);
    console.log('[highlight.js] Highlights applied')
  }

  // Restore reading position
  const savedBlockID = reader.dataset.readingProgress;
  if (savedBlockID) {
    const target = reader.querySelector(`[data-block-id="${savedBlockID}"]`);
    if (target) target.scrollIntoView({ behavior: "instant", block: "start" });
  }

  // Save reading progress
  let saveTimer = null;

  function updateProgressBar() {
    if (progressBar) progressBar.style.width = computeProgress(allBlocks) + "%";
  }

  function saveProgress() {
    const blockID = firstVisibleBlockID(allBlocks);
    if (!blockID) return;
    navigator.sendBeacon(
      `/read/${jobID}/progress`,
      new Blob([JSON.stringify({ block_id: blockID })], { type: "application/json" })
    );
  }

  function scheduleSave() {
    clearTimeout(saveTimer);
    saveTimer = setTimeout(saveProgress, 2000);
  }

  window.addEventListener("scroll", () => {
    updateProgressBar();
    scheduleSave();
    if (popoverOpen) {
      const currentTop = parseFloat(popoverContent.style.top) || 0;
      const popoverHeight = popoverContent.offsetHeight || 200;
      const clamped = Math.max(8, Math.min(currentTop, window.innerHeight - popoverHeight - 8));
      popoverContent.style.top = clamped + "px";
    }
  }, { passive: true });

  window.addEventListener("pagehide", saveProgress);

  updateProgressBar();

  const highlightPanel = document.getElementById("highlights-panel");
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

  function hideError() {
    if (errorEl) {
      errorEl.textContent = "";
      errorEl.classList.add("hidden");
    }
  }

  function showError(msg) {
    if (errorEl) {
      errorEl.textContent = msg;
      errorEl.classList.remove("hidden");
    }
  }

  popoverContent.addEventListener("toggle", (e) => {
    if (e.newState === "closed") {
      popoverContent.setAttribute("data-tui-popover-open", "false");
      popoverOpen = false;
      pendingSelection = null;
      hideError();
      window.getSelection()?.removeAllRanges();
    }
  });

  function openPopover(rect) {
    if (popoverOpen) return;
    popoverContent.showPopover();
    const { top, left } = calcPopoverPosition(rect, popoverContent.offsetWidth, window.innerWidth);
    popoverContent.style.top = top + "px";
    popoverContent.style.left = left + "px";
    popoverContent.setAttribute("data-tui-popover-open", "true");
    popoverOpen = true;
  }

  function closePopover() {
    if (!popoverOpen) return;
    popoverContent.hidePopover();
  }

  document.addEventListener("click", (e) => {
    if (suppressNextClick) {
      suppressNextClick = false;
      return;
    }
    if (popoverOpen && !popoverContent.contains(e.target)) {
      closePopover();
    }
  });

  function captureSelection(fromTouch) {
    const sel = window.getSelection();
    const built = buildPendingSelection(reader, jobID, sel);

    if (!built) return;

    pendingSelection = built;
    if (fromTouch) suppressNextClick = true;
    openPopover(sel.getRangeAt(0).getBoundingClientRect());
  }

  document.addEventListener("mouseup", () => setTimeout(() => captureSelection(false), 0));
  document.addEventListener("touchend", () => setTimeout(() => captureSelection(true), 0), { passive: true });

  cancelBtn.addEventListener("click", () => {
    pendingSelection = null;
    hideError();
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
      hideError();
      closePopover();
      pendingSelection = null;
      window.location.reload();
    } else {
      let msg = "Failed to save highlight";
      try {
        const body = await resp.json();
        if (body.error) msg = body.error;
      } catch { /* use default msg */ }
      showError(msg);
    }
  });

  // --- Highlights panel ---

  document.addEventListener("click", async (e) => {
    // Delete button
    const deleteBtn = e.target.closest("[data-delete-highlight]");
    if (deleteBtn) {
      const id = deleteBtn.dataset.deleteHighlight;
      if (confirm("Delete Highlight ?")) {
        const resp = await fetch(`/highlights/${id}`, { method: "DELETE", headers: { "Accept": "text/html" } });
        const html = await resp.text();
        document.body.insertAdjacentHTML("beforeend", html);
        if (resp.ok) {
          removeHighlightMarks(reader, id);
          removeHighlightCard(highlightPanel, id);
        }
        return;
      }
    }

    // Scroll-to on card click
    const card = e.target.closest("[data-scroll-to-highlight]");
    if (card) {
      const id = card.dataset.scrollToHighlight;
      const mark = reader.querySelector(`mark[data-highlight-id="${id}"]`);
      if (mark) mark.scrollIntoView({ behavior: "smooth", block: "center" });
    }
  });
}

if (typeof document !== "undefined" && document.getElementById("reader")) {
  bootstrap();
  setupSidebarStateMirror();

  const tocSidebar = document.querySelector('[data-tui-sidebar-content="epub-toc-sidebar"]');
  setupTOCActiveOnClick(tocSidebar);

  const readerEl = document.getElementById("reader");
  const currentChapterIdx = readerEl ? readerEl.dataset.currentChapter : undefined;
  if (currentChapterIdx !== undefined && currentChapterIdx !== "-1") {
    setupTOCScrollTracking(readerEl, tocSidebar, currentChapterIdx);
  }
}
