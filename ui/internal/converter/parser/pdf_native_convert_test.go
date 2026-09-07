package parser_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/converter/parser"
	"github.com/GustavoCaso/folio/ui/internal/hub"

	"github.com/ardanlabs/kronk/sdk/kronk/model"
)

// fakeAIClient is a parser.AIClient test double that always returns a
// single fixed prose block, regardless of input — sufficient for
// Convert-level tests that only assert non-empty chapter output, not exact
// block content. Avoids downloading a real Kronk model during tests.
type fakeAIClient struct{}

func (fakeAIClient) ChatStreaming(ctx context.Context, d model.D) (<-chan model.ChatResponse, error) {
	ch := make(chan model.ChatResponse, 1)
	ch <- model.ChatResponse{
		Choices: []model.Choice{{
			Delta: &model.ResponseMessage{Content: `{"blocks":[{"kind":"prose","text":"stub text"}]}`},
		}},
	}
	close(ch)
	return ch, nil
}

// helloWorldPDF is a minimal single-page PDF fixture with no bookmarks,
// copied (MIT license) from github.com/klippa-app/go-pdfium's own test
// fixtures — used here to exercise nativePDFParser.Convert end-to-end
// against real pdfium WASM output, not just the pure-function pieces.
func helloWorldPDF(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "hello_world.pdf"))
	if err != nil {
		t.Fatalf("ReadFile fixture: %v", err)
	}
	return data
}

func TestNativePDFConvert_WritesChapterAndTOCAndMarksDone(t *testing.T) {
	dataDir := t.TempDir()
	store := &fakeStore{}
	p, err := parser.NewNativePDF(store, nil, dataDir, parser.AIConfig{}, parser.WithAIClient(fakeAIClient{}))
	if err != nil {
		t.Fatalf("NewNativePDF: %v", err)
	}

	pdfBytes := helloWorldPDF(t)
	if err := p.Convert(context.Background(), "job-native-1", "req-1", "hello.pdf", pdfBytes, nil); err != nil {
		t.Fatalf("Convert: %v", err)
	}

	if store.doneID != "job-native-1" {
		t.Fatalf("job not marked done: %+v", store)
	}

	entries, err := os.ReadDir(filepath.Join(dataDir, "job-native-1"))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var sawChapter, sawTOC bool
	for _, e := range entries {
		if e.Name() == "toc.json" {
			sawTOC = true
		}
		if filepath.Ext(e.Name()) == ".html" {
			sawChapter = true
		}
	}
	if !sawChapter || !sawTOC {
		t.Fatalf("expected chapter html + toc.json, got %v", entries)
	}

	tocBytes, err := os.ReadFile(filepath.Join(dataDir, "job-native-1", "toc.json"))
	if err != nil {
		t.Fatalf("ReadFile toc.json: %v", err)
	}
	var toc []tocEntry
	if err := json.Unmarshal(tocBytes, &toc); err != nil {
		t.Fatalf("Unmarshal toc.json: %v", err)
	}
	if len(toc) != 1 {
		t.Fatalf("expected 1 synthetic toc entry (no bookmarks fixture), got %d: %+v", len(toc), toc)
	}

	chapterHTML, err := os.ReadFile(filepath.Join(dataDir, "job-native-1", "chapter-0.html"))
	if err != nil {
		t.Fatalf("ReadFile chapter-0.html: %v", err)
	}
	if len(chapterHTML) == 0 {
		t.Errorf("expected non-empty chapter HTML")
	}
}

func TestNativePDFConvert_PublishesPerPageProgress(t *testing.T) {
	dataDir := t.TempDir()
	store := &fakeStore{}
	h, err := hub.New(slog.Default())
	if err != nil {
		t.Fatalf("hub.New: %v", err)
	}
	p, err := parser.NewNativePDF(store, h, dataDir, parser.AIConfig{}, parser.WithAIClient(fakeAIClient{}))
	if err != nil {
		t.Fatalf("NewNativePDF: %v", err)
	}

	ch := h.Subscribe("job-native-progress")
	defer h.Unsubscribe("job-native-progress", ch)

	pdfBytes := helloWorldPDF(t)
	if err := p.Convert(context.Background(), "job-native-progress", "req-1", "hello.pdf", pdfBytes, h); err != nil {
		t.Fatalf("Convert: %v", err)
	}

	var sawPageProgress, sawDone bool
	for {
		select {
		case evt := <-ch:
			if evt.Status == "PROCESSING" && evt.Message == "page 1/1" {
				sawPageProgress = true
			}
			if evt.Status == "DONE" {
				sawDone = true
			}
		default:
			if !sawPageProgress {
				t.Errorf("expected a PROCESSING event with Message %q", "page 1/1")
			}
			if !sawDone {
				t.Errorf("expected a terminal DONE event")
			}
			return
		}
	}
}

// bookmarksPDF is a 2-page PDF fixture with bookmarks, copied (MIT
// license) from github.com/klippa-app/go-pdfium's own test fixtures — used
// here to exercise multi-page
// resume behavior: a job that fails partway through must not redo already-
// completed pages' AI cleanup on retry.
func bookmarksPDF(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "bookmarks.pdf"))
	if err != nil {
		t.Fatalf("ReadFile fixture: %v", err)
	}
	return data
}

// failOnNthCallAIClient is a parser.AIClient test double that fails its
// Nth call (1-indexed) and succeeds on every other call, for simulating a
// job that fails partway through a multi-page document.
type failOnNthCallAIClient struct {
	failOn int
	calls  int
}

func (f *failOnNthCallAIClient) ChatStreaming(ctx context.Context, d model.D) (<-chan model.ChatResponse, error) {
	f.calls++
	if f.calls == f.failOn {
		return nil, errors.New("simulated failure")
	}
	return fakeAIClient{}.ChatStreaming(ctx, d)
}

func TestNativePDFConvert_RetryAfterPartialFailureSkipsAlreadyCleanedPages(t *testing.T) {
	dataDir := t.TempDir()
	store := &fakeStore{}
	failingClient := &failOnNthCallAIClient{failOn: 2}
	p, err := parser.NewNativePDF(store, nil, dataDir, parser.AIConfig{}, parser.WithAIClient(failingClient))
	if err != nil {
		t.Fatalf("NewNativePDF: %v", err)
	}

	pdfBytes := bookmarksPDF(t)
	const jobID = "job-native-resume"
	if err := p.Convert(context.Background(), jobID, "req-1", "bookmarks.pdf", pdfBytes, nil); err == nil {
		t.Fatal("expected Convert to fail on the second page's AI cleanup call")
	}
	if store.failedID != jobID {
		t.Fatalf("expected job marked failed after partial progress, got %+v", store)
	}

	fragments, err := filepath.Glob(filepath.Join(dataDir, jobID, "page-*.html.part"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(fragments) != 1 {
		t.Fatalf("expected exactly 1 checkpointed page fragment after failing on page 2, got %d: %v", len(fragments), fragments)
	}

	// Retry: same jobID/outDir (mirrors handlers.RetryDocument reusing the
	// job's ID), a fresh AI client that always succeeds. If the resume
	// logic works, the previously-checkpointed page is served from its
	// fragment file, not re-sent to the (now-succeeding) client -- so the
	// total call count on this second client should be 1 (only the
	// second, previously-failing page), not 2.
	succeedingClient := &failOnNthCallAIClient{failOn: 0}
	p2, err := parser.NewNativePDF(store, nil, dataDir, parser.AIConfig{}, parser.WithAIClient(succeedingClient))
	if err != nil {
		t.Fatalf("NewNativePDF (retry): %v", err)
	}
	if err := p2.Convert(context.Background(), jobID, "req-1", "bookmarks.pdf", pdfBytes, nil); err != nil {
		t.Fatalf("Convert (retry): %v", err)
	}
	if store.doneID != jobID {
		t.Fatalf("expected job marked done after retry, got %+v", store)
	}
	if succeedingClient.calls != 1 {
		t.Errorf("expected retry to only re-run AI cleanup for the 1 unfinished page, got %d calls", succeedingClient.calls)
	}

	remaining, err := filepath.Glob(filepath.Join(dataDir, jobID, "page-*.html.part"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("expected page fragments cleaned up after successful completion, got %v", remaining)
	}
}

func TestNativePDFConvert_MarksFailedOnInvalidPDF(t *testing.T) {
	store := &fakeStore{}
	p, err := parser.NewNativePDF(store, nil, t.TempDir(), parser.AIConfig{})
	if err != nil {
		t.Fatalf("NewNativePDF: %v", err)
	}

	_ = p.Convert(context.Background(), "job-native-2", "req-1", "bad.pdf", []byte("not a pdf"), nil)

	if store.failedID != "job-native-2" || store.failedErr == "" {
		t.Errorf("expected job-native-2 marked failed with an error, got %+v", store)
	}
}
