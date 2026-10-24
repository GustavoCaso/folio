package converter

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/GustavoCaso/folio/ui/internal/converter/parser"
	"github.com/GustavoCaso/folio/ui/internal/domain"
	"github.com/GustavoCaso/folio/ui/internal/hub"
	"github.com/GustavoCaso/folio/ui/internal/logging"
	"github.com/GustavoCaso/folio/ui/internal/repository"
)

// Runner dispatches document conversions to a format-specific parser.Parser
// and manages per-document cancellation, independent of which parser handles it.
type Runner struct {
	store   repository.Store
	hub     *hub.Hub
	parsers map[domain.DocumentFormat]parser.Parser
	logger  *slog.Logger
	cancels sync.Map // documentID → context.CancelFunc
}

func New(store repository.Store, h *hub.Hub, parsers map[domain.DocumentFormat]parser.Parser, logger *slog.Logger) *Runner {
	return &Runner{
		store:   store,
		hub:     h,
		parsers: parsers,
		logger:  logger,
	}
}

// Cancel cancels a running conversion for documentID. No-op if not running.
func (r *Runner) Cancel(documentID string) {
	if val, ok := r.cancels.Load(documentID); ok {
		if cancel, ok := val.(context.CancelFunc); ok {
			cancel()
		}
	}
}

// HasCancel reports whether an active cancel func exists for documentID.
func (r *Runner) HasCancel(documentID string) bool {
	_, ok := r.cancels.Load(documentID)
	return ok
}

// IsSync reports whether format's parser should run inside the request.
// Unregistered formats report false; Run marks them failed either way.
func (r *Runner) IsSync(format domain.DocumentFormat) bool {
	p, ok := r.parsers[format]
	return ok && p.Sync()
}

// Run converts the document. The parser records the outcome (DONE or FAILED
// with its error) in the store, so there is nothing to return.
func (r *Runner) Run(documentID, requestID string, format domain.DocumentFormat, filename string, data []byte) {
	log := r.logger.With("document_id", documentID, "request_id", requestID, "filename", filename, "format", format)
	start := time.Now()
	log.Info("conversion start", "bytes", len(data))

	p, ok := r.parsers[format]
	if !ok {
		r.handleUnknownFormat(log, documentID, format)
		return
	}

	convertCtx, convertCancel := context.WithCancel(context.Background())
	r.cancels.Store(documentID, convertCancel)
	err := p.Convert(convertCtx, documentID, requestID, filename, data, r.hub)
	r.cancels.Delete(documentID)
	convertCancel()

	if err != nil {
		log.Error("conversion failed", logging.Err(err), "dur_ms", time.Since(start).Milliseconds())
		return
	}
	log.Info("conversion done", "dur_ms", time.Since(start).Milliseconds())
}

func (r *Runner) handleUnknownFormat(log *slog.Logger, documentID string, format domain.DocumentFormat) {
	errMsg := fmt.Sprintf("no parser registered for format %q", format)
	log.Error(errMsg)
	if markErr := r.store.MarkDocumentFailed(context.Background(), documentID, errMsg); markErr != nil {
		log.Error("mark document failed errored", logging.Err(markErr))
	}
	r.hub.Publish(documentID, hub.StatusEvent{Status: "FAILED", Error: errMsg})
}
