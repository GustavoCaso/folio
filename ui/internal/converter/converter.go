package converter

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
	"sync"
	"time"

	"github.com/GustavoCaso/folio/ui/internal/hub"
	"github.com/GustavoCaso/folio/ui/internal/logging"
	"github.com/GustavoCaso/folio/ui/internal/parser/client"
	"github.com/GustavoCaso/folio/ui/internal/repository"
)

// Runner executes document conversions in the foreground and manages their cancellation.
type Runner struct {
	store   repository.Store
	hub     *hub.Hub
	parser  client.Client
	dataDir string
	logger  *slog.Logger
	cancels sync.Map // jobID → context.CancelFunc
}

func New(store repository.Store, h *hub.Hub, parser client.Client, dataDir string, logger *slog.Logger) *Runner {
	return &Runner{
		store:   store,
		hub:     h,
		parser:  parser,
		dataDir: dataDir,
		logger:  logger,
	}
}

// Cancel cancels a running conversion for jobID. No-op if not running.
func (r *Runner) Cancel(jobID string) {
	if val, ok := r.cancels.Load(jobID); ok {
		if cancel, ok := val.(context.CancelFunc); ok {
			cancel()
		}
	}
}

// HasCancel reports whether an active cancel func exists for jobID.
func (r *Runner) HasCancel(jobID string) bool {
	_, ok := r.cancels.Load(jobID)
	return ok
}

func (r *Runner) Run(jobID, requestID, filename string, pdfBytes []byte) {
	log := r.logger.With("job_id", jobID, "request_id", requestID, "filename", filename)
	start := time.Now()
	log.Info("conversion start", "bytes", len(pdfBytes))

	convertCtx, convertCancel := context.WithCancel(context.Background())
	r.cancels.Store(jobID, convertCancel)
	result, err := r.parser.Convert(convertCtx, jobID, requestID, filename, pdfBytes, r.hub)
	r.cancels.Delete(jobID)
	convertCancel()

	if err != nil {
		r.handleError(log, jobID, err, start, "conversion failed")
		return
	}

	safe := safeSlug(strings.TrimSuffix(filename, filepath.Ext(filename)))
	r.writeAndComplete(jobID, safe, log, result, start)
}

func (r *Runner) RunFromURL(jobID, requestID, sourceURL string) {
	log := r.logger.With("job_id", jobID, "request_id", requestID, "source_url", sourceURL)
	start := time.Now()
	log.Info("url conversion start")

	convertCtx, convertCancel := context.WithCancel(context.Background())
	r.cancels.Store(jobID, convertCancel)
	result, err := r.parser.ConvertFromURL(convertCtx, jobID, requestID, sourceURL, r.hub)
	r.cancels.Delete(jobID)
	convertCancel()

	if err != nil {
		r.handleError(log, jobID, err, start, "url conversion failed")
		return
	}

	parsed, _ := url.Parse(sourceURL)
	safe := safeSlug(parsed.Host + parsed.Path)
	if len(safe) > 64 {
		safe = safe[:64]
	}
	r.writeAndComplete(jobID, safe, log, result, start)
}

func (r *Runner) handleError(log *slog.Logger, jobID string, err error, start time.Time, msg string) {
	log.Error(msg, logging.Err(err), "dur_ms", time.Since(start).Milliseconds())
	errMsg := err.Error()
	if errors.Is(err, context.Canceled) {
		errMsg = "cancelled by user"
	}
	if markErr := r.store.MarkJobFailed(context.Background(), jobID, errMsg); markErr != nil {
		log.Error("mark job failed errored", logging.Err(markErr))
	}
	r.hub.Publish(jobID, hub.StatusEvent{Status: "FAILED", Error: errMsg})
}

func (r *Runner) writeAndComplete(jobID, nameSlug string, log *slog.Logger, result client.ConversionResult, start time.Time) {
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
	outputPath := filepath.Join(r.dataDir, fmt.Sprintf("%s-%s.md", nameSlug, jobID[:8]))

	if err := os.WriteFile(outputPath, md, 0644); err != nil {
		log.Error("write markdown failed", logging.Err(err), "output_path", outputPath)
		if markErr := r.store.MarkJobFailed(context.Background(), jobID, err.Error()); markErr != nil {
			log.Error("mark job failed errored", logging.Err(markErr))
		}
		r.hub.Publish(jobID, hub.StatusEvent{Status: "FAILED", Error: err.Error()})
		return
	}

	if err := r.store.MarkJobDone(context.Background(), jobID, outputPath, result.Title, result.Author, result.Cover); err != nil {
		log.Error("mark job done failed", logging.Err(err))
		return
	}

	log.Info("conversion done",
		"output_path", outputPath,
		"md_bytes", len(md),
		"dur_ms", time.Since(start).Milliseconds(),
	)
	r.hub.Publish(jobID, hub.StatusEvent{
		Status: "DONE",
		Title:  result.Title,
		Author: result.Author,
		Cover:  base64.StdEncoding.EncodeToString(result.Cover),
	})
}

func safeSlug(s string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, s)
}
