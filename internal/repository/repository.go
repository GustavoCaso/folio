package repository

import (
	"context"

	"github.com/GustavoCaso/folio/ui/internal/domain"
)

type DocumentRepository interface {
	CreateDocument(ctx context.Context, filename string, content []byte, requestID string, format domain.DocumentFormat) (domain.Document, error)
	GetDocument(ctx context.Context, id string) (domain.Document, error)
	GetPendingDocuments(ctx context.Context) ([]domain.Document, error)
	ListDocuments(ctx context.Context, filter domain.DocumentFilter) ([]domain.Document, error)
	DocumentFacets(ctx context.Context, filter domain.DocumentFilter) (domain.Facets, error)
	UpdateDocumentStatus(ctx context.Context, id, status string) error
	UpdateReadingProgress(ctx context.Context, id, blockID string) error
	RetryDocument(ctx context.Context, id string) error
	MarkDocumentDone(ctx context.Context, id, outputPath, title, author string, cover []byte) error
	UpdateDocument(ctx context.Context, id, title, author string, tags []string, cover []byte) error
	DeleteDocument(ctx context.Context, id string) error
	MarkDocumentFailed(ctx context.Context, id, errMsg string) error
}

type HighlightRepository interface {
	CreateHighlight(ctx context.Context, h domain.Highlight) (domain.Highlight, error)
	ListHighlights(ctx context.Context, documentID string) ([]domain.Highlight, error)
	ListChapterHighlights(ctx context.Context, documentID string, chapter int) ([]domain.Highlight, error)
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
	DocumentRepository
	HighlightRepository
	ExportRepository
	WithTx(ctx context.Context, fn func(ctx context.Context, store Store) error) error
	Close() error
}
