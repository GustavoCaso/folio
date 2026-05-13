package db_test

import (
	"context"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/domain"
	"github.com/GustavoCaso/folio/ui/internal/repository"
)

// seedJobAndHighlightForExport creates a DONE job, one highlight, and a PENDING export row for backendName.
func seedJobAndHighlightForExport(t *testing.T, store repository.Store, backendName string) (domain.Job, domain.Highlight) {
	t.Helper()
	job, err := store.CreateJob(context.Background(), "book.pdf", []byte{1}, "req")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkJobDone(context.Background(), job.ID, "/data/book.md"); err != nil {
		t.Fatal(err)
	}
	h, err := store.CreateHighlight(context.Background(), domain.Highlight{
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
	if err := store.CreateHighlightExport(context.Background(), h.ID, backendName); err != nil {
		t.Fatal(err)
	}
	return job, h
}

// pendingExportID fetches the single PENDING export row ID for a highlight+backend.
func pendingExportID(t *testing.T, store repository.Store, backendName string) string {
	t.Helper()
	records, err := store.ListUnexportedHighlights(context.Background(), backendName)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least one pending export record")
	}
	return records[0].ID
}

func TestListUnexportedHighlights_AllUnexported(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	_, h := seedJobAndHighlightForExport(t, database, "readwise")

	records, err := database.ListUnexportedHighlights(context.Background(), "readwise")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 unexported record, got %d", len(records))
	}
	if records[0].HighlightID != h.ID {
		t.Errorf("expected highlight %s, got %s", h.ID, records[0].HighlightID)
	}
	if records[0].JobFilename != "book.pdf" {
		t.Errorf("expected book.pdf, got %s", records[0].JobFilename)
	}
	if records[0].HighlightText != "hello world" {
		t.Errorf("expected text %q, got %q", "hello world", records[0].HighlightText)
	}
	if records[0].HighlightTag != "important" {
		t.Errorf("expected tag %q, got %q", "important", records[0].HighlightTag)
	}
}

func TestListUnexportedHighlights_MarkedExportedIsExcluded(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	seedJobAndHighlightForExport(t, database, "readwise")
	exportID := pendingExportID(t, database, "readwise")

	if err := database.MarkHighlightExported(context.Background(), exportID, "ext-1"); err != nil {
		t.Fatal(err)
	}

	records, err := database.ListUnexportedHighlights(context.Background(), "readwise")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Fatalf("expected 0 after marking exported, got %d", len(records))
	}
}

func TestListUnexportedHighlights_FailedIsNotRetried(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	seedJobAndHighlightForExport(t, database, "readwise")
	exportID := pendingExportID(t, database, "readwise")

	if err := database.MarkHighlightExportFailed(context.Background(), exportID, "timeout"); err != nil {
		t.Fatal(err)
	}

	// FAILED rows are not PENDING, so they don't appear in ListUnexportedHighlights.
	records, err := database.ListUnexportedHighlights(context.Background(), "readwise")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Fatalf("expected 0 (failed row is not PENDING), got %d", len(records))
	}
}

func TestListUnexportedHighlights_IsolatedByBackend(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	_, h := seedJobAndHighlightForExport(t, database, "readwise")

	// Also create a PENDING row for other-backend.
	if err := database.CreateHighlightExport(context.Background(), h.ID, "other-backend"); err != nil {
		t.Fatal(err)
	}

	exportID := pendingExportID(t, database, "readwise")
	if err := database.MarkHighlightExported(context.Background(), exportID, "ext-1"); err != nil {
		t.Fatal(err)
	}

	// other-backend still has a PENDING row.
	records, err := database.ListUnexportedHighlights(context.Background(), "other-backend")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 for other-backend, got %d", len(records))
	}
}

func TestMarkHighlightExported(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	_, h := seedJobAndHighlightForExport(t, database, "readwise")
	exportID := pendingExportID(t, database, "readwise")

	if err := database.MarkHighlightExported(context.Background(), exportID, "ext-999"); err != nil {
		t.Fatal(err)
	}

	records, err := database.ListExportsByHighlight(context.Background(), h.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	rec := records[0]
	if rec.Status != "EXPORTED" {
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
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	_, h := seedJobAndHighlightForExport(t, database, "readwise")
	exportID := pendingExportID(t, database, "readwise")

	if err := database.MarkHighlightExportFailed(context.Background(), exportID, "connection refused"); err != nil {
		t.Fatal(err)
	}

	records, err := database.ListExportsByHighlight(context.Background(), h.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	rec := records[0]
	if rec.Status != "FAILED" {
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
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	_, h := seedJobAndHighlightForExport(t, database, "readwise")
	exportID := pendingExportID(t, database, "readwise")

	if err := database.MarkHighlightExported(context.Background(), exportID, "ext-1"); err != nil {
		t.Fatal(err)
	}

	records, err := database.ListAllExports(context.Background())
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
	if rec.Status != "EXPORTED" {
		t.Errorf("expected status exported, got %s", rec.Status)
	}
	_ = h
}

func TestListAllExports_ReturnsAllRecords(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	_, h1 := seedJobAndHighlightForExport(t, database, "readwise")
	exportID1 := pendingExportID(t, database, "readwise")

	// Second highlight on the same job.
	h2, err := database.CreateHighlight(context.Background(), domain.Highlight{
		JobID: h1.JobID, StartBlockID: "p-2", EndBlockID: "p-2",
		StartPos: 0, EndPos: 3, Text: "bye",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.CreateHighlightExport(context.Background(), h2.ID, "readwise"); err != nil {
		t.Fatal(err)
	}

	records, err := database.ListUnexportedHighlights(context.Background(), "readwise")
	if err != nil {
		t.Fatal(err)
	}
	var exportID2 string
	for _, r := range records {
		if r.HighlightID == h2.ID {
			exportID2 = r.ID
		}
	}
	if exportID2 == "" {
		t.Fatal("could not find export row for h2")
	}

	if err := database.MarkHighlightExported(context.Background(), exportID1, "ext-1"); err != nil {
		t.Fatal(err)
	}
	if err := database.MarkHighlightExported(context.Background(), exportID2, "ext-2"); err != nil {
		t.Fatal(err)
	}

	all, err := database.ListAllExports(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 records, got %d", len(all))
	}
	seen := map[string]bool{}
	for _, r := range all {
		seen[r.ExternalID] = true
	}
	if !seen["ext-1"] || !seen["ext-2"] {
		t.Errorf("expected both ext-1 and ext-2 in results, got %v", seen)
	}
}

func TestExportRecord_CascadesOnHighlightDelete(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	_, h := seedJobAndHighlightForExport(t, database, "readwise")
	exportID := pendingExportID(t, database, "readwise")

	if err := database.MarkHighlightExported(context.Background(), exportID, "ext-1"); err != nil {
		t.Fatal(err)
	}

	if err := database.DeleteHighlight(context.Background(), h.ID); err != nil {
		t.Fatal(err)
	}

	records, err := database.ListAllExports(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 export records after highlight delete, got %d", len(records))
	}
}
