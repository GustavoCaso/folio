package db_test

import (
	"context"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/domain"
)

func TestCreateAndListHighlights(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	job, _ := database.CreateJob(context.Background(), "book.pdf", content, "")

	h := domain.Highlight{
		JobID:        job.ID,
		StartBlockID: "introduction",
		EndBlockID:   "introduction",
		StartPos:     10,
		EndPos:       50,
		Text:         "selected text",
		Tag:          "important",
		Note:         "my note",
	}

	created, err := database.CreateHighlight(context.Background(), h)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" {
		t.Fatal("expected non-empty highlight ID")
	}

	highlights, err := database.ListHighlights(context.Background(), job.ID)
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
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	job, err := database.CreateJob(context.Background(), "book.pdf", content, "")
	if err != nil {
		t.Fatal(err)
	}
	h, err := database.CreateHighlight(context.Background(), domain.Highlight{
		JobID: job.ID, StartBlockID: "intro", EndBlockID: "intro", StartPos: 0, EndPos: 10, Text: "text",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := database.DeleteHighlight(context.Background(), h.ID); err != nil {
		t.Fatal(err)
	}

	highlights, _ := database.ListHighlights(context.Background(), job.ID)
	if len(highlights) != 0 {
		t.Fatalf("expected 0 highlights, got %d", len(highlights))
	}
}

func TestDeleteHighlight_NotFound(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	err = database.DeleteHighlight(context.Background(), "no-such-id")
	if err == nil {
		t.Fatal("expected an error when deleting a non-existent highlight, got nil")
	}
}
