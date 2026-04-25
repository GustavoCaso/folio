package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/logging"
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

	var req createHighlightRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("invalid highlight JSON", logging.Err(err))
		http.Error(w, "invalid JSON", http.StatusBadRequest)
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
		http.Error(w, "failed to save highlight", http.StatusInternalServerError)
		return
	}

	log.Info("highlight created",
		"job_id", req.JobID,
		"highlight_id", created.ID,
		"start_block_id", req.StartBlockID,
		"end_block_id", req.EndBlockID,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(created); err != nil {
		log.Error("encode highlight response failed", logging.Err(err), "highlight_id", created.ID)
	}
}

func (h *Handlers) DeleteHighlight(w http.ResponseWriter, r *http.Request) {
	log := logging.LoggerFrom(r.Context())
	id := r.PathValue("id")
	if err := h.store.DeleteHighlight(r.Context(), id); err != nil {
		log.Error("delete highlight failed", logging.Err(err), "highlight_id", id)
		http.Error(w, "failed to delete highlight", http.StatusInternalServerError)
		return
	}
	log.Info("highlight deleted", "highlight_id", id)
	w.WriteHeader(http.StatusNoContent)
}
