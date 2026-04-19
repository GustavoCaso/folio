package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/GustavoCaso/folio/ui/internal/db"
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
	var req createHighlightRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
		http.Error(w, "failed to save highlight", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}

func (h *Handlers) DeleteHighlight(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteHighlight(r.Context(), id); err != nil {
		http.Error(w, "failed to delete highlight", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
