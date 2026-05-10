package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/logging"
	"github.com/templui/templui/components/toast"
)

type createHighlightRequest struct {
	JobID        string `json:"job_id"`
	StartBlockID string `json:"start_block_id"`
	EndBlockID   string `json:"end_block_id"`
	StartPos     int    `json:"start_pos"`
	EndPos       int    `json:"end_pos"`
	Text         string `json:"text"`
	Tag          string `json:"tag"`
	Note         string `json:"note"`
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

	created, err := h.store.CreateHighlight(r.Context(), db.Highlight{
		JobID:        req.JobID,
		StartBlockID: req.StartBlockID,
		EndBlockID:   req.EndBlockID,
		StartPos:     req.StartPos,
		EndPos:       req.EndPos,
		Text:         req.Text,
		Tag:          req.Tag,
		Note:         req.Note,
	})
	if err != nil {
		log.Error("create highlight failed", logging.Err(err), "job_id", req.JobID)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorMessage{Error: "failed to save highlight"}) //nolint:errcheck
		return
	}

	log.Info("highlight created",
		"job_id", req.JobID,
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
		if exp.Status != "exported" || exp.ExternalID == "" {
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
