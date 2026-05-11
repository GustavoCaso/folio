package handlers_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/export"
	"github.com/GustavoCaso/folio/ui/internal/handlers"
	"github.com/GustavoCaso/folio/ui/internal/hub"
)

// recordingBackend implements export.Backend and records calls for assertions.
type recordingBackend struct {
	name     string
	deleteCh chan string
}

func (r *recordingBackend) Name() string { return r.name }

func (r *recordingBackend) Export(_ context.Context, _ []db.ExportRecord) ([]export.ExportResult, error) {
	return nil, nil
}

func (r *recordingBackend) Delete(_ context.Context, externalID string) error {
	if r.deleteCh != nil {
		r.deleteCh <- externalID
	}
	return nil
}

func newTestMuxWithBackends(t *testing.T, store *db.Store, backends ...export.Backend) (http.Handler, *hub.Hub) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := hub.New(logger)
	if err != nil {
		t.Fatal(err)
	}
	mux, err := handlers.Register(store, h, nil, t.TempDir(), logger, backends)
	if err != nil {
		t.Fatal(err)
	}
	return mux, h
}

func TestListExports_Empty(t *testing.T) {
	store := newTestStore(t)
	req := httptest.NewRequest(http.MethodGet, "/exports", nil)
	rec := httptest.NewRecorder()

	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Export History") {
		t.Errorf("expected Export History heading in response")
	}
	if !strings.Contains(body, "No exports yet") {
		t.Errorf("expected empty-state message in response, got:\n%s", body)
	}
}

func TestListExports_ShowsExportedRecord(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	job, err := store.CreateJob(ctx, "paper.pdf", []byte{}, "req-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkJobDone(ctx, job.ID, "/data/paper.md"); err != nil {
		t.Fatal(err)
	}
	h, err := store.CreateHighlight(ctx, db.Highlight{
		JobID:        job.ID,
		StartBlockID: "p-1",
		EndBlockID:   "p-1",
		StartPos:     0,
		EndPos:       4,
		Text:         "some important text",
		Tag:          "research",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateHighlightExport(ctx, h.ID, "readwise"); err != nil {
		t.Fatal(err)
	}
	records, err := store.ListUnexportedHighlights(ctx, "readwise")
	if err != nil || len(records) == 0 {
		t.Fatal("expected pending export record")
	}
	if err := store.MarkHighlightExported(ctx, records[0].ID, "ext-123"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/exports", nil)
	rec := httptest.NewRecorder()

	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "some important text") {
		t.Errorf("expected highlight text in response, got:\n%s", body)
	}
	if !strings.Contains(body, "paper.pdf") {
		t.Errorf("expected document filename in response, got:\n%s", body)
	}
	if !strings.Contains(body, "readwise") {
		t.Errorf("expected backend name in response, got:\n%s", body)
	}
	if !strings.Contains(body, "Exported") {
		t.Errorf("expected Exported status badge in response, got:\n%s", body)
	}
}

func TestListExports_ShowsFailedRecord(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	job, err := store.CreateJob(ctx, "doc.pdf", []byte{}, "req-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkJobDone(ctx, job.ID, "/data/doc.md"); err != nil {
		t.Fatal(err)
	}
	h, err := store.CreateHighlight(ctx, db.Highlight{
		JobID: job.ID, StartBlockID: "p-1", EndBlockID: "p-1",
		StartPos: 0, EndPos: 4, Text: "highlight text",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateHighlightExport(ctx, h.ID, "readwise"); err != nil {
		t.Fatal(err)
	}
	expRecords, err := store.ListUnexportedHighlights(ctx, "readwise")
	if err != nil || len(expRecords) == 0 {
		t.Fatal("expected pending export record")
	}
	if err := store.MarkHighlightExportFailed(ctx, expRecords[0].ID, "connection refused"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/exports", nil)
	rec := httptest.NewRecorder()

	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "FAILED") {
		t.Errorf("expected FAILED status badge in response, got:\n%s", body)
	}
	if !strings.Contains(body, "connection refused") {
		t.Errorf("expected error message in response, got:\n%s", body)
	}
}

func TestDeleteHighlight_CallsBackendDelete(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, h := seedJobAndHighlight(t, store)

	// Record that this highlight was exported to a fake backend.
	if err := store.CreateHighlightExport(ctx, h.ID, "fake"); err != nil {
		t.Fatal(err)
	}
	fakeRecords, err := store.ListUnexportedHighlights(ctx, "fake")
	if err != nil || len(fakeRecords) == 0 {
		t.Fatal("expected pending export record for fake backend")
	}
	if err := store.MarkHighlightExported(ctx, fakeRecords[0].ID, "ext-999"); err != nil {
		t.Fatal(err)
	}

	// A Backend that records delete calls.
	deleted := make(chan string, 1)
	fake := &recordingBackend{name: "fake", deleteCh: deleted}

	mux, _ := newTestMuxWithBackends(t, store, fake)

	req := httptest.NewRequest(http.MethodDelete, "/highlights/"+h.ID, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	select {
	case id := <-deleted:
		if id != "ext-999" {
			t.Errorf("expected delete called with ext-999, got %s", id)
		}
	default:
		t.Error("expected backend.Delete to be called, but it wasn't")
	}
}
