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
	var created domain.Document

	err = database.WithTx(ctx, func(ctx context.Context, tx repository.Store) error {
		var err error
		created, err = tx.CreateDocument(ctx, "book.epub", content, "req-1", domain.EpubFormat)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := database.GetDocument(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Filename != "book.epub" {
		t.Fatalf("expected book.epub, got %s", got.Filename)
	}
}

func TestWithTx_RollsBackOnError(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	ctx := context.Background()
	var documentID string

	err = database.WithTx(ctx, func(ctx context.Context, tx repository.Store) error {
		document, err := tx.CreateDocument(ctx, "book.epub", content, "req-1", domain.EpubFormat)
		if err != nil {
			return err
		}
		documentID = document.ID
		return errors.New("something went wrong")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	_, err = database.GetDocument(ctx, documentID)
	if err == nil {
		t.Fatal("expected document to be absent after rollback, got nil error")
	}
}

func TestWithTx_MultipleOpsAtomic(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	ctx := context.Background()
	var documentID string

	// Transaction creates a document and a highlight; fails after the highlight —
	// both should be rolled back.
	err = database.WithTx(ctx, func(ctx context.Context, tx repository.Store) error {
		document, err := tx.CreateDocument(ctx, "book.epub", content, "req-1", domain.EpubFormat)
		if err != nil {
			return err
		}
		documentID = document.ID
		if _, err := tx.CreateHighlight(ctx, domain.Highlight{
			DocumentID: document.ID, StartBlockID: "p-1", EndBlockID: "p-1",
			StartPos: 0, EndPos: 5, Text: "hello",
		}); err != nil {
			return err
		}
		return errors.New("abort")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	_, err = database.GetDocument(ctx, documentID)
	if err == nil {
		t.Fatal("expected document to be absent after rollback")
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
