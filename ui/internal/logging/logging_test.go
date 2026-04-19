package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInitEmitsJSON(t *testing.T) {
	var buf bytes.Buffer
	l := initTo(&buf, "info")
	l.Info("hello", "job_id", "abc")

	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("expected JSON line, got %q: %v", buf.String(), err)
	}
	if out["msg"] != "hello" {
		t.Errorf("want msg=hello, got %v", out["msg"])
	}
	if out["level"] != "info" {
		t.Errorf("want level=info, got %v", out["level"])
	}
	if out["job_id"] != "abc" {
		t.Errorf("want job_id=abc, got %v", out["job_id"])
	}
	if _, ok := out["ts"]; !ok {
		t.Errorf("want ts field, got %v", out)
	}
}

func TestLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	l := initTo(&buf, "warn")
	l.Debug("debug-msg")
	l.Info("info-msg")
	l.Warn("warn-msg")

	got := buf.String()
	if strings.Contains(got, "debug-msg") || strings.Contains(got, "info-msg") {
		t.Errorf("lower levels leaked through warn filter: %s", got)
	}
	if !strings.Contains(got, "warn-msg") {
		t.Errorf("warn-msg missing: %s", got)
	}
}

func TestParseLevelUnknownDefaultsToInfo(t *testing.T) {
	if got := parseLevel("whatever"); got != slog.LevelInfo {
		t.Errorf("want info for unknown, got %v", got)
	}
	if got := parseLevel("DEBUG"); got != slog.LevelDebug {
		t.Errorf("case-insensitive DEBUG failed, got %v", got)
	}
}

func TestLoggerFromRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	l := initTo(&buf, "info").With("k", "v")
	ctx := WithLogger(context.Background(), l)
	if LoggerFrom(ctx) != l {
		t.Fatal("LoggerFrom did not return stored logger")
	}
	if LoggerFrom(context.Background()) == nil {
		t.Fatal("LoggerFrom must fall back to default logger")
	}
}

// flushableRecorder embeds httptest.ResponseRecorder and implements
// http.Flusher — mimics a real server's ResponseWriter which supports SSE.
type flushableRecorder struct {
	*httptest.ResponseRecorder
	flushed int
}

func (f *flushableRecorder) Flush() { f.flushed++ }

func TestMiddlewarePreservesFlusher(t *testing.T) {
	var buf bytes.Buffer
	base := initTo(&buf, "info")

	var sawFlusher bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, sawFlusher = w.(http.Flusher)
		if sawFlusher {
			w.(http.Flusher).Flush()
		}
	})

	srv := Middleware(base)(handler)
	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	rr := &flushableRecorder{ResponseRecorder: httptest.NewRecorder()}
	srv.ServeHTTP(rr, req)

	if !sawFlusher {
		t.Fatal("handler did not see http.Flusher through middleware")
	}
	if rr.flushed == 0 {
		t.Fatal("Flush() was not forwarded to underlying writer")
	}
}

func TestMiddlewareLogsRequestAndPropagatesID(t *testing.T) {
	var buf bytes.Buffer
	base := initTo(&buf, "info")

	var seenReqID string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l := LoggerFrom(r.Context())
		if l == nil {
			t.Fatal("no logger in ctx")
		}
		seenReqID = w.Header().Get("X-Request-ID")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("ok"))
	})

	srv := Middleware(base)(handler)
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if seenReqID == "" {
		t.Fatal("request_id missing in response header")
	}
	// The last JSON line should be the http request log.
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	last := lines[len(lines)-1]
	var out map[string]any
	if err := json.Unmarshal([]byte(last), &out); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	if out["msg"] != "http request" {
		t.Errorf("want msg=http request, got %v", out["msg"])
	}
	if out["status"].(float64) != 201 {
		t.Errorf("want status=201, got %v", out["status"])
	}
	if out["request_id"] != seenReqID {
		t.Errorf("request_id mismatch: log=%v header=%s", out["request_id"], seenReqID)
	}
}
