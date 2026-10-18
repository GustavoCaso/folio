package hub

import (
	"errors"
	"log/slog"
	"sync"
)

// StatusEvent carries a parser status update for a single document.
type StatusEvent struct {
	Status  string
	Error   string
	Message string
	// populated only when Status == "DONE"
	Title  string
	Author string
	Cover  string // base64-encoded PNG; empty if unavailable
}

// Hub routes StatusEvents from conversion goroutines to SSE connections.
// Each document ID can have multiple subscribers (e.g. two browser tabs watching the same document).
// Events published before any subscriber connects are buffered and replayed on Subscribe.
type Hub struct {
	mu     sync.Mutex
	subs   map[string][]chan StatusEvent
	replay map[string][]StatusEvent
	logger *slog.Logger
}

func New(logger *slog.Logger) (*Hub, error) {
	if logger == nil {
		return nil, errors.New("hub.New: logger is required")
	}
	return &Hub{
		subs:   make(map[string][]chan StatusEvent),
		replay: make(map[string][]StatusEvent),
		logger: logger,
	}, nil
}

// Subscribe returns a channel that will receive events for the given document ID.
// Any events published before this call are replayed immediately into the channel.
func (h *Hub) Subscribe(documentID string) chan StatusEvent {
	h.mu.Lock()
	past := h.replay[documentID]
	ch := make(chan StatusEvent, len(past)+8)
	for _, e := range past {
		ch <- e
	}
	h.subs[documentID] = append(h.subs[documentID], ch)
	n := len(h.subs[documentID])
	h.mu.Unlock()
	h.logger.Debug("subscribe", "document_id", documentID, "subscribers", n, "replayed", len(past))
	return ch
}

// Unsubscribe removes the channel from the document's subscriber list.
func (h *Hub) Unsubscribe(documentID string, ch chan StatusEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	subs := h.subs[documentID]
	for i, c := range subs {
		if c == ch {
			h.subs[documentID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	if len(h.subs[documentID]) == 0 {
		delete(h.subs, documentID)
		delete(h.replay, documentID)
	}
	h.logger.Debug("unsubscribe", "document_id", documentID, "subscribers", len(h.subs[documentID]))
}

// Publish sends an event to all subscribers for the given document ID.
// The event is also appended to the replay buffer so late subscribers receive it.
// Slow subscribers are skipped (non-blocking send).
func (h *Hub) Publish(documentID string, event StatusEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.replay[documentID] = append(h.replay[documentID], event)
	for _, ch := range h.subs[documentID] {
		select {
		case ch <- event:
		default:
			h.logger.Warn("dropping event, slow subscriber",
				"document_id", documentID,
				"status", event.Status,
			)
		}
	}
}
