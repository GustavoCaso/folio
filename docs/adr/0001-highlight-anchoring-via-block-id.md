# Highlights anchor to block IDs, not raw character offsets

Highlights store a `block_id` (identifying a single block-level element — heading, paragraph, code block) plus start/end character positions scoped to that block's text, rather than a single character offset into the whole rendered document. A raw whole-document offset would break whenever upstream rendering shifted by even one character — a goldmark version bump, a Docling output tweak, or (for EPUB) any chapter re-render. Scoping the offset to a stable block ID means highlights only break if the block structure itself changes, which is rare.

EPUB extends the same mechanism with chapter-prefixed IDs (`ch{N}-{tag}-{M}`) instead of introducing a new highlight-scoping column — a book's multiple chapter documents are handled by the ID scheme alone. [ADR 0009](0009-highlight-chapter-column.md) later added a `chapter` column copied from that prefix so chapter pages can query their highlights directly; the ID scheme is unchanged.
