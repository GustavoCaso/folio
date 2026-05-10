package export

import (
	"context"
	"fmt"

	"github.com/GustavoCaso/folio/ui/internal/db"
)

// ExportResult holds the outcome of exporting a single highlight.
type ExportResult struct {
	HighlightID string
	ExternalID  string // ID assigned by the external service; "" on failure
	Err         error
}

// Backend is an external service that can receive and delete highlights.
type Backend interface {
	Name() string
	// Export sends highlights to the external service.
	// Returns one ExportResult per input highlight in the same order.
	Export(ctx context.Context, highlights []db.HighlightWithJob) ([]ExportResult, error)
	// Delete removes a highlight from the external service by its external ID.
	Delete(ctx context.Context, externalID string) error
}

// New instantiates a backend by name using the provided config map.
// Panics for unknown names — callers in main only pass compile-time constants.
func New(name string, config map[string]string) Backend {
	switch name {
	case "readwise":
		return newReadwise(config)
	default:
		panic(fmt.Sprintf("unknown export backend %q", name))
	}
}
