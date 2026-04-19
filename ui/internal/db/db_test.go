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
	defer store.Close()

	job, err := store.CreateJob(context.Background(), "book.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if job.ID == "" {
		t.Fatal("expected non-empty job ID")
	}
	if job.Status != "PENDING" {
		t.Fatalf("expected PENDING, got %s", job.Status)
	}

	got, err := store.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Filename != "book.pdf" {
		t.Fatalf("expected book.pdf, got %s", got.Filename)
	}
}

func TestUpdateJobProgress(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	job, _ := store.CreateJob(context.Background(), "book.pdf")
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

func TestMarkJobDone(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	job, _ := store.CreateJob(context.Background(), "book.pdf")
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
	defer store.Close()

	job, _ := store.CreateJob(context.Background(), "book.pdf")

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

func TestDeleteHighlight(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	job, _ := store.CreateJob(context.Background(), "book.pdf")
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
