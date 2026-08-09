package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/domain"
	"github.com/GustavoCaso/folio/ui/internal/repository"
)

func TestWithTx_CommitsOnSuccess(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	ctx := context.Background()
	var created domain.Job

	err = database.WithTx(ctx, func(ctx context.Context, tx repository.Store) error {
		var err error
		created, err = tx.CreateJob(ctx, "book.pdf", content, "req-1", domain.PdfFormat)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := database.GetJob(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Filename != "book.pdf" {
		t.Fatalf("expected book.pdf, got %s", got.Filename)
	}
}

func TestWithTx_RollsBackOnError(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	ctx := context.Background()
	var jobID string

	err = database.WithTx(ctx, func(ctx context.Context, tx repository.Store) error {
		job, err := tx.CreateJob(ctx, "book.pdf", content, "req-1", domain.PdfFormat)
		if err != nil {
			return err
		}
		jobID = job.ID
		return errors.New("something went wrong")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	_, err = database.GetJob(ctx, jobID)
	if err == nil {
		t.Fatal("expected job to be absent after rollback, got nil error")
	}
}

func TestWithTx_MultipleOpsAtomic(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	ctx := context.Background()
	var jobID string

	// Transaction creates a job and a highlight; fails after the highlight —
	// both should be rolled back.
	err = database.WithTx(ctx, func(ctx context.Context, tx repository.Store) error {
		job, err := tx.CreateJob(ctx, "book.pdf", content, "req-1", domain.PdfFormat)
		if err != nil {
			return err
		}
		jobID = job.ID
		if _, err := tx.CreateHighlight(ctx, domain.Highlight{
			JobID: job.ID, StartBlockID: "p-1", EndBlockID: "p-1",
			StartPos: 0, EndPos: 5, Text: "hello",
		}); err != nil {
			return err
		}
		return errors.New("abort")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	_, err = database.GetJob(ctx, jobID)
	if err == nil {
		t.Fatal("expected job to be absent after rollback")
	}
}

func TestWithTx_NestedReturnsError(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	err = database.WithTx(ctx, func(ctx context.Context, tx repository.Store) error {
		return tx.WithTx(ctx, func(ctx context.Context, inner repository.Store) error {
			return nil
		})
	})
	if err == nil {
		t.Fatal("expected error for nested transaction, got nil")
	}

	if err.Error() != "db.WithTx: cannot start a transaction inside a transaction" {
		t.Fatalf("expected error for nested transaction, got %s", err.Error())
	}
}
