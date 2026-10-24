package handlers_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GustavoCaso/folio/ui/internal/converter/parser"
	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/domain"
	"github.com/GustavoCaso/folio/ui/internal/export"
	"github.com/GustavoCaso/folio/ui/internal/handlers"
	"github.com/GustavoCaso/folio/ui/internal/hub"
	"github.com/GustavoCaso/folio/ui/internal/repository"
)

func newTestStore(t *testing.T) repository.Store {
	t.Helper()
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func newTestMux(t *testing.T, store repository.Store) (http.Handler, *hub.Hub) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := hub.New(logger)
	if err != nil {
		t.Fatal(err)
	}
	mux, err := handlers.Register(store, h, t.TempDir(), logger, []export.Backend{})
	if err != nil {
		t.Fatal(err)
	}
	return mux, h
}

// newTestMuxWithParser registers p as the EPUB parser so tests control
// conversion outcome and timing.
func newTestMuxWithParser(t *testing.T, store repository.Store, p parser.Parser) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := hub.New(logger)
	if err != nil {
		t.Fatal(err)
	}
	parsers := map[domain.DocumentFormat]parser.Parser{domain.EpubFormat: p}
	mux, err := handlers.RegisterWithParsers(store, h, t.TempDir(), logger, []export.Backend{}, parsers)
	if err != nil {
		t.Fatal(err)
	}
	return mux
}

// errorParser fails without touching the document, leaving its status as-is.
type errorParser struct{}

func (errorParser) Convert(_ context.Context, _, _, _ string, _ []byte, _ *hub.Hub) error {
	return errors.New("stubbed")
}

// epubUploadRequest builds a POST /documents multipart request uploading
// filename with the given bytes.
func epubUploadRequest(t *testing.T, filename string, data []byte) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	fw, err := mw.CreateFormFile("document", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/documents", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

// emptyMultipartRequest builds a POST /documents request with a valid
// multipart body but no "document" field.
func emptyMultipartRequest(t *testing.T) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/documents", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestUploadDocumentError_ShowsExistingDocuments(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.CreateDocument(context.Background(), "existing.epub", []byte{}, "req-seed", domain.EpubFormat); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, emptyMultipartRequest(t))

	got := rec.Body.String()
	if !strings.Contains(got, "existing.epub") {
		t.Errorf("expected existing document 'existing.epub' in error response, got:\n%s", got)
	}
}

func TestUploadDocumentError_ShowsBanner(t *testing.T) {
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, newTestStore(t))
	mux.ServeHTTP(rec, emptyMultipartRequest(t))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `role="alert"`) {
		t.Errorf("expected alert element in response, got:\n%s", rec.Body.String())
	}
}

func TestUploadDocument_EPUBSetsFormat(t *testing.T) {
	store := newTestStore(t)
	mux := newTestMuxWithParser(t, store, errorParser{})

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, epubUploadRequest(t, "book.epub", []byte("not a real epub but that's ok for this test")))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	documents, err := store.ListDocuments(context.Background(), domain.DocumentFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(documents))
	}
	if documents[0].Format != domain.EpubFormat {
		t.Errorf("Format = %q, want %q", documents[0].Format, domain.EpubFormat)
	}
}

func TestUploadDocument_RejectsNonEPUB(t *testing.T) {
	store := newTestStore(t)
	mux, _ := newTestMux(t, store)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, epubUploadRequest(t, "paper.pdf", []byte("%PDF-1.4 fake")))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	documents, err := store.ListDocuments(context.Background(), domain.DocumentFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(documents) != 0 {
		t.Errorf("expected no documents, got %d", len(documents))
	}
}

func TestDeleteDocument_RejectsPending(t *testing.T) {
	store := newTestStore(t)
	document, err := store.CreateDocument(context.Background(), "pending.epub", []byte{}, "req-1", domain.EpubFormat)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/documents/"+document.ID, nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestDeleteDocument_DeletesFailedDocument(t *testing.T) {
	store := newTestStore(t)
	document, err := store.CreateDocument(context.Background(), "failed.epub", []byte{}, "req-1", domain.EpubFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkDocumentFailed(context.Background(), document.ID, "parse error"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/documents/"+document.ID, nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	_, err = store.GetDocument(context.Background(), document.ID)
	if err == nil {
		t.Error("expected document to be deleted")
	}
}

func TestDeleteDocument_DeletesDoneDocumentAndFile(t *testing.T) {
	store := newTestStore(t)
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "out.md")
	if err := os.WriteFile(mdPath, []byte("# hello"), 0644); err != nil {
		t.Fatal(err)
	}

	document, err := store.CreateDocument(context.Background(), "done.epub", []byte{}, "req-1", domain.EpubFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkDocumentDone(context.Background(), document.ID, mdPath, "", "", nil); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/documents/"+document.ID, nil)
	rec := httptest.NewRecorder()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, _ := hub.New(logger)
	mux, _ := handlers.Register(store, h, dir, logger, []export.Backend{})
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	if _, err := os.Stat(mdPath); !errors.Is(err, os.ErrNotExist) {
		t.Error("expected markdown file to be deleted")
	}

	_, err = store.GetDocument(context.Background(), document.ID)
	if err == nil {
		t.Error("expected document to be deleted")
	}
}

func TestDeleteDocument_NotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/documents/nonexistent-id", nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, newTestStore(t))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestRetryDocument_NotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/documents/nonexistent-id/retry", nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, newTestStore(t))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestRetryDocument_RejectsNonFailed(t *testing.T) {
	store := newTestStore(t)
	document, err := store.CreateDocument(context.Background(), "pending.epub", []byte{}, "req-1", domain.EpubFormat)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/documents/"+document.ID+"/retry", nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestRetryDocument_HappyPath(t *testing.T) {
	store := newTestStore(t)
	document, err := store.CreateDocument(context.Background(), "broken.epub", []byte("epub"), "req-1", domain.EpubFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkDocumentFailed(context.Background(), document.ID, "parse error"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/documents/"+document.ID+"/retry", nil)
	rec := httptest.NewRecorder()
	newTestMuxWithParser(t, store, errorParser{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rec.Code)
	}

	got, err := store.GetDocument(context.Background(), document.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "PENDING" {
		t.Errorf("expected status PENDING, got %s", got.Status)
	}
	if got.RetryCount != 1 {
		t.Errorf("expected retry_count 1, got %d", got.RetryCount)
	}
}

func TestCancelDocument_UnknownID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/documents/no-such-id/cancel", nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, newTestStore(t))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestCancelDocument_InvalidState(t *testing.T) {
	store := newTestStore(t)
	document, err := store.CreateDocument(context.Background(), "stuck.epub", []byte("epub"), "req-1", domain.EpubFormat)
	if err != nil {
		t.Fatal(err)
	}

	err = store.MarkDocumentDone(context.Background(), document.ID, "fake/path", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/documents/"+document.ID+"/cancel", nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCancelDocument_ParserNotRunnig(t *testing.T) {
	store := newTestStore(t)
	document, err := store.CreateDocument(context.Background(), "stuck.epub", []byte("epub"), "req-1", domain.EpubFormat)
	if err != nil {
		t.Fatal(err)
	}
	// Document exists in DB but no goroutine is running it (not in cancels map).
	mux, hub := newTestMux(t, store)
	ch := hub.Subscribe(document.ID)
	req := httptest.NewRequest(http.MethodPost, "/documents/"+document.ID+"/cancel", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}

	// Even when returnig a 409 we mark the documents as failed so user can retry
	document, err = store.GetDocument(context.Background(), document.ID)
	if err != nil {
		t.Fatal(err)
	}
	if document.Status != "FAILED" || document.Error != "cancelled by user" {
		t.Fatal("document must be mark as FAILED and the error message must by 'cancelled by user'")
	}

	select {
	case got := <-ch:
		if got.Status != "FAILED" {
			t.Fatal("hub status incorrect")
		}
		if got.Error != "cancelled by user" {
			t.Fatal("hub error incorrect")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("hub message never received")
	}
	hub.Unsubscribe(document.ID, ch)
}

// blockingParser blocks Convert until its context is cancelled.
// On cancel it marks the document failed, as real parsers own the document lifecycle.
type blockingParser struct {
	store   repository.Store
	started chan struct{}
}

func (p *blockingParser) Convert(ctx context.Context, documentID, _, _ string, _ []byte, h *hub.Hub) error {
	close(p.started)
	<-ctx.Done()
	_ = p.store.MarkDocumentFailed(context.Background(), documentID, "cancelled by user")
	h.Publish(documentID, hub.StatusEvent{Status: "FAILED", Error: "cancelled by user"})
	return ctx.Err()
}

func TestCancelDocument_OK(t *testing.T) {
	store := newTestStore(t)
	bp := &blockingParser{store: store, started: make(chan struct{})}
	mux := newTestMuxWithParser(t, store, bp)

	// Upload an EPUB to start a conversion goroutine.
	uploadRec := httptest.NewRecorder()
	mux.ServeHTTP(uploadRec, epubUploadRequest(t, "test.epub", []byte("fake epub")))
	if uploadRec.Code != http.StatusSeeOther {
		t.Fatalf("upload: expected 303, got %d", uploadRec.Code)
	}

	// Wait for the parser goroutine to start blocking.
	select {
	case <-bp.started:
	case <-time.After(3 * time.Second):
		t.Fatal("parser goroutine never started")
	}

	// Find the document ID.
	documents, err := store.GetPendingDocuments(context.Background())
	if err != nil || len(documents) == 0 {
		t.Fatalf("expected a pending document, err=%v documents=%v", err, documents)
	}
	documentID := documents[0].ID

	// Cancel it.
	cancelReq := httptest.NewRequest(http.MethodPost, "/documents/"+documentID+"/cancel", nil)
	cancelRec := httptest.NewRecorder()
	mux.ServeHTTP(cancelRec, cancelReq)
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("cancel: expected 200, got %d", cancelRec.Code)
	}

	// Wait for the goroutine to finish and mark the document FAILED.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		document, err := store.GetDocument(context.Background(), documentID)
		if err != nil {
			t.Fatal(err)
		}
		if document.Status == "FAILED" {
			if document.Error != "cancelled by user" {
				t.Errorf("expected error 'cancelled by user', got %q", document.Error)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("document never transitioned to FAILED after cancel")
}

func TestListDocuments_RendersDocuments_And_PendingDocuments(t *testing.T) {
	store := newTestStore(t)
	document1, err := store.CreateDocument(t.Context(), "report.epub", []byte{}, "", domain.EpubFormat)
	if err != nil {
		t.Fatal(err)
	}

	document2, err := store.CreateDocument(t.Context(), "pending-report.epub", []byte{}, "", domain.EpubFormat)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.MarkDocumentDone(t.Context(), document1.ID, "./whatever", "", "", nil); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "report.epub") {
		t.Errorf("expected 'report.epub' in response, got:\n%s", body)
	}
	if !strings.Contains(body, "pending-report.epub") {
		t.Errorf("expected 'pending-report.epub' in response, got:\n%s", body)
	}
	if !strings.Contains(body, fmt.Sprintf(`data-document-ids="%s"`, document2.ID)) {
		t.Errorf("expected data-document-ids attribute with document id, got: %s", body)
	}
	if strings.Contains(body, `class="error-banner"`) {
		t.Errorf("unexpected error-banner on successful list, got:\n%s", body)
	}
}

func seedBook(t *testing.T, store repository.Store, filename, title, author string, tags []string) {
	t.Helper()
	doc, err := store.CreateDocument(t.Context(), filename, []byte{}, "", domain.EpubFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateDocument(t.Context(), doc.ID, title, author, tags, nil); err != nil {
		t.Fatal(err)
	}
}

func TestListDocuments_FiltersByQueryAuthorAndTag(t *testing.T) {
	store := newTestStore(t)
	seedBook(t, store, "dune.epub", "Dune", "Frank Herbert", []string{"sci-fi"})
	seedBook(t, store, "messiah.epub", "Dune Messiah", "Frank Herbert", []string{"sequel"})
	seedBook(t, store, "earthsea.epub", "A Wizard of Earthsea", "Ursula K. Le Guin", []string{"fantasy"})
	mux, _ := newTestMux(t, store)

	cases := []struct {
		url      string
		want     []string
		dontWant []string
	}{
		{"/?q=dune", []string{"dune.epub", "messiah.epub"}, []string{"earthsea.epub"}},
		{"/?author=Ursula+K.+Le+Guin", []string{"earthsea.epub"}, []string{"dune.epub"}},
		{"/?q=dune&tag=sequel", []string{"messiah.epub"}, []string{"dune.epub", "earthsea.epub"}},
	}
	for _, tc := range cases {
		t.Run(tc.url, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.url, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", rec.Code)
			}
			body := rec.Body.String()
			for _, w := range tc.want {
				if !strings.Contains(body, w) {
					t.Errorf("expected %q in response", w)
				}
			}
			for _, w := range tc.dontWant {
				if strings.Contains(body, w) {
					t.Errorf("did not expect %q in response", w)
				}
			}
		})
	}
}

func TestListDocuments_FacetCountsFollowFilter(t *testing.T) {
	store := newTestStore(t)
	seedBook(t, store, "dune.epub", "Dune", "Frank Herbert", []string{"sci-fi"})
	seedBook(t, store, "earthsea.epub", "A Wizard of Earthsea", "Ursula K. Le Guin", []string{"fantasy"})
	mux, _ := newTestMux(t, store)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/?tag=fantasy", nil))
	body := rec.Body.String()

	// Facet links are the only place author names appear as toggle hrefs
	// that keep the active tag.
	if !strings.Contains(body, `href="/?author=Ursula+K.+Le+Guin&amp;tag=fantasy"`) {
		t.Errorf("expected Le Guin facet keeping tag filter, got:\n%s", body)
	}
	if strings.Contains(body, "Frank Herbert") {
		t.Errorf("expected Herbert facet to be narrowed away, got:\n%s", body)
	}
}

func TestListDocuments_EmptyFilterResult(t *testing.T) {
	store := newTestStore(t)
	seedBook(t, store, "dune.epub", "Dune", "Frank Herbert", nil)
	mux, _ := newTestMux(t, store)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/?q=nothing", nil))
	if !strings.Contains(rec.Body.String(), "No books match these filters.") {
		t.Errorf("expected empty filter state, got:\n%s", rec.Body.String())
	}
}

func TestListDocuments_ViewFromCookie(t *testing.T) {
	store := newTestStore(t)
	mux, _ := newTestMux(t, store)

	cases := []struct {
		cookie string
		want   string
	}{
		{"", `data-view="grid"`},
		{"list", `data-view="list"`},
		{"grid", `data-view="grid"`},
		{"bogus", `data-view="grid"`},
	}
	for _, tc := range cases {
		t.Run("cookie="+tc.cookie, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "folio_view", Value: tc.cookie})
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if !strings.Contains(rec.Body.String(), tc.want) {
				t.Errorf("expected %s in response", tc.want)
			}
		})
	}
}
