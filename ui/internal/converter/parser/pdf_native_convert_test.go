package parser_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/converter/parser"
)

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
	p, err := parser.NewNativePDF(store, nil, dataDir)
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

func TestNativePDFConvert_MarksFailedOnInvalidPDF(t *testing.T) {
	store := &fakeStore{}
	p, err := parser.NewNativePDF(store, nil, t.TempDir())
	if err != nil {
		t.Fatalf("NewNativePDF: %v", err)
	}

	_ = p.Convert(context.Background(), "job-native-2", "req-1", "bad.pdf", []byte("not a pdf"), nil)

	if store.failedID != "job-native-2" || store.failedErr == "" {
		t.Errorf("expected job-native-2 marked failed with an error, got %+v", store)
	}
}
