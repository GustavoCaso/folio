package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"

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
	logger *slog.Logger
}

func New(addr string, logger *slog.Logger) (*Client, error) {
	if logger == nil {
		return nil, fmt.Errorf("client.New: logger is required")
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpc dial %s: %w", addr, err)
	}
	logger.Info("grpc client ready", "addr", addr)
	return &Client{conn: conn, client: pb.NewParserServiceClient(conn), logger: logger}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

// Convert sends pdfBytes to the parser, publishes StatusEvents to h for jobID,
// and returns the assembled Markdown when the stream completes.
// Blocking: returns only when conversion is done or has failed.
func (c *Client) Convert(ctx context.Context, jobID, requestID, filename string, pdfBytes []byte, h *hub.Hub) (ConversionResult, error) {
	log := c.logger.With("job_id", jobID, "request_id", requestID, "filename", filename)
	start := time.Now()

	stream, err := c.client.ConvertDocument(ctx)
	if err != nil {
		log.Error("open stream failed", "err", err.Error())
		return ConversionResult{}, fmt.Errorf("open stream: %w", err)
	}
	log.Info("stream opened", "bytes", len(pdfBytes))

	// Send meta frame first
	if err := stream.Send(&pb.ConvertChunk{
		Payload: &pb.ConvertChunk_Meta{
			Meta: &pb.ConvertMeta{Filename: filename, RequestId: requestID},
		},
	}); err != nil {
		log.Error("send meta failed", "err", err.Error())
		return ConversionResult{}, fmt.Errorf("send meta: %w", err)
	}

	// Send PDF bytes in chunks
	reader := bytes.NewReader(pdfBytes)
	buf := make([]byte, chunkSize)
	chunksSent := 0
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			if err := stream.Send(&pb.ConvertChunk{
				Payload: &pb.ConvertChunk_Data{Data: buf[:n]},
			}); err != nil {
				log.Error("send data failed", "err", err.Error(), "chunks_sent", chunksSent)
				return ConversionResult{}, fmt.Errorf("send data: %w", err)
			}
			chunksSent++
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Error("read pdf failed", "err", err.Error())
			return ConversionResult{}, fmt.Errorf("read pdf: %w", err)
		}
	}
	log.Debug("pdf sent", "chunks", chunksSent)

	// Signal end of client stream
	if err := stream.CloseSend(); err != nil {
		log.Error("close send failed", "err", err.Error())
		return ConversionResult{}, fmt.Errorf("close send: %w", err)
	}

	// Receive status updates and markdown chunks
	var mdBuf bytes.Buffer
	mdChunks := 0
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Error("recv failed", "err", err.Error())
			h.Publish(jobID, hub.StatusEvent{Status: "FAILED", Error: err.Error()})
			return ConversionResult{}, fmt.Errorf("recv: %w", err)
		}

		switch p := msg.Payload.(type) {
		case *pb.ConvertResult_Status:
			log.Debug("status",
				"status", p.Status.Status,
				"stage", p.Status.Stage,
				"pages_done", p.Status.PagesDone,
				"pages_total", p.Status.PagesTotal,
			)
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
			mdChunks++
		}
	}

	log.Info("stream done",
		"dur_ms", time.Since(start).Milliseconds(),
		"md_bytes", mdBuf.Len(),
		"md_chunks", mdChunks,
	)
	return ConversionResult{Markdown: mdBuf.Bytes()}, nil
}

// Health returns true if the parser reports SERVING via grpc health protocol.
func (c *Client) Health(ctx context.Context) bool {
	hc := grpc_health_v1.NewHealthClient(c.conn)
	resp, err := hc.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return false
	}
	return resp.GetStatus() == grpc_health_v1.HealthCheckResponse_SERVING
}
