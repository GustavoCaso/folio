package db_test

import (
	"context"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/db"
)

func TestCreateAndGetJob(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	job, err := store.CreateJob(context.Background(), "book.pdf", "req-123")
	if err != nil {
		t.Fatal(err)
	}
	if job.ID == "" {
		t.Fatal("expected non-empty job ID")
	}
	if job.Status != "PENDING" {
		t.Fatalf("expected PENDING, got %s", job.Status)
	}
	if job.RequestID != "req-123" {
		t.Fatalf("expected request_id req-123, got %q", job.RequestID)
	}

	got, err := store.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Filename != "book.pdf" {
		t.Fatalf("expected book.pdf, got %s", got.Filename)
	}
	if got.RequestID != "req-123" {
		t.Fatalf("expected request_id req-123 after read, got %q", got.RequestID)
	}
}

func TestUpdateJobProgress(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	job, _ := store.CreateJob(context.Background(), "book.pdf", "")
	if err := store.UpdateJobProgress(context.Background(), job.ID, "PROCESSING", 5, 100); err != nil {
		t.Fatal(err)
	}

	got, _ := store.GetJob(context.Background(), job.ID)
	if got.Status != "PROCESSING" {
		t.Fatalf("expected PROCESSING, got %s", got.Status)
	}
	if got.PagesDone != 5 || got.PagesTotal != 100 {
		t.Fatalf("unexpected progress: %d/%d", got.PagesDone, got.PagesTotal)
	}
}

func TestGetPendingJobs(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()

	pending1, _ := store.CreateJob(ctx, "a.pdf", "")
	pending2, _ := store.CreateJob(ctx, "b.pdf", "")
	processing, _ := store.CreateJob(ctx, "c.pdf", "")
	done, _ := store.CreateJob(ctx, "d.pdf", "")
	failed, _ := store.CreateJob(ctx, "e.pdf", "")

	if err := store.UpdateJobProgress(ctx, processing.ID, "PROCESSING", 1, 10); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkJobDone(ctx, done.ID, "/data/d.md"); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkJobFailed(ctx, failed.ID, "boom"); err != nil {
		t.Fatal(err)
	}

	jobs, err := store.GetPendingJobs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 pending jobs, got %d", len(jobs))
	}

	got := map[string]bool{}
	for _, j := range jobs {
		if j.Status != "PENDING" {
			t.Errorf("expected PENDING, got %s for %s", j.Status, j.ID)
		}
		got[j.ID] = true
	}
	if !got[pending1.ID] || !got[pending2.ID] {
		t.Fatalf("missing pending jobs in result: %v", got)
	}
}

func TestGetPendingJobsEmpty(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	jobs, err := store.GetPendingJobs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 0 {
		t.Fatalf("expected 0 pending jobs, got %d", len(jobs))
	}
}

func TestMarkJobDone(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	job, _ := store.CreateJob(context.Background(), "book.pdf", "")
	if err := store.MarkJobDone(context.Background(), job.ID, "/data/book.md"); err != nil {
		t.Fatal(err)
	}

	got, _ := store.GetJob(context.Background(), job.ID)
	if got.Status != "DONE" {
		t.Fatalf("expected DONE, got %s", got.Status)
	}
	if got.OutputPath != "/data/book.md" {
		t.Fatalf("expected /data/book.md, got %s", got.OutputPath)
	}
}

func TestCreateAndListHighlights(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	job, _ := store.CreateJob(context.Background(), "book.pdf", "")

	h := db.Highlight{
		JobID:        job.ID,
		StartBlockID: "introduction",
		EndBlockID:   "introduction",
		StartPos:     10,
		EndPos:       50,
		Text:         "selected text",
		Tag:          "important",
		Note:         "my note",
	}

	created, err := store.CreateHighlight(context.Background(), h)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" {
		t.Fatal("expected non-empty highlight ID")
	}

	highlights, err := store.ListHighlights(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(highlights) != 1 {
		t.Fatalf("expected 1 highlight, got %d", len(highlights))
	}
	if highlights[0].Text != "selected text" {
		t.Fatalf("expected 'selected text', got %q", highlights[0].Text)
	}
}

func TestDeleteJob(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()

	job, err := store.CreateJob(ctx, "test.pdf", "req-1")
	if err != nil {
		t.Fatal(err)
	}

	if err := store.DeleteJob(ctx, job.ID); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}

	_, err = store.GetJob(ctx, job.ID)
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestDeleteHighlight(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	job, _ := store.CreateJob(context.Background(), "book.pdf", "")
	h, _ := store.CreateHighlight(context.Background(), db.Highlight{
		JobID: job.ID, StartBlockID: "intro", EndBlockID: "intro", StartPos: 0, EndPos: 10, Text: "text",
	})

	if err := store.DeleteHighlight(context.Background(), h.ID); err != nil {
		t.Fatal(err)
	}

	highlights, _ := store.ListHighlights(context.Background(), job.ID)
	if len(highlights) != 0 {
		t.Fatalf("expected 0 highlights, got %d", len(highlights))
	}
}
