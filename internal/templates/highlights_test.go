package templates_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/domain"
	"github.com/GustavoCaso/folio/ui/internal/templates"
	"github.com/GustavoCaso/folio/ui/internal/views"
)

func renderDocumentHighlights(t *testing.T, document domain.Document, groups []views.ChapterHighlights, total int) string {
	t.Helper()
	var buf bytes.Buffer
	if err := templates.DocumentHighlights(document, groups, total).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func TestDocumentHighlights_EmptyState(t *testing.T) {
	got := renderDocumentHighlights(t, domain.Document{ID: "d1", Title: "Dune", Status: "DONE"}, nil, 0)

	for _, want := range []string{"Dune", "0 highlights", "No highlights yet", `href="/read/d1"`} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestDocumentHighlights_ChapterSectionsLinkToReader(t *testing.T) {
	groups := []views.ChapterHighlights{{
		Chapter: 2,
		Title:   "The Desert",
		Highlights: []domain.Highlight{
			{ID: "h1", Chapter: 2, Text: "Fear is the mind-killer.", Note: "classic", Tag: "quote"},
		},
	}}
	got := renderDocumentHighlights(t, domain.Document{ID: "d1", Title: "Dune", Author: "Frank Herbert", Status: "DONE"}, groups, 1)

	for _, want := range []string{
		"Frank Herbert", "1 highlight", "The Desert", "Fear is the mind-killer.", "classic", "quote",
		`href="/read/d1?chapter=2&amp;highlight=h1"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got:\n%s", want, got)
		}
	}
}
