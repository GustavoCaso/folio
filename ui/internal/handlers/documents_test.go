package handlers_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/handlers"
	"github.com/GustavoCaso/folio/ui/internal/hub"
)

func newTestStore(t *testing.T) *db.Store {
	t.Helper()
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func newTestMux(t *testing.T, store *db.Store) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := hub.New(logger)
	if err != nil {
		t.Fatal(err)
	}
	mux, err := handlers.Register(store, h, nil, t.TempDir(), logger)
	if err != nil {
		t.Fatal(err)
	}
	return mux
}

// emptyMultipartRequest builds a POST /documents request with a valid
// multipart body but no "document" field.
func emptyMultipartRequest(t *testing.T) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	w.Close()
	req := httptest.NewRequest(http.MethodPost, "/documents", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestUploadDocumentError_ShowsExistingJobs(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.CreateJob(context.Background(), "existing.pdf", "req-seed"); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	newTestMux(t, store).ServeHTTP(rec, emptyMultipartRequest(t))

	got := rec.Body.String()
	if !strings.Contains(got, "existing.pdf") {
		t.Errorf("expected existing job 'existing.pdf' in error response, got:\n%s", got)
	}
}

func TestUploadDocumentError_ShowsBanner(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestMux(t, newTestStore(t)).ServeHTTP(rec, emptyMultipartRequest(t))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `class="error-banner"`) {
		t.Errorf("expected error-banner in response, got:\n%s", rec.Body.String())
	}
}

func TestListDocuments_RendersJobs(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.CreateJob(context.Background(), "report.pdf", ""); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	newTestMux(t, store).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "report.pdf") {
		t.Errorf("expected 'report.pdf' in response, got:\n%s", body)
	}
	if strings.Contains(body, `class="error-banner"`) {
		t.Errorf("unexpected error-banner on successful list, got:\n%s", body)
	}
}
