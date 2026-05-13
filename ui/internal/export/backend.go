package export

import (
	"context"
)

// ExportRecord holds the information for exporting
// When export is successful the Export function must update the ExternalID
// otherwise must populate the Err
type ExportRecord struct {
	ExportID     string // ID of the highlight_exports row
	HighlightText string
	Title        string
	Note         string
	Tag          string
	ExternalID   string // ID assigned by the external service; "" on failure
	Err          error
}

// Backend is an external service that can receive and delete highlights.
type Backend interface {
	Name() string
	// Export sends highlights to the external service.
	//
	// Implementations must mutate each record in place:
	//   - On success: set ExternalID to the ID assigned by the service.
	//   - On per-item failure: set Err; leave ExternalID empty.
	// Every record must have ExternalID or Err set before returning nil.
	// Return a non-nil error only when the entire batch fails (e.g. auth error,
	// network failure); in that case individual records need not be updated.
	Export(ctx context.Context, records []*ExportRecord) error
	// Delete removes a highlight from the external service by its external ID.
	Delete(ctx context.Context, externalID string) error
}
