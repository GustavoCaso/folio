package export_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/export"
)

func newTestReadwise(t *testing.T, handler http.Handler) (export.Backend, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	b := export.NewReadwise(export.ReadwiseConfig{
		APIToken: "test-token",
		Timeout:  5 * time.Second,
		BaseURL:  srv.URL + "/api/v2",
	})
	return b, srv
}

func TestReadwiseExport_HappyPath(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/highlights/" || r.Method != http.MethodPost {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Token test-token" {
			t.Errorf("expected Authorization header Token test-token, got %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", got)
		}

		var req struct {
			Highlights []struct {
				Text     string `json:"text"`
				Title    string `json:"title"`
				Category string `json:"category"`
				Note     string `json:"note"`
			} `json:"highlights"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(req.Highlights) != 1 {
			t.Fatalf("expected 1 highlight in request, got %d", len(req.Highlights))
		}
		if req.Highlights[0].Text != "some text" {
			t.Errorf("expected text %q, got %q", "some text", req.Highlights[0].Text)
		}
		if req.Highlights[0].Title != "book.pdf" {
			t.Errorf("expected title book.pdf, got %q", req.Highlights[0].Title)
		}
		if req.Highlights[0].Category != "books" {
			t.Errorf("expected category books, got %q", req.Highlights[0].Category)
		}
		if req.Highlights[0].Note != "my note" {
			t.Errorf("expected note %q, got %q", "my note", req.Highlights[0].Note)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{{"id": 42}}) //nolint:errcheck
	})

	b, _ := newTestReadwise(t, handler)

	results, err := b.Export(context.Background(), []db.HighlightWithJob{
		{
			Highlight:   db.Highlight{ID: "h1", Text: "some text", Note: "my note"},
			JobFilename: "book.pdf",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].HighlightID != "h1" {
		t.Errorf("expected highlight_id h1, got %s", results[0].HighlightID)
	}
	if results[0].ExternalID != "42" {
		t.Errorf("expected external_id 42, got %s", results[0].ExternalID)
	}
	if results[0].Err != nil {
		t.Errorf("expected no error, got %v", results[0].Err)
	}
}

func TestReadwiseExport_SendsTagAfterCreation(t *testing.T) {
	tagRequests := 0

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2/highlights/" && r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]map[string]any{{"id": 7}}) //nolint:errcheck

		case r.URL.Path == "/api/v2/highlights/7/tags/" && r.Method == http.MethodPost:
			tagRequests++
			var body struct {
				Name string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode tag request: %v", err)
			}
			if body.Name != "science" {
				t.Errorf("expected tag name science, got %q", body.Name)
			}
			w.WriteHeader(http.StatusOK)

		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	})

	b, _ := newTestReadwise(t, handler)

	_, err := b.Export(context.Background(), []db.HighlightWithJob{
		{Highlight: db.Highlight{ID: "h1", Text: "text", Tag: "science"}, JobFilename: "book.pdf"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tagRequests != 1 {
		t.Errorf("expected 1 tag request, got %d", tagRequests)
	}
}

func TestReadwiseExport_NoTagRequestWhenTagEmpty(t *testing.T) {
	tagRequests := 0

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/highlights/" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]map[string]any{{"id": 1}}) //nolint:errcheck
			return
		}
		tagRequests++
	})

	b, _ := newTestReadwise(t, handler)

	_, err := b.Export(context.Background(), []db.HighlightWithJob{
		{Highlight: db.Highlight{ID: "h1", Text: "text", Tag: ""}, JobFilename: "book.pdf"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tagRequests != 0 {
		t.Errorf("expected no tag requests for empty tag, got %d", tagRequests)
	}
}

func TestReadwiseExport_NonOKStatusReturnsError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	b, _ := newTestReadwise(t, handler)

	_, err := b.Export(context.Background(), []db.HighlightWithJob{
		{Highlight: db.Highlight{ID: "h1", Text: "text"}, JobFilename: "book.pdf"},
	})
	if err == nil {
		t.Fatal("expected error for non-2xx status, got nil")
	}
}

func TestReadwiseExport_BatchPreservesOrder(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{ //nolint:errcheck
			{"id": 10},
			{"id": 20},
		})
	})

	b, _ := newTestReadwise(t, handler)

	results, err := b.Export(context.Background(), []db.HighlightWithJob{
		{Highlight: db.Highlight{ID: "h1", Text: "first"}, JobFilename: "a.pdf"},
		{Highlight: db.Highlight{ID: "h2", Text: "second"}, JobFilename: "a.pdf"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ExternalID != "10" || results[1].ExternalID != "20" {
		t.Errorf("unexpected external IDs: %s, %s", results[0].ExternalID, results[1].ExternalID)
	}
}

func TestReadwiseDelete_HappyPath(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/highlights/99/" || r.Method != http.MethodDelete {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Token test-token" {
			t.Errorf("expected Authorization header, got %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	b, _ := newTestReadwise(t, handler)

	if err := b.Delete(context.Background(), "99"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestReadwiseDelete_NonOKStatusReturnsError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	b, _ := newTestReadwise(t, handler)

	if err := b.Delete(context.Background(), "99"); err == nil {
		t.Fatal("expected error for non-2xx status, got nil")
	}
}
