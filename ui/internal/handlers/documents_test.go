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

	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/handlers"
	"github.com/GustavoCaso/folio/ui/internal/hub"
	parserclient "github.com/GustavoCaso/folio/ui/internal/parser/client"
)

func newTestStore(t *testing.T) *db.Store {
	t.Helper()
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
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

func newTestMuxWithParser(t *testing.T, store *db.Store, parser handlers.ParserClient) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := hub.New(logger)
	if err != nil {
		t.Fatal(err)
	}
	mux, err := handlers.Register(store, h, parser, t.TempDir(), logger)
	if err != nil {
		t.Fatal(err)
	}
	return mux
}

type errorParser struct{}

func (errorParser) Convert(_ context.Context, _, _, _ string, _ []byte, _ *hub.Hub) (parserclient.ConversionResult, error) {
	return parserclient.ConversionResult{}, errors.New("stubbed")
}
func (errorParser) Health(_ context.Context) bool { return false }

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

func TestUploadDocumentError_ShowsExistingJobs(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.CreateJob(context.Background(), "existing.pdf", []byte{}, "req-seed"); err != nil {
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
	if !strings.Contains(rec.Body.String(), `role="alert"`) {
		t.Errorf("expected alert element in response, got:\n%s", rec.Body.String())
	}
}

func TestDeleteDocument_RejectsPending(t *testing.T) {
	store := newTestStore(t)
	job, err := store.CreateJob(context.Background(), "pending.pdf", []byte{}, "req-1")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/documents/"+job.ID+"/delete", nil)
	rec := httptest.NewRecorder()
	newTestMux(t, store).ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestDeleteDocument_DeletesFailedJob(t *testing.T) {
	store := newTestStore(t)
	job, err := store.CreateJob(context.Background(), "failed.pdf", []byte{}, "req-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkJobFailed(context.Background(), job.ID, "parse error"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/documents/"+job.ID+"/delete", nil)
	rec := httptest.NewRecorder()
	newTestMux(t, store).ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rec.Code)
	}

	_, err = store.GetJob(context.Background(), job.ID)
	if err == nil {
		t.Error("expected job to be deleted")
	}
}

func TestDeleteDocument_DeletesDoneJobAndFile(t *testing.T) {
	store := newTestStore(t)
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "out.md")
	if err := os.WriteFile(mdPath, []byte("# hello"), 0644); err != nil {
		t.Fatal(err)
	}

	job, err := store.CreateJob(context.Background(), "done.pdf", []byte{}, "req-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkJobDone(context.Background(), job.ID, mdPath); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/documents/"+job.ID+"/delete", nil)
	rec := httptest.NewRecorder()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, _ := hub.New(logger)
	mux, _ := handlers.Register(store, h, nil, dir, logger)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rec.Code)
	}

	if _, err := os.Stat(mdPath); !errors.Is(err, os.ErrNotExist) {
		t.Error("expected markdown file to be deleted")
	}

	_, err = store.GetJob(context.Background(), job.ID)
	if err == nil {
		t.Error("expected job to be deleted")
	}
}

func TestDeleteDocument_NotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/documents/nonexistent-id/delete", nil)
	rec := httptest.NewRecorder()
	newTestMux(t, newTestStore(t)).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestRetryDocument_NotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/documents/nonexistent-id/retry", nil)
	rec := httptest.NewRecorder()
	newTestMux(t, newTestStore(t)).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestRetryDocument_RejectsNonFailed(t *testing.T) {
	store := newTestStore(t)
	job, err := store.CreateJob(context.Background(), "pending.pdf", []byte{}, "req-1")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/documents/"+job.ID+"/retry", nil)
	rec := httptest.NewRecorder()
	newTestMux(t, store).ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestRetryDocument_HappyPath(t *testing.T) {
	store := newTestStore(t)
	job, err := store.CreateJob(context.Background(), "broken.pdf", []byte("pdf"), "req-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkJobFailed(context.Background(), job.ID, "parse error"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/documents/"+job.ID+"/retry", nil)
	rec := httptest.NewRecorder()
	newTestMuxWithParser(t, store, errorParser{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rec.Code)
	}

	got, err := store.GetJob(context.Background(), job.ID)
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

func TestListDocuments_RendersJobs_And_PendingJobs(t *testing.T) {
	store := newTestStore(t)
	job1, err := store.CreateJob(t.Context(), "report.pdf", []byte{}, "")
	if err != nil {
		t.Fatal(err)
	}

	job2, err := store.CreateJob(t.Context(), "pending-report.pdf", []byte{}, "")
	if err != nil {
		t.Fatal(err)
	}

	if err := store.MarkJobDone(t.Context(), job1.ID, "./whatever"); err != nil {
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
	if !strings.Contains(body, "pending-report.pdf") {
		t.Errorf("expected 'pending-report.pdf' in response, got:\n%s", body)
	}
	if !strings.Contains(body, fmt.Sprintf(`data-job-ids="%s"`, job2.ID)) {
		t.Errorf("expected data-job-ids attribute with job id, got: %s", body)
	}
	if strings.Contains(body, `class="error-banner"`) {
		t.Errorf("unexpected error-banner on successful list, got:\n%s", body)
	}
}
