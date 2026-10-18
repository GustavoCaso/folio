package db_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/domain"
)

var content = []byte{'a', 'b', 'c'}

func TestCreateAndGetDocument(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	document, err := database.CreateDocument(context.Background(), "book.epub", content, "req-123", domain.EpubFormat)
	if err != nil {
		t.Fatal(err)
	}
	if document.ID == "" {
		t.Fatal("expected non-empty document ID")
	}
	if document.Status != "PENDING" {
		t.Fatalf("expected PENDING, got %s", document.Status)
	}
	if document.RequestID != "req-123" {
		t.Fatalf("expected request_id req-123, got %q", document.RequestID)
	}
	if !reflect.DeepEqual(document.Content, content) {
		t.Fatalf("expected content %s, got %q", content, document.Content)
	}

	got, err := database.GetDocument(context.Background(), document.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Filename != "book.epub" {
		t.Fatalf("expected book.epub, got %s", got.Filename)
	}
	if got.RequestID != "req-123" {
		t.Fatalf("expected request_id req-123 after read, got %q", got.RequestID)
	}
	if !reflect.DeepEqual(got.Content, content) {
		t.Fatalf("expected content %s, got %q", content, document.Content)
	}
}

func TestCreateDocument_DefaultsFormatToPDF(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	document, err := database.CreateDocument(context.Background(), "book.epub", content, "req-1", domain.EpubFormat)
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	if document.Format != domain.EpubFormat {
		t.Errorf("Format = %q, want %q", document.Format, domain.EpubFormat)
	}
}

func TestUpdateDocumentStatus(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	document, err := database.CreateDocument(context.Background(), "book.epub", content, "", domain.EpubFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.UpdateDocumentStatus(context.Background(), document.ID, "PROCESSING"); err != nil {
		t.Fatal(err)
	}

	got, _ := database.GetDocument(context.Background(), document.ID)
	if got.Status != "PROCESSING" {
		t.Fatalf("expected PROCESSING, got %s", got.Status)
	}
}

func TestUpdateReadingProgress(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	document, _ := database.CreateDocument(context.Background(), "book.epub", content, "", domain.EpubFormat)
	if err := database.UpdateReadingProgress(context.Background(), document.ID, "paragraph-42"); err != nil {
		t.Fatal(err)
	}

	got, _ := database.GetDocument(context.Background(), document.ID)
	if got.ReadingProgress != "paragraph-42" {
		t.Fatalf("expected paragraph-42, got %q", got.ReadingProgress)
	}
}

func TestGetPendingDocuments(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	pending1, _ := database.CreateDocument(ctx, "a.epub", content, "", domain.EpubFormat)
	pending2, _ := database.CreateDocument(ctx, "b.epub", content, "", domain.EpubFormat)
	processing, _ := database.CreateDocument(ctx, "c.epub", content, "", domain.EpubFormat)
	done, _ := database.CreateDocument(ctx, "d.epub", content, "", domain.EpubFormat)
	failed, _ := database.CreateDocument(ctx, "e.epub", content, "", domain.EpubFormat)

	if err := database.UpdateDocumentStatus(ctx, processing.ID, "PROCESSING"); err != nil {
		t.Fatal(err)
	}
	if err := database.MarkDocumentDone(ctx, done.ID, "/data/d.md", "", "", nil); err != nil {
		t.Fatal(err)
	}
	if err := database.MarkDocumentFailed(ctx, failed.ID, "boom"); err != nil {
		t.Fatal(err)
	}

	documents, err := database.GetPendingDocuments(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(documents) != 2 {
		t.Fatalf("expected 2 pending documents, got %d", len(documents))
	}

	got := map[string]bool{}
	for _, j := range documents {
		if j.Status != "PENDING" {
			t.Errorf("expected PENDING, got %s for %s", j.Status, j.ID)
		}
		if len(j.Content) != 0 {
			t.Errorf("expected content to not be populated, got %v for %s", j.Content, j.ID)
		}
		got[j.ID] = true
	}
	if !got[pending1.ID] || !got[pending2.ID] {
		t.Fatalf("missing pending documents in result: %v", got)
	}
}

func TestGetPendingDocumentsEmpty(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	documents, err := database.GetPendingDocuments(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(documents) != 0 {
		t.Fatalf("expected 0 pending documents, got %d", len(documents))
	}
}

func TestMarkDocumentDone(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	document, _ := database.CreateDocument(context.Background(), "book.epub", content, "", domain.EpubFormat)
	if len(document.Content) == 0 {
		t.Fatalf("expected content to not be empty, got %v", document.Content)
	}

	if err := database.MarkDocumentDone(context.Background(), document.ID, "/data/book.md", "", "", nil); err != nil {
		t.Fatal(err)
	}

	got, _ := database.GetDocument(context.Background(), document.ID)
	if got.Status != "DONE" {
		t.Fatalf("expected DONE, got %s", got.Status)
	}
	if got.OutputPath != "/data/book.md" {
		t.Fatalf("expected /data/book.md, got %s", got.OutputPath)
	}

	if len(got.Content) != 0 {
		t.Fatalf("expected empty content, got %v", got.Content)
	}
}

func TestDeleteDocument(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	document, err := database.CreateDocument(ctx, "test.epub", content, "req-1", domain.EpubFormat)
	if err != nil {
		t.Fatal(err)
	}

	if err := database.DeleteDocument(ctx, document.ID); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}

	_, err = database.GetDocument(ctx, document.ID)
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestMarkDocumentDonePersistsMetadata(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	ctx := context.Background()
	document, err := database.CreateDocument(ctx, "test.epub", []byte("%PDF"), "req-1", domain.EpubFormat)
	if err != nil {
		t.Fatal(err)
	}

	cover := []byte{0x89, 0x50, 0x4e, 0x47}
	if err := database.MarkDocumentDone(ctx, document.ID, "/data/out.md", "My Title", "Jane Doe", cover); err != nil {
		t.Fatal(err)
	}

	got, err := database.GetDocument(ctx, document.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "DONE" {
		t.Errorf("status: got %q, want %q", got.Status, "DONE")
	}
	if got.Title != "My Title" {
		t.Errorf("title: got %q, want %q", got.Title, "My Title")
	}
	if got.Author != "Jane Doe" {
		t.Errorf("author: got %q, want %q", got.Author, "Jane Doe")
	}
	if !reflect.DeepEqual(got.Cover, cover) {
		t.Errorf("cover: got %v, want %v", got.Cover, cover)
	}
}

func TestRetryDocument(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	document, err := database.CreateDocument(ctx, "test.epub", content, "req-1", domain.EpubFormat)
	if err != nil {
		t.Fatal(err)
	}

	if document.RetryCount != 0 {
		t.Fatalf("document must retry count to 0 after CreateDocument, got: %d", document.RetryCount)
	}

	err = database.MarkDocumentFailed(ctx, document.ID, "testing retry logic")
	if err != nil {
		t.Fatal(err)
	}

	document, err = database.GetDocument(ctx, document.ID)
	if err != nil {
		t.Fatal(err)
	}

	if document.Status != "FAILED" {
		t.Fatalf("document must have failed status after MarkDocumentFailed, got: %s", document.Status)
	}

	err = database.RetryDocument(ctx, document.ID)
	if err != nil {
		t.Fatal(err)
	}

	document, err = database.GetDocument(ctx, document.ID)
	if err != nil {
		t.Fatal(err)
	}

	if document.Status != "PENDING" {
		t.Fatalf("document must have pending status after RetryDocument, got: %s", document.Status)
	}

	if document.RetryCount != 1 {
		t.Fatalf("document must retry count to 1 after RetryDocument, got: %d", document.RetryCount)
	}
}
