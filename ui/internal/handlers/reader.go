package handlers

import (
	"net/http"
	"os"

	"github.com/GustavoCaso/folio/ui/internal/renderer"
	"github.com/GustavoCaso/folio/ui/internal/templates"
)

func (h *Handlers) ReadDocument(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("jobID")

	job, err := h.store.GetJob(r.Context(), jobID)
	if err != nil || job.OutputPath == "" {
		http.Error(w, "document not found or not yet converted", http.StatusNotFound)
		return
	}

	src, err := os.ReadFile(job.OutputPath)
	if err != nil {
		http.Error(w, "failed to read markdown file", http.StatusInternalServerError)
		return
	}

	rendered, err := renderer.Render(src)
	if err != nil {
		http.Error(w, "failed to render markdown", http.StatusInternalServerError)
		return
	}

	highlights, err := h.store.ListHighlights(r.Context(), jobID)
	if err != nil {
		http.Error(w, "failed to load highlights", http.StatusInternalServerError)
		return
	}

	templates.Reader(job, rendered, highlights).Render(r.Context(), w)
}
