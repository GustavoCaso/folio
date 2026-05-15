package handlers

import (
	"context"
	"encoding/base64"
	"errors"
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
	"github.com/templui/templui/components/toast"
)

func (h *Handlers) ListDocuments(w http.ResponseWriter, r *http.Request) {
	log := logging.LoggerFrom(r.Context())
	jobs, err := h.store.ListJobs(r.Context())
	if err != nil {
		log.Error("list jobs failed", logging.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
		if renderErr := templates.Documents(nil, nil, "Failed to load documents. Please try again.").Render(r.Context(), w); renderErr != nil {
			log.Error("render error page failed", logging.Err(renderErr))
		}
		return
	}
	watchJobs := []string{}
	pendingJobs, err := h.store.GetPendingJobs(r.Context())
	if err != nil {
		log.Error("get pending jobs failed", logging.Err(err))
	} else {
		for _, pendingJob := range pendingJobs {
			watchJobs = append(watchJobs, pendingJob.ID)
		}
	}

	if err := templates.Documents(jobs, watchJobs, "").Render(r.Context(), w); err != nil {
		log.Error("render documents failed", logging.Err(err))
	}
}

func (h *Handlers) UploadDocument(w http.ResponseWriter, r *http.Request) {
	log := logging.LoggerFrom(r.Context())

	renderErr := func(status int, msg string) {
		jobs, listErr := h.store.ListJobs(r.Context())
		if listErr != nil {
			log.Error("list jobs failed during error render", logging.Err(listErr))
		}
		w.WriteHeader(status)
		if err := templates.Documents(jobs, nil, msg).Render(r.Context(), w); err != nil {
			log.Error("render error page failed", logging.Err(err))
		}
	}

	if err := r.ParseMultipartForm(128 << 20); err != nil { // 128 MB
		log.Warn("upload too large", logging.Err(err))
		renderErr(http.StatusBadRequest, "File too large (max 128 MB).")
		return
	}

	file, header, err := r.FormFile("document")
	if err != nil {
		log.Warn("missing document field", logging.Err(err))
		renderErr(http.StatusBadRequest, "No document file selected.")
		return
	}
	defer func() { _ = file.Close() }()

	pdfBytes, err := io.ReadAll(file)
	if err != nil {
		log.Error("read upload failed", logging.Err(err), "filename", header.Filename)
		renderErr(http.StatusInternalServerError, fmt.Sprintf("Failed to read uploaded file. %v", err))
		return
	}

	reqID := logging.RequestIDFrom(r.Context())
	job, err := h.store.CreateJob(r.Context(), header.Filename, pdfBytes, reqID)
	if err != nil {
		log.Error("create job failed", logging.Err(err), "filename", header.Filename)
		renderErr(http.StatusInternalServerError, fmt.Sprintf("Failed to store job. %v", err))
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

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handlers) RetryDocument(w http.ResponseWriter, r *http.Request) {
	log := logging.LoggerFrom(r.Context())
	id := r.PathValue("id")

	renderErr := func(status int, msg string) {
		w.WriteHeader(status)
		if err := templates.ErrorPage(msg).Render(r.Context(), w); err != nil {
			log.Error("render error page failed", logging.Err(err))
		}
	}

	job, err := h.store.GetJob(r.Context(), id)
	if err != nil {
		log.Warn("retry: job not found", "id", id, logging.Err(err))
		renderErr(http.StatusNotFound, "job not found")
		return
	}

	if job.Status != "FAILED" {
		log.Warn("retry: job not in terminal state", "id", id, "status", job.Status)
		renderErr(http.StatusConflict, "retry: job not in terminal state")
		return
	}

	if err := h.store.RetryJob(r.Context(), id); err != nil {
		log.Warn("retry: fail to update job", "id", id, logging.Err(err))
		renderErr(http.StatusInternalServerError, "retry: failed to update job")
		return
	}

	// Start conversion in background — does not block the HTTP response.
	// Snapshot the request_id so parser logs link back to the originating upload.
	go h.runConversion(job.ID, job.RequestID, job.Filename, job.Content)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handlers) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	log := logging.LoggerFrom(r.Context())
	id := r.PathValue("id")

	renderErr := func(status int, msg string) {
		w.WriteHeader(status)
		toast.Toast(toast.Props{
			Description:   msg,
			Variant:       toast.VariantError,
			Icon:          true,
			Position:      toast.PositionBottomRight,
			ShowIndicator: true,
		}).Render(r.Context(), w) //nolint:errcheck
	}

	job, err := h.store.GetJob(r.Context(), id)
	if err != nil {
		log.Warn("delete: job not found", "id", id, logging.Err(err))
		renderErr(http.StatusNotFound, "job not found")
		return
	}

	if job.Status == "PENDING" || job.Status == "PROCESSING" {
		log.Warn("delete: job not in terminal state", "id", id, "status", job.Status)
		renderErr(http.StatusConflict, "delete: job not in terminal state")
		return
	}

	if err := h.store.DeleteJob(r.Context(), id); err != nil {
		log.Error("delete: db delete failed", "id", id, logging.Err(err))
		renderErr(http.StatusInternalServerError, fmt.Sprintf("delete: db delete failed. %v", err))
		return
	}

	if job.OutputPath != "" {
		if err := os.Remove(job.OutputPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Error("delete: remove markdown failed", "path", job.OutputPath, logging.Err(err))
		}
	}

	w.WriteHeader(http.StatusOK)
	toast.Toast(toast.Props{
		Description:   "Document deleted",
		Variant:       toast.VariantSuccess,
		Icon:          true,
		Position:      toast.PositionBottomRight,
		ShowIndicator: true,
	}).Render(r.Context(), w) //nolint:errcheck
}

func (h *Handlers) CancelDocument(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	log := logging.LoggerFrom(r.Context())

	renderErr := func(status int, msg string) {
		w.WriteHeader(status)
		toast.Toast(toast.Props{
			Description:   msg,
			Variant:       toast.VariantError,
			Icon:          true,
			Position:      toast.PositionBottomRight,
			ShowIndicator: true,
		}).Render(r.Context(), w) //nolint:errcheck
	}

	id := r.PathValue("id")

	job, err := h.store.GetJob(r.Context(), id)
	if err != nil {
		log.Warn("cancel: job not found", "id", id, logging.Err(err))
		renderErr(http.StatusNotFound, "job not found")
		return
	}

	status := job.Status
	if status != "PROCESSING" && status != "PENDING" {
		log.Warn("cancel: job not in correct state", "id", id)
		renderErr(http.StatusBadRequest, "job in invalid state")
		return
	}

	val, ok := h.cancels.Load(id)
	if !ok {
		if markErr := h.store.MarkJobFailed(context.Background(), id, "cancelled by user"); markErr != nil {
			log.Error("mark job failed errored", logging.Err(markErr))
		}
		h.hub.Publish(id, hub.StatusEvent{
			Status: "FAILED",
			Error:  "cancelled by user",
		})
		renderErr(http.StatusConflict, "could not cancel safely. Mark job as failed, reload and try again")
		return
	}
	cancel, ok := val.(context.CancelFunc)
	if ok {
		cancel()
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) runConversion(jobID, requestID, filename string, pdfBytes []byte) {
	log := h.logger.With("job_id", jobID, "request_id", requestID, "filename", filename)
	start := time.Now()

	log.Info("conversion start", "bytes", len(pdfBytes))

	convertCtx, convertCancel := context.WithCancel(context.Background())
	h.cancels.Store(jobID, convertCancel)
	result, err := h.parser.Convert(convertCtx, jobID, requestID, filename, pdfBytes, h.hub)
	h.cancels.Delete(jobID)
	convertCancel()

	if err != nil {
		log.Error("conversion failed", logging.Err(err), "dur_ms", time.Since(start).Milliseconds())
		errMsg := err.Error()
		if errors.Is(err, context.Canceled) {
			errMsg = "cancelled by user"
		}
		if markErr := h.store.MarkJobFailed(context.Background(), jobID, errMsg); markErr != nil {
			log.Error("mark job failed errored", logging.Err(markErr))
		}
		h.hub.Publish(jobID, hub.StatusEvent{Status: "FAILED", Error: errMsg})
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
		if markErr := h.store.MarkJobFailed(context.Background(), jobID, err.Error()); markErr != nil {
			log.Error("mark job failed errored", logging.Err(markErr))
		}
		h.hub.Publish(jobID, hub.StatusEvent{Status: "FAILED", Error: err.Error()})
		return
	}

	title := result.Title
	author := result.Author

	if err := h.store.MarkJobDone(context.Background(), jobID, outputPath, title, author, result.Cover); err != nil {
		log.Error("mark job done failed", logging.Err(err))
		return
	}

	log.Info("conversion done",
		"output_path", outputPath,
		"md_bytes", len(result.Markdown),
		"dur_ms", time.Since(start).Milliseconds(),
	)
	h.hub.Publish(jobID, hub.StatusEvent{
		Status: "DONE",
		Title:  title,
		Author: author,
		Cover:  base64.StdEncoding.EncodeToString(result.Cover),
	})
}
