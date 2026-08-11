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
	"github.com/klippa-app/go-pdfium/enums"
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
}

// NewNativePDF constructs the pdfium-based PDF Parser. h may be nil, in
// which case status events are not published (Store is still updated).
func NewNativePDF(store Store, h *hub.Hub, dataDir string) (Parser, error) {
	pool, err := webassembly.Init(webassembly.Config{MinIdle: 1, MaxIdle: 1, MaxTotal: 1})
	if err != nil {
		return nil, fmt.Errorf("init pdfium pool: %w", err)
	}
	return &nativePDFParser{store: store, hub: h, dataDir: dataDir, logger: slog.Default(), pool: pool}, nil
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
		chapterHTML, err := p.renderChapter(instance, doc.Document, ch, i)
		if err != nil {
			return p.fail(log, jobID, fmt.Sprintf("render chapter %d: %v", i, err))
		}
		if err := os.WriteFile(filepath.Join(outDir, fmt.Sprintf("chapter-%d.html", i)), []byte(chapterHTML), 0o644); err != nil {
			return p.fail(log, jobID, fmt.Sprintf("write chapter %d: %v", i, err))
		}
	}

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

// renderChapter walks every page in ch's range, classifies its text into
// blocks, inserts embedded raster images inline at their page position, and
// renders the result as HTML with data-block-id anchoring.
func (p *nativePDFParser) renderChapter(instance pdfium.Pdfium, doc references.FPDF_DOCUMENT, ch chapterRange, chapterIdx int) (string, error) {
	var out strings.Builder
	for page := ch.StartPage; page <= ch.EndPage; page++ {
		pageRef := requests.Page{ByIndex: &requests.PageByIndex{Document: doc, Index: page}}

		rects, err := pageTextRects(instance, pageRef)
		if err != nil {
			return "", fmt.Errorf("page %d text: %w", page, err)
		}
		out.WriteString(renderChapterHTML(chapterIdx, classifyBlocks(rects)))

		images, err := pageEmbeddedImages(instance, doc, pageRef)
		if err != nil {
			return "", fmt.Errorf("page %d images: %w", page, err)
		}
		for _, img := range images {
			out.WriteString(renderImageTag(img.pngBytes, "image/png"))
		}
	}
	return out.String(), nil
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
		rects = append(rects, textRect{Text: r.Text, FontName: fontName})
	}
	return rects, nil
}

type pageImage struct {
	pngBytes []byte
}

// pageEmbeddedImages extracts every FPDF_PAGEOBJ_IMAGE object on the page as
// a standalone PNG, per-object (not a full-page render) — validated in
// scripts/pdf-extract-test/FINDINGS.md. Vector-drawn illustrations
// (FPDF_PAGEOBJ_PATH) are out of scope, see the design doc.
func pageEmbeddedImages(instance pdfium.Pdfium, doc references.FPDF_DOCUMENT, pageRef requests.Page) ([]pageImage, error) {
	countResp, err := instance.FPDFPage_CountObjects(&requests.FPDFPage_CountObjects{Page: pageRef})
	if err != nil {
		return nil, err
	}

	var images []pageImage
	for i := 0; i < countResp.Count; i++ {
		objResp, err := instance.FPDFPage_GetObject(&requests.FPDFPage_GetObject{Page: pageRef, Index: i})
		if err != nil {
			continue
		}
		typeResp, err := instance.FPDFPageObj_GetType(&requests.FPDFPageObj_GetType{PageObject: objResp.PageObject})
		if err != nil || typeResp.Type != enums.FPDF_PAGEOBJ_IMAGE {
			continue
		}

		png, ok := renderImageObjectPNG(instance, doc, pageRef, objResp.PageObject)
		if ok {
			images = append(images, pageImage{pngBytes: png})
		}
	}
	return images, nil
}

func renderImageObjectPNG(instance pdfium.Pdfium, doc references.FPDF_DOCUMENT, pageRef requests.Page, obj references.FPDF_PAGEOBJECT) ([]byte, bool) {
	bmpResp, err := instance.FPDFImageObj_GetRenderedBitmap(&requests.FPDFImageObj_GetRenderedBitmap{
		Document: doc, Page: pageRef, ImageObject: obj,
	})
	if err != nil {
		return nil, false
	}
	widthResp, err := instance.FPDFBitmap_GetWidth(&requests.FPDFBitmap_GetWidth{Bitmap: bmpResp.Bitmap})
	if err != nil {
		return nil, false
	}
	heightResp, err := instance.FPDFBitmap_GetHeight(&requests.FPDFBitmap_GetHeight{Bitmap: bmpResp.Bitmap})
	if err != nil {
		return nil, false
	}
	strideResp, err := instance.FPDFBitmap_GetStride(&requests.FPDFBitmap_GetStride{Bitmap: bmpResp.Bitmap})
	if err != nil {
		return nil, false
	}
	bufResp, err := instance.FPDFBitmap_GetBuffer(&requests.FPDFBitmap_GetBuffer{Bitmap: bmpResp.Bitmap})
	if err != nil {
		return nil, false
	}

	img := bgraBufferToImage(bufResp.Buffer, widthResp.Width, heightResp.Height, strideResp.Stride)
	return encodePNG(img)
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
