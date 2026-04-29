package client_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	parserclient "github.com/GustavoCaso/folio/ui/internal/parser/client"
	"google.golang.org/grpc"
	grpchealth "google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func startHealthServer(t *testing.T, status grpc_health_v1.HealthCheckResponse_ServingStatus) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	hs := grpchealth.NewServer()
	grpc_health_v1.RegisterHealthServer(srv, hs)
	hs.SetServingStatus("", status)
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)
	return lis.Addr().String()
}

func newClient(t *testing.T, addr string) *parserclient.Client {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	c, err := parserclient.New(addr, logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestHealth_Serving(t *testing.T) {
	addr := startHealthServer(t, grpc_health_v1.HealthCheckResponse_SERVING)
	c := newClient(t, addr)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if !c.Health(ctx) {
		t.Fatal("expected healthy, got unhealthy")
	}
}

func TestHealth_NotServing(t *testing.T) {
	addr := startHealthServer(t, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	c := newClient(t, addr)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if c.Health(ctx) {
		t.Fatal("expected unhealthy, got healthy")
	}
}
