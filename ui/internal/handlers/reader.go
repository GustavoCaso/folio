package handlers

import (
	"net/http"
	"os"

	"github.com/GustavoCaso/folio/ui/internal/logging"
	"github.com/GustavoCaso/folio/ui/internal/renderer"
	"github.com/GustavoCaso/folio/ui/internal/templates"
)

func (h *Handlers) ReadDocument(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("jobID")
	log := logging.LoggerFrom(r.Context()).With("job_id", jobID)

	job, err := h.store.GetJob(r.Context(), jobID)
	if err != nil || job.OutputPath == "" {
		log.Warn("document not found", logging.Err(err), "output_path", job.OutputPath)
		http.Error(w, "document not found or not yet converted", http.StatusNotFound)
		return
	}

	src, err := os.ReadFile(job.OutputPath)
	if err != nil {
		log.Error("read markdown failed", logging.Err(err), "output_path", job.OutputPath)
		http.Error(w, "failed to read markdown file", http.StatusInternalServerError)
		return
	}

	rendered, err := renderer.Render(src)
	if err != nil {
		log.Error("render markdown failed", logging.Err(err))
		http.Error(w, "failed to render markdown", http.StatusInternalServerError)
		return
	}

	highlights, err := h.store.ListHighlights(r.Context(), jobID)
	if err != nil {
		log.Error("list highlights failed", logging.Err(err))
		http.Error(w, "failed to load highlights", http.StatusInternalServerError)
		return
	}

	log.Debug("reader rendered", "highlights", len(highlights), "md_bytes", len(src))
	templates.Reader(job, rendered, highlights).Render(r.Context(), w)
}
