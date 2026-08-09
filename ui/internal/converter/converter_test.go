package converter_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"io"
	"log/slog"

	"github.com/GustavoCaso/folio/ui/internal/converter"
	"github.com/GustavoCaso/folio/ui/internal/converter/parser"
	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/hub"
	"github.com/GustavoCaso/folio/ui/internal/repository"
)

func newStore(t *testing.T) repository.Store {
	t.Helper()
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func newHub(t *testing.T) *hub.Hub {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := hub.New(logger)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func newLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// successParser marks the job done immediately.
type successParser struct{ store repository.Store }

func (p *successParser) Convert(ctx context.Context, jobID, _, _ string, _ []byte, h *hub.Hub) error {
	if err := p.store.MarkJobDone(ctx, jobID, "/tmp/out.md", "Title", "Author", nil); err != nil {
		return err
	}
	h.Publish(jobID, hub.StatusEvent{Status: "DONE"})
	return nil
}
func (p *successParser) ConvertFromURL(ctx context.Context, jobID, _, _ string, h *hub.Hub) error {
	return p.Convert(ctx, jobID, "", "", nil, h)
}

// failParser always fails the job.
type failParser struct {
	store repository.Store
	err   error
}

func (p *failParser) Convert(ctx context.Context, jobID, _, _ string, _ []byte, h *hub.Hub) error {
	_ = p.store.MarkJobFailed(ctx, jobID, p.err.Error())
	h.Publish(jobID, hub.StatusEvent{Status: "FAILED", Error: p.err.Error()})
	return p.err
}
func (p *failParser) ConvertFromURL(ctx context.Context, jobID, _, _ string, h *hub.Hub) error {
	return p.Convert(ctx, jobID, "", "", nil, h)
}

// blockingParser blocks until its context is cancelled, then marks the job
// failed with "cancelled by user" — mirrors what pdfParser does on
// context.Canceled, since converter.Runner no longer does this itself.
type blockingParser struct {
	store   repository.Store
	started chan struct{}
}

func (p *blockingParser) Convert(ctx context.Context, jobID, _, _ string, _ []byte, h *hub.Hub) error {
	close(p.started)
	<-ctx.Done()
	_ = p.store.MarkJobFailed(context.Background(), jobID, "cancelled by user")
	h.Publish(jobID, hub.StatusEvent{Status: "FAILED", Error: "cancelled by user"})
	return ctx.Err()
}
func (p *blockingParser) ConvertFromURL(ctx context.Context, jobID, _, _ string, h *hub.Hub) error {
	return p.Convert(ctx, jobID, "", "", nil, h)
}

func TestRun_MarksDone(t *testing.T) {
	store := newStore(t)
	job, err := store.CreateJob(context.Background(), "test.pdf", []byte("%PDF"), "req-1", "pdf")
	if err != nil {
		t.Fatal(err)
	}

	parsers := map[string]parser.Parser{"pdf": &successParser{store: store}}
	r := converter.New(store, newHub(t), parsers, newLogger())
	r.Run(job.ID, "req-1", "pdf", "test.pdf", []byte("%PDF"))

	got, err := store.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "DONE" {
		t.Errorf("expected DONE, got %s", got.Status)
	}
	if got.OutputPath == "" {
		t.Error("expected OutputPath to be set")
	}
}

func TestRunFromURL_MarksDone(t *testing.T) {
	store := newStore(t)
	job, err := store.CreateJobFromURL(context.Background(), "https://example.com/doc", "example.com/doc", "req-1")
	if err != nil {
		t.Fatal(err)
	}

	parsers := map[string]parser.Parser{"pdf": &successParser{store: store}}
	r := converter.New(store, newHub(t), parsers, newLogger())
	r.RunFromURL(job.ID, "req-1", "pdf", "https://example.com/doc")

	got, err := store.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "DONE" {
		t.Errorf("expected DONE, got %s", got.Status)
	}
}

func TestRun_ParserError_MarksJobFailed(t *testing.T) {
	store := newStore(t)
	job, err := store.CreateJob(context.Background(), "fail.pdf", []byte("%PDF"), "req-1", "pdf")
	if err != nil {
		t.Fatal(err)
	}

	parsers := map[string]parser.Parser{"pdf": &failParser{store: store, err: errors.New("parser exploded")}}
	r := converter.New(store, newHub(t), parsers, newLogger())
	r.Run(job.ID, "req-1", "pdf", "fail.pdf", []byte("%PDF"))

	got, err := store.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "FAILED" {
		t.Errorf("expected FAILED, got %s", got.Status)
	}
	if !strings.Contains(got.Error, "parser exploded") {
		t.Errorf("expected error message, got: %s", got.Error)
	}
}

func TestRun_CancelledContext_MarksJobCancelledByUser(t *testing.T) {
	store := newStore(t)
	job, err := store.CreateJob(context.Background(), "cancel.pdf", []byte("%PDF"), "req-1", "pdf")
	if err != nil {
		t.Fatal(err)
	}

	bp := &blockingParser{store: store, started: make(chan struct{})}
	parsers := map[string]parser.Parser{"pdf": bp}
	r := converter.New(store, newHub(t), parsers, newLogger())

	done := make(chan struct{})
	go func() {
		defer close(done)
		r.Run(job.ID, "req-1", "pdf", "cancel.pdf", []byte("%PDF"))
	}()

	<-bp.started
	r.Cancel(job.ID)

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run never returned after cancel")
	}

	got, err := store.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "FAILED" {
		t.Errorf("expected FAILED, got %s", got.Status)
	}
	if got.Error != "cancelled by user" {
		t.Errorf("expected 'cancelled by user', got %q", got.Error)
	}
}

func TestRunFromURL_CancelledContext_MarksJobCancelledByUser(t *testing.T) {
	store := newStore(t)
	job, err := store.CreateJobFromURL(context.Background(), "https://example.com/doc", "example.com/doc", "req-1")
	if err != nil {
		t.Fatal(err)
	}

	bp := &blockingParser{store: store, started: make(chan struct{})}
	parsers := map[string]parser.Parser{"pdf": bp}
	r := converter.New(store, newHub(t), parsers, newLogger())

	done := make(chan struct{})
	go func() {
		defer close(done)
		r.RunFromURL(job.ID, "req-1", "pdf", "https://example.com/doc")
	}()

	<-bp.started
	r.Cancel(job.ID)

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("RunFromURL never returned after cancel")
	}

	got, err := store.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "FAILED" {
		t.Errorf("expected FAILED, got %s", got.Status)
	}
	if got.Error != "cancelled by user" {
		t.Errorf("expected 'cancelled by user', got %q", got.Error)
	}
}

func TestRun_UnknownFormat_MarksJobFailed(t *testing.T) {
	store := newStore(t)
	job, err := store.CreateJob(context.Background(), "test.mobi", []byte("x"), "req-1", "mobi")
	if err != nil {
		t.Fatal(err)
	}

	parsers := map[string]parser.Parser{"pdf": &successParser{store: store}}
	r := converter.New(store, newHub(t), parsers, newLogger())
	r.Run(job.ID, "req-1", "mobi", "test.mobi", []byte("x"))

	got, err := store.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "FAILED" {
		t.Errorf("expected FAILED, got %s", got.Status)
	}
	if !strings.Contains(got.Error, "mobi") {
		t.Errorf("expected error to mention unknown format, got: %s", got.Error)
	}
}
