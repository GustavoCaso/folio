# Native PDF pipeline (pdfium, Go-native) — design

Based on findings in `scripts/pdf-extract-test/FINDINGS.md`.

## Goal

Add a second PDF conversion pipeline, selectable via ENV var, that parses
PDF in-process (Go, pdfium WASM) instead of round-tripping to the Python
Docling gRPC service. Output uses the same on-disk shape as epub jobs
(chapter-N.html + toc.json) so the existing epub reader path is reused
as-is.

## Pipeline selection

- New ENV var `PDF_PIPELINE` (`docling` default | `native`).
- Read once at server wiring time (`cmd/`), decides which `parser.Parser`
  implementation is registered under the `"pdf"` slot in
  `converter.Runner`'s `map[string]parser.Parser`.
- No per-job override, no dual-registration — matches existing
  format-keyed dispatch (ADR 0002).

## New Job.Format value

- `"pdf-native"` — set by `converter.Runner` at job creation when the
  uploaded file is `.pdf` and `PDF_PIPELINE=native`. `.pdf` under
  `PDF_PIPELINE=docling` still gets `Format="pdf"` (existing behavior,
  unchanged).
- Reader/handler dispatch on `Job.Format`: `"pdf-native"` routes to the
  epub-shaped chapter+toc reader path instead of the single-markdown-file
  path used by `"pdf"`.

## nativePDFParser (implements parser.Parser)

Library: `github.com/klippa-app/go-pdfium`, WASM mode (no cgo, no native
pdfium install — validated in FINDINGS.md).

`Convert`:

1. Load PDF bytes into a pdfium WASM instance.
2. `GetBookmarks()` → nested TOC tree. No bookmarks → single synthetic
   top-level entry spanning the whole document (page 1..N), mirroring
   epub's fallback-entry behavior for spine items with no TOC coverage.
3. Resolve each bookmark node to a page range (its start page through the
   page before its next sibling/parent boundary) — this range is the
   chapter unit, playing the role epub's spine index plays.
4. Per chapter, walk pages and extract text rects, classify each into
   CODE / heading tier / PROSE:
   - Mono font-name substring match (`mono|courier|consolas|menlo|
     sourcecodepro`, case-insensitive) → CODE. Contiguous CODE rects
     merge into one block; a run promotes to a real `<pre><code>` block
     only if ≥2 consecutive lines, otherwise renders as an inline
     `<code>` span in the surrounding prose (fixes FINDINGS.md gap #2).
   - Rects with missing font info inherit the previous rect's
     classification instead of defaulting to PROSE (fixes FINDINGS.md
     gap #1).
   - Known heading font names (book-specific: `FuturaStd-CondensedBold`
     = chapter number, `DogmaOT-Bold` = chapter title, `FuturaStd-Bold` =
     section heading) map to h1/h2/h3. Font-size clustering is NOT used
     for heading tiers — `FontInformation.Size` returns 1.0 for every
     rect in pdfium's WASM mode (unfixed unit/scale bug per FINDINGS.md);
     name-based mapping is brittle to other publishers' font sets but
     ships today without that dependency.
   - Everything else → PROSE paragraph.
   Contiguous same-classification rects merge into blocks; render as
   block-level HTML tags with `data-block-id="ch{N}-{tag}-{M}"`, same
   anchoring scheme as `renderer/epub` (ADR 0001) — no new highlight
   storage logic needed.
5. Per page in the chapter: `FPDFPage_CountObjects` → filter objects of
   type `FPDF_PAGEOBJ_IMAGE` → `FPDFImageObj_GetRenderedBitmap` → BGRA→RGBA
   channel swap → PNG-encode → embed as base64 `<img>` inline in the block
   flow at that position (matches epub's inline base64 image handling).
   Per-object raster extraction only — vector-drawn illustrations (path
   objects) are out of scope, per FINDINGS.md's noted gap.
6. Write `chapter-N.html` per chapter + `toc.json` (same `tocEntry` shape
   as `parser/epub.go`) to `dataDir/jobID`.
7. Title/Author from pdfium doc metadata. Cover = first raster image
   found on page 1, else nil (no full-page-render fallback).
8. `store.MarkJobDone(...)`, hub publish `DONE` — same tail as
   `epubParser.Convert`.

`ConvertFromURL`: same as `epubParser` — not supported, PDFs in this
pipeline are always uploaded. (Existing docling `"pdf"` pipeline keeps its
own `ConvertFromURL` for URL imports; native pipeline does not need to
match that capability.)

Error handling mirrors `epubParser.fail`: any pdfium load/parse error,
page-render error, or disk-write error routes through `MarkJobFailed` +
hub `FAILED` publish.

## Testing

Mirror `epub_test.go` structure:
- Classifier unit tests: mono-font detection, ≥2-line promotion rule,
  inherit-classification-on-missing-font-info.
- Bookmark → page-range resolution unit tests, including the
  no-bookmarks fallback.
- BGRA→RGBA conversion unit test.
- Open item for implementation phase: need a small PDF fixture (real or
  synthetic) checked into the test dir, equivalent to whatever small
  epub fixture `epub_test.go` uses today.

## Out of scope

- Vector-illustration extraction (bounding-box clustering of path
  objects) — noted as unsolved in FINDINGS.md, not attempted here.
- Ligature-decode fixes — not applicable, pdfium doesn't exhibit the bug
  ledongthuc has.
- Per-job pipeline override / UI toggle — ENV var only.
