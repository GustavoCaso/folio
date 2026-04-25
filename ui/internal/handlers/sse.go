package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/GustavoCaso/folio/ui/internal/logging"
)

func (h *Handlers) WatchJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("jobID")
	log := logging.LoggerFrom(r.Context()).With("job_id", jobID)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Error("streaming not supported by ResponseWriter")
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ch := h.hub.Subscribe(jobID)
	defer h.hub.Unsubscribe(jobID, ch)
	log.Debug("sse subscribed")

	for {
		select {
		case event := <-ch:
			data, marshalErr := json.Marshal(event)
			if marshalErr != nil {
				log.Warn("sse marshal failed", "err", marshalErr)
				return
			}
			if _, err := fmt.Fprintf(w, "event: status\ndata: %s\n\n", data); err != nil {
				log.Debug("sse write failed", "err", err)
				return
			}
			flusher.Flush()
			if event.Status == "DONE" || event.Status == "FAILED" {
				log.Debug("sse terminal", "status", event.Status)
				return
			}
		case <-r.Context().Done():
			log.Debug("sse client disconnected")
			return
		}
	}
}
