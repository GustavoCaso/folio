package client_test

import (
	"io"
	"log/slog"
	"testing"

	parserclient "github.com/GustavoCaso/folio/ui/internal/parser/client"
)

func TestNewClientBadAddress(t *testing.T) {
	// grpc.Dial is lazy — connection errors appear at first RPC, not at New().
	// We just verify construction doesn't panic.
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	c, err := parserclient.New("localhost:0", logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = c.Close() }()
}

func TestNewClientRequiresLogger(t *testing.T) {
	if _, err := parserclient.New("localhost:0", nil); err == nil {
		t.Fatal("expected error for nil logger")
	}
}
