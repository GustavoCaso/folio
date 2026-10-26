package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/GustavoCaso/folio/ui/internal/domain"
	"github.com/GustavoCaso/folio/ui/internal/logging"
	"github.com/GustavoCaso/folio/ui/internal/templates"
	"github.com/GustavoCaso/folio/ui/internal/views"
	"github.com/templui/templui/components/toast"
)

type createHighlightRequest struct {
	DocumentID   string `json:"document_id"`
	StartBlockID string `json:"start_block_id"`
	EndBlockID   string `json:"end_block_id"`
	StartPos     int    `json:"start_pos"`
	EndPos       int    `json:"end_pos"`
	Text         string `json:"text"`
	Tag          string `json:"tag"`
	Note         string `json:"note"`
}

// chapterFromBlockID parses N from a ch{N}-{tag}-{M} block ID (ADR 0001).
// IDs without that prefix fall back to chapter 0.
func chapterFromBlockID(blockID string) int {
	rest, ok := strings.CutPrefix(blockID, "ch")
	if !ok {
		return 0
	}
	n, _, ok := strings.Cut(rest, "-")
	if !ok {
		return 0
	}
	chapter, err := strconv.Atoi(n)
	if err != nil {
		return 0
	}
	return chapter
}

func (h *Handlers) CreateHighlight(w http.ResponseWriter, r *http.Request) {
	log := logging.LoggerFrom(r.Context())
	w.Header().Set("Content-Type", "application/json")

	var req createHighlightRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("invalid highlight JSON", logging.Err(err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorMessage{Error: "invalid JSON"}) //nolint:errcheck
		return
	}

	created, err := h.store.CreateHighlight(r.Context(), domain.Highlight{
		DocumentID:   req.DocumentID,
		StartBlockID: req.StartBlockID,
		EndBlockID:   req.EndBlockID,
		Chapter:      chapterFromBlockID(req.StartBlockID),
		StartPos:     req.StartPos,
		EndPos:       req.EndPos,
		Text:         req.Text,
		Tag:          req.Tag,
		Note:         req.Note,
	})
	if err != nil {
		log.Error("create highlight failed", logging.Err(err), "document_id", req.DocumentID)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorMessage{Error: "failed to save highlight"}) //nolint:errcheck
		return
	}

	for _, backend := range h.backends {
		if err := h.store.CreateHighlightExport(r.Context(), created.ID, backend.Name()); err != nil {
			log.Warn("create highlight export record failed", logging.Err(err), "highlight_id", created.ID, "backend", backend.Name())
		}
	}

	log.Info("highlight created",
		"document_id", req.DocumentID,
		"highlight_id", created.ID,
		"start_block_id", req.StartBlockID,
		"end_block_id", req.EndBlockID,
	)

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(created); err != nil {
		log.Error("encode highlight response failed", logging.Err(err), "highlight_id", created.ID)
	}
}

func (h *Handlers) DeleteHighlight(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	log := logging.LoggerFrom(r.Context())
	id := r.PathValue("id")

	// Remove the highlight from any external backends before deleting locally.
	exports, err := h.store.ListExportsByHighlight(r.Context(), id)
	if err != nil {
		log.Warn("list exports for highlight failed", logging.Err(err), "highlight_id", id)
	}
	for _, exp := range exports {
		if exp.Status != "EXPORTED" || exp.ExternalID == "" {
			continue
		}
		backend := h.backendByName(exp.BackendName)
		if backend == nil {
			continue
		}
		if err := backend.Delete(r.Context(), exp.ExternalID); err != nil {
			log.Warn("delete from backend failed", logging.Err(err), "backend", exp.BackendName, "highlight_id", id)
		}
	}

	if err := h.store.DeleteHighlight(r.Context(), id); err != nil {
		log.Error("delete highlight failed", logging.Err(err), "highlight_id", id)

		toast.Toast(toast.Props{
			Description:   "Fail to delete highlight",
			Variant:       toast.VariantError,
			Icon:          true,
			Position:      toast.PositionTopLeft,
			ShowIndicator: true,
		}).Render(r.Context(), w) //nolint:errcheck
		return
	}

	log.Info("highlight deleted", "highlight_id", id)
	toast.Toast(toast.Props{
		Description:   "Highlight deleted",
		Variant:       toast.VariantSuccess,
		Icon:          true,
		Position:      toast.PositionTopLeft,
		ShowIndicator: true,
	}).Render(r.Context(), w) //nolint:errcheck
}

func (h *Handlers) ListDocumentHighlights(w http.ResponseWriter, r *http.Request) {
	documentID := r.PathValue("id")
	log := logging.LoggerFrom(r.Context()).With("document_id", documentID)

	renderErr := func(status int, msg string) {
		w.WriteHeader(status)
		if err := templates.ErrorPage(msg).Render(r.Context(), w); err != nil {
			log.Error("render error page failed", logging.Err(err))
		}
	}

	document, err := h.store.GetDocument(r.Context(), documentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			renderErr(http.StatusNotFound, "Document not found.")
		} else {
			log.Error("get document failed", logging.Err(err))
			renderErr(http.StatusInternalServerError, "Failed to load document. Please try again.")
		}
		return
	}

	highlights, err := h.store.ListHighlights(r.Context(), document.ID)
	if err != nil {
		log.Error("list highlights failed", logging.Err(err))
		renderErr(http.StatusInternalServerError, "Failed to load highlights. Please try again.")
		return
	}

	// A missing TOC only costs chapter titles; headings fall back to numbers.
	var toc []views.EpubTOCEntry
	if document.OutputPath != "" {
		if toc, err = loadTOC(document.OutputPath); err != nil {
			log.Warn("load toc failed", logging.Err(err))
		}
	}

	groups := views.GroupHighlightsByChapter(highlights, toc)
	if err := templates.DocumentHighlights(document, groups, len(highlights)).Render(r.Context(), w); err != nil {
		log.Error("render highlights page failed", logging.Err(err))
	}
}
