package export_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/domain"
	"github.com/GustavoCaso/folio/ui/internal/export"
	"github.com/GustavoCaso/folio/ui/internal/repository"
)

// fakeBackend is a controllable Backend for testing the Worker.
type fakeBackend struct {
	name      string
	exportErr error
	// perRecord maps ExportID -> ExternalID or Err to return.
	perRecord map[string]export.ExportRecord
	deleted   []string
}

func (f *fakeBackend) Name() string { return f.name }

func (f *fakeBackend) Export(_ context.Context, records []*export.ExportRecord) error {
	if f.exportErr != nil {
		return f.exportErr
	}
	for _, rec := range records {
		if override, ok := f.perRecord[rec.ExportID]; ok {
			rec.ExternalID = override.ExternalID
			rec.Err = override.Err
		} else {
			rec.ExternalID = "ext-" + rec.ExportID
		}
	}
	return nil
}

func (f *fakeBackend) Delete(_ context.Context, externalID string) error {
	f.deleted = append(f.deleted, externalID)
	return nil
}

// newWorkerStore creates an in-memory store seeded with a DONE job and one highlight with a PENDING export row.
func newWorkerStore(t *testing.T, backendName string) (repository.Store, domain.Highlight) {
	t.Helper()
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	job, err := database.CreateJob(context.Background(), "book.pdf", []byte{1}, "req")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.MarkJobDone(context.Background(), job.ID, "/data/book.md", "", "", nil); err != nil {
		t.Fatal(err)
	}
	h, err := database.CreateHighlight(context.Background(), domain.Highlight{
		JobID: job.ID, StartBlockID: "p-1", EndBlockID: "p-1",
		StartPos: 0, EndPos: 5, Text: "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.CreateHighlightExport(context.Background(), h.ID, backendName); err != nil {
		t.Fatal(err)
	}
	return database, h
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newWorker(t *testing.T, store repository.ExportRepository, backends []export.Backend) *export.Worker {
	t.Helper()
	w, err := export.NewWorker(store, backends, time.Hour, silentLogger())
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func runWorkerOnce(w *export.Worker) {
	w.RunOnce(context.Background())
}

func TestWorker_SuccessfulExportMarksHighlight(t *testing.T) {
	store, h := newWorkerStore(t, "readwise")

	records, err := store.ListUnexportedHighlights(context.Background(), "readwise")
	if err != nil || len(records) == 0 {
		t.Fatal("expected pending export record")
	}

	fake := &fakeBackend{
		name:      "readwise",
		perRecord: map[string]export.ExportRecord{records[0].ID: {ExternalID: "ext-42"}},
	}

	w := newWorker(t, store, []export.Backend{fake})
	runWorkerOnce(w)

	got, err := store.ListExportsByHighlight(context.Background(), h.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 export record, got %d", len(got))
	}
	if got[0].Status != "EXPORTED" {
		t.Errorf("expected status EXPORTED, got %s", got[0].Status)
	}
	if got[0].ExternalID != "ext-42" {
		t.Errorf("expected external_id ext-42, got %s", got[0].ExternalID)
	}
}

func TestWorker_BackendErrorMarksAllAsFailed(t *testing.T) {
	store, h := newWorkerStore(t, "readwise")
	fake := &fakeBackend{
		name:      "readwise",
		exportErr: errors.New("network timeout"),
	}

	w := newWorker(t, store, []export.Backend{fake})
	runWorkerOnce(w)

	got, err := store.ListExportsByHighlight(context.Background(), h.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 export record, got %d", len(got))
	}
	if got[0].Status != "FAILED" {
		t.Errorf("expected status FAILED, got %s", got[0].Status)
	}
	if got[0].Error != "network timeout" {
		t.Errorf("expected error %q, got %q", "network timeout", got[0].Error)
	}
}

func TestWorker_PerResultErrorMarksAsFailed(t *testing.T) {
	store, h := newWorkerStore(t, "readwise")

	records, err := store.ListUnexportedHighlights(context.Background(), "readwise")
	if err != nil || len(records) == 0 {
		t.Fatal("expected pending export record")
	}

	fake := &fakeBackend{
		name:      "readwise",
		perRecord: map[string]export.ExportRecord{records[0].ID: {Err: errors.New("bad highlight")}},
	}

	w := newWorker(t, store, []export.Backend{fake})
	runWorkerOnce(w)

	got, err := store.ListExportsByHighlight(context.Background(), h.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 export record, got %d", len(got))
	}
	if got[0].Status != "FAILED" {
		t.Errorf("expected status FAILED, got %s", got[0].Status)
	}
}

func TestWorker_EmptyExternalIDWithNoErrMarksAsFailed(t *testing.T) {
	store, h := newWorkerStore(t, "readwise")

	records, err := store.ListUnexportedHighlights(context.Background(), "readwise")
	if err != nil || len(records) == 0 {
		t.Fatal("expected pending export record")
	}

	// Backend returns nil error but does not set ExternalID — violates the contract.
	fake := &fakeBackend{
		name:      "readwise",
		perRecord: map[string]export.ExportRecord{records[0].ID: {ExternalID: ""}},
	}

	w := newWorker(t, store, []export.Backend{fake})
	runWorkerOnce(w)

	got, err := store.ListExportsByHighlight(context.Background(), h.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 export record, got %d", len(got))
	}
	if got[0].Status != "FAILED" {
		t.Errorf("expected status FAILED when ExternalID is empty, got %s", got[0].Status)
	}
}

func TestWorker_AlreadyExportedIsSkipped(t *testing.T) {
	store, h := newWorkerStore(t, "readwise")

	// Mark the PENDING row as exported.
	records, err := store.ListUnexportedHighlights(context.Background(), "readwise")
	if err != nil || len(records) == 0 {
		t.Fatal("expected pending export record")
	}
	if err := store.MarkHighlightExported(context.Background(), records[0].ID, "ext-1"); err != nil {
		t.Fatal(err)
	}

	fake := &fakeBackend{name: "readwise"}
	w := newWorker(t, store, []export.Backend{fake})
	runWorkerOnce(w)

	// Worker should not have called Export (no PENDING rows), so still exactly 1 record.
	got, err := store.ListExportsByHighlight(context.Background(), h.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 record (no duplicate), got %d", len(got))
	}
}

func TestWorker_MultipleBackendsRunIndependently(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	job, err := database.CreateJob(context.Background(), "book.pdf", []byte{1}, "req")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.MarkJobDone(context.Background(), job.ID, "/data/book.md", "", "", nil); err != nil {
		t.Fatal(err)
	}
	h, err := database.CreateHighlight(context.Background(), domain.Highlight{
		JobID: job.ID, StartBlockID: "p-1", EndBlockID: "p-1",
		StartPos: 0, EndPos: 5, Text: "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Create a PENDING export row for each backend.
	for _, name := range []string{"backend-a", "backend-b"} {
		if err := database.CreateHighlightExport(context.Background(), h.ID, name); err != nil {
			t.Fatal(err)
		}
	}

	a := &fakeBackend{name: "backend-a"}
	b := &fakeBackend{name: "backend-b"}

	w := newWorker(t, database, []export.Backend{a, b})
	runWorkerOnce(w)

	got, err := database.ListExportsByHighlight(context.Background(), h.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 records (one per backend), got %d", len(got))
	}
}

func TestNewWorker_RequiresLogger(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	_, err = export.NewWorker(database, nil, time.Hour, nil)
	if err == nil {
		t.Fatal("expected error when logger is nil")
	}
}

func TestNewWorker_RequiresStore(t *testing.T) {
	_, err := export.NewWorker(nil, nil, time.Hour, silentLogger())
	if err == nil {
		t.Fatal("expected error when store is nil")
	}
}

func TestNewWorker_RequiresPositiveInterval(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	_, err = export.NewWorker(database, nil, 0, silentLogger())
	if err == nil {
		t.Fatal("expected error when interval is zero")
	}
}
