package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/domain"
	"github.com/GustavoCaso/folio/ui/internal/repository"
)

// seedJobAndHighlight creates a job, marks it done, then creates a highlight
// attached to that job. Returns both for further assertions.
func seedJobAndHighlight(t *testing.T, store repository.Store) (domain.Job, domain.Highlight) {
	t.Helper()
	job, err := store.CreateJob(context.Background(), "doc.pdf", []byte{}, "req-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkJobDone(context.Background(), job.ID, "/dev/null", "", "", nil); err != nil {
		t.Fatal(err)
	}
	h, err := store.CreateHighlight(context.Background(), domain.Highlight{
		JobID:        job.ID,
		StartBlockID: "paragraph-1",
		EndBlockID:   "paragraph-1",
		StartPos:     0,
		EndPos:       5,
		Text:         "hello",
		Tag:          "important",
		Note:         "remember",
	})
	if err != nil {
		t.Fatal(err)
	}
	return job, h
}

func TestCreateHighlight_HappyPath(t *testing.T) {
	store := newTestStore(t)
	job, err := store.CreateJob(context.Background(), "doc.pdf", []byte{}, "req-1")
	if err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(map[string]any{
		"job_id":         job.ID,
		"start_block_id": "paragraph-1",
		"end_block_id":   "paragraph-1",
		"start_pos":      0,
		"end_pos":        5,
		"text":           "hello",
		"tag":            "important",
		"note":           "a note",
	})
	req := httptest.NewRequest(http.MethodPost, "/highlights", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("expected application/json Content-Type, got %q", ct)
	}

	var got map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("could not decode response: %v", err)
	}
	if got["ID"] == "" || got["ID"] == nil {
		t.Errorf("expected non-empty ID in response, got %v", got)
	}
	if got["Text"] != "hello" {
		t.Errorf("expected Text=hello, got %v", got["Text"])
	}
}

func TestCreateHighlight_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/highlights", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, newTestStore(t))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("expected application/json Content-Type, got %q", ct)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("expected JSON error body, got: %s", rec.Body.String())
	}
	if body["error"] == "" {
		t.Errorf("expected non-empty 'error' key in response, got %v", body)
	}
}

func TestDeleteHighlight_HappyPath(t *testing.T) {
	store := newTestStore(t)
	_, h := seedJobAndHighlight(t, store)

	req := httptest.NewRequest(http.MethodDelete, "/highlights/"+h.ID, nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("expected text/html Content-Type, got %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "Highlight deleted") {
		t.Errorf("expected success toast in response, got:\n%s", rec.Body.String())
	}
}

func TestDeleteHighlight_RemovesFromStore(t *testing.T) {
	store := newTestStore(t)
	job, h := seedJobAndHighlight(t, store)

	req := httptest.NewRequest(http.MethodDelete, "/highlights/"+h.ID, nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	highlights, err := store.ListHighlights(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(highlights) != 0 {
		t.Errorf("expected highlight to be deleted, got %d remaining", len(highlights))
	}
}

func TestDeleteHighlight_UnknownID_ReturnsErrorToast(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/highlights/no-such-id", nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, newTestStore(t))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Fail to delete highlight") {
		t.Errorf("expected error toast in response, got:\n%s", rec.Body.String())
	}
}
