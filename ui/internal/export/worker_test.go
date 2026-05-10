package export_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/export"
)

// fakeBackend is a controllable Backend for testing the Worker.
type fakeBackend struct {
	name      string
	results   []export.ExportResult
	exportErr error
	deleted   []string
}

func (f *fakeBackend) Name() string { return f.name }

func (f *fakeBackend) Export(_ context.Context, highlights []db.HighlightWithJob) ([]export.ExportResult, error) {
	if f.exportErr != nil {
		return nil, f.exportErr
	}
	// Return preset results matched by position; fall back to a generated ID.
	out := make([]export.ExportResult, len(highlights))
	for i, h := range highlights {
		if i < len(f.results) {
			out[i] = f.results[i]
		} else {
			out[i] = export.ExportResult{HighlightID: h.ID, ExternalID: "ext-" + h.ID}
		}
	}
	return out, nil
}

func (f *fakeBackend) Delete(_ context.Context, externalID string) error {
	f.deleted = append(f.deleted, externalID)
	return nil
}

// newWorkerStore creates an in-memory store seeded with a DONE job and one highlight.
func newWorkerStore(t *testing.T) (*db.Store, db.Highlight) {
	t.Helper()
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	job, err := store.CreateJob(context.Background(), "book.pdf", []byte{1}, "req")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkJobDone(context.Background(), job.ID, "/data/book.md"); err != nil {
		t.Fatal(err)
	}
	h, err := store.CreateHighlight(context.Background(), db.Highlight{
		JobID: job.ID, StartBlockID: "p-1", EndBlockID: "p-1",
		StartPos: 0, EndPos: 5, Text: "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	return store, h
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func runWorkerOnce(w *export.Worker) {
	w.RunOnce(context.Background())
}

func TestWorker_SuccessfulExportMarksHighlight(t *testing.T) {
	store, h := newWorkerStore(t)
	fake := &fakeBackend{
		name:    "readwise",
		results: []export.ExportResult{{HighlightID: h.ID, ExternalID: "ext-42"}},
	}

	w := export.NewWorker(store, []export.Backend{fake}, time.Hour, silentLogger())
	runWorkerOnce(w)

	records, err := store.ListExportsByHighlight(context.Background(), h.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 export record, got %d", len(records))
	}
	if records[0].Status != "exported" {
		t.Errorf("expected status exported, got %s", records[0].Status)
	}
	if records[0].ExternalID != "ext-42" {
		t.Errorf("expected external_id ext-42, got %s", records[0].ExternalID)
	}
}

func TestWorker_BackendErrorMarksAllAsFailed(t *testing.T) {
	store, h := newWorkerStore(t)
	fake := &fakeBackend{
		name:      "readwise",
		exportErr: errors.New("network timeout"),
	}

	w := export.NewWorker(store, []export.Backend{fake}, time.Hour, silentLogger())
	runWorkerOnce(w)

	records, err := store.ListExportsByHighlight(context.Background(), h.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 export record, got %d", len(records))
	}
	if records[0].Status != "failed" {
		t.Errorf("expected status failed, got %s", records[0].Status)
	}
	if records[0].Error != "network timeout" {
		t.Errorf("expected error %q, got %q", "network timeout", records[0].Error)
	}
}

func TestWorker_PerResultErrorMarksAsFailed(t *testing.T) {
	store, h := newWorkerStore(t)
	fake := &fakeBackend{
		name: "readwise",
		results: []export.ExportResult{
			{HighlightID: h.ID, Err: errors.New("bad highlight")},
		},
	}

	w := export.NewWorker(store, []export.Backend{fake}, time.Hour, silentLogger())
	runWorkerOnce(w)

	records, err := store.ListExportsByHighlight(context.Background(), h.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 export record, got %d", len(records))
	}
	if records[0].Status != "failed" {
		t.Errorf("expected status failed, got %s", records[0].Status)
	}
}

func TestWorker_AlreadyExportedIsSkipped(t *testing.T) {
	store, h := newWorkerStore(t)

	if err := store.MarkHighlightExported(context.Background(), h.ID, "readwise", "ext-1"); err != nil {
		t.Fatal(err)
	}

	callCount := 0
	fake := &fakeBackend{name: "readwise"}
	origExport := fake.Export
	_ = origExport // not called; callCount stays 0

	w := export.NewWorker(store, []export.Backend{fake}, time.Hour, silentLogger())
	runWorkerOnce(w)

	// Verify no new records were added.
	records, err := store.ListExportsByHighlight(context.Background(), h.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Should still be exactly 1 (the one we seeded).
	if len(records) != 1 {
		t.Errorf("expected 1 record (no duplicate), got %d", len(records))
	}
	_ = callCount
}

func TestWorker_MultipleBackendsRunIndependently(t *testing.T) {
	store, h := newWorkerStore(t)
	a := &fakeBackend{name: "backend-a"}
	b := &fakeBackend{name: "backend-b"}

	w := export.NewWorker(store, []export.Backend{a, b}, time.Hour, silentLogger())
	runWorkerOnce(w)

	// Both backends should have an exported record.
	records, err := store.ListExportsByHighlight(context.Background(), h.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records (one per backend), got %d", len(records))
	}
	_ = h
}
