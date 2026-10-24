package views

import (
	"encoding/base64"

	"github.com/GustavoCaso/folio/ui/internal/domain"
)

// CoverBase64 encodes the cover for a data: URI src.
func CoverBase64(j domain.Document) string {
	return base64.StdEncoding.EncodeToString(j.Cover)
}
