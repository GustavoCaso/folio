package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/domain"
	"github.com/GustavoCaso/folio/ui/internal/repository"
)

// seedDocumentAndHighlight creates a document, marks it done, then creates a highlight
// attached to that document. Returns both for further assertions.
func seedDocumentAndHighlight(t *testing.T, store repository.Store) (domain.Document, domain.Highlight) {
	t.Helper()
	document, err := store.CreateDocument(context.Background(), "doc.epub", []byte{}, "req-1", domain.EpubFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkDocumentDone(context.Background(), document.ID, "/dev/null", "", "", nil); err != nil {
		t.Fatal(err)
	}
	h, err := store.CreateHighlight(context.Background(), domain.Highlight{
		DocumentID:   document.ID,
		StartBlockID: "paragraph-1",
		EndBlockID:   "paragraph-1",
		StartPos:     0,
		EndPos:       5,
		Text:         "hello",
		Tag:          "important",
		Note:         "remember",
	})
	if err != nil {
		t.Fatal(err)
	}
	return document, h
}

func TestCreateHighlight_HappyPath(t *testing.T) {
	store := newTestStore(t)
	document, err := store.CreateDocument(context.Background(), "doc.epub", []byte{}, "req-1", domain.EpubFormat)
	if err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(map[string]any{
		"document_id":    document.ID,
		"start_block_id": "paragraph-1",
		"end_block_id":   "paragraph-1",
		"start_pos":      0,
		"end_pos":        5,
		"text":           "hello",
		"tag":            "important",
		"note":           "a note",
	})
	req := httptest.NewRequest(http.MethodPost, "/highlights", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("expected application/json Content-Type, got %q", ct)
	}

	var got map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("could not decode response: %v", err)
	}
	if got["id"] == "" || got["id"] == nil {
		t.Errorf("expected non-empty id in response, got %v", got)
	}
	if got["text"] != "hello" {
		t.Errorf("expected text=hello, got %v", got["text"])
	}
}

func TestCreateHighlight_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/highlights", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, newTestStore(t))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("expected application/json Content-Type, got %q", ct)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("expected JSON error body, got: %s", rec.Body.String())
	}
	if body["error"] == "" {
		t.Errorf("expected non-empty 'error' key in response, got %v", body)
	}
}

func TestDeleteHighlight_HappyPath(t *testing.T) {
	store := newTestStore(t)
	_, h := seedDocumentAndHighlight(t, store)

	req := httptest.NewRequest(http.MethodDelete, "/highlights/"+h.ID, nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("expected text/html Content-Type, got %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "Highlight deleted") {
		t.Errorf("expected success toast in response, got:\n%s", rec.Body.String())
	}
}

func TestDeleteHighlight_RemovesFromStore(t *testing.T) {
	store := newTestStore(t)
	document, h := seedDocumentAndHighlight(t, store)

	req := httptest.NewRequest(http.MethodDelete, "/highlights/"+h.ID, nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	highlights, err := store.ListHighlights(context.Background(), document.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(highlights) != 0 {
		t.Errorf("expected highlight to be deleted, got %d remaining", len(highlights))
	}
}

func TestDeleteHighlight_UnknownID_ReturnsErrorToast(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/highlights/no-such-id", nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, newTestStore(t))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Fail to delete highlight") {
		t.Errorf("expected error toast in response, got:\n%s", rec.Body.String())
	}
}

func TestCreateHighlight_DerivesChapterFromStartBlockID(t *testing.T) {
	cases := map[string]int{
		"ch0-p-1":     0,
		"ch12-h2-3":   12,
		"paragraph-1": 0,
	}
	for blockID, want := range cases {
		t.Run(blockID, func(t *testing.T) {
			store := newTestStore(t)
			document, err := store.CreateDocument(context.Background(), "doc.epub", []byte{}, "req-1", domain.EpubFormat)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := json.Marshal(map[string]any{
				"document_id":    document.ID,
				"start_block_id": blockID,
				"end_block_id":   blockID,
				"start_pos":      0,
				"end_pos":        1,
				"text":           "x",
			})
			req := httptest.NewRequest(http.MethodPost, "/highlights", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			mux, _ := newTestMux(t, store)
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusCreated {
				t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
			}
			var got domain.Highlight
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatal(err)
			}
			if got.Chapter != want {
				t.Errorf("expected chapter %d, got %d", want, got.Chapter)
			}
		})
	}
}

func TestListDocumentHighlights_GroupsByTOCChapter(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	dir := t.TempDir()
	toc := `[{"title":"Opening","chapter_idx":0,"anchor":"","items":null},{"title":"The Desert","chapter_idx":1,"anchor":"","items":null}]`
	if err := os.WriteFile(filepath.Join(dir, "toc.json"), []byte(toc), 0o644); err != nil {
		t.Fatal(err)
	}
	document, err := store.CreateDocument(ctx, "dune.epub", []byte{}, "req-1", domain.EpubFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkDocumentDone(ctx, document.ID, dir, "Dune", "Frank Herbert", nil); err != nil {
		t.Fatal(err)
	}
	h, err := store.CreateHighlight(ctx, domain.Highlight{
		DocumentID: document.ID, StartBlockID: "ch1-p-3", EndBlockID: "ch1-p-3", Chapter: 1, EndPos: 5, Text: "Fear is the mind-killer.",
	})
	if err != nil {
		t.Fatal(err)
	}

	mux, _ := newTestMux(t, store)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/documents/"+document.ID+"/highlights", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Dune", "The Desert", "Fear is the mind-killer.",
		"/read/" + document.ID + "?chapter=1&amp;highlight=" + h.ID,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %q in body", want)
		}
	}
	if strings.Contains(body, "Opening") {
		t.Errorf("chapter without highlights should not render a section")
	}
}

func TestListDocumentHighlights_MissingTOCFallsBackToChapterNumber(t *testing.T) {
	store := newTestStore(t)
	document, _ := seedDocumentAndHighlight(t, store)

	mux, _ := newTestMux(t, store)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/documents/"+document.ID+"/highlights", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Chapter 1") {
		t.Errorf("expected fallback chapter heading, got:\n%s", rec.Body.String())
	}
}

func TestListDocumentHighlights_UnknownDocument(t *testing.T) {
	store := newTestStore(t)
	mux, _ := newTestMux(t, store)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/documents/nope/highlights", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
