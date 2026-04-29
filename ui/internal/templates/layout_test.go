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
	if err := templates.Layout("Test").Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func TestLayout_HasParserDot(t *testing.T) {
	got := renderLayout(t)
	if !strings.Contains(got, `id="parser-status"`) {
		t.Errorf("expected parser-status dot in layout, got:\n%s", got)
	}
}

func TestLayout_ParserDotStartsChecking(t *testing.T) {
	got := renderLayout(t)
	if !strings.Contains(got, `class="parser-dot checking"`) {
		t.Errorf("expected checking state on initial render, got:\n%s", got)
	}
}

func TestLayout_HasParserStatusText(t *testing.T) {
	got := renderLayout(t)
	if !strings.Contains(got, `id="parser-status-text"`) {
		t.Errorf("expected parser-status-text element in layout, got:\n%s", got)
	}
}

func TestLayout_HasPollingScript(t *testing.T) {
	got := renderLayout(t)
	if !strings.Contains(got, "/health/parser") {
		t.Errorf("expected polling script referencing /health/parser in layout, got:\n%s", got)
	}
}

func TestLayout_ScriptAfterChildren(t *testing.T) {
	got := renderLayout(t)
	childrenPos := strings.Index(got, `<br>`)
	scriptPos := strings.Index(got, `/health/parser`)
	if childrenPos == -1 || scriptPos == -1 {
		t.Fatal("missing <br> or /health/parser in layout output")
	}
	if scriptPos < childrenPos {
		t.Errorf("polling script appears before children slot — script must come after children so #upload-btn is in DOM")
	}
}
