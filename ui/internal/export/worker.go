package export

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/GustavoCaso/folio/ui/internal/db"
)

type Worker struct {
	store    *db.Store
	backends []Backend
	interval time.Duration
	logger   *slog.Logger
}

func NewWorker(store *db.Store, backends []Backend, interval time.Duration, logger *slog.Logger) (*Worker, error) {
	if logger == nil {
		return nil, errors.New("export.NewWorker: logger is required")
	}

	if store == nil {
		return nil, errors.New("export.NewWorker: store is required")
	}

	if interval <= 0 {
		return nil, errors.New("export.NewWorker: interval must be greater than zero")
	}

	return &Worker{
		store:    store,
		backends: backends,
		interval: interval,
		logger:   logger,
	}, nil
}

func (w *Worker) Run(ctx context.Context) {
	w.RunOnce(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			w.RunOnce(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// RunOnce executes one export pass across all backends. Exported for testing.
func (w *Worker) RunOnce(ctx context.Context) {
	for _, backend := range w.backends {
		records, err := w.store.ListUnexportedHighlights(ctx, backend.Name())
		if err != nil {
			w.logger.Error("list unexported highlights failed", "backend", backend.Name(), "err", err)
			continue
		}
		if len(records) == 0 {
			continue
		}

		exports := make([]*ExportRecord, 0, len(records))

		for _, record := range records {
			exports = append(exports, &ExportRecord{
				ExportID:      record.ID,
				Title:         record.JobFilename,
				HighlightText: record.HighlightText,
				Note:          record.HighlightNote,
				Tag:           record.HighlightTag,
			})
		}

		w.logger.Info("exporting highlights", "backend", backend.Name(), "count", len(records))

		err = backend.Export(ctx, exports)
		if err != nil {
			w.logger.Error("export failed", "backend", backend.Name(), "err", err)
			for _, rec := range records {
				if dbErr := w.store.MarkHighlightExportFailed(ctx, rec.ID, err.Error()); dbErr != nil {
					w.logger.Error("mark export failed", "export_id", rec.ID, "err", dbErr)
				}
			}
			continue
		}

		for _, res := range exports {
			if res.Err != nil || res.ExternalID == "" {
				var errorMessage string
				if res.Err != nil {
					errorMessage = res.Err.Error()
				} else {
					errorMessage = fmt.Sprintf("backend %s did not populate ExternalID for highlight_expert %s", backend.Name(), res.ExportID)
				}

				w.logger.Warn("highlight export failed", "backend", backend.Name(), "export_id", res.ExportID, "err", res.Err)
				if dbErr := w.store.MarkHighlightExportFailed(ctx, res.ExportID, errorMessage); dbErr != nil {
					w.logger.Error("mark export failed", "export_id", res.ExportID, "err", dbErr)
				}
			} else {
				w.logger.Info("highlight exported", "backend", backend.Name(), "export_id", res.ExportID, "external_id", res.ExternalID)
				if dbErr := w.store.MarkHighlightExported(ctx, res.ExportID, res.ExternalID); dbErr != nil {
					w.logger.Error("mark highlight exported", "export_id", res.ExportID, "err", dbErr)
				}
			}
		}
	}
}
