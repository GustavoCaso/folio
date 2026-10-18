package handlers

import (
	"bytes"
	"database/sql"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"

	"github.com/GustavoCaso/folio/ui/internal/logging"
	"github.com/GustavoCaso/folio/ui/internal/templates"
)

func (h *Handlers) EditDocumentForm(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	log := logging.LoggerFrom(r.Context()).With("document_id", id)

	renderErr := func(status int, msg string) {
		w.WriteHeader(status)
		if err := templates.ErrorPage(msg).Render(r.Context(), w); err != nil {
			log.Error("render error page failed", logging.Err(err))
		}
	}

	document, err := h.store.GetDocument(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			renderErr(http.StatusNotFound, "Document not found.")
		} else {
			log.Error("get document failed", logging.Err(err))
			renderErr(http.StatusInternalServerError, "Failed to load document.")
		}
		return
	}

	if err := templates.EditDocument(document, "").Render(r.Context(), w); err != nil {
		log.Error("render edit page failed", logging.Err(err))
	}
}

func (h *Handlers) EditDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	log := logging.LoggerFrom(r.Context()).With("document_id", id)

	renderFormErr := func(status int, msg string) {
		document, getErr := h.store.GetDocument(r.Context(), id)
		if getErr != nil {
			w.WriteHeader(http.StatusInternalServerError)
			if err := templates.ErrorPage("Failed to load document.").Render(r.Context(), w); err != nil {
				log.Error("render error page failed", logging.Err(err))
			}
			return
		}
		w.WriteHeader(status)
		if err := templates.EditDocument(document, msg).Render(r.Context(), w); err != nil {
			log.Error("render edit page failed", logging.Err(err))
		}
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		renderFormErr(http.StatusBadRequest, "Failed to parse form.")
		return
	}

	title := r.FormValue("title")
	author := r.FormValue("author")
	var tags []string
	for _, t := range strings.Split(r.FormValue("tags"), ",") {
		if t = strings.TrimSpace(t); t != "" {
			tags = append(tags, t)
		}
	}

	// Read optional cover upload.
	var coverBytes []byte
	file, _, err := r.FormFile("cover")
	if err == nil {
		defer func() { _ = file.Close() }()
		data, readErr := io.ReadAll(file)
		if readErr != nil {
			log.Error("read cover failed", logging.Err(readErr))
			renderFormErr(http.StatusInternalServerError, "Failed to read cover image.")
			return
		}
		if len(data) > 0 {
			if _, _, decodeErr := image.Decode(bytes.NewReader(data)); decodeErr != nil {
				renderFormErr(http.StatusBadRequest, "Cover must be a valid image (PNG or JPEG).")
				return
			}
			coverBytes = data
		}
	}

	// If no new cover was uploaded, keep the existing one.
	if coverBytes == nil {
		document, getErr := h.store.GetDocument(r.Context(), id)
		if getErr != nil {
			if errors.Is(getErr, sql.ErrNoRows) {
				w.WriteHeader(http.StatusNotFound)
				if err := templates.ErrorPage("Document not found.").Render(r.Context(), w); err != nil {
					log.Error("render error page failed", logging.Err(err))
				}
				return
			}
			log.Error("get document failed", logging.Err(getErr))
			renderFormErr(http.StatusInternalServerError, "Failed to load document.")
			return
		}
		coverBytes = document.Cover
	}

	if err := h.store.UpdateDocument(r.Context(), id, title, author, tags, coverBytes); err != nil {
		log.Error("update document failed", logging.Err(err))
		renderFormErr(http.StatusInternalServerError, "Failed to save changes.")
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
