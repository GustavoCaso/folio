# AI page classification (one-shot per-page LLM formatting)

Supersedes the current two-step AI cleanup path (`classifyBlocks` deterministic
pass → `aiCleaner.Clean` merge/reclassify patch) with a single LLM call per
page that owns the entire prose/code/heading classification and block
grouping directly, when `AI_PIPELINE_ENABLED=true`.

## Motivation

The current pipeline runs `classifyBlocks` (deterministic, font-name based)
first, then sends its already-merged blocks to Kronk asking it to *patch*
mistakes (merge wrongly-split runs, reclassify). This works but:

- Still depends on `classifyBlocks`'s heuristics being right enough to produce
  a reasonable starting point (inline-code-vs-block promotion rule, paragraph
  break detection, etc.) — the LLM can only patch boundaries the deterministic
  pass already drew.
- Two-stage design (classify, then clean) for a problem the LLM could solve
  in one pass given the same raw signal (font names) deterministic
  classification uses.

## Non-AI path: unchanged

`PDF_PIPELINE=native` with `AI_PIPELINE_ENABLED=false` (default) is untouched:
`pageTextRects` (`GetPageTextStructuredModeRects`) → `classifyBlocks` →
`renderChapterHTML`, exactly as today.

## AI path: pdfium extraction layer

New function alongside `pageTextRects`, used only when `p.ai != nil`:

```go
func pageTextLines(instance pdfium.Pdfium, pageRef requests.Page) ([]aiLine, error)
```

Calls `GetPageTextStructured` with `Mode: GetPageTextStructuredModeBoth`,
`CollectFontInformation: true` — fetching both `Rects` (unused here) and
`Chars` (new). `groupCharsIntoLines` (ported from
`scripts/pdf-extract-test/pdfium_pipeline/extract.go`, adapted for font
attribution) buckets chars by `PointPosition.Bottom` with `lineYTolerance =
5.0` (same tuning validated in FINDINGS.md against real book text), sorts
each bucket left-to-right by `Left`, concatenates text, and takes the font
name from the first char in the bucket carrying non-nil `FontInformation`
(mirrors the "missing font metadata inherits" reasoning already in
`classifyRect`).

```go
type aiLine struct {
	Text     string
	FontName string
}
```

This is a **second, AI-only extraction path** — non-AI pages never pay for
`ModeBoth`/char-level work.

## AI path: prompt, schema, classification call

Replaces `Clean`/`applyAIFix`/`aiFixResult`/`aiFixedBlock` entirely.

```go
func (c *aiCleaner) ClassifyPage(ctx context.Context, lines []aiLine) ([]block, error)
```

Schema:

```go
{
  "blocks": [
    {"kind": "prose"|"code"|"h1"|"h2"|"h3", "text": "..."}
  ]
}
```

One entry per **final logical block** in reading order — the model performs
grouping, boundary decisions, and classification in a single pass, echoing
back each block's full text (not indices into the input). Prompt lists every
line as `[i] (font: X)\ntext`, instructs the model to:

- Use font name as the primary signal, same rules as `monoFontNeedles`
  (contains "mono"/"courier"/"consolas"/"menlo"/"sourcecodepro" → code) and
  `headingFontTiers` (exact font name match → h1/h2/h3).
- Group consecutive same-kind lines into one block; a run of monospace lines
  is one code block.
- A single monospace line surrounded by prose (inline code mention) stays
  part of the surrounding prose block, not its own code block — same
  promotion rule `classifyBlocks` enforces today (FINDINGS.md gap #2).
- Lines with missing/empty font name inherit the surrounding block's kind
  rather than defaulting to prose (the original motivating bug from
  FINDINGS.md).

Result type:

```go
type aiPageBlock struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}
type aiPageResult struct {
	Blocks []aiPageBlock `json:"blocks"`
}
```

`kindFromLabel(string) (blockKind, bool)` maps `"prose"|"code"|"h1"|"h2"|"h3"`
to `blockKind`; unknown/empty label defaults to `blockProse` (fail-safe, same
spirit as today's classifier default). `ClassifyPage` maps `aiPageResult`
directly to `[]block{Kind, Text}` — no merge post-processing needed, since
the model already emits final blocks.

Timeout/token-budget scaling moves from block-count to **line-count** (input
size is now known upfront; output block count isn't, until the model
responds): `TimeoutBase + max(0, len(lines)-timeoutFreeLines) *
TimeoutPerLine`, and `maxTokens = max(minMaxTokens, maxTokensPerLine *
len(lines))`. `AIConfig` field names change from `*PerBlock` to `*PerLine` to
match (env var names change too — see below).

## Wiring in `pdf_native.go`

`renderChapter`, when `p.ai != nil`:

1. `pageTextLines(instance, pageRef)` (new `ModeBoth` path).
2. `p.ai.ClassifyPage(ctx, lines)`.
3. On success: use returned blocks directly.
4. On error/timeout: log the existing "ai cleanup failed, falling back to raw
   blocks" warning, then fall back to `pageTextRects` (`ModeRects`) +
   `classifyBlocks` — the existing non-AI path, re-fetched via the other
   pdfium mode. This means a second pdfium call only on the fallback branch,
   never on the happy path.

## Config changes

| Old | New | Notes |
|---|---|---|
| `AI_PIPELINE_TIMEOUT_PER_BLOCK` | `AI_PIPELINE_TIMEOUT_PER_LINE` | scaling now keyed on input line count |
| (new) | `AI_PIPELINE_TIMEOUT_FREE_LINES` | optional; default mirrors old `timeoutFreeBlocks = 10`, may need retuning since line count on a page is typically higher than block count |

`AI_PIPELINE_ENABLED`, `AI_PIPELINE_MODEL`, `AI_PIPELINE_TIMEOUT_BASE`,
`AI_PIPELINE_KRONK_LIB_PATH` unchanged. Docs (`ui/CLAUDE.md`, root `README.md`)
updated to match.

## Removed

- `aiFixedBlock`, `aiFixResult`, `applyAIFix`, `Clean`, `aiCleanupSchema`,
  `aiCleanupPromptPreamble`, `blockKindLabel` (superseded by
  `kindFromLabel`/an updated label helper).
- Corresponding tests: `TestApplyAIFix_*`, `TestAICleaner_Clean_*` (from
  `ai_cleanup_test.go`).

## Testing

- `groupCharsIntoLines`: unit tests independent of any LLM call — bucket-by-
  `Bottom`, tolerance boundary behavior, font-from-first-char-with-info,
  empty input.
- `ClassifyPage`: same `fakeChatter` double pattern as today —
  schema round-trip (mixed code/prose/heading), missing-font line grouped
  correctly by context, single monospace line inside prose classified as
  prose, malformed JSON / chat error surfaced to caller.
- `renderChapter` fallback: AI error still produces valid HTML via
  `classifyBlocks`, matching today's fallback test coverage.
- Full existing native-PDF conversion tests (`pdf_native_convert_test.go`)
  continue to pass with `AIConfig{}` (disabled), unchanged.

## Known risk (accepted)

The model echoes block text back rather than the pipeline reassembling it
from indexed input lines. A small model (Qwen3-4B) could in principle
paraphrase or drop characters from code while echoing it, which the old
index-based `merge_with_previous` design avoided entirely. Accepted as a
tradeoff for a simpler one-shot contract; revisit if garbled code output is
observed in practice.
