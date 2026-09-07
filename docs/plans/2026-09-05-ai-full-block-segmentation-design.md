# AI full-block segmentation for PDF native cleanup

## Problem

Today the PDF native pipeline (`ui/internal/converter/parser/pdf_native.go`) does:

1. `pageTextRects` — extract raw rects (text, font name, top/bottom position) via `GetPageTextStructured`.
2. `classifyBlocks(rects)` (`pdf_native_classify.go`) — deterministic heuristic: matches font name against a mono-font needle list and a heading-font-tier map, and detects paragraph breaks via a vertical-gap multiplier. Produces `[]block{Kind, Text}`.
3. `aiCleaner.Clean(ctx, blocks)` (`ai_cleanup.go`) — optional small-model pass that only disambiguates code-vs-prose and fixes wrongly-split blocks, working from the classifier's *already-collapsed* block text plus its guessed label. It never sees raw rects, font size, or weight, and never touches headings (h1/h2/h3 pass through untouched).
4. `renderChapterHTML(blocks)` — maps `Kind` to an HTML tag (p/pre/h1/h2/h3).

The heuristic in step 2 is brittle (hardcoded font-name lists, magic gap multiplier) and the AI pass in step 3 is boxed into fixing only what the heuristic got wrong, blind to the actual layout data.

## Change

Replace steps 2+3 with a single AI pass that receives the raw rects (extended with font size and weight) and returns fully segmented, classified blocks directly in the shape `renderChapterHTML` already expects.

### Rect payload sent to the model

`textRect` is unchanged (`Text`, `FontName`, `Top`, `Bottom`) — no font size/weight. `pdf_native_classify.go`'s existing comment documents that pdfium's WASM build returns `FontInformation.Size == 1.0` for every rect (unresolved bug), and `Weight`/size both come from pdfium's "experimental support" which the WASM build doesn't populate reliably. Sending constant/unreliable numbers to the model would be noise, not signal, so the model gets font name + vertical position only — the same data the current heuristic already trusts.

### New response schema

```json
{
  "type": "object",
  "properties": {
    "blocks": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "kind": {"type": "string", "enum": ["prose", "code", "h1", "h2", "h3"]},
          "text": {"type": "string"}
        },
        "required": ["kind", "text"]
      }
    }
  },
  "required": ["blocks"]
}
```

This matches `block{Kind, Text}` directly — `applyAIFix`'s merge-by-index reconciliation goes away; the model's block list *is* the output.

### New prompt

Explains: each numbered entry is one raw line/rect of PDF-extracted text with its font name, font size, weight, and vertical position. Model must reconstruct the page's logical structure: join wrapped lines into paragraphs, detect headings by relative font size/weight, detect code listings (monospace font and/or indentation), and emit the block list per the schema. No more "classifier said" framing — there is no upstream classifier anymore.

### Pipeline wiring

`renderChapter` (`pdf_native.go:163`) calls `p.ai.Clean(ctx, rects)` directly; `classifyBlocks` and the old `aiFixedBlock`/`aiFixResult`/`applyAIFix` machinery are deleted. `AIConfig.Enabled` is removed — cleanup is mandatory for the PDF native path, no raw/heuristic fallback branch remains in `renderChapter`.

Timeout and max-token scaling (`TimeoutPerBlock`, `maxTokensPerBlock`) keep their existing formulas but now scale off rect count instead of pre-collapsed block count — likely needs empirical re-tuning since rect count is higher, but that's a follow-up tuning pass, not a design change.

### scripts/model-test

Trimmed to extract rects and print them in the new prompt format (no local `classifyBlocks` call) so it keeps mirroring the real pipeline for manual prompt/model testing.

## Out of scope

- Re-tuning timeout/token constants for the new (larger) input size — do after real-world testing.
- Any change to EPUB path or non-PDF-native parsers.
