package client

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/GustavoCaso/folio/ui/internal/hub"
	pb "github.com/GustavoCaso/folio/ui/internal/parser/proto"
)

const chunkSize = 512 * 1024 // 512 KB per gRPC send chunk

// ConversionResult is returned when the stream completes successfully.
type ConversionResult struct {
	Markdown []byte
}

// Client wraps the gRPC ParserService connection.
type Client struct {
	conn   *grpc.ClientConn
	client pb.ParserServiceClient
}

func New(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpc dial %s: %w", addr, err)
	}
	return &Client{conn: conn, client: pb.NewParserServiceClient(conn)}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

// Convert sends pdfBytes to the parser, publishes StatusEvents to h for jobID,
// and returns the assembled Markdown when the stream completes.
// Blocking: returns only when conversion is done or has failed.
func (c *Client) Convert(ctx context.Context, jobID, filename string, pdfBytes []byte, h *hub.Hub) (ConversionResult, error) {
	stream, err := c.client.ConvertDocument(ctx)
	if err != nil {
		return ConversionResult{}, fmt.Errorf("open stream: %w", err)
	}

	// Send meta frame first
	if err := stream.Send(&pb.ConvertChunk{
		Payload: &pb.ConvertChunk_Meta{
			Meta: &pb.ConvertMeta{Filename: filename},
		},
	}); err != nil {
		return ConversionResult{}, fmt.Errorf("send meta: %w", err)
	}

	// Send PDF bytes in chunks
	reader := bytes.NewReader(pdfBytes)
	buf := make([]byte, chunkSize)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			if err := stream.Send(&pb.ConvertChunk{
				Payload: &pb.ConvertChunk_Data{Data: buf[:n]},
			}); err != nil {
				return ConversionResult{}, fmt.Errorf("send data: %w", err)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return ConversionResult{}, fmt.Errorf("read pdf: %w", err)
		}
	}

	// Signal end of client stream
	if err := stream.CloseSend(); err != nil {
		return ConversionResult{}, fmt.Errorf("close send: %w", err)
	}

	// Receive status updates and markdown chunks
	var mdBuf bytes.Buffer
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			h.Publish(jobID, hub.StatusEvent{Status: "FAILED", Error: err.Error()})
			return ConversionResult{}, fmt.Errorf("recv: %w", err)
		}

		switch p := msg.Payload.(type) {
		case *pb.ConvertResult_Status:
			h.Publish(jobID, hub.StatusEvent{
				Status:     p.Status.Status,
				PagesDone:  int(p.Status.PagesDone),
				PagesTotal: int(p.Status.PagesTotal),
				Error:      p.Status.Error,
				Stage:      p.Status.Stage,
				Message:    p.Status.Message,
			})
		case *pb.ConvertResult_MarkdownChunk:
			mdBuf.Write(p.MarkdownChunk)
		}
	}

	return ConversionResult{Markdown: mdBuf.Bytes()}, nil
}
