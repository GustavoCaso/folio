package hub_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/hub"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

func newHub(t *testing.T) *hub.Hub {
	t.Helper()
	h, err := hub.New(testLogger)
	if err != nil {
		t.Fatalf("hub.New: %v", err)
	}
	return h
}

func TestNewRequiresLogger(t *testing.T) {
	if _, err := hub.New(nil); err == nil {
		t.Fatal("expected error for nil logger")
	}
}

func TestPublishReachesSubscriber(t *testing.T) {
	h := newHub(t)
	ch := h.Subscribe("job-1")
	defer h.Unsubscribe("job-1", ch)

	event := hub.StatusEvent{Status: "PROCESSING", Message: "progress"}
	h.Publish("job-1", event)

	select {
	case got := <-ch:
		if got.Status != "PROCESSING" {
			t.Fatalf("expected PROCESSING, got %s", got.Status)
		}
		if got.Message != "progress" {
			t.Fatalf("expected Message 'progress', got %q", got.Message)
		}
	default:
		t.Fatal("expected event on channel, got nothing")
	}
}

func TestPublishDoesNotReachUnrelatedJob(t *testing.T) {
	h := newHub(t)
	ch := h.Subscribe("job-1")
	defer h.Unsubscribe("job-1", ch)

	h.Publish("job-2", hub.StatusEvent{Status: "DONE"})

	select {
	case <-ch:
		t.Fatal("received unexpected event for unrelated job")
	default:
	}
}

func TestPublishCarriesMessage(t *testing.T) {
	h := newHub(t)
	ch := h.Subscribe("job-3")
	defer h.Unsubscribe("job-3", ch)

	h.Publish("job-3", hub.StatusEvent{
		Status:  "PROCESSING",
		Message: "converted page 3/10",
	})

	select {
	case got := <-ch:
		if got.Message != "converted page 3/10" {
			t.Fatalf("unexpected message %q", got.Message)
		}
	default:
		t.Fatal("expected event on channel")
	}
}

func TestStatusEventJSONShape(t *testing.T) {
	evt := hub.StatusEvent{
		Status:  "PROCESSING",
		Message: "loading document",
	}
	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"Status", "Message"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("missing JSON field %q in %s", key, data)
		}
	}
}

func TestSubscribeReplaysEventsPublishedBeforeConnect(t *testing.T) {
	h := newHub(t)

	h.Publish("job-r", hub.StatusEvent{Status: "PROCESSING", Message: "batch 1"})
	h.Publish("job-r", hub.StatusEvent{Status: "PROCESSING", Message: "batch 2"})

	ch := h.Subscribe("job-r")
	defer h.Unsubscribe("job-r", ch)

	for _, want := range []string{"batch 1", "batch 2"} {
		select {
		case got := <-ch:
			if got.Message != want {
				t.Fatalf("expected %q, got %q", want, got.Message)
			}
		default:
			t.Fatalf("expected replayed event %q, got nothing", want)
		}
	}
}

func TestReplayBufferClearedOnLastUnsubscribe(t *testing.T) {
	h := newHub(t)

	h.Publish("job-c", hub.StatusEvent{Status: "PROCESSING", Message: "batch 1"})
	ch := h.Subscribe("job-c")
	h.Unsubscribe("job-c", ch)

	// New subscriber after cleanup should get no replayed events.
	ch2 := h.Subscribe("job-c")
	defer h.Unsubscribe("job-c", ch2)

	select {
	case got := <-ch2:
		t.Fatalf("expected no events after cleanup, got %+v", got)
	default:
	}
}

func TestUnsubscribeStopsDelivery(t *testing.T) {
	h := newHub(t)
	ch := h.Subscribe("job-1")
	h.Unsubscribe("job-1", ch)

	h.Publish("job-1", hub.StatusEvent{Status: "DONE"})

	select {
	case <-ch:
		t.Fatal("received event after unsubscribe")
	default:
	}
}
