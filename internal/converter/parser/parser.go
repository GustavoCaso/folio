package parser

import (
	"context"

	"github.com/GustavoCaso/folio/ui/internal/hub"
)

// Parser converts a job's raw bytes and is
// responsible for the full conversion lifecycle: writing output, marking
// the job done or failed via its own Store, and publishing hub events.
type Parser interface {
	Convert(ctx context.Context, jobID, requestID, filename string, data []byte, h *hub.Hub) error
}
