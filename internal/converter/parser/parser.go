package parser

import (
	"context"

	"github.com/GustavoCaso/folio/ui/internal/hub"
)

// Parser converts a document's raw bytes and is
// responsible for the full conversion lifecycle: writing output, marking
// the document done or failed via its own Store, and publishing hub events.
type Parser interface {
	Convert(ctx context.Context, documentID, requestID, filename string, data []byte, h *hub.Hub) error
	// Sync reports whether Convert is fast enough to run inside the upload
	// request. Sync formats are finished before the redirect, so the list page
	// renders their final state; the rest run in the background and report
	// progress over SSE.
	Sync() bool
}
