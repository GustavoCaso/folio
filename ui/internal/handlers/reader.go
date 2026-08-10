package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/GustavoCaso/folio/ui/internal/domain"
	"github.com/GustavoCaso/folio/ui/internal/logging"
	"github.com/GustavoCaso/folio/ui/internal/renderer/markdown"
	"github.com/GustavoCaso/folio/ui/internal/templates"
)

func (h *Handlers) ReadDocument(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("jobID")
	log := logging.LoggerFrom(r.Context()).With("job_id", jobID)

	renderErr := func(status int, msg string) {
		w.WriteHeader(status)
		if err := templates.ErrorPage(msg).Render(r.Context(), w); err != nil {
			log.Error("render error page failed", logging.Err(err))
		}
	}

	job, err := h.store.GetJob(r.Context(), jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Warn("document not found", "job_id", jobID)
			renderErr(http.StatusNotFound, "Document not found.")
		} else {
			log.Error("get job failed", logging.Err(err))
			renderErr(http.StatusInternalServerError, "Failed to load document. Please try again.")
		}
		return
	}

	if job.OutputPath == "" {
		log.Warn("document not ready", "status", job.Status)
		msg := "Document is not ready yet."
		if job.Status == "FAILED" {
			msg = "Document conversion failed."
		}
		renderErr(http.StatusNotFound, msg)
		return
	}
	if job.Format == "epub" {
		h.readEPUBChapter(w, r, job)
		return
	}

	jobPath := job.OutputPath
	// We are running the server locally
	if strings.Contains(h.dataDir, "..") {
		localDir := filepath.Dir(h.dataDir)
		jobPath = filepath.Join(localDir, job.OutputPath)
	}

	src, err := os.ReadFile(jobPath)
	if err != nil {
		log.Error("read markdown failed", logging.Err(err), "output_path", job.OutputPath)
		renderErr(http.StatusInternalServerError, "Failed to read document. Please try again.")
		return
	}

	rendered, err := markdown.Render(src)
	if err != nil {
		log.Error("render markdown failed", logging.Err(err))
		renderErr(http.StatusInternalServerError, "Failed to render document. Please try again.")
		return
	}

	highlights, err := h.store.ListHighlights(r.Context(), jobID)
	if err != nil {
		log.Error("list highlights failed", logging.Err(err))
		renderErr(http.StatusInternalServerError, "Failed to load highlights. Please try again.")
		return
	}

	log.Debug("reader rendered", "highlights", len(highlights), "md_bytes", len(src))
	if err := templates.Reader(job, rendered, highlights, nil, 0, 0, false).Render(r.Context(), w); err != nil {
		log.Error("render reader failed", logging.Err(err))
	}
}

func (h *Handlers) readEPUBChapter(w http.ResponseWriter, r *http.Request, job domain.Job) {
	log := logging.LoggerFrom(r.Context()).With("job_id", job.ID)
	renderErr := func(status int, msg string) {
		w.WriteHeader(status)
		if err := templates.ErrorPage(msg).Render(r.Context(), w); err != nil {
			log.Error("render error page failed", logging.Err(err))
		}
	}

	tocEntries, err := loadTOC(job.OutputPath)
	if err != nil {
		log.Error("load toc failed", logging.Err(err))
		renderErr(http.StatusInternalServerError, "Failed to load table of contents.")
		return
	}
	chapterCount := countChapterFiles(job.OutputPath)

	full := r.URL.Query().Get("full") == "1"

	var html string
	var chapterIdx int
	if full {
		html, err = concatChapters(job.OutputPath, chapterCount)
		chapterIdx = -1
	} else {
		chapterIdx = parseChapterParam(r.URL.Query().Get("chapter"), job.ReadingProgress)
		html, err = readChapterFile(job.OutputPath, chapterIdx)
	}
	if err != nil {
		log.Error("read chapter failed", logging.Err(err))
		renderErr(http.StatusInternalServerError, "Failed to read document. Please try again.")
		return
	}

	highlights, err := h.store.ListHighlights(r.Context(), job.ID)
	if err != nil {
		log.Error("list highlights failed", logging.Err(err))
		renderErr(http.StatusInternalServerError, "Failed to load highlights. Please try again.")
		return
	}

	if err := templates.Reader(job, html, highlights, tocEntries, chapterIdx, chapterCount-1, full).Render(r.Context(), w); err != nil {
		log.Error("render reader failed", logging.Err(err))
	}
}

// loadTOC reads and unmarshals toc.json from outputDir directly into
// templates.EpubTOCEntry — its JSON tags match the shape written by
// epubconvert.Runner's own tocEntry struct, so no separate read-side struct
// is needed here.
func loadTOC(outputDir string) ([]templates.EpubTOCEntry, error) {
	data, err := os.ReadFile(filepath.Join(outputDir, "toc.json"))
	if err != nil {
		return nil, fmt.Errorf("read toc.json: %w", err)
	}
	var entries []templates.EpubTOCEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("unmarshal toc.json: %w", err)
	}
	return entries, nil
}

// countChapterFiles counts chapter-N.html files present in outputDir, to
// determine the true chapter count for prev/next bounds and full-view
// concatenation. This is independent of len(tocEntries), since the TOC tree
// may have fewer top-level entries than spine chapters (nested subsections)
// or, in principle, more (multiple top-level entries could theoretically
// point at chapters already covered, though buildTOCTree/appendFallback in
// epubconvert do not currently produce that).
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

// concatChapters reads chapter-0.html through chapter-{chapterCount-1}.html
// in order and concatenates their contents.
func concatChapters(outputDir string, chapterCount int) (string, error) {
	var b strings.Builder
	for i := range chapterCount {
		content, err := readChapterFile(outputDir, i)
		if err != nil {
			return "", err
		}
		if i > 0 {
			b.WriteString("<hr>")
		}
		b.WriteString(content)
	}
	return b.String(), nil
}

// parseChapterParam resolves the chapter index to render. It prefers a
// valid non-negative integer chapterParam. Failing that, it falls back to
// the chapter index encoded in readingProgress ("{chapterIdx}:{blockID}").
// If neither is usable, it returns 0. Never panics on malformed input.
func parseChapterParam(chapterParam, readingProgress string) int {
	if chapterParam != "" {
		if idx, err := strconv.Atoi(chapterParam); err == nil && idx >= 0 {
			return idx
		}
	}
	if colon := strings.IndexByte(readingProgress, ':'); colon > 0 {
		if idx, err := strconv.Atoi(readingProgress[:colon]); err == nil && idx >= 0 {
			return idx
		}
	}
	return 0
}

func (h *Handlers) UpdateReadingProgress(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("jobID")
	log := logging.LoggerFrom(r.Context())

	var body struct {
		BlockID string `json:"block_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.BlockID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateReadingProgress(r.Context(), jobID, body.BlockID); err != nil {
		log.Error("update reading progress failed", logging.Err(err), "job_id", jobID)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
