package views_test

import (
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/domain"
	"github.com/GustavoCaso/folio/ui/internal/views"
)

func TestCoverBase64(t *testing.T) {
	if got := views.CoverBase64(domain.Document{Cover: []byte("hi")}); got != "aGk=" {
		t.Errorf("got %q, want %q", got, "aGk=")
	}
	if got := views.CoverBase64(domain.Document{}); got != "" {
		t.Errorf("no cover should encode to empty, got %q", got)
	}
}
