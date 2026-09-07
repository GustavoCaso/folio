package parser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/GustavoCaso/folio/ui/internal/hub"
	"github.com/GustavoCaso/folio/ui/internal/logging"
	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/responses"
	"github.com/klippa-app/go-pdfium/webassembly"
)

// nativePDFPool is the subset of the pdfium WASM pool this package needs —
// matches webassembly.Pool, narrowed for testability.
type nativePDFPool interface {
	GetInstance(timeout time.Duration) (pdfium.Pdfium, error)
	Close() error
}

// nativePDFParser converts PDF bytes in-process via pdfium (WASM mode,
// github.com/klippa-app/go-pdfium), writing one chapter-N.html per top-level
// bookmark plus toc.json, in the same on-disk shape epubParser produces —
// see docs/plans/2026-08-11-pdf-native-pipeline-design.md.
type nativePDFParser struct {
	store   Store
	hub     *hub.Hub
	dataDir string
	logger  *slog.Logger
	pool    nativePDFPool
	ai      *aiCleaner
}

// NativePDFOption configures optional behavior on a nativePDFParser,
// primarily for test injection.
type NativePDFOption func(*nativePDFParser)

// WithAIClient overrides the AI cleaner's chat client, bypassing the lazy
// Kronk model load. For tests only.
func WithAIClient(client AIClient) NativePDFOption {
	return func(p *nativePDFParser) {
		p.ai.client = client
		p.ai.initOnce.Do(func() {}) // mark loaded, skip ensureLoaded's real download
	}
}

// NewNativePDF constructs the pdfium-based PDF Parser. h may be nil, in
// which case status events are not published (Store is still updated).
// Every page's raw text rects are segmented and classified by a mandatory
// Kronk-backed AI pass (lazily loading the model on first job) — see
// ai_cleanup.go and ui/CLAUDE.md's Configuration table.
func NewNativePDF(store Store, h *hub.Hub, dataDir string, aiCfg AIConfig, opts ...NativePDFOption) (Parser, error) {
	pool, err := webassembly.Init(webassembly.Config{MinIdle: 1, MaxIdle: 1, MaxTotal: 1})
	if err != nil {
		return nil, fmt.Errorf("init pdfium pool: %w", err)
	}
	p := &nativePDFParser{store: store, hub: h, dataDir: dataDir, logger: slog.Default(), pool: pool}
	p.ai = newAICleaner(aiCfg)
	for _, opt := range opts {
		opt(p)
	}
	return p, nil
}

func (p *nativePDFParser) Convert(ctx context.Context, jobID, requestID, filename string, data []byte, h *hub.Hub) error {
	log := p.logger.With("job_id", jobID)
	start := time.Now()
	log.Info("native pdf conversion start", "bytes", len(data))

	instance, err := p.pool.GetInstance(time.Second * 30)
	if err != nil {
		return p.fail(log, jobID, fmt.Sprintf("get pdfium instance: %v", err))
	}
	defer func() {
		if closeErr := instance.Close(); closeErr != nil {
			log.Warn("close pdfium instance failed", logging.Err(closeErr))
		}
	}()

	doc, err := instance.OpenDocument(&requests.OpenDocument{File: &data})
	if err != nil {
		return p.fail(log, jobID, fmt.Sprintf("open pdf: %v", err))
	}
	defer func() {
		if _, closeErr := instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document}); closeErr != nil {
			log.Warn("close pdf document failed", logging.Err(closeErr))
		}
	}()

	pageCountResp, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: doc.Document})
	if err != nil {
		return p.fail(log, jobID, fmt.Sprintf("get page count: %v", err))
	}

	bookmarksResp, err := instance.GetBookmarks(&requests.GetBookmarks{Document: doc.Document})
	if err != nil {
		return p.fail(log, jobID, fmt.Sprintf("get bookmarks: %v", err))
	}

	chapters, tocEntries := resolveChapters(convertBookmarks(bookmarksResp.Bookmarks), pageCountResp.PageCount)

	outDir := filepath.Join(p.dataDir, jobID)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return p.fail(log, jobID, fmt.Sprintf("create output dir: %v", err))
	}

	for i, ch := range chapters {
		chapterHTML, err := p.renderChapter(ctx, log, instance, doc.Document, ch, i, jobID, outDir, pageCountResp.PageCount)
		if err != nil {
			return p.fail(log, jobID, fmt.Sprintf("render chapter %d: %v", i, err))
		}
		if err := os.WriteFile(filepath.Join(outDir, fmt.Sprintf("chapter-%d.html", i)), []byte(chapterHTML), 0o644); err != nil {
			return p.fail(log, jobID, fmt.Sprintf("write chapter %d: %v", i, err))
		}
	}
	removePageFragments(log, outDir, pageCountResp.PageCount)

	tocBytes, err := json.Marshal(tocEntries)
	if err != nil {
		return p.fail(log, jobID, fmt.Sprintf("marshal toc: %v", err))
	}
	if err := os.WriteFile(filepath.Join(outDir, "toc.json"), tocBytes, 0o644); err != nil {
		return p.fail(log, jobID, fmt.Sprintf("write toc: %v", err))
	}

	title, author := documentMetadata(instance, doc.Document)
	cover := firstPageCover(instance, doc.Document)

	if err := p.store.MarkJobDone(ctx, jobID, outDir, title, author, cover); err != nil {
		log.Error("mark job done failed", logging.Err(err))
		return err
	}

	log.Info("native pdf conversion done",
		"output_path", outDir,
		"chapters", len(chapters),
		"dur_ms", time.Since(start).Milliseconds(),
	)
	if p.hub != nil {
		p.hub.Publish(jobID, hub.StatusEvent{Status: "DONE", Title: title, Author: author})
	}
	return nil
}

// ConvertFromURL is not supported for native-pdf jobs — sources are always
// uploaded, mirroring epubParser.
func (p *nativePDFParser) ConvertFromURL(ctx context.Context, jobID, requestID, sourceURL string, h *hub.Hub) error {
	return errors.New("pdf-native: import from URL is not supported")
}

func (p *nativePDFParser) fail(log *slog.Logger, jobID, msg string) error {
	log.Error("native pdf conversion failed", "error", msg)
	if err := p.store.MarkJobFailed(context.Background(), jobID, msg); err != nil {
		log.Error("mark job failed errored", logging.Err(err))
	}
	if p.hub != nil {
		p.hub.Publish(jobID, hub.StatusEvent{Status: "FAILED", Error: msg})
	}
	return errors.New(msg)
}

// renderChapter walks every page in ch's range, sends its raw text rects
// through AI segmentation/classification, inserts embedded raster images
// inline at their page position, and renders the result as HTML with
// data-block-id anchoring. A cleanup error or timeout fails the chapter.
// After each page, a PROCESSING status event reports progress across the
// whole document (page is already a document-global 0-based index).
//
// Each page's rendered fragment is checkpointed to outDir (see
// pageFragmentPath) as soon as it's produced, and reused instead of
// recomputed on a retry -- the AI cleanup pass is the slowest and most
// failure-prone step.
func (p *nativePDFParser) renderChapter(ctx context.Context, log *slog.Logger, instance pdfium.Pdfium, doc references.FPDF_DOCUMENT, ch chapterRange, chapterIdx int, jobID, outDir string, totalPages int) (string, error) {
	var out strings.Builder
	for page := ch.StartPage; page <= ch.EndPage; page++ {
		fragmentPath := pageFragmentPath(outDir, page)

		fragment, err := os.ReadFile(fragmentPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return "", fmt.Errorf("page %d: read fragment: %w", page, err)
			}

			pageRef := requests.Page{ByIndex: &requests.PageByIndex{Document: doc, Index: page}}

			rects, err := pageTextRects(instance, pageRef)
			if err != nil {
				return "", fmt.Errorf("page %d text: %w", page, err)
			}
			blocks, err := p.ai.Clean(ctx, rects)
			if err != nil {
				return "", fmt.Errorf("page %d ai cleanup: %w", page, err)
			}

			var pageHTML strings.Builder
			pageHTML.WriteString(renderChapterHTML(chapterIdx, blocks))

			images, err := pageEmbeddedImages(instance, doc, pageRef)
			if err != nil {
				return "", fmt.Errorf("page %d images: %w", page, err)
			}
			for _, img := range images {
				pageHTML.WriteString(renderImageTag(img.pngBytes, "image/png"))
			}

			fragment = []byte(pageHTML.String())
			if err := os.WriteFile(fragmentPath, fragment, 0o644); err != nil {
				return "", fmt.Errorf("page %d: write fragment: %w", page, err)
			}
		} else {
			log.Info("resuming from checkpointed page", "page", page)
		}

		out.Write(fragment)

		if p.hub != nil {
			p.hub.Publish(jobID, hub.StatusEvent{
				Status:  "PROCESSING",
				Message: fmt.Sprintf("page %d/%d", page+1, totalPages),
			})
		}
	}
	return out.String(), nil
}

// pageFragmentPath is the on-disk checkpoint location for one document-
// global page's rendered HTML fragment, keyed by page index alone since
// pages are globally numbered independent of chapter boundaries (see
// resolveChapters).
func pageFragmentPath(outDir string, page int) string {
	return filepath.Join(outDir, fmt.Sprintf("page-%d.html.part", page))
}

// removePageFragments deletes every page checkpoint file after a
// successful conversion -- they exist only to make a failed job's retry
// skip already-cleaned pages, and serve no purpose once the chapter HTML
// they were assembled into is written. A removal failure is logged, not
// fatal, since the job has already succeeded.
func removePageFragments(log *slog.Logger, outDir string, pageCount int) {
	for page := 0; page < pageCount; page++ {
		if err := os.Remove(pageFragmentPath(outDir, page)); err != nil && !os.IsNotExist(err) {
			log.Warn("remove page fragment failed", "page", page, logging.Err(err))
		}
	}
}

func pageTextRects(instance pdfium.Pdfium, pageRef requests.Page) ([]textRect, error) {
	text, err := instance.GetPageTextStructured(&requests.GetPageTextStructured{
		Page:                   pageRef,
		Mode:                   requests.GetPageTextStructuredModeRects,
		CollectFontInformation: true,
	})
	if err != nil {
		return nil, err
	}
	rects := make([]textRect, 0, len(text.Rects))
	for _, r := range text.Rects {
		fontName := ""
		if r.FontInformation != nil {
			fontName = r.FontInformation.Name
		}
		rects = append(rects, textRect{
			Text:     r.Text,
			FontName: fontName,
			Top:      r.PointPosition.Top,
			Bottom:   r.PointPosition.Bottom,
			Left:     r.PointPosition.Left,
			Right:    r.PointPosition.Right,
		})
	}
	return mergeSameLineRects(rects), nil
}

// sameLineEpsilon is how close a rect's Bottom (baseline, in points) must be
// to a line run's anchor baseline to be considered part of that line --
// pdfium sometimes returns per-character rects instead of per-line/word
// runs (observed on a real PDF: 768 single-letter rects with empty
// FontName for one page), which starves the AI segmentation model of any
// real grouping signal and produces garbled, letter-scrambled output
// regardless of sampling settings. Merging consecutive same-line rects here
// restores word/line-level granularity before the model ever sees the
// text. Descender glyphs (p, g, y, j, q) sit below the true line baseline
// (observed: "p" at Bottom 750.54 vs neighboring "rogram" at ~753.30-753.42
// on the same visual line, a ~2.9pt dip), so a plain pairwise Bottom
// comparison breaks the run right at every descender. sameLineEpsilon is
// wide enough to absorb that dip.
const sameLineEpsilon = 4.0

// mergeSameLineRects concatenates consecutive rects that share the same
// FontName and whose Bottom is within sameLineEpsilon of the current run's
// anchor baseline, so a page pdfium extracts at character granularity is
// normalized back to line-level runs. The anchor is the max Bottom seen so
// far in the run (the non-descender baseline), compared against -- not the
// immediately preceding rect -- so a run of descenders can't drift the
// anchor down one glyph at a time and merge into the next real line.
// Top/Bottom of the merged rect are taken from the first rect in the run.
func mergeSameLineRects(rects []textRect) []textRect {
	if len(rects) == 0 {
		return rects
	}
	merged := make([]textRect, 0, len(rects))
	cur := rects[0]
	anchorBottom := cur.Bottom
	for _, r := range rects[1:] {
		sameLine := r.FontName == cur.FontName && abs(r.Bottom-anchorBottom) <= sameLineEpsilon
		if sameLine {
			cur.Text += r.Text
			if r.Bottom > anchorBottom {
				anchorBottom = r.Bottom
			}
			continue
		}
		merged = append(merged, cur)
		cur = r
		anchorBottom = cur.Bottom
	}
	return append(merged, cur)
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// convertBookmarks maps pdfium's response bookmark tree to the package's own
// bookmark type, decoupling resolveChapters from pdfium's types.
func convertBookmarks(in []responses.GetBookmarksBookmark) []bookmark {
	out := make([]bookmark, 0, len(in))
	for _, b := range in {
		pageIndex := 0
		if b.DestInfo != nil {
			pageIndex = b.DestInfo.PageIndex
		}
		out = append(out, bookmark{
			Title:     b.Title,
			PageIndex: pageIndex,
			Children:  convertBookmarks(b.Children),
		})
	}
	return out
}

func documentMetadata(instance pdfium.Pdfium, doc references.FPDF_DOCUMENT) (title, author string) {
	if resp, err := instance.FPDF_GetMetaText(&requests.FPDF_GetMetaText{Document: doc, Tag: "Title"}); err == nil {
		title = resp.Tag
	}
	if resp, err := instance.FPDF_GetMetaText(&requests.FPDF_GetMetaText{Document: doc, Tag: "Author"}); err == nil {
		author = resp.Tag
	}
	return title, author
}

// firstPageCover returns the first raster image found on page 1, or nil if
// none exists (no full-page-render fallback, per the design doc).
func firstPageCover(instance pdfium.Pdfium, doc references.FPDF_DOCUMENT) []byte {
	pageRef := requests.Page{ByIndex: &requests.PageByIndex{Document: doc, Index: 0}}
	images, err := pageEmbeddedImages(instance, doc, pageRef)
	if err != nil || len(images) == 0 {
		return nil
	}
	return images[0].pngBytes
}
