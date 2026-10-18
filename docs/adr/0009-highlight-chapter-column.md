# Store the chapter on each highlight

[ADR 0001](0001-highlight-anchoring-via-block-id.md) scoped EPUB highlights to a chapter through the `ch{N}-` prefix on `start_block_id` alone, with no separate column. The database couldn't filter on that prefix cheaply, so the reader loaded every highlight in the book on every chapter page, and `reader.js` dropped the ones from other chapters in the browser. Long books with many highlights paid for the whole set on every page, and the highlights panel had to choose between showing the whole book and showing the chapter.

We added an indexed `highlights.chapter` column (migration 0012), keyed by `(job_id, chapter)`.

## Decision

- `chapter` is derived on the server from `start_block_id` when a highlight is created (`chapterFromBlockID` in `handlers/highlights.go`). The client never sends it, so it can't disagree with the block ID.
- Block IDs keep the `ch{N}-{tag}-{M}` shape. The column is a copy of the prefix that the database can filter on. It does not replace the prefix.
- A highlight that spans chapters belongs to the chapter it starts in. In single-chapter view it would only render partly anyway. The full-book view still loads every highlight.
- Migration 0012 backfills existing rows from the prefix. IDs without a `ch{N}-` prefix (old Markdown `kind-N` IDs on PDF jobs, see [ADR 0008](0008-remove-markdown-renderer.md)) fall back to chapter 0.

## Consequences

- `GET /read/{id}?chapter=N` calls `ListChapterHighlights`, and `?full=1` calls `ListHighlights`. The highlights panel and its count show only the current chapter in single-chapter view.
- The client-side `filterHighlightsByChapter` was removed.
- If the block ID scheme changes, the derivation and the backfill have to change with it.
