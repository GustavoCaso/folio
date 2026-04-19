package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/GustavoCaso/folio/ui/internal/hub"
	"github.com/GustavoCaso/folio/ui/internal/logging"
	"github.com/GustavoCaso/folio/ui/internal/templates"
)

func (h *Handlers) ListDocuments(w http.ResponseWriter, r *http.Request) {
	log := logging.LoggerFrom(r.Context())
	jobs, err := h.store.ListJobs(r.Context())
	if err != nil {
		log.Error("list jobs failed", logging.Err(err))
		http.Error(w, "failed to list jobs", http.StatusInternalServerError)
		return
	}
	watchJobID := r.URL.Query().Get("job_id")
	templates.Documents(jobs, watchJobID).Render(r.Context(), w)
}

func (h *Handlers) UploadDocument(w http.ResponseWriter, r *http.Request) {
	log := logging.LoggerFrom(r.Context())

	if err := r.ParseMultipartForm(128 << 20); err != nil { // 128 MB
		log.Warn("upload too large", logging.Err(err))
		http.Error(w, "request too large", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("document")
	if err != nil {
		log.Warn("missing document field", logging.Err(err))
		http.Error(w, "missing document field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	pdfBytes, err := io.ReadAll(file)
	if err != nil {
		log.Error("read upload failed", logging.Err(err), "filename", header.Filename)
		http.Error(w, "failed to read file", http.StatusInternalServerError)
		return
	}

	reqID := logging.RequestIDFrom(r.Context())
	job, err := h.store.CreateJob(r.Context(), header.Filename, reqID)
	if err != nil {
		log.Error("create job failed", logging.Err(err), "filename", header.Filename)
		http.Error(w, "failed to create job", http.StatusInternalServerError)
		return
	}

	log.Info("upload accepted",
		"job_id", job.ID,
		"filename", header.Filename,
		"bytes", len(pdfBytes),
	)

	// Start conversion in background — does not block the HTTP response.
	// Snapshot the request_id so parser logs link back to the originating upload.
	go h.runConversion(job.ID, reqID, header.Filename, pdfBytes)

	http.Redirect(w, r, fmt.Sprintf("/?job_id=%s", job.ID), http.StatusSeeOther)
}

func (h *Handlers) runConversion(jobID, requestID, filename string, pdfBytes []byte) {
	ctx := context.Background()
	log := h.logger.With("job_id", jobID, "request_id", requestID, "filename", filename)
	start := time.Now()

	log.Info("conversion start", "bytes", len(pdfBytes))

	result, err := h.parser.Convert(ctx, jobID, requestID, filename, pdfBytes, h.hub)
	if err != nil {
		log.Error("conversion failed", logging.Err(err), "dur_ms", time.Since(start).Milliseconds())
		if markErr := h.store.MarkJobFailed(ctx, jobID, err.Error()); markErr != nil {
			log.Error("mark job failed errored", logging.Err(markErr))
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
		log.Error("write markdown failed", logging.Err(err), "output_path", outputPath)
		if markErr := h.store.MarkJobFailed(ctx, jobID, err.Error()); markErr != nil {
			log.Error("mark job failed errored", logging.Err(markErr))
		}
		h.hub.Publish(jobID, hub.StatusEvent{Status: "FAILED", Error: err.Error()})
		return
	}

	if err := h.store.MarkJobDone(ctx, jobID, outputPath); err != nil {
		log.Error("mark job done failed", logging.Err(err))
		return
	}

	log.Info("conversion done",
		"output_path", outputPath,
		"md_bytes", len(result.Markdown),
		"dur_ms", time.Since(start).Milliseconds(),
	)
	h.hub.Publish(jobID, hub.StatusEvent{Status: "DONE"})
}
