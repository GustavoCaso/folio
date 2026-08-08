package converter_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"io"
	"log/slog"

	"github.com/GustavoCaso/folio/ui/internal/converter"
	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/hub"
	parserclient "github.com/GustavoCaso/folio/ui/internal/parser/client"
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

// successParser returns a fixed markdown result.
type successParser struct {
	markdown []byte
	images   []parserclient.ImageFile
}

func (p *successParser) Convert(_ context.Context, _, _, _ string, _ []byte, _ *hub.Hub) (parserclient.ConversionResult, error) {
	return parserclient.ConversionResult{Markdown: p.markdown, Images: p.images}, nil
}
func (p *successParser) ConvertFromURL(_ context.Context, _, _, _ string, _ *hub.Hub) (parserclient.ConversionResult, error) {
	return parserclient.ConversionResult{Markdown: p.markdown, Images: p.images}, nil
}
func (p *successParser) Health(_ context.Context) bool { return true }
func (p *successParser) Close() error {
	return nil
}

// failParser always returns an error.
type failParser struct{ err error }

func (p *failParser) Convert(_ context.Context, _, _, _ string, _ []byte, _ *hub.Hub) (parserclient.ConversionResult, error) {
	return parserclient.ConversionResult{}, p.err
}
func (p *failParser) ConvertFromURL(_ context.Context, _, _, _ string, _ *hub.Hub) (parserclient.ConversionResult, error) {
	return parserclient.ConversionResult{}, p.err
}
func (p *failParser) Health(_ context.Context) bool { return false }
func (p *failParser) Close() error {
	return nil
}

// blockingParser blocks until its context is cancelled.
type blockingParser struct {
	started chan struct{}
}

func (p *blockingParser) Convert(ctx context.Context, _, _, _ string, _ []byte, _ *hub.Hub) (parserclient.ConversionResult, error) {
	close(p.started)
	<-ctx.Done()
	return parserclient.ConversionResult{}, ctx.Err()
}
func (p *blockingParser) ConvertFromURL(ctx context.Context, _, _, _ string, _ *hub.Hub) (parserclient.ConversionResult, error) {
	close(p.started)
	<-ctx.Done()
	return parserclient.ConversionResult{}, ctx.Err()
}
func (p *blockingParser) Health(_ context.Context) bool { return true }
func (p *blockingParser) Close() error {
	return nil
}

func TestRun_MarksDone(t *testing.T) {
	store := newStore(t)
	dir := t.TempDir()
	job, err := store.CreateJob(context.Background(), "test.pdf", []byte("%PDF"), "req-1", "pdf")
	if err != nil {
		t.Fatal(err)
	}

	r := converter.New(store, newHub(t), &successParser{markdown: []byte("# Hello")}, dir, newLogger())
	r.Run(job.ID, "req-1", "test.pdf", []byte("%PDF"))

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
	content, err := os.ReadFile(got.OutputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(content), "# Hello") {
		t.Errorf("expected markdown in output, got: %s", content)
	}
}

func TestRunFromURL_MarksDone(t *testing.T) {
	store := newStore(t)
	dir := t.TempDir()
	job, err := store.CreateJobFromURL(context.Background(), "https://example.com/doc", "example.com/doc", "req-1")
	if err != nil {
		t.Fatal(err)
	}

	r := converter.New(store, newHub(t), &successParser{markdown: []byte("# From URL")}, dir, newLogger())
	r.RunFromURL(job.ID, "req-1", "https://example.com/doc")

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
	dir := t.TempDir()
	job, err := store.CreateJob(context.Background(), "fail.pdf", []byte("%PDF"), "req-1", "pdf")
	if err != nil {
		t.Fatal(err)
	}

	r := converter.New(store, newHub(t), &failParser{err: errors.New("parser exploded")}, dir, newLogger())
	r.Run(job.ID, "req-1", "fail.pdf", []byte("%PDF"))

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
	dir := t.TempDir()
	job, err := store.CreateJob(context.Background(), "cancel.pdf", []byte("%PDF"), "req-1", "pdf")
	if err != nil {
		t.Fatal(err)
	}

	bp := &blockingParser{started: make(chan struct{})}
	r := converter.New(store, newHub(t), bp, dir, newLogger())

	done := make(chan struct{})
	go func() {
		defer close(done)
		r.Run(job.ID, "req-1", "cancel.pdf", []byte("%PDF"))
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

func TestRun_RewritesImageRefsToBase64(t *testing.T) {
	store := newStore(t)
	dir := t.TempDir()
	job, err := store.CreateJob(context.Background(), "img.pdf", []byte("%PDF"), "req-1", "pdf")
	if err != nil {
		t.Fatal(err)
	}

	md := []byte("# Doc\n\n![fig](/tmp/tmpXXX/doc_artifacts/image_000.png)\n")
	parser := &successParser{
		markdown: md,
		images:   []parserclient.ImageFile{{Filename: "image_000.png", Data: []byte{0x89, 'P', 'N', 'G'}}},
	}

	r := converter.New(store, newHub(t), parser, dir, newLogger())
	r.Run(job.ID, "req-1", "img.pdf", []byte("%PDF"))

	got, err := store.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(got.OutputPath)
	if strings.Contains(string(content), "image_000.png") {
		t.Error("image filename still in markdown — ref not rewritten")
	}
	if !strings.Contains(string(content), "data:image/png;base64,") {
		t.Errorf("expected base64 data URI, got:\n%s", content)
	}
}

func TestRunFromURL_CancelledContext_MarksJobCancelledByUser(t *testing.T) {
	store := newStore(t)
	dir := t.TempDir()
	job, err := store.CreateJobFromURL(context.Background(), "https://example.com/doc", "example.com/doc", "req-1")
	if err != nil {
		t.Fatal(err)
	}

	bp := &blockingParser{started: make(chan struct{})}
	r := converter.New(store, newHub(t), bp, dir, newLogger())

	done := make(chan struct{})
	go func() {
		defer close(done)
		r.RunFromURL(job.ID, "req-1", "https://example.com/doc")
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
