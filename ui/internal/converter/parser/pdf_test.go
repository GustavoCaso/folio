package parser_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/GustavoCaso/folio/ui/internal/converter/parser"
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

type successClient struct {
	markdown []byte
	images   []parserclient.ImageFile
}

func (c *successClient) Convert(_ context.Context, _, _, _ string, _ []byte, _ *hub.Hub) (parserclient.ConversionResult, error) {
	return parserclient.ConversionResult{Markdown: c.markdown, Images: c.images}, nil
}
func (c *successClient) ConvertFromURL(_ context.Context, _, _, _ string, _ *hub.Hub) (parserclient.ConversionResult, error) {
	return parserclient.ConversionResult{Markdown: c.markdown, Images: c.images}, nil
}
func (c *successClient) Health(_ context.Context) bool { return true }
func (c *successClient) Close() error                  { return nil }

type failClient struct{ err error }

func (c *failClient) Convert(_ context.Context, _, _, _ string, _ []byte, _ *hub.Hub) (parserclient.ConversionResult, error) {
	return parserclient.ConversionResult{}, c.err
}
func (c *failClient) ConvertFromURL(_ context.Context, _, _, _ string, _ *hub.Hub) (parserclient.ConversionResult, error) {
	return parserclient.ConversionResult{}, c.err
}
func (c *failClient) Health(_ context.Context) bool { return false }
func (c *failClient) Close() error                  { return nil }

type blockingClient struct{ started chan struct{} }

func (c *blockingClient) Convert(ctx context.Context, _, _, _ string, _ []byte, _ *hub.Hub) (parserclient.ConversionResult, error) {
	close(c.started)
	<-ctx.Done()
	return parserclient.ConversionResult{}, ctx.Err()
}
func (c *blockingClient) ConvertFromURL(ctx context.Context, _, _, _ string, _ *hub.Hub) (parserclient.ConversionResult, error) {
	close(c.started)
	<-ctx.Done()
	return parserclient.ConversionResult{}, ctx.Err()
}
func (c *blockingClient) Health(_ context.Context) bool { return true }
func (c *blockingClient) Close() error                  { return nil }

func TestPDFParser_Convert_MarksDone(t *testing.T) {
	store := newStore(t)
	dir := t.TempDir()
	job, err := store.CreateJob(context.Background(), "test.pdf", []byte("%PDF"), "req-1", "pdf")
	if err != nil {
		t.Fatal(err)
	}

	p := parser.NewPDF(store, newHub(t), &successClient{markdown: []byte("# Hello")}, dir, newLogger())
	err = p.Convert(context.Background(), job.ID, "req-1", "test.pdf", []byte("%PDF"), newHub(t))
	if err != nil {
		t.Fatal(err)
	}

	got, err := store.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "DONE" {
		t.Errorf("expected DONE, got %s", got.Status)
	}
	content, err := os.ReadFile(got.OutputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(content), "# Hello") {
		t.Errorf("expected markdown in output, got: %s", content)
	}
}

func TestPDFParser_Convert_RewritesImageRefsToBase64(t *testing.T) {
	store := newStore(t)
	dir := t.TempDir()
	job, err := store.CreateJob(context.Background(), "img.pdf", []byte("%PDF"), "req-1", "pdf")
	if err != nil {
		t.Fatal(err)
	}

	md := []byte("# Doc\n\n![fig](/tmp/tmpXXX/doc_artifacts/image_000.png)\n")
	client := &successClient{
		markdown: md,
		images:   []parserclient.ImageFile{{Filename: "image_000.png", Data: []byte{0x89, 'P', 'N', 'G'}}},
	}

	p := parser.NewPDF(store, newHub(t), client, dir, newLogger())
	if err := p.Convert(context.Background(), job.ID, "req-1", "img.pdf", []byte("%PDF"), newHub(t)); err != nil {
		t.Fatal(err)
	}

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

func TestPDFParser_Convert_ClientError_MarksJobFailed(t *testing.T) {
	store := newStore(t)
	dir := t.TempDir()
	job, err := store.CreateJob(context.Background(), "fail.pdf", []byte("%PDF"), "req-1", "pdf")
	if err != nil {
		t.Fatal(err)
	}

	p := parser.NewPDF(store, newHub(t), &failClient{err: errors.New("client exploded")}, dir, newLogger())
	err = p.Convert(context.Background(), job.ID, "req-1", "fail.pdf", []byte("%PDF"), newHub(t))
	if err == nil {
		t.Fatal("expected error")
	}

	got, err2 := store.GetJob(context.Background(), job.ID)
	if err2 != nil {
		t.Fatal(err2)
	}
	if got.Status != "FAILED" {
		t.Errorf("expected FAILED, got %s", got.Status)
	}
	if !strings.Contains(got.Error, "client exploded") {
		t.Errorf("expected error message, got: %s", got.Error)
	}
}

func TestPDFParser_Convert_CancelledContext_MarksJobCancelledByUser(t *testing.T) {
	store := newStore(t)
	dir := t.TempDir()
	job, err := store.CreateJob(context.Background(), "cancel.pdf", []byte("%PDF"), "req-1", "pdf")
	if err != nil {
		t.Fatal(err)
	}

	bc := &blockingClient{started: make(chan struct{})}
	p := parser.NewPDF(store, newHub(t), bc, dir, newLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = p.Convert(ctx, job.ID, "req-1", "cancel.pdf", []byte("%PDF"), newHub(t))
	}()

	<-bc.started
	cancel()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Convert never returned after cancel")
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
