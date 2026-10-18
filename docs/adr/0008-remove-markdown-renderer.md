# Remove the Markdown renderer and PDF job format

ADR 0007 kept `domain.PdfFormat` and `renderer/markdown/` so PDF jobs converted before the Python parser was removed would still render. Nothing uses them any more, so we removed both, along with the goldmark dependencies and the Markdown read path in `ReadDocument`.

## Consequences

- The reader only serves EPUB jobs. A job with any other `Format` gets a 422 "Unsupported document format" page instead of rendered content. Its row, highlights and exports stay in the DB untouched.
- Highlight block IDs are now only the EPUB shape `ch{N}-{tag}-{M}` ([ADR 0001](0001-highlight-anchoring-via-block-id.md)). Old Markdown `kind-N` IDs on PDF-job highlights are no longer rendered.
- The reader template's non-sidebar branch existed for single-file Markdown and had no chapter navigation. EPUBs whose TOC came back empty (no usable nav/NCX, an empty TOC, or no entry resolving to a spine item) landed there and were stuck on one chapter. That branch is gone: every reader page renders prev/next controls and `data-current-chapter`, and only the TOC sidebar is omitted when the TOC is empty.
