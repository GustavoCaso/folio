package parser

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/GustavoCaso/folio/ui/internal/hub"
	"github.com/GustavoCaso/folio/ui/internal/logging"
	"github.com/GustavoCaso/folio/ui/internal/parser/client"
	"github.com/GustavoCaso/folio/ui/internal/repository"
)

// pdfParser converts PDF/URL jobs via the gRPC parser client, writing a
// single Markdown file per job.
type pdfParser struct {
	store   repository.Store
	hub     *hub.Hub
	client  client.Client
	dataDir string
	logger  *slog.Logger
}

// NewPDF constructs the pdf/markdown Parser.
func NewPDF(store repository.Store, h *hub.Hub, c client.Client, dataDir string, logger *slog.Logger) Parser {
	return &pdfParser{store: store, hub: h, client: c, dataDir: dataDir, logger: logger}
}

func (p *pdfParser) Convert(ctx context.Context, jobID, requestID, filename string, data []byte, h *hub.Hub) error {
	log := p.logger.With("job_id", jobID, "request_id", requestID, "filename", filename)
	start := time.Now()
	log.Info("conversion start", "bytes", len(data))

	result, err := p.client.Convert(ctx, jobID, requestID, filename, data, h)
	if err != nil {
		p.handleError(log, jobID, err, start, "conversion failed")
		return err
	}

	safe := safeSlug(strings.TrimSuffix(filename, filepath.Ext(filename)))
	return p.writeAndComplete(jobID, safe, log, result, start)
}

func (p *pdfParser) ConvertFromURL(ctx context.Context, jobID, requestID, sourceURL string, h *hub.Hub) error {
	log := p.logger.With("job_id", jobID, "request_id", requestID, "source_url", sourceURL)
	start := time.Now()
	log.Info("url conversion start")

	result, err := p.client.ConvertFromURL(ctx, jobID, requestID, sourceURL, h)
	if err != nil {
		p.handleError(log, jobID, err, start, "url conversion failed")
		return err
	}

	parsed, _ := url.Parse(sourceURL)
	safe := safeSlug(parsed.Host + parsed.Path)
	if len(safe) > 64 {
		safe = safe[:64]
	}
	return p.writeAndComplete(jobID, safe, log, result, start)
}

func (p *pdfParser) handleError(log *slog.Logger, jobID string, err error, start time.Time, msg string) {
	log.Error(msg, logging.Err(err), "dur_ms", time.Since(start).Milliseconds())
	errMsg := err.Error()
	if errors.Is(err, context.Canceled) {
		errMsg = "cancelled by user"
	}
	if markErr := p.store.MarkJobFailed(context.Background(), jobID, errMsg); markErr != nil {
		log.Error("mark job failed errored", logging.Err(markErr))
	}
	p.hub.Publish(jobID, hub.StatusEvent{Status: "FAILED", Error: errMsg})
}

func (p *pdfParser) writeAndComplete(jobID, nameSlug string, log *slog.Logger, result client.ConversionResult, start time.Time) error {
	log.Debug("rewriting images", "image_count", len(result.Images))
	stringMarkdown := string(result.Markdown)

	for _, img := range result.Images {
		dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(img.Data)
		pat := regexp.MustCompile(`\]\([^)]*` + regexp.QuoteMeta(img.Filename) + `\)`)
		before := stringMarkdown
		stringMarkdown = pat.ReplaceAllString(stringMarkdown, "]("+dataURI+")")
		if stringMarkdown == before {
			log.Warn("image ref not found in markdown", "filename", img.Filename)
		} else {
			log.Debug("image rewritten", "filename", img.Filename, "data_uri_bytes", len(dataURI))
		}
	}

	md := []byte(stringMarkdown)
	outputPath := filepath.Join(p.dataDir, fmt.Sprintf("%s-%s.md", nameSlug, jobID[:8]))

	if err := os.WriteFile(outputPath, md, 0644); err != nil {
		log.Error("write markdown failed", logging.Err(err), "output_path", outputPath)
		if markErr := p.store.MarkJobFailed(context.Background(), jobID, err.Error()); markErr != nil {
			log.Error("mark job failed errored", logging.Err(markErr))
		}
		p.hub.Publish(jobID, hub.StatusEvent{Status: "FAILED", Error: err.Error()})
		return err
	}

	if err := p.store.MarkJobDone(context.Background(), jobID, outputPath, result.Title, result.Author, result.Cover); err != nil {
		log.Error("mark job done failed", logging.Err(err))
		return err
	}

	log.Info("conversion done",
		"output_path", outputPath,
		"md_bytes", len(md),
		"dur_ms", time.Since(start).Milliseconds(),
	)
	p.hub.Publish(jobID, hub.StatusEvent{
		Status: "DONE",
		Title:  result.Title,
		Author: result.Author,
		Cover:  base64.StdEncoding.EncodeToString(result.Cover),
	})
	return nil
}

func safeSlug(s string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, s)
}
