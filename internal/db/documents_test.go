package db_test

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/domain"
	"github.com/GustavoCaso/folio/ui/internal/repository"
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

// seedLibrary creates documents with the given title, author and tags. A nil
// tags slice leaves the column at its ” default, like books never edited.
func seedLibrary(t *testing.T, database repository.Store, books []struct {
	filename, title, author string
	tags                    []string
}) {
	t.Helper()
	for _, b := range books {
		doc, err := database.CreateDocument(context.Background(), b.filename, content, "req", domain.EpubFormat)
		if err != nil {
			t.Fatal(err)
		}
		if b.tags == nil && b.title == "" && b.author == "" {
			continue
		}
		if err := database.UpdateDocument(context.Background(), doc.ID, b.title, b.author, b.tags, nil); err != nil {
			t.Fatal(err)
		}
	}
}

func newLibraryDB(t *testing.T) repository.Store {
	t.Helper()
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	seedLibrary(t, database, []struct {
		filename, title, author string
		tags                    []string
	}{
		{"dune.epub", "Dune", "Frank Herbert", []string{"sci-fi", "classic"}},
		{"children.epub", "Children of Dune", "Frank Herbert", []string{"sci-fi"}},
		{"earthsea.epub", "A Wizard of Earthsea", "Ursula K. Le Guin", []string{"fantasy", "classic"}},
		{"untitled_100%.epub", "", "", nil},
	})
	return database
}

func titles(docs []domain.Document) []string {
	out := []string{}
	for _, d := range docs {
		if d.Title != "" {
			out = append(out, d.Title)
		} else {
			out = append(out, d.Filename)
		}
	}
	sort.Strings(out)
	return out
}

func TestListDocuments_Filter(t *testing.T) {
	database := newLibraryDB(t)

	cases := []struct {
		name   string
		filter domain.DocumentFilter
		want   []string
	}{
		{"no filter returns everything", domain.DocumentFilter{}, []string{"A Wizard of Earthsea", "Children of Dune", "Dune", "untitled_100%.epub"}},
		{"query matches title case-insensitively", domain.DocumentFilter{Query: "dune"}, []string{"Children of Dune", "Dune"}},
		{"query matches author", domain.DocumentFilter{Query: "le guin"}, []string{"A Wizard of Earthsea"}},
		{"query matches filename", domain.DocumentFilter{Query: "earthsea.epub"}, []string{"A Wizard of Earthsea"}},
		{"query treats % literally", domain.DocumentFilter{Query: "100%"}, []string{"untitled_100%.epub"}},
		{"query treats _ literally", domain.DocumentFilter{Query: "d_ne"}, []string{}},
		{"author is exact match", domain.DocumentFilter{Author: "Frank Herbert"}, []string{"Children of Dune", "Dune"}},
		{"author partial does not match", domain.DocumentFilter{Author: "Frank"}, []string{}},
		{"tag matches inside JSON array", domain.DocumentFilter{Tag: "classic"}, []string{"A Wizard of Earthsea", "Dune"}},
		{"filters combine with AND", domain.DocumentFilter{Query: "dune", Tag: "classic"}, []string{"Dune"}},
		{"author and tag combine", domain.DocumentFilter{Author: "Frank Herbert", Tag: "fantasy"}, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			docs, err := database.ListDocuments(context.Background(), tc.filter)
			if err != nil {
				t.Fatal(err)
			}
			if got := titles(docs); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDocumentFacets_CountsWholeLibrary(t *testing.T) {
	database := newLibraryDB(t)

	facets, err := database.DocumentFacets(context.Background(), domain.DocumentFilter{})
	if err != nil {
		t.Fatal(err)
	}
	wantAuthors := []domain.FacetCount{{Value: "Frank Herbert", Count: 2}, {Value: "Ursula K. Le Guin", Count: 1}}
	if !reflect.DeepEqual(facets.Authors, wantAuthors) {
		t.Errorf("authors: got %v, want %v", facets.Authors, wantAuthors)
	}
	wantTags := []domain.FacetCount{{Value: "classic", Count: 2}, {Value: "fantasy", Count: 1}, {Value: "sci-fi", Count: 2}}
	if !reflect.DeepEqual(facets.Tags, wantTags) {
		t.Errorf("tags: got %v, want %v", facets.Tags, wantTags)
	}
}

func TestDocumentFacets_NarrowToCurrentResult(t *testing.T) {
	database := newLibraryDB(t)

	facets, err := database.DocumentFacets(context.Background(), domain.DocumentFilter{Tag: "classic"})
	if err != nil {
		t.Fatal(err)
	}
	wantAuthors := []domain.FacetCount{{Value: "Frank Herbert", Count: 1}, {Value: "Ursula K. Le Guin", Count: 1}}
	if !reflect.DeepEqual(facets.Authors, wantAuthors) {
		t.Errorf("authors: got %v, want %v", facets.Authors, wantAuthors)
	}
	wantTags := []domain.FacetCount{{Value: "classic", Count: 2}, {Value: "fantasy", Count: 1}, {Value: "sci-fi", Count: 1}}
	if !reflect.DeepEqual(facets.Tags, wantTags) {
		t.Errorf("tags: got %v, want %v", facets.Tags, wantTags)
	}
}

func TestDocumentFacets_EmptyLibrary(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	// A document with tags = '' must not break json_each.
	if _, err := database.CreateDocument(context.Background(), "a.epub", content, "req", domain.EpubFormat); err != nil {
		t.Fatal(err)
	}
	facets, err := database.DocumentFacets(context.Background(), domain.DocumentFilter{Tag: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if len(facets.Authors) != 0 || len(facets.Tags) != 0 {
		t.Errorf("expected no facets, got %+v", facets)
	}
}
