# Store the chapter on each highlight

[ADR 0001](0001-highlight-anchoring-via-block-id.md) scoped EPUB highlights to a chapter through the `ch{N}-` prefix on `start_block_id` alone, with no separate column. The database couldn't filter on that prefix cheaply, so the reader loaded every highlight in the book on every chapter page, and `reader.js` dropped the ones from other chapters in the browser. Long books with many highlights paid for the whole set on every page, and the highlights panel had to choose between showing the whole book and showing the chapter.

We added an indexed `highlights.chapter` column (migration 0012), keyed by `(document_id, chapter)`.

## Decision

- `chapter` is derived on the server from `start_block_id` when a highlight is created (`chapterFromBlockID` in `handlers/highlights.go`). The client never sends it, so it can't disagree with the block ID.
- Block IDs keep the `ch{N}-{tag}-{M}` shape. The column is a copy of the prefix that the database can filter on. It does not replace the prefix.
- A highlight that spans chapters belongs to the chapter it starts in. The reader only renders one chapter at a time, so it renders partly.
- Migration 0012 backfills existing rows from the prefix. IDs without a `ch{N}-` prefix (old Markdown `kind-N` IDs on PDF documents, see [ADR 0008](0008-remove-markdown-renderer.md)) fall back to chapter 0.

## Consequences

- `GET /read/{id}?chapter=N` calls `ListChapterHighlights`. The highlights panel and its count show only the current chapter. (The `?full=1` view that called `ListHighlights` was removed in [ADR 0010](0010-clean-epub-output-remove-full-view.md).)
- The client-side `filterHighlightsByChapter` was removed.
- If the block ID scheme changes, the derivation and the backfill have to change with it.
