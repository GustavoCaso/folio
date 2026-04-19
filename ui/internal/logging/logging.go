// Package logging sets up a structured JSON logger and exposes context-bound
// helpers used across the ui service.
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

type ctxKey struct{}
type reqIDKey struct{}

// Init builds a JSON slog.Logger at the requested level and installs it as the
// default logger. Level is parsed case-insensitively; unknown values default to
// info.
func Init(level string) *slog.Logger {
	return initTo(os.Stderr, level)
}

func initTo(w io.Writer, level string) *slog.Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: parseLevel(level),
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.TimeKey:
				a.Key = "ts"
			case slog.LevelKey:
				a.Value = slog.StringValue(strings.ToLower(a.Value.String()))
			case slog.MessageKey:
				a.Key = "msg"
			}
			return a
		},
	})
	l := slog.New(h)
	slog.SetDefault(l)
	return l
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// WithLogger stores l on ctx so downstream code can recover it via LoggerFrom.
func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// LoggerFrom returns the logger previously attached with WithLogger, or the
// default logger if none is present.
func LoggerFrom(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// WithRequestID stores reqID on ctx so handlers can propagate it across
// goroutines and service boundaries (e.g. gRPC metadata).
func WithRequestID(ctx context.Context, reqID string) context.Context {
	return context.WithValue(ctx, reqIDKey{}, reqID)
}

// RequestIDFrom returns the request ID stored on ctx, or "" if none is set.
func RequestIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(reqIDKey{}).(string); ok {
		return v
	}
	return ""
}

// Err formats an error as a string attribute under the key "err".
func Err(err error) slog.Attr {
	if err == nil {
		return slog.String("err", "")
	}
	return slog.String("err", fmt.Sprintf("%v", err))
}
