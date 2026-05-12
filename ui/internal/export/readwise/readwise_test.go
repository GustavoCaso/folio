package readwise_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/GustavoCaso/folio/ui/internal/export"
	"github.com/GustavoCaso/folio/ui/internal/export/readwise"
)

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestReadwise(t *testing.T, handler http.Handler) (export.Backend, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	b, err := readwise.New(readwise.Config{
		APIToken: "test-token",
		Timeout:  5 * time.Second,
		BaseURL:  srv.URL + "/api/v2",
	}, silentLogger())
	if err != nil {
		t.Fatal(err)
	}
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

		var req readwise.ReadwiseCreateRequest
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
		result := []readwise.ReadwiseHighlightResponse{{ModifiedHighlights: []int64{42}}}
		json.NewEncoder(w).Encode(result) //nolint:errcheck
	})

	b, _ := newTestReadwise(t, handler)

	records := []*export.ExportRecord{
		{ExportID: "exp-1", HighlighText: "some text", Note: "my note", Title: "book.pdf"},
	}
	err := b.Export(context.Background(), records)
	if err != nil {
		t.Fatal(err)
	}
	if records[0].ExternalID != "42" {
		t.Errorf("expected external_id 42, got %s", records[0].ExternalID)
	}
	if records[0].Err != nil {
		t.Errorf("expected no error, got %v", records[0].Err)
	}
}

func TestReadwiseExport_SendsTagAfterCreation(t *testing.T) {
	tagRequests := 0

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2/highlights/" && r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			result := []readwise.ReadwiseHighlightResponse{{ModifiedHighlights: []int64{7}}}
			json.NewEncoder(w).Encode(result) //nolint:errcheck

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

	records := []*export.ExportRecord{
		{ExportID: "exp-1", HighlighText: "text", Tag: "science", Title: "book.pdf"},
	}
	if err := b.Export(context.Background(), records); err != nil {
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
			result := []readwise.ReadwiseHighlightResponse{{ModifiedHighlights: []int64{1}}}
			json.NewEncoder(w).Encode(result) //nolint:errcheck
			return
		}
		tagRequests++
	})

	b, _ := newTestReadwise(t, handler)

	records := []*export.ExportRecord{
		{ExportID: "exp-1", HighlighText: "text", Title: "book.pdf"},
	}
	if err := b.Export(context.Background(), records); err != nil {
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

	records := []*export.ExportRecord{
		{ExportID: "exp-1", HighlighText: "text", Title: "book.pdf"},
	}
	if err := b.Export(context.Background(), records); err == nil {
		t.Fatal("expected error for non-2xx status, got nil")
	}
}

func TestReadwiseExport_BatchPreservesOrder(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		result := []readwise.ReadwiseHighlightResponse{{ModifiedHighlights: []int64{10, 20}}}
		json.NewEncoder(w).Encode(result) //nolint:errcheck
	})

	b, _ := newTestReadwise(t, handler)

	records := []*export.ExportRecord{
		{ExportID: "exp-1", HighlighText: "first", Title: "a.pdf"},
		{ExportID: "exp-2", HighlighText: "second", Title: "a.pdf"},
	}
	if err := b.Export(context.Background(), records); err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].ExternalID != "10" || records[1].ExternalID != "20" {
		t.Errorf("unexpected external IDs: %s, %s", records[0].ExternalID, records[1].ExternalID)
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

func TestReadwiseNew_RequiresLogger(t *testing.T) {
	_, err := readwise.New(readwise.Config{APIToken: "tok"}, nil)
	if err == nil {
		t.Fatal("expected error when logger is nil")
	}
}
