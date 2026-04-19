package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (h *Handlers) WatchJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("jobID")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ch := h.hub.Subscribe(jobID)
	defer h.hub.Unsubscribe(jobID, ch)

	for {
		select {
		case event := <-ch:
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "event: status\ndata: %s\n\n", data)
			flusher.Flush()
			if event.Status == "DONE" || event.Status == "FAILED" {
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}
