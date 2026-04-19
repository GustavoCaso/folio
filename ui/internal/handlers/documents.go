package handlers

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/GustavoCaso/folio/ui/internal/hub"
	"github.com/GustavoCaso/folio/ui/internal/templates"
)

func (h *Handlers) ListDocuments(w http.ResponseWriter, r *http.Request) {
	jobs, err := h.store.ListJobs(r.Context())
	if err != nil {
		http.Error(w, "failed to list jobs", http.StatusInternalServerError)
		return
	}
	watchJobID := r.URL.Query().Get("job_id")
	templates.Documents(jobs, watchJobID).Render(r.Context(), w)
}

func (h *Handlers) UploadDocument(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(128 << 20); err != nil { // 128 MB
		http.Error(w, "request too large", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("document")
	if err != nil {
		http.Error(w, "missing document field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	pdfBytes, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "failed to read file", http.StatusInternalServerError)
		return
	}

	job, err := h.store.CreateJob(r.Context(), header.Filename)
	if err != nil {
		http.Error(w, "failed to create job", http.StatusInternalServerError)
		return
	}

	// Start conversion in background — does not block the HTTP response.
	go h.runConversion(job.ID, header.Filename, pdfBytes)

	http.Redirect(w, r, fmt.Sprintf("/?job_id=%s", job.ID), http.StatusSeeOther)
}

func (h *Handlers) runConversion(jobID, filename string, pdfBytes []byte) {
	ctx := context.Background()

	result, err := h.parser.Convert(ctx, jobID, filename, pdfBytes, h.hub)
	if err != nil {
		log.Printf("conversion failed for job %s: %v", jobID, err)
		if err := h.store.MarkJobFailed(ctx, jobID, err.Error()); err != nil {
			log.Printf("failed to mark job %s failed: %v", jobID, err)
		}
		h.hub.Publish(jobID, hub.StatusEvent{Status: "FAILED", Error: err.Error()})
		return
	}

	// Write Markdown to disk
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, strings.TrimSuffix(filename, filepath.Ext(filename)))
	outputPath := filepath.Join(h.dataDir, fmt.Sprintf("%s-%s.md", safe, jobID[:8]))

	if err := os.WriteFile(outputPath, result.Markdown, 0644); err != nil {
		log.Printf("failed to write markdown for job %s: %v", jobID, err)
		if err := h.store.MarkJobFailed(ctx, jobID, err.Error()); err != nil {
			log.Printf("failed to mark job %s failed: %v", jobID, err)
		}
		h.hub.Publish(jobID, hub.StatusEvent{Status: "FAILED", Error: err.Error()})
		return
	}

	if err := h.store.MarkJobDone(ctx, jobID, outputPath); err != nil {
		log.Printf("failed to mark job %s done: %v", jobID, err)
		return
	}

	h.hub.Publish(jobID, hub.StatusEvent{Status: "DONE"})
}
