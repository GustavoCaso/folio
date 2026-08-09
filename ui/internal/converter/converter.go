package converter

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/GustavoCaso/folio/ui/internal/converter/parser"
	"github.com/GustavoCaso/folio/ui/internal/hub"
	"github.com/GustavoCaso/folio/ui/internal/logging"
	"github.com/GustavoCaso/folio/ui/internal/repository"
)

// Runner dispatches document conversions to a format-specific parser.Parser
// and manages per-job cancellation, independent of which parser handles it.
type Runner struct {
	store   repository.Store
	hub     *hub.Hub
	parsers map[string]parser.Parser
	logger  *slog.Logger
	cancels sync.Map // jobID → context.CancelFunc
}

func New(store repository.Store, h *hub.Hub, parsers map[string]parser.Parser, logger *slog.Logger) *Runner {
	return &Runner{
		store:   store,
		hub:     h,
		parsers: parsers,
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

func (r *Runner) Run(jobID, requestID, format, filename string, data []byte) {
	log := r.logger.With("job_id", jobID, "request_id", requestID, "filename", filename, "format", format)
	start := time.Now()
	log.Info("conversion start", "bytes", len(data))

	p, ok := r.parsers[format]
	if !ok {
		r.handleUnknownFormat(log, jobID, format)
		return
	}

	convertCtx, convertCancel := context.WithCancel(context.Background())
	r.cancels.Store(jobID, convertCancel)
	err := p.Convert(convertCtx, jobID, requestID, filename, data, r.hub)
	r.cancels.Delete(jobID)
	convertCancel()

	if err != nil {
		log.Error("conversion failed", logging.Err(err), "dur_ms", time.Since(start).Milliseconds())
		return
	}
	log.Info("conversion done", "dur_ms", time.Since(start).Milliseconds())
}

func (r *Runner) RunFromURL(jobID, requestID, format, sourceURL string) {
	log := r.logger.With("job_id", jobID, "request_id", requestID, "source_url", sourceURL, "format", format)
	start := time.Now()
	log.Info("url conversion start")

	p, ok := r.parsers[format]
	if !ok {
		r.handleUnknownFormat(log, jobID, format)
		return
	}

	convertCtx, convertCancel := context.WithCancel(context.Background())
	r.cancels.Store(jobID, convertCancel)
	err := p.ConvertFromURL(convertCtx, jobID, requestID, sourceURL, r.hub)
	r.cancels.Delete(jobID)
	convertCancel()

	if err != nil {
		log.Error("url conversion failed", logging.Err(err), "dur_ms", time.Since(start).Milliseconds())
		return
	}
	log.Info("url conversion done", "dur_ms", time.Since(start).Milliseconds())
}

func (r *Runner) handleUnknownFormat(log *slog.Logger, jobID, format string) {
	errMsg := fmt.Sprintf("no parser registered for format %q", format)
	log.Error(errMsg)
	if markErr := r.store.MarkJobFailed(context.Background(), jobID, errMsg); markErr != nil {
		log.Error("mark job failed errored", logging.Err(markErr))
	}
	r.hub.Publish(jobID, hub.StatusEvent{Status: "FAILED", Error: errMsg})
}
