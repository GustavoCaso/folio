package hub

import "sync"

// StatusEvent carries a parser status update for a single job.
type StatusEvent struct {
	Status     string
	PagesDone  int
	PagesTotal int
	Error      string
	Stage      string
	Message    string
}

// Hub routes StatusEvents from gRPC stream goroutines to SSE connections.
// Each job ID can have multiple subscribers (e.g. two browser tabs watching the same job).
type Hub struct {
	mu   sync.Mutex
	subs map[string][]chan StatusEvent
}

func New() *Hub {
	return &Hub{subs: make(map[string][]chan StatusEvent)}
}

// Subscribe returns a channel that will receive events for the given job ID.
func (h *Hub) Subscribe(jobID string) chan StatusEvent {
	ch := make(chan StatusEvent, 8)
	h.mu.Lock()
	h.subs[jobID] = append(h.subs[jobID], ch)
	h.mu.Unlock()
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
}

// Publish sends an event to all subscribers for the given job ID.
// Slow subscribers are skipped (non-blocking send).
func (h *Hub) Publish(jobID string, event StatusEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ch := range h.subs[jobID] {
		select {
		case ch <- event:
		default: // subscriber is too slow; drop rather than block
		}
	}
}
