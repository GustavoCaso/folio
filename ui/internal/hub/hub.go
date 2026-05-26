package hub

import (
	"errors"
	"log/slog"
	"sync"
)

// StatusEvent carries a parser status update for a single job.
type StatusEvent struct {
	Status  string
	Error   string
	Message string
	// populated only when Status == "DONE"
	Title  string
	Author string
	Cover  string // base64-encoded PNG; empty if unavailable
}

// Hub routes StatusEvents from gRPC stream goroutines to SSE connections.
// Each job ID can have multiple subscribers (e.g. two browser tabs watching the same job).
type Hub struct {
	mu     sync.Mutex
	subs   map[string][]chan StatusEvent
	logger *slog.Logger
}

func New(logger *slog.Logger) (*Hub, error) {
	if logger == nil {
		return nil, errors.New("hub.New: logger is required")
	}
	return &Hub{subs: make(map[string][]chan StatusEvent), logger: logger}, nil
}

// Subscribe returns a channel that will receive events for the given job ID.
func (h *Hub) Subscribe(jobID string) chan StatusEvent {
	ch := make(chan StatusEvent, 8)
	h.mu.Lock()
	h.subs[jobID] = append(h.subs[jobID], ch)
	n := len(h.subs[jobID])
	h.mu.Unlock()
	h.logger.Debug("subscribe", "job_id", jobID, "subscribers", n)
	return ch
}

// Unsubscribe removes the channel from the job's subscriber list.
func (h *Hub) Unsubscribe(jobID string, ch chan StatusEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	subs := h.subs[jobID]
	for i, c := range subs {
		if c == ch {
			h.subs[jobID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	if len(h.subs[jobID]) == 0 {
		delete(h.subs, jobID)
	}
	h.logger.Debug("unsubscribe", "job_id", jobID, "subscribers", len(h.subs[jobID]))
}

// Publish sends an event to all subscribers for the given job ID.
// Slow subscribers are skipped (non-blocking send).
func (h *Hub) Publish(jobID string, event StatusEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ch := range h.subs[jobID] {
		select {
		case ch <- event:
		default:
			h.logger.Warn("dropping event, slow subscriber",
				"job_id", jobID,
				"status", event.Status,
			)
		}
	}
}
