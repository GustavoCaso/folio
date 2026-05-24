package templates_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/GustavoCaso/folio/ui/internal/domain"
	"github.com/GustavoCaso/folio/ui/internal/templates"
)

func renderExports(t *testing.T, records []domain.ExportRecord) string {
	t.Helper()
	var buf bytes.Buffer
	if err := templates.Exports(records).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func TestExports_EmptyState(t *testing.T) {
	got := renderExports(t, nil)

	if !strings.Contains(got, "Export History") {
		t.Errorf("expected Export History heading, got:\n%s", got)
	}
	if !strings.Contains(got, "No exports yet") {
		t.Errorf("expected empty-state message, got:\n%s", got)
	}
}

func TestExports_ShowsHighlightTextAndFilename(t *testing.T) {
	got := renderExports(t, []domain.ExportRecord{
		{
			HighlightText: "some important text",
			JobFilename:   "paper.pdf",
			BackendName:   "readwise",
			Status:        "EXPORTED",
			ExportedAt:    time.Now(),
		},
	})

	for _, want := range []string{"some important text", "paper.pdf", "readwise", "Exported"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestExports_ShowsNoteTagsAndJobTags(t *testing.T) {
	got := renderExports(t, []domain.ExportRecord{
		{
			HighlightText: "highlighted passage",
			HighlightNote: "important finding",
			HighlightTag:  "key-insight",
			JobFilename:   "thesis.pdf",
			JobTags:       []string{"science", "research"},
			BackendName:   "readwise",
			Status:        "EXPORTED",
			ExportedAt:    time.Now(),
		},
	})

	for _, want := range []string{
		"highlighted passage",
		"important finding",
		"key-insight",
		"science",
		"research",
		"thesis.pdf",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestExports_EmptyNoteAndTagsShowDash(t *testing.T) {
	got := renderExports(t, []domain.ExportRecord{
		{
			HighlightText: "some text",
			JobFilename:   "doc.pdf",
			BackendName:   "readwise",
			Status:        "EXPORTED",
			ExportedAt:    time.Now(),
		},
	})

	if strings.Count(got, "--") < 2 {
		t.Errorf("expected at least 2 '--' placeholders for empty note and tags, got:\n%s", got)
	}
}

func TestExports_ShowsFailedStatus(t *testing.T) {
	got := renderExports(t, []domain.ExportRecord{
		{
			HighlightText: "highlight text",
			JobFilename:   "doc.pdf",
			BackendName:   "readwise",
			Status:        "FAILED",
			Error:         "connection refused",
		},
	})

	if !strings.Contains(got, "Failed") {
		t.Errorf("expected Failed status badge, got:\n%s", got)
	}
	if !strings.Contains(got, "connection refused") {
		t.Errorf("expected error message, got:\n%s", got)
	}
}
