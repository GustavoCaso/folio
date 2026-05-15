package client_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/GustavoCaso/folio/ui/internal/hub"
	pb "github.com/GustavoCaso/folio/ui/internal/parser/proto"
)

// fakeParserServer implements pb.ParserServiceServer for testing.
type fakeParserServer struct {
	pb.UnimplementedParserServiceServer
	responses []*pb.ConvertResult
}

func (s *fakeParserServer) ConvertDocument(stream grpc.BidiStreamingServer[pb.ConvertChunk, pb.ConvertResult]) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	for _, r := range s.responses {
		if err := stream.Send(r); err != nil {
			return err
		}
	}
	return nil
}

func startFakeParser(t *testing.T, responses []*pb.ConvertResult) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	pb.RegisterParserServiceServer(srv, &fakeParserServer{responses: responses})
	go srv.Serve(lis) //nolint:errcheck
	t.Cleanup(srv.Stop)
	return lis.Addr().String()
}

func newHub(t *testing.T) *hub.Hub {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := hub.New(logger)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// collectBuffered drains all events already buffered in ch without blocking.
func collectBuffered(ch chan hub.StatusEvent) []hub.StatusEvent {
	var events []hub.StatusEvent
	for {
		select {
		case e := <-ch:
			events = append(events, e)
		default:
			return events
		}
	}
}

func TestConvert_DoesNotPublishDONEEvent(t *testing.T) {
	responses := []*pb.ConvertResult{
		{Payload: &pb.ConvertResult_Status{Status: &pb.StatusUpdate{
			Status: "PROCESSING", Stage: "layout", PagesDone: 0, PagesTotal: 1,
		}}},
		{Payload: &pb.ConvertResult_MarkdownChunk{MarkdownChunk: []byte("# Hello")}},
		{Payload: &pb.ConvertResult_Metadata{Metadata: &pb.DocumentMetadata{
			Title: "My Book", Author: "Alice", Cover: []byte{1, 2, 3},
		}}},
		{Payload: &pb.ConvertResult_Status{Status: &pb.StatusUpdate{
			Status: "DONE", Stage: "done", PagesDone: 1, PagesTotal: 1,
		}}},
	}

	addr := startFakeParser(t, responses)
	c := newClient(t, addr)
	h := newHub(t)

	const jobID = "test-job-1"
	ch := h.Subscribe(jobID)
	defer h.Unsubscribe(jobID, ch)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := c.Convert(ctx, jobID, "req-1", "test.pdf", []byte("%PDF-1.4"), h)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	events := collectBuffered(ch)

	for _, e := range events {
		if e.Status == "DONE" {
			t.Errorf("client published DONE event; only runConversion should publish DONE")
		}
	}

	// Sanity: PROCESSING event must be present.
	var hasProcessing bool
	for _, e := range events {
		if e.Status == "PROCESSING" {
			hasProcessing = true
		}
	}
	if !hasProcessing {
		t.Error("expected at least one PROCESSING event")
	}

	// Sanity: metadata is returned in result.
	if result.Title != "My Book" {
		t.Errorf("Title = %q, want %q", result.Title, "My Book")
	}
	if result.Author != "Alice" {
		t.Errorf("Author = %q, want %q", result.Author, "Alice")
	}
	if len(result.Cover) != 3 {
		t.Errorf("Cover len = %d, want 3", len(result.Cover))
	}
}

func TestConvert_PublishesFAILEDEvent(t *testing.T) {
	responses := []*pb.ConvertResult{
		{Payload: &pb.ConvertResult_Status{Status: &pb.StatusUpdate{
			Status: "FAILED", Error: "something went wrong",
		}}},
	}

	addr := startFakeParser(t, responses)
	c := newClient(t, addr)
	h := newHub(t)

	const jobID = "test-job-2"
	ch := h.Subscribe(jobID)
	defer h.Unsubscribe(jobID, ch)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c.Convert(ctx, jobID, "req-2", "test.pdf", []byte("%PDF-1.4"), h) //nolint:errcheck

	events := collectBuffered(ch)

	var hasFailed bool
	for _, e := range events {
		if e.Status == "FAILED" {
			hasFailed = true
		}
	}
	if !hasFailed {
		t.Error("expected a FAILED event to be published")
	}
}
