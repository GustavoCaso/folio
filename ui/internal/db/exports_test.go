package db_test

import (
	"context"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/db"
)

// seedJobAndHighlightForExport creates a DONE job and one highlight for export tests.
func seedJobAndHighlightForExport(t *testing.T, store *db.Store) (db.Job, db.Highlight) {
	t.Helper()
	job, err := store.CreateJob(context.Background(), "book.pdf", []byte{1}, "req")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkJobDone(context.Background(), job.ID, "/data/book.md"); err != nil {
		t.Fatal(err)
	}
	h, err := store.CreateHighlight(context.Background(), db.Highlight{
		JobID:        job.ID,
		StartBlockID: "p-1",
		EndBlockID:   "p-1",
		StartPos:     0,
		EndPos:       5,
		Text:         "hello world",
		Tag:          "important",
	})
	if err != nil {
		t.Fatal(err)
	}
	return job, h
}

func TestListUnexportedHighlightsWithJob_AllUnexported(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	_, h := seedJobAndHighlightForExport(t, store)

	highlights, err := store.ListUnexportedHighlightsWithJob(context.Background(), "readwise")
	if err != nil {
		t.Fatal(err)
	}
	if len(highlights) != 1 {
		t.Fatalf("expected 1 unexported highlight, got %d", len(highlights))
	}
	if highlights[0].ID != h.ID {
		t.Errorf("expected highlight %s, got %s", h.ID, highlights[0].ID)
	}
	if highlights[0].JobFilename != "book.pdf" {
		t.Errorf("expected book.pdf, got %s", highlights[0].JobFilename)
	}
}

func TestListUnexportedHighlightsWithJob_MarkedExportedIsExcluded(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	_, h := seedJobAndHighlightForExport(t, store)

	if err := store.MarkHighlightExported(context.Background(), h.ID, "readwise", "ext-1"); err != nil {
		t.Fatal(err)
	}

	highlights, err := store.ListUnexportedHighlightsWithJob(context.Background(), "readwise")
	if err != nil {
		t.Fatal(err)
	}
	if len(highlights) != 0 {
		t.Fatalf("expected 0 after marking exported, got %d", len(highlights))
	}
}

func TestListUnexportedHighlightsWithJob_FailedIsRetried(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	_, h := seedJobAndHighlightForExport(t, store)

	if err := store.MarkHighlightExportFailed(context.Background(), h.ID, "readwise", "timeout"); err != nil {
		t.Fatal(err)
	}

	// A failed export has no 'exported' row, so the highlight should still appear.
	highlights, err := store.ListUnexportedHighlightsWithJob(context.Background(), "readwise")
	if err != nil {
		t.Fatal(err)
	}
	if len(highlights) != 1 {
		t.Fatalf("expected 1 (failed should be retried), got %d", len(highlights))
	}
	_ = h
}

func TestListUnexportedHighlightsWithJob_IsolatedByBackend(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	_, h := seedJobAndHighlightForExport(t, store)

	if err := store.MarkHighlightExported(context.Background(), h.ID, "readwise", "ext-1"); err != nil {
		t.Fatal(err)
	}

	// A different backend should still see the highlight as unexported.
	highlights, err := store.ListUnexportedHighlightsWithJob(context.Background(), "other-backend")
	if err != nil {
		t.Fatal(err)
	}
	if len(highlights) != 1 {
		t.Fatalf("expected 1 for other-backend, got %d", len(highlights))
	}
}

func TestMarkHighlightExported(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	_, h := seedJobAndHighlightForExport(t, store)

	if err := store.MarkHighlightExported(context.Background(), h.ID, "readwise", "ext-999"); err != nil {
		t.Fatal(err)
	}

	records, err := store.ListExportsByHighlight(context.Background(), h.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	rec := records[0]
	if rec.Status != "exported" {
		t.Errorf("expected status exported, got %s", rec.Status)
	}
	if rec.ExternalID != "ext-999" {
		t.Errorf("expected external_id ext-999, got %s", rec.ExternalID)
	}
	if rec.BackendName != "readwise" {
		t.Errorf("expected backend readwise, got %s", rec.BackendName)
	}
	if rec.Error != "" {
		t.Errorf("expected empty error, got %q", rec.Error)
	}
	if rec.ExportedAt.IsZero() {
		t.Error("expected non-zero exported_at")
	}
}

func TestMarkHighlightExportFailed(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	_, h := seedJobAndHighlightForExport(t, store)

	if err := store.MarkHighlightExportFailed(context.Background(), h.ID, "readwise", "connection refused"); err != nil {
		t.Fatal(err)
	}

	records, err := store.ListExportsByHighlight(context.Background(), h.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	rec := records[0]
	if rec.Status != "failed" {
		t.Errorf("expected status failed, got %s", rec.Status)
	}
	if rec.Error != "connection refused" {
		t.Errorf("expected error %q, got %q", "connection refused", rec.Error)
	}
	if rec.ExternalID != "" {
		t.Errorf("expected empty external_id for failed, got %q", rec.ExternalID)
	}
}

func TestListAllExports_JoinsHighlightAndJob(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	_, h := seedJobAndHighlightForExport(t, store)

	if err := store.MarkHighlightExported(context.Background(), h.ID, "readwise", "ext-1"); err != nil {
		t.Fatal(err)
	}

	records, err := store.ListAllExports(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	rec := records[0]
	if rec.HighlightText != "hello world" {
		t.Errorf("expected highlight text %q, got %q", "hello world", rec.HighlightText)
	}
	if rec.JobFilename != "book.pdf" {
		t.Errorf("expected job filename book.pdf, got %s", rec.JobFilename)
	}
	if rec.Status != "exported" {
		t.Errorf("expected status exported, got %s", rec.Status)
	}
}

func TestListAllExports_ReturnsAllRecords(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	_, h1 := seedJobAndHighlightForExport(t, store)

	// Second highlight on the same job.
	h2, err := store.CreateHighlight(context.Background(), db.Highlight{
		JobID: h1.JobID, StartBlockID: "p-2", EndBlockID: "p-2",
		StartPos: 0, EndPos: 3, Text: "bye",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := store.MarkHighlightExported(context.Background(), h1.ID, "readwise", "ext-1"); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkHighlightExported(context.Background(), h2.ID, "readwise", "ext-2"); err != nil {
		t.Fatal(err)
	}

	records, err := store.ListAllExports(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	seen := map[string]bool{}
	for _, r := range records {
		seen[r.ExternalID] = true
	}
	if !seen["ext-1"] || !seen["ext-2"] {
		t.Errorf("expected both ext-1 and ext-2 in results, got %v", seen)
	}
}

func TestExportRecord_CascadesOnHighlightDelete(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	_, h := seedJobAndHighlightForExport(t, store)

	if err := store.MarkHighlightExported(context.Background(), h.ID, "readwise", "ext-1"); err != nil {
		t.Fatal(err)
	}

	if err := store.DeleteHighlight(context.Background(), h.ID); err != nil {
		t.Fatal(err)
	}

	records, err := store.ListAllExports(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 export records after highlight delete, got %d", len(records))
	}
}
