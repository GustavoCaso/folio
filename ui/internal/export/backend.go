package export

import (
	"context"

	"github.com/GustavoCaso/folio/ui/internal/db"
)

// ExportResult holds the outcome of exporting a single highlight.
type ExportResult struct {
	ExportID   string // ID of the highlight_exports row
	ExternalID string // ID assigned by the external service; "" on failure
	Err        error
}

// Backend is an external service that can receive and delete highlights.
type Backend interface {
	Name() string
	// Export sends highlights to the external service.
	// Returns one ExportResult per input record in the same order.
	Export(ctx context.Context, records []db.ExportRecord) ([]ExportResult, error)
	// Delete removes a highlight from the external service by its external ID.
	Delete(ctx context.Context, externalID string) error
}
