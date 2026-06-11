package templates_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/templates"
)

func renderLayout(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	if err := templates.Layout("Test", templates.NoAction()).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func TestLayout_HasParserStatusBadge(t *testing.T) {
	got := renderLayout(t)
	if !strings.Contains(got, `id="parser-status-badge"`) {
		t.Errorf("expected parser-status-badge in layout, got:\n%s", got)
	}
}

func TestLayout_ParserBadgeShowsChecking(t *testing.T) {
	got := renderLayout(t)
	if !strings.Contains(got, "Parser checking") {
		t.Errorf("expected checking state on initial render, got:\n%s", got)
	}
}

func TestLayout_HasPollingScript(t *testing.T) {
	got := renderLayout(t)
	if !strings.Contains(got, "/health/parser") {
		t.Errorf("expected polling script referencing /health/parser in layout, got:\n%s", got)
	}
}
