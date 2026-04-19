package client_test

import (
	"testing"

	parserclient "github.com/GustavoCaso/folio/ui/internal/parser/client"
)

func TestNewClientBadAddress(t *testing.T) {
	// grpc.Dial is lazy — connection errors appear at first RPC, not at New().
	// We just verify construction doesn't panic.
	c, err := parserclient.New("localhost:0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer c.Close()
}
