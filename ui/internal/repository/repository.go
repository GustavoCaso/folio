package repository

import (
	"context"

	"github.com/GustavoCaso/folio/ui/internal/domain"
)

type JobRepository interface {
	CreateJob(ctx context.Context, filename string, content []byte, requestID string) (domain.Job, error)
	GetJob(ctx context.Context, id string) (domain.Job, error)
	GetPendingJobs(ctx context.Context) ([]domain.Job, error)
	ListJobs(ctx context.Context) ([]domain.Job, error)
	UpdateJobStatus(ctx context.Context, id, status string) error
	UpdateReadingProgress(ctx context.Context, id, blockID string) error
	RetryJob(ctx context.Context, id string) error
	MarkJobDone(ctx context.Context, id, outputPath, title, author string, cover []byte) error
	UpdateJob(ctx context.Context, id, title, author string, tags []string, cover []byte) error
	DeleteJob(ctx context.Context, id string) error
	MarkJobFailed(ctx context.Context, id, errMsg string) error
}

type HighlightRepository interface {
	CreateHighlight(ctx context.Context, h domain.Highlight) (domain.Highlight, error)
	ListHighlights(ctx context.Context, jobID string) ([]domain.Highlight, error)
	DeleteHighlight(ctx context.Context, id string) error
}

type ExportRepository interface {
	ListUnexportedHighlights(ctx context.Context, backendName string) ([]domain.ExportRecord, error)
	CreateHighlightExport(ctx context.Context, highlightID, backendName string) error
	MarkHighlightExported(ctx context.Context, exportID, externalID string) error
	MarkHighlightExportFailed(ctx context.Context, exportID, errMsg string) error
	ListExportsByHighlight(ctx context.Context, highlightID string) ([]domain.ExportRecord, error)
	ListAllExports(ctx context.Context) ([]domain.ExportRecord, error)
}

type Store interface {
	JobRepository
	HighlightRepository
	ExportRepository
	WithTx(ctx context.Context, fn func(ctx context.Context, store Store) error) error
	Close() error
}
