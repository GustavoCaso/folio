package logging

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// statusRecorder captures the HTTP status code written by downstream handlers.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

// Flush forwards to the underlying ResponseWriter when it implements
// http.Flusher. Required so SSE handlers can stream events — otherwise the
// wrapper hides the Flusher interface from type assertions.
func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Middleware generates a request_id, binds a logger with it onto the request
// context, and logs one line per request with method, path, status, and dur_ms.
func Middleware(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := uuid.NewString()
			l := base.With("request_id", reqID)
			ctx := WithLogger(r.Context(), l)
			ctx = WithRequestID(ctx, reqID)
			r = r.WithContext(ctx)

			w.Header().Set("X-Request-ID", reqID)
			rec := &statusRecorder{ResponseWriter: w}
			start := time.Now()
			next.ServeHTTP(rec, r)

			l.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"bytes", rec.bytes,
				"dur_ms", time.Since(start).Milliseconds(),
				"remote", r.RemoteAddr,
			)
		})
	}
}
