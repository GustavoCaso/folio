package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/GustavoCaso/folio/ui/internal/logging"
	"github.com/GustavoCaso/folio/ui/internal/renderer"
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

	rendered, err := renderer.Render(src)
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
	if err := templates.Reader(job, rendered, highlights).Render(r.Context(), w); err != nil {
		log.Error("render reader failed", logging.Err(err))
	}
}
