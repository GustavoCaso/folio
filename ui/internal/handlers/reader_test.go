package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadDocument_NotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/read/no-such-id", nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, newTestStore(t))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Document not found") {
		t.Errorf("expected 'Document not found' in response, got:\n%s", rec.Body.String())
	}
}

func TestReadDocument_JobPending_NotReady(t *testing.T) {
	store := newTestStore(t)
	job, err := store.CreateJob(context.Background(), "pending.pdf", []byte{}, "", "pdf")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/read/"+job.ID, nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "not ready") {
		t.Errorf("expected 'not ready' in response, got:\n%s", rec.Body.String())
	}
}

func TestReadDocument_JobFailed(t *testing.T) {
	store := newTestStore(t)
	job, err := store.CreateJob(context.Background(), "broken.pdf", []byte{}, "", "pdf")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkJobFailed(context.Background(), job.ID, "parser crashed"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/read/"+job.ID, nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "conversion failed") {
		t.Errorf("expected 'conversion failed' in response, got:\n%s", rec.Body.String())
	}
}

func TestReadDocument_MarkdownFileMissing(t *testing.T) {
	store := newTestStore(t)
	job, err := store.CreateJob(context.Background(), "gone.pdf", []byte{}, "", "pdf")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkJobDone(context.Background(), job.ID, "/nonexistent/path/gone.md", "", "", nil); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/read/"+job.ID, nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Failed to read document") {
		t.Errorf("expected 'Failed to read document' in response, got:\n%s", rec.Body.String())
	}
}

func TestReadDocument_HappyPath(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(mdPath, []byte("# Hello\n\nWorld paragraph."), 0644); err != nil {
		t.Fatal(err)
	}

	store := newTestStore(t)
	job, err := store.CreateJob(context.Background(), "doc.pdf", []byte{}, "", "pdf")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkJobDone(context.Background(), job.ID, mdPath, "", "", nil); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/read/"+job.ID, nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	got := rec.Body.String()
	if !strings.Contains(got, "Hello") {
		t.Errorf("expected rendered heading in response, got:\n%s", got)
	}
	if !strings.Contains(got, "World paragraph") {
		t.Errorf("expected rendered paragraph in response, got:\n%s", got)
	}
	if strings.Contains(got, `class="error-banner"`) {
		t.Errorf("unexpected error banner in successful reader response")
	}
}
