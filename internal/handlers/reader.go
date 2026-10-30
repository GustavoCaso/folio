package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/GustavoCaso/folio/ui/internal/domain"
	"github.com/GustavoCaso/folio/ui/internal/logging"
	"github.com/GustavoCaso/folio/ui/internal/templates"
	"github.com/GustavoCaso/folio/ui/internal/views"
)

func (h *Handlers) ReadDocument(w http.ResponseWriter, r *http.Request) {
	documentID := r.PathValue("documentID")
	log := logging.LoggerFrom(r.Context()).With("document_id", documentID)

	renderErr := func(status int, msg string) {
		w.WriteHeader(status)
		if err := templates.ErrorPage(msg).Render(r.Context(), w); err != nil {
			log.Error("render error page failed", logging.Err(err))
		}
	}

	document, err := h.store.GetDocument(r.Context(), documentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Warn("document not found", "document_id", documentID)
			renderErr(http.StatusNotFound, "Document not found.")
		} else {
			log.Error("get document failed", logging.Err(err))
			renderErr(http.StatusInternalServerError, "Failed to load document. Please try again.")
		}
		return
	}

	if document.OutputPath == "" {
		log.Warn("document not ready", "status", document.Status)
		msg := "Document is not ready yet."
		if document.Status == "FAILED" {
			msg = "Document conversion failed."
		}
		renderErr(http.StatusNotFound, msg)
		return
	}
	if document.Format != domain.EpubFormat {
		log.Warn("unsupported document format", "format", document.Format)
		renderErr(http.StatusUnprocessableEntity, "Unsupported document format.")
		return
	}
	h.readEPUBChapter(w, r, document)
}

func (h *Handlers) readEPUBChapter(w http.ResponseWriter, r *http.Request, document domain.Document) {
	log := logging.LoggerFrom(r.Context()).With("document_id", document.ID)
	renderErr := func(status int, msg string) {
		w.WriteHeader(status)
		if err := templates.ErrorPage(msg).Render(r.Context(), w); err != nil {
			log.Error("render error page failed", logging.Err(err))
		}
	}

	tocEntries, err := loadTOC(document.OutputPath)
	if err != nil {
		log.Error("load toc failed", logging.Err(err))
		renderErr(http.StatusInternalServerError, "Failed to load table of contents.")
		return
	}
	chapterCount := countChapterFiles(document.OutputPath)

	chapterIdx := parseChapterParam(r.URL.Query().Get("chapter"), document.ReadingProgress)
	html, err := readChapterFile(document.OutputPath, chapterIdx)
	if err != nil {
		log.Error("read chapter failed", logging.Err(err))
		renderErr(http.StatusInternalServerError, "Failed to read document. Please try again.")
		return
	}

	highlights, err := h.store.ListChapterHighlights(r.Context(), document.ID, chapterIdx)
	if err != nil {
		log.Error("list highlights failed", logging.Err(err))
		renderErr(http.StatusInternalServerError, "Failed to load highlights. Please try again.")
		return
	}

	if err := templates.Reader(document, html, highlights, tocEntries, chapterIdx, chapterCount-1).Render(r.Context(), w); err != nil {
		log.Error("render reader failed", logging.Err(err))
	}
}

// loadTOC reads and unmarshals toc.json from outputDir directly into
// views.EpubTOCEntry — its JSON tags match the shape written by
// parser.NewEPUB's own tocEntry struct, so no separate read-side struct
// is needed here.
func loadTOC(outputDir string) ([]views.EpubTOCEntry, error) {
	data, err := os.ReadFile(filepath.Join(outputDir, "toc.json"))
	if err != nil {
		return nil, fmt.Errorf("read toc.json: %w", err)
	}
	var entries []views.EpubTOCEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("unmarshal toc.json: %w", err)
	}
	return entries, nil
}

// countChapterFiles counts chapter-N.html files present in outputDir, to
// determine the true chapter count for prev/next bounds. This is independent of len(tocEntries), since the TOC tree
// may have fewer top-level entries than spine chapters (nested subsections)
// or, in principle, more (multiple top-level entries could theoretically
// point at chapters already covered, though buildTOCTree in
// parser/epub.go does not currently produce that).
func countChapterFiles(outputDir string) int {
	count := 0
	for {
		if _, err := os.Stat(filepath.Join(outputDir, fmt.Sprintf("chapter-%d.html", count))); err != nil {
			break
		}
		count++
	}
	return count
}

// readChapterFile reads chapter-{chapterIdx}.html from outputDir.
func readChapterFile(outputDir string, chapterIdx int) (string, error) {
	data, err := os.ReadFile(filepath.Join(outputDir, fmt.Sprintf("chapter-%d.html", chapterIdx)))
	if err != nil {
		return "", fmt.Errorf("chapter %d not found: %w", chapterIdx, err)
	}
	return string(data), nil
}

var readingProgressRe = regexp.MustCompile(`^ch(?P<idx>\d+)-[^-]+-[^-]+$`)

// parseChapterParam resolves the chapter index to render. It prefers a
// valid non-negative integer chapterParam. Failing that, it falls back to
// the chapter index encoded in readingProgress ("ch{chapterIdx}-{element}-{elementID}").
// If neither is usable, it returns 0. Never panics on malformed input.
func parseChapterParam(chapterParam, readingProgress string) int {
	if chapterParam != "" {
		if idx, err := strconv.Atoi(chapterParam); err == nil && idx >= 0 {
			return idx
		}
	}

	if match := readingProgressRe.FindStringSubmatch(readingProgress); match != nil {
		if idx, err := strconv.Atoi(match[readingProgressRe.SubexpIndex("idx")]); err == nil {
			return idx
		}
	}

	return 0
}

func (h *Handlers) UpdateReadingProgress(w http.ResponseWriter, r *http.Request) {
	documentID := r.PathValue("documentID")
	log := logging.LoggerFrom(r.Context())

	var body struct {
		BlockID string `json:"block_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.BlockID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateReadingProgress(r.Context(), documentID, body.BlockID); err != nil {
		log.Error("update reading progress failed", logging.Err(err), "document_id", documentID)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
