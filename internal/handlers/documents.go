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
	documents, err := h.store.ListDocuments(r.Context())
	if err != nil {
		log.Error("list documents failed", logging.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
		if renderErr := templates.Documents(nil, nil, "Failed to load documents. Please try again.").Render(r.Context(), w); renderErr != nil {
			log.Error("render error page failed", logging.Err(renderErr))
		}
		return
	}
	watchDocuments := []string{}
	pendingDocuments, err := h.store.GetPendingDocuments(r.Context())
	if err != nil {
		log.Error("get pending documents failed", logging.Err(err))
	} else {
		for _, pendingDocument := range pendingDocuments {
			watchDocuments = append(watchDocuments, pendingDocument.ID)
		}
	}

	if err := templates.Documents(documents, watchDocuments, "").Render(r.Context(), w); err != nil {
		log.Error("render documents failed", logging.Err(err))
	}
}

func (h *Handlers) UploadDocument(w http.ResponseWriter, r *http.Request) {
	log := logging.LoggerFrom(r.Context())

	renderErr := func(status int, msg string) {
		documents, listErr := h.store.ListDocuments(r.Context())
		if listErr != nil {
			log.Error("list documents failed during error render", logging.Err(listErr))
		}
		w.WriteHeader(status)
		if err := templates.Documents(documents, nil, msg).Render(r.Context(), w); err != nil {
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
	document, err := h.store.CreateDocument(r.Context(), header.Filename, documentBytes, reqID, format)
	if err != nil {
		log.Error("create document failed", logging.Err(err), "filename", header.Filename)
		renderErr(http.StatusInternalServerError, fmt.Sprintf("Failed to store document. %v", err))
		return
	}

	log.Info("upload accepted",
		"document_id", document.ID,
		"filename", header.Filename,
		"bytes", len(documentBytes),
		"format", format,
	)

	go h.converter.Run(document.ID, reqID, format, header.Filename, documentBytes)

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

	document, err := h.store.GetDocument(r.Context(), id)
	if err != nil {
		log.Warn("retry: document not found", "id", id, logging.Err(err))
		renderErr(http.StatusNotFound, "document not found")
		return
	}

	if document.Status != "FAILED" {
		log.Warn("retry: document not in terminal state", "id", id, "status", document.Status)
		renderErr(http.StatusConflict, "retry: document not in terminal state")
		return
	}

	if err := h.store.RetryDocument(r.Context(), id); err != nil {
		log.Warn("retry: fail to update document", "id", id, logging.Err(err))
		renderErr(http.StatusInternalServerError, "retry: failed to update document")
		return
	}

	go h.converter.Run(document.ID, document.RequestID, document.Format, document.Filename, document.Content)

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

	document, err := h.store.GetDocument(r.Context(), id)
	if err != nil {
		log.Warn("delete: document not found", "id", id, logging.Err(err))
		renderErr(http.StatusNotFound, "document not found")
		return
	}

	if document.Status == "PENDING" || document.Status == "PROCESSING" {
		log.Warn("delete: document not in terminal state", "id", id, "status", document.Status)
		renderErr(http.StatusConflict, "delete: document not in terminal state")
		return
	}

	if err := h.store.DeleteDocument(r.Context(), id); err != nil {
		log.Error("delete: db delete failed", "id", id, logging.Err(err))
		renderErr(http.StatusInternalServerError, fmt.Sprintf("delete: db delete failed. %v", err))
		return
	}

	if document.OutputPath != "" {
		if err := os.RemoveAll(document.OutputPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Error("delete: remove markdown failed", "path", document.OutputPath, logging.Err(err))
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

	document, err := h.store.GetDocument(r.Context(), id)
	if err != nil {
		log.Warn("cancel: document not found", "id", id, logging.Err(err))
		renderErr(http.StatusNotFound, "document not found")
		return
	}

	status := document.Status
	if status != "PROCESSING" && status != "PENDING" {
		log.Warn("cancel: document not in correct state", "id", id)
		renderErr(http.StatusBadRequest, "document in invalid state")
		return
	}

	if h.converter == nil || !h.converter.HasCancel(id) {
		if markErr := h.store.MarkDocumentFailed(context.Background(), id, "cancelled by user"); markErr != nil {
			log.Error("mark document failed errored", logging.Err(markErr))
		}
		h.hub.Publish(id, hub.StatusEvent{
			Status: "FAILED",
			Error:  "cancelled by user",
		})
		renderErr(http.StatusConflict, "could not cancel safely. Mark document as failed, reload and try again")
		return
	}
	h.converter.Cancel(id)

	w.WriteHeader(http.StatusOK)
}
