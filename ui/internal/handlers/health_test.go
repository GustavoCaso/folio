package handlers_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/handlers"
	"github.com/GustavoCaso/folio/ui/internal/hub"
	parserclient "github.com/GustavoCaso/folio/ui/internal/parser/client"
)

type fakeParserClient struct{ healthy bool }

func (f *fakeParserClient) Health(_ context.Context) bool { return f.healthy }

func (f *fakeParserClient) Convert(_ context.Context, _, _, _ string, _ []byte, _ *hub.Hub) (parserclient.ConversionResult, error) {
	return parserclient.ConversionResult{}, nil
}

func newHealthMux(t *testing.T, healthy bool) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := hub.New(logger)
	if err != nil {
		t.Fatal(err)
	}
	mux, err := handlers.Register(nil, h, &fakeParserClient{healthy: healthy}, t.TempDir(), logger, nil)
	if err != nil {
		t.Fatal(err)
	}
	return mux
}

func TestParserHealth_Healthy(t *testing.T) {
	req := httptest.NewRequest("GET", "/health/parser", nil)
	w := httptest.NewRecorder()
	newHealthMux(t, true).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "healthy" {
		t.Fatalf("expected healthy, got %q", body["status"])
	}
}

func TestParserHealth_Unhealthy(t *testing.T) {
	req := httptest.NewRequest("GET", "/health/parser", nil)
	w := httptest.NewRecorder()
	newHealthMux(t, false).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "unhealthy" {
		t.Fatalf("expected unhealthy, got %q", body["status"])
	}
}
