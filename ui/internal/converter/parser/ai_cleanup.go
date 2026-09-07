package parser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ardanlabs/kronk/sdk/kronk"
	"github.com/ardanlabs/kronk/sdk/kronk/model"
	"github.com/ardanlabs/kronk/sdk/tools/libs"
	"github.com/ardanlabs/kronk/sdk/tools/models"
)

// AIConfig configures the Kronk-backed page-segmentation cleanup pass. See
// ui/CLAUDE.md's Configuration table for the env vars that populate this
// from AIConfigFromEnv.
type AIConfig struct {
	// Model is the Kronk model source string (e.g. "unsloth/Qwen3-4B-Q8_0"),
	// downloaded on first use via github.com/ardanlabs/kronk/sdk/tools/models.
	Model string
	// TimeoutBase is the minimum per-page cleanup call timeout.
	TimeoutBase time.Duration
	// TimeoutPerBlock is added to TimeoutBase for every rect beyond
	// timeoutFreeBlocks, since a real page can have dozens of rects and a
	// flat timeout starves dense pages — see FINDINGS.md.
	TimeoutPerBlock time.Duration
	// KronkLibPath points Kronk at a pre-assembled llama.cpp/ggml library
	// directory (see FINDINGS.md's brew-install workaround for Kronk's own
	// broken macOS arm64 downloader). Empty uses Kronk's own downloader.
	KronkLibPath string
	// Temperature is the sampling temperature for the segmentation call.
	// Kronk exposes no repeat-penalty knob, so greedy decoding (0) is prone
	// to degenerate repetition loops that burn the max_tokens budget before
	// completing the JSON (see FINDINGS.md) -- keep this above 0.
	Temperature float64
	// RetryTemperatureBump is added to Temperature for a single retry
	// attempt when a chunk truncates before completing its JSON (see
	// truncationRetryTemperatureBump for how this default was tuned).
	RetryTemperatureBump float64
}

// timeoutFreeBlocks is the rect count under which TimeoutPerBlock does not
// add to TimeoutBase — mirrors the tuning validated in
// scripts/pdfium_pipeline/cleanup.go.
const timeoutFreeBlocks = 10

// chunkSize is the max rects sent to the model per call. A dense real page
// can produce a raw-rect prompt too large for a small model's context
// window (e.g. a 3B model errored on a 33K-token single-page prompt), so
// Clean splits a page's rects into fixed-size chunks and issues one call
// per chunk, concatenating the results in order. A chunk boundary can
// occasionally split a paragraph or code listing into two blocks instead
// of one -- an accepted tradeoff for this internal, non-interactive pass.
const chunkSize = 30

// maxTokensPerBlock and minMaxTokens mirror scripts/pdfium_pipeline/cleanup.go's
// scaling of output budget with rect count, avoiding silent JSON truncation
// on dense pages (see FINDINGS.md).
const (
	maxTokensPerBlock = 64
	minMaxTokens      = 2048
)

// AIConfigFromEnv reads AI_PIPELINE_* env vars into an AIConfig, applying
// defaults for anything unset or invalid.
func AIConfigFromEnv(getenv func(string) string) AIConfig {
	cfg := AIConfig{
		Model:                "unsloth/Qwen3-4B-Q8_0",
		TimeoutBase:          120 * time.Second,
		TimeoutPerBlock:      5 * time.Second,
		KronkLibPath:         getenv("AI_PIPELINE_KRONK_LIB_PATH"),
		Temperature:          0.2,
		RetryTemperatureBump: 0.4,
	}
	if v := getenv("AI_PIPELINE_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := getenv("AI_PIPELINE_TIMEOUT_BASE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.TimeoutBase = d
		}
	}
	if v := getenv("AI_PIPELINE_TIMEOUT_PER_BLOCK"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.TimeoutPerBlock = d
		}
	}
	if v := getenv("AI_PIPELINE_TEMPERATURE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Temperature = f
		}
	}
	if v := getenv("AI_PIPELINE_RETRY_TEMPERATURE_BUMP"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.RetryTemperatureBump = f
		}
	}
	return cfg
}

// AIClient is the subset of *kronk.Kronk this package needs, narrowed for
// testability. Exported so callers outside this package (e.g. external
// _test packages) can inject a fake via WithAIClient — see pdf_native.go.
type AIClient interface {
	ChatStreaming(ctx context.Context, d model.D) (<-chan model.ChatResponse, error)
}

// aiCleaner segments a page's raw text rects into rendered blocks (prose,
// code, or heading levels) by asking a small local LLM to reconstruct the
// page's logical structure from font-name and position signals — replacing
// the old deterministic classifyBlocks heuristic entirely.
type aiCleaner struct {
	cfg AIConfig

	initOnce sync.Once
	initErr  error
	client   AIClient
	unload   func(context.Context) error
}

// newAICleaner constructs an aiCleaner that lazily loads the Kronk model on
// first Clean call, shared across all subsequent calls (and jobs).
func newAICleaner(cfg AIConfig) *aiCleaner {
	return &aiCleaner{cfg: cfg}
}

// TextRect is the exported mirror of textRect, for external tooling (see
// ui/cmd/aicleanup-validate) that needs to drive the AI segmentation pass
// for a single page without running the full Convert pipeline.
type TextRect struct {
	Text     string
	FontName string
	Top      float64
	Bottom   float64
	Left     float64
	Right    float64
}

// CleanPageHTML runs the real AI segmentation pass on one page's rects and
// renders the result as chapter-0 HTML, loading its own Kronk model
// instance (not shared with any running server) -- for standalone
// validation tooling only, not used by the real pipeline.
func CleanPageHTML(ctx context.Context, cfg AIConfig, rects []TextRect) (string, error) {
	internalRects := make([]textRect, len(rects))
	for i, r := range rects {
		internalRects[i] = textRect(r)
	}
	internalRects = mergeSameLineRects(internalRects)
	c := newAICleaner(cfg)
	blocks, err := c.Clean(ctx, internalRects)
	if err != nil {
		return "", err
	}
	return renderChapterHTML(0, blocks), nil
}

func (c *aiCleaner) ensureLoaded(ctx context.Context) error {
	c.initOnce.Do(func() {
		if c.cfg.KronkLibPath != "" {
			// Pre-assembled library directory (e.g. the macOS brew-install
			// workaround in FINDINGS.md) — load directly, no download.
			if err := kronk.Init(kronk.WithLibPath(c.cfg.KronkLibPath)); err != nil {
				c.initErr = fmt.Errorf("kronk init: %w", err)
				return
			}
		} else {
			// No override: detect the host triple and download Kronk's
			// precompiled llama.cpp/ggml libraries into its default cache
			// (see github.com/ardanlabs/kronk/sdk/tools/libs), mirroring
			// the sequence in kronk's own pool_test.go — Init() alone does
			// not fetch libraries, it only loads what's already on disk.
			libsDownloadCtx, libsCancel := context.WithTimeout(ctx, 15*time.Minute)
			lb, err := libs.New(libs.WithDetect(libsDownloadCtx, kronk.FmtLogger))
			if err != nil {
				libsCancel()
				c.initErr = fmt.Errorf("kronk libs init: %w", err)
				return
			}
			if _, err := lb.Download(libsDownloadCtx, kronk.FmtLogger); err != nil {
				libsCancel()
				c.initErr = fmt.Errorf("download kronk libs: %w", err)
				return
			}
			libsCancel()

			if err := kronk.Init(); err != nil {
				c.initErr = fmt.Errorf("kronk init: %w", err)
				return
			}
		}

		mdls, err := models.New()
		if err != nil {
			c.initErr = fmt.Errorf("kronk models init: %w", err)
			return
		}

		downloadCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
		defer cancel()

		mp, err := mdls.Download(downloadCtx, kronk.FmtLogger, c.cfg.Model)
		if err != nil {
			c.initErr = fmt.Errorf("download model %q: %w", c.cfg.Model, err)
			return
		}

		krn, err := kronk.New(model.WithModelFiles(mp.ModelFiles), model.WithAutoTune(true))
		if err != nil {
			c.initErr = fmt.Errorf("create kronk model: %w", err)
			return
		}
		c.client = krn
		c.unload = krn.Unload
		slog.Default().Info("ai cleanup model loaded", "model", c.cfg.Model, "n_seq_max", krn.ModelConfig().NSeqMax())
	})
	return c.initErr
}

// Close releases the loaded model, if any was loaded.
func (c *aiCleaner) Close(ctx context.Context) error {
	if c.unload != nil {
		return c.unload(ctx)
	}
	return nil
}

// aiSegmentedBlock is one block the model reconstructed from raw rects.
type aiSegmentedBlock struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

type aiSegmentResult struct {
	Blocks []aiSegmentedBlock `json:"blocks"`
}

var aiSegmentSchema = model.D{
	"type": "object",
	"properties": model.D{
		"blocks": model.D{
			"type": "array",
			"items": model.D{
				"type": "object",
				"properties": model.D{
					"kind": model.D{"type": "string", "enum": []string{"prose", "code", "h1", "h2", "h3"}},
					"text": model.D{"type": "string"},
				},
				"required": []string{"kind", "text"},
			},
		},
	},
	"required": []string{"blocks"},
}

const aiSegmentPromptPreamble = "You are reconstructing the logical structure of a PDF page from its raw " +
	"extracted text.\n\n" +
	"Each numbered entry below is one line/run of text as pdfium extracted it, " +
	"in reading order, along with its font name and vertical position on the " +
	"page (Top/Bottom, in points, higher Top = closer to top of page). PDF text " +
	"extraction gives you individual lines, not paragraphs -- wrapped lines of " +
	"the same paragraph appear as separate consecutive entries with similar " +
	"font and only a small vertical gap between them.\n\n" +
	"Your job: group these entries into the page's real logical blocks and " +
	"classify each one as one of:\n" +
	"- \"h1\", \"h2\", \"h3\": headings, in decreasing size/prominence order. " +
	"Headings are usually short, visually distinct (different/bolder font than " +
	"body text), and stand alone on their own line(s).\n" +
	"- \"code\": a source code listing. Code lines are typically in a monospace " +
	"font and span two or more consecutive lines. A single short line in a " +
	"monospace-looking font surrounded by prose (e.g. an inline `variable` " +
	"mention) is NOT a code block -- classify it as prose.\n" +
	"- \"prose\": ordinary paragraph text.\n\n" +
	"Merge wrapped lines belonging to the same paragraph or the same code " +
	"listing into a single block's text (joined with a single space for prose, " +
	"or a newline for code, preserving reading order). Start a new block " +
	"whenever the font changes in a way that signals a real structural " +
	"boundary (heading start/end, code start/end) or when the vertical gap " +
	"between two prose lines is noticeably larger than the normal line-to-line " +
	"gap (a paragraph break).\n\n" +
	"The text extraction sometimes duplicates a short letter run right where " +
	"it occurs, producing a typo like \"Proj oj ect\" or \"defafaults\" (the " +
	"fragment \"oj\"/\"fa\" is repeated). When you recognize this pattern in a " +
	"real word, silently repair it to the correct spelling in your output " +
	"instead of reproducing the duplication -- but do not otherwise alter " +
	"wording, and do not \"fix\" things that are not this specific glitch.\n\n" +
	"Entries:\n"

// blockKindFromLabel maps the model's kind string to a blockKind, defaulting
// unknown/malformed values to prose so a bad model response degrades to
// plain text instead of dropping the block.
func blockKindFromLabel(label string) blockKind {
	switch label {
	case "code":
		return blockCode
	case "h1":
		return blockH1
	case "h2":
		return blockH2
	case "h3":
		return blockH3
	default:
		return blockProse
	}
}

// stripJSONFence removes a leading/trailing ```json or ``` code fence, and
// trims surrounding whitespace, in case the model wraps its JSON response in
// markdown despite being told not to.
func stripJSONFence(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// validJSONEscapes are the single-character escapes JSON strings allow after
// a backslash; \u is handled separately since it takes 4 more hex digits.
var validJSONEscapes = map[byte]bool{
	'"': true, '\\': true, '/': true,
	'b': true, 'f': true, 'n': true, 'r': true, 't': true,
}

// escapeInvalidJSONBackslashes walks s outside of any already-valid escape
// sequence and doubles up any backslash that isn't starting a valid JSON
// escape, so a stray `\o` or `\x` (unconstrained models emit these -- see
// FINDINGS.md) becomes a literal backslash instead of failing json.Unmarshal
// with "invalid escape sequence". Only walks inside string literals is not
// attempted; this is a best-effort repair over the whole raw text.
func escapeInvalidJSONBackslashes(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i == len(s)-1 {
			sb.WriteByte(c)
			continue
		}
		next := s[i+1]
		if next == 'u' && i+5 < len(s) {
			sb.WriteByte(c)
			sb.WriteByte(next)
			sb.WriteString(s[i+2 : i+6])
			i += 5
			continue
		}
		if validJSONEscapes[next] {
			sb.WriteByte(c)
			sb.WriteByte(next)
			i++
			continue
		}
		sb.WriteString(`\\`)
	}
	return sb.String()
}

// Clean sends one page's raw text rects to the model, chunking them into
// groups of at most chunkSize so a dense page's prompt doesn't overrun a
// small model's context window, and returns the fully segmented, classified
// block list (chunk results concatenated in order) ready for
// renderChapterHTML. On any error (including timeout) the caller should
// surface it -- there is no deterministic fallback path.
func (c *aiCleaner) Clean(ctx context.Context, rects []textRect) ([]block, error) {
	if len(rects) == 0 {
		return nil, nil
	}
	if err := c.ensureLoaded(ctx); err != nil {
		return nil, err
	}

	var blocks []block
	for start := 0; start < len(rects); start += chunkSize {
		end := min(start+chunkSize, len(rects))
		chunkBlocks, err := c.cleanChunk(ctx, rects[start:end])
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, chunkBlocks...)
	}
	return blocks, nil
}

// maxTemperature caps the retry bump so a caller-configured temperature
// already near 1.0 doesn't get pushed into invalid/nonsensical territory.
const maxTemperature = 1.0

// cleanChunk sends one chunk of a page's raw text rects to the model and
// returns the segmented, classified blocks for that chunk alone. If the
// model truncates the response (degenerate repetition burning the
// max_tokens budget before valid JSON completes), it retries once with
// Temperature raised by RetryTemperatureBump before giving up -- a
// degenerate repeating generation (see FINDINGS.md) is more likely under
// low-temperature/greedy-leaning sampling, so a one-off bump gives the
// retry a real chance to escape the same loop instead of repeating it
// deterministically. A +0.2 bump was tried first and was not enough on a
// real repeat-loop (temperature 0.2 -> 0.4 both truncated on the same
// page); +0.4 (landing at 0.6 from the 0.2 default) was confirmed to
// escape it in a real test against ui/cmd/aicleanup-validate.
//
// The retry also fires on a plain JSON unmarshal failure (errMalformed),
// not just detected truncation (errTruncated) -- a real production error
// showed grammar-constrained sampling still occasionally producing
// structurally broken JSON (a stray tab and truncated key) without ever
// reporting FinishReasonLength, on a chunk of short/ambiguous prose
// fragments. This was confirmed intermittent, not reliably reproducible
// (3 of 3 follow-up attempts against the same page succeeded), consistent
// with real sampling randomness landing in a bad state occasionally rather
// than a deterministic bug in that chunk's content.
func (c *aiCleaner) cleanChunk(ctx context.Context, rects []textRect) ([]block, error) {
	blocks, err := c.cleanChunkAttempt(ctx, rects, c.cfg.Temperature)
	if !errors.Is(err, errTruncated) && !errors.Is(err, errMalformed) {
		return blocks, err
	}
	retryTemp := min(c.cfg.Temperature+c.cfg.RetryTemperatureBump, maxTemperature)
	return c.cleanChunkAttempt(ctx, rects, retryTemp)
}

// errTruncated marks a cleanChunkAttempt failure caused by the model
// hitting max_tokens before completing its JSON, so cleanChunk can decide
// to retry without string-matching the error message.
var errTruncated = errors.New("model output truncated before completing JSON")

// errMalformed marks a cleanChunkAttempt failure caused by the model's
// output failing to unmarshal as JSON despite finishing normally (not
// truncated) -- a rarer, intermittent sampling failure distinct from a
// truncation loop, but retried the same way since a resample is cheap and
// often succeeds (see cleanChunk).
var errMalformed = errors.New("model output failed to unmarshal as JSON")

// cleanChunkAttempt makes one model call for a chunk at the given
// temperature and parses the result. Errors wrapping errTruncated are
// retryable by the caller.
func (c *aiCleaner) cleanChunkAttempt(ctx context.Context, rects []textRect, temperature float64) ([]block, error) {
	timeout := c.cfg.TimeoutBase
	if extra := len(rects) - timeoutFreeBlocks; extra > 0 {
		timeout += time.Duration(extra) * c.cfg.TimeoutPerBlock
	}
	maxTokens := maxTokensPerBlock * len(rects)
	if maxTokens < minMaxTokens {
		maxTokens = minMaxTokens
	}

	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var sb strings.Builder
	for i, r := range rects {
		fmt.Fprintf(&sb, "[%d] (font: %s, top: %.2f, bottom: %.2f)\n%s\n\n", i, r.FontName, r.Top, r.Bottom, r.Text)
	}

	d := model.D{
		// aiSegmentPromptPreamble is byte-identical across every call, in
		// its own message so Kronk's incremental message cache (enabled by
		// default, see model.WithIncrementalCache) can match this exact
		// prefix and skip re-prefilling it, instead of only the rect data
		// changing within one concatenated user message.
		"messages": model.DocumentArray(
			model.TextMessage(model.RoleSystem, aiSegmentPromptPreamble),
			model.TextMessage(model.RoleUser, sb.String()),
		),
		"json_schema":     aiSegmentSchema,
		"enable_thinking": false,
		"temperature":     temperature,
		"max_tokens":      maxTokens,
	}

	ch, err := c.client.ChatStreaming(callCtx, d)
	if err != nil {
		return nil, fmt.Errorf("chat streaming: %w", err)
	}

	var out string
	var truncated bool
	for resp := range ch {
		if len(resp.Choices) == 0 {
			continue
		}
		if resp.Choices[0].FinishReason() == model.FinishReasonError {
			return nil, fmt.Errorf("error from model: %s", resp.Choices[0].Delta.Content)
		}
		// The terminal delta (whichever FinishReason it carries) can still
		// include trailing content -- e.g. FinishReasonStop's final chunk
		// commonly bundles the last bit of text alongside the stop signal.
		// Appending unconditionally, instead of only in a default case that
		// excluded FinishReasonLength/FinishReasonStop, previously dropped
		// that content both from a truncation error's reported raw output
		// and from otherwise-successful responses.
		if resp.Choices[0].Delta != nil {
			out += resp.Choices[0].Delta.Content
		}
		if resp.Choices[0].FinishReason() == model.FinishReasonLength {
			truncated = true
		}
	}
	if truncated {
		return nil, fmt.Errorf("%w: max_tokens (%d), temperature %.2f; raw output %q", errTruncated, maxTokens, temperature, out)
	}

	out = stripJSONFence(out)
	out = escapeInvalidJSONBackslashes(out)

	var result aiSegmentResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, fmt.Errorf("%w: unmarshal model output %q: %v", errMalformed, out, err)
	}

	blocks := make([]block, 0, len(result.Blocks))
	for _, b := range result.Blocks {
		blocks = append(blocks, block{Kind: blockKindFromLabel(b.Kind), Text: b.Text})
	}
	return blocks, nil
}
