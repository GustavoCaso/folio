package hub_test

import (
	"encoding/json"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/hub"
)

func TestPublishReachesSubscriber(t *testing.T) {
	h := hub.New()
	ch := h.Subscribe("job-1")
	defer h.Unsubscribe("job-1", ch)

	event := hub.StatusEvent{Status: "PROCESSING", PagesDone: 5, PagesTotal: 100}
	h.Publish("job-1", event)

	select {
	case got := <-ch:
		if got.Status != "PROCESSING" {
			t.Fatalf("expected PROCESSING, got %s", got.Status)
		}
		if got.PagesDone != 5 {
			t.Fatalf("expected PagesDone 5, got %d", got.PagesDone)
		}
	default:
		t.Fatal("expected event on channel, got nothing")
	}
}

func TestPublishDoesNotReachUnrelatedJob(t *testing.T) {
	h := hub.New()
	ch := h.Subscribe("job-1")
	defer h.Unsubscribe("job-1", ch)

	h.Publish("job-2", hub.StatusEvent{Status: "DONE"})

	select {
	case <-ch:
		t.Fatal("received unexpected event for unrelated job")
	default:
		// correct: nothing received
	}
}

func TestPublishCarriesStageAndMessage(t *testing.T) {
	h := hub.New()
	ch := h.Subscribe("job-3")
	defer h.Unsubscribe("job-3", ch)

	h.Publish("job-3", hub.StatusEvent{
		Status:     "PROCESSING",
		Stage:      "processing",
		Message:    "converted page 3/10",
		PagesDone:  3,
		PagesTotal: 10,
	})

	select {
	case got := <-ch:
		if got.Stage != "processing" {
			t.Fatalf("expected stage processing, got %q", got.Stage)
		}
		if got.Message != "converted page 3/10" {
			t.Fatalf("unexpected message %q", got.Message)
		}
	default:
		t.Fatal("expected event on channel")
	}
}

func TestStatusEventJSONShape(t *testing.T) {
	evt := hub.StatusEvent{
		Status:     "PROCESSING",
		PagesDone:  2,
		PagesTotal: 5,
		Stage:      "loading",
		Message:    "loading document",
	}
	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"Status", "PagesDone", "PagesTotal", "Stage", "Message"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("missing JSON field %q in %s", key, data)
		}
	}
}

func TestUnsubscribeStopsDelivery(t *testing.T) {
	h := hub.New()
	ch := h.Subscribe("job-1")
	h.Unsubscribe("job-1", ch)

	h.Publish("job-1", hub.StatusEvent{Status: "DONE"})

	select {
	case <-ch:
		t.Fatal("received event after unsubscribe")
	default:
		// correct
	}
}
