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

	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/export"
	"github.com/GustavoCaso/folio/ui/internal/handlers"
	"github.com/GustavoCaso/folio/ui/internal/hub"
	"github.com/GustavoCaso/folio/ui/internal/parser/client"
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
	mux, err := handlers.Register(store, h, nil, t.TempDir(), logger, []export.Backend{})
	if err != nil {
		t.Fatal(err)
	}
	return mux, h
}

func newTestMuxWithParser(t *testing.T, store repository.Store, parser client.Client) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := hub.New(logger)
	if err != nil {
		t.Fatal(err)
	}
	mux, err := handlers.Register(store, h, parser, t.TempDir(), logger, []export.Backend{})
	if err != nil {
		t.Fatal(err)
	}
	return mux
}

type errorParser struct{}

func (errorParser) Convert(_ context.Context, _, _, _ string, _ []byte, _ *hub.Hub) (client.ConversionResult, error) {
	return client.ConversionResult{}, errors.New("stubbed")
}
func (errorParser) ConvertFromURL(_ context.Context, _, _, _ string, _ *hub.Hub) (client.ConversionResult, error) {
	return client.ConversionResult{}, errors.New("stubbed")
}
func (errorParser) Health(_ context.Context) bool { return false }
func (errorParser) Close() error {
	return nil
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

func TestUploadDocumentError_ShowsExistingJobs(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.CreateJob(context.Background(), "existing.pdf", []byte{}, "req-seed"); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, emptyMultipartRequest(t))

	got := rec.Body.String()
	if !strings.Contains(got, "existing.pdf") {
		t.Errorf("expected existing job 'existing.pdf' in error response, got:\n%s", got)
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

func TestDeleteDocument_RejectsPending(t *testing.T) {
	store := newTestStore(t)
	job, err := store.CreateJob(context.Background(), "pending.pdf", []byte{}, "req-1")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/documents/"+job.ID, nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

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

	req := httptest.NewRequest(http.MethodDelete, "/documents/"+job.ID, nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
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
	if err := store.MarkJobDone(context.Background(), job.ID, mdPath, "", "", nil); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/documents/"+job.ID, nil)
	rec := httptest.NewRecorder()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, _ := hub.New(logger)
	mux, _ := handlers.Register(store, h, nil, dir, logger, []export.Backend{})
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
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
	job, err := store.CreateJob(context.Background(), "pending.pdf", []byte{}, "req-1")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/documents/"+job.ID+"/retry", nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

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
	job, err := store.CreateJob(context.Background(), "stuck.pdf", []byte("%PDF"), "req-1")
	if err != nil {
		t.Fatal(err)
	}

	err = store.MarkJobDone(context.Background(), job.ID, "fake/path", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/documents/"+job.ID+"/cancel", nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCancelDocument_ParserNotRunnig(t *testing.T) {
	store := newTestStore(t)
	job, err := store.CreateJob(context.Background(), "stuck.pdf", []byte("%PDF"), "req-1")
	if err != nil {
		t.Fatal(err)
	}
	// Job exists in DB but no goroutine is running it (not in cancels map).
	mux, hub := newTestMux(t, store)
	ch := hub.Subscribe(job.ID)
	req := httptest.NewRequest(http.MethodPost, "/documents/"+job.ID+"/cancel", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}

	// Even when returnig a 409 we mark the jobs as failed so user can retry
	job, err = store.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != "FAILED" || job.Error != "cancelled by user" {
		t.Fatal("job must be mark as FAILED and the error message must by 'cancelled by user'")
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
	hub.Unsubscribe(job.ID, ch)
}

// blockingParser blocks Convert until its context is cancelled.
type blockingParser struct {
	started chan struct{}
}

func (p *blockingParser) Convert(ctx context.Context, _, _, _ string, _ []byte, _ *hub.Hub) (client.ConversionResult, error) {
	close(p.started)
	<-ctx.Done()
	return client.ConversionResult{}, ctx.Err()
}
func (p *blockingParser) ConvertFromURL(_ context.Context, _, _, _ string, _ *hub.Hub) (client.ConversionResult, error) {
	return client.ConversionResult{}, errors.New("not implemented in test stub")
}
func (p *blockingParser) Health(_ context.Context) bool { return true }
func (p *blockingParser) Close() error {
	return nil
}

func TestCancelDocument_OK(t *testing.T) {
	store := newTestStore(t)
	parser := &blockingParser{started: make(chan struct{})}
	mux := newTestMuxWithParser(t, store, parser)

	// Upload a PDF to start a conversion goroutine.
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	fw, err := mw.CreateFormFile("document", "test.pdf")
	if err != nil {
		t.Fatal(err)
	}
	fw.Write([]byte("%PDF-1.4 fake")) //nolint:errcheck
	mw.Close()                        //nolint:errcheck
	uploadReq := httptest.NewRequest(http.MethodPost, "/documents", body)
	uploadReq.Header.Set("Content-Type", mw.FormDataContentType())
	uploadRec := httptest.NewRecorder()
	mux.ServeHTTP(uploadRec, uploadReq)
	if uploadRec.Code != http.StatusSeeOther {
		t.Fatalf("upload: expected 303, got %d", uploadRec.Code)
	}

	// Wait for the parser goroutine to start blocking.
	select {
	case <-parser.started:
	case <-time.After(3 * time.Second):
		t.Fatal("parser goroutine never started")
	}

	// Find the job ID.
	jobs, err := store.GetPendingJobs(context.Background())
	if err != nil || len(jobs) == 0 {
		t.Fatalf("expected a pending job, err=%v jobs=%v", err, jobs)
	}
	jobID := jobs[0].ID

	// Cancel it.
	cancelReq := httptest.NewRequest(http.MethodPost, "/documents/"+jobID+"/cancel", nil)
	cancelRec := httptest.NewRecorder()
	mux.ServeHTTP(cancelRec, cancelReq)
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("cancel: expected 200, got %d", cancelRec.Code)
	}

	// Wait for the goroutine to finish and mark the job FAILED.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job, err := store.GetJob(context.Background(), jobID)
		if err != nil {
			t.Fatal(err)
		}
		if job.Status == "FAILED" {
			if job.Error != "cancelled by user" {
				t.Errorf("expected error 'cancelled by user', got %q", job.Error)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job never transitioned to FAILED after cancel")
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

	if err := store.MarkJobDone(t.Context(), job1.ID, "./whatever", "", "", nil); err != nil {
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
