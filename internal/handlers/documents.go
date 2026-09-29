package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/GustavoCaso/folio/ui/internal/domain"
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

	documentBytes, err := io.ReadAll(file)
	if err != nil {
		log.Error("read upload failed", logging.Err(err), "filename", header.Filename)
		renderErr(http.StatusInternalServerError, fmt.Sprintf("Failed to read uploaded file. %v", err))
		return
	}

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".epub") {
		log.Warn("unsupported upload format", "filename", header.Filename)
		renderErr(http.StatusBadRequest, "Only EPUB files are supported.")
		return
	}
	format := domain.EpubFormat

	reqID := logging.RequestIDFrom(r.Context())
	job, err := h.store.CreateJob(r.Context(), header.Filename, documentBytes, reqID, format)
	if err != nil {
		log.Error("create job failed", logging.Err(err), "filename", header.Filename)
		renderErr(http.StatusInternalServerError, fmt.Sprintf("Failed to store job. %v", err))
		return
	}

	log.Info("upload accepted",
		"job_id", job.ID,
		"filename", header.Filename,
		"bytes", len(documentBytes),
		"format", format,
	)

	go h.converter.Run(job.ID, reqID, format, header.Filename, documentBytes)

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

	go h.converter.Run(job.ID, job.RequestID, job.Format, job.Filename, job.Content)

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

	if h.converter == nil || !h.converter.HasCancel(id) {
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
	h.converter.Cancel(id)

	w.WriteHeader(http.StatusOK)
}
