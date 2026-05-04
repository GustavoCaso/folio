package templates_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/templates"
)

func renderReader(t *testing.T, job db.Job, html string, hls []db.Highlight) string {
	t.Helper()
	var buf bytes.Buffer
	if err := templates.Reader(job, html, hls).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func TestReaderEmbedsHighlightsJSON(t *testing.T) {
	hls := []db.Highlight{{ID: "h1", StartBlockID: "paragraph-1", EndBlockID: "paragraph-1"}}
	got := renderReader(t, db.Job{ID: "j1", Filename: "doc.pdf"}, "<p>x</p>", hls)

	if !strings.Contains(got, `id="highlights-data"`) {
		t.Errorf("expected highlights-data script tag, got: %s", got)
	}
	if !strings.Contains(got, `type="application/json"`) {
		t.Errorf("expected application/json type, got: %s", got)
	}
	if !strings.Contains(got, `"h1"`) {
		t.Errorf("expected highlight ID in JSON payload, got: %s", got)
	}
}

func TestReaderScriptHasNoUnrenderedTemplExpression(t *testing.T) {
	got := renderReader(t, db.Job{ID: "j1"}, "", nil)

	// Regression: templ does NOT interpolate { ... } inside raw <script>.
	// Previous code shipped `window.__highlights = { templ.Raw(...) };` literally,
	// causing a JS SyntaxError.
	for _, bad := range []string{"templ.Raw", "templ.JSONString", "mustJSON("} {
		if strings.Contains(got, bad) {
			t.Errorf("script contains unrendered templ expression %q in: %s", bad, got)
		}
	}
}

func TestReaderRendersHTMLAndJobID(t *testing.T) {
	got := renderReader(t, db.Job{ID: "abc", Filename: "doc.pdf"}, "<h1>Title</h1>", nil)

	if !strings.Contains(got, `data-job-id="abc"`) {
		t.Errorf("expected job ID data attr, got: %s", got)
	}
	if !strings.Contains(got, "<h1>Title</h1>") {
		t.Errorf("expected raw rendered HTML, got: %s", got)
	}
}

func TestReaderRendersPopover(t *testing.T) {
	got := renderReader(t, db.Job{ID: "j1"}, "", nil)

	if !strings.Contains(got, `id="hl-popover"`) {
		t.Errorf("expected popover root id, got: %s", got)
	}
	if !strings.Contains(got, `id="hl-popover-content"`) {
		t.Errorf("expected popover content id, got: %s", got)
	}
	if !strings.Contains(got, `id="hl-save"`) {
		t.Errorf("expected save button, got: %s", got)
	}
	if !strings.Contains(got, `id="hl-cancel"`) {
		t.Errorf("expected cancel button, got: %s", got)
	}
	if !strings.Contains(got, `id="hl-tag"`) {
		t.Errorf("expected tag input, got: %s", got)
	}
	if !strings.Contains(got, `id="hl-note"`) {
		t.Errorf("expected note textarea, got: %s", got)
	}
}

func TestReaderRendersPopoverErrorElement(t *testing.T) {
	got := renderReader(t, db.Job{ID: "j1"}, "", nil)

	if !strings.Contains(got, `id="hl-error" class="hidden`) {
		t.Errorf("expected error element id, got: %s", got)
	}
}
func TestReaderRendersTooltip(t *testing.T) {
	got := renderReader(t, db.Job{ID: "j1"}, "", nil)

	if !strings.Contains(got, `id="hl-tooltip"`) {
		t.Errorf("expected tooltip container, got: %s", got)
	}
	if !strings.Contains(got, `id="hl-tooltip-content"`) {
		t.Errorf("expected tooltip content element, got: %s", got)
	}
	if !strings.Contains(got, `role="status"`) {
		t.Errorf("expected role=status on tooltip, got: %s", got)
	}
}

func TestReaderHighlightsPanelEmpty(t *testing.T) {
	got := renderReader(t, db.Job{ID: "j1"}, "", nil)

	if !strings.Contains(got, `id="highlights-panel"`) {
		t.Errorf("expected highlights panel, got: %s", got)
	}
	if !strings.Contains(got, "No highlights yet") {
		t.Errorf("expected empty-state message, got: %s", got)
	}
}

func TestReaderHighlightsPanelWithHighlights(t *testing.T) {
	hls := []db.Highlight{
		{ID: "h1", Text: "selected text", Tag: "important", Note: "a note"},
		{ID: "h2", Text: "another selection", Tag: "", Note: ""},
	}
	got := renderReader(t, db.Job{ID: "j1"}, "", hls)

	if strings.Contains(got, "No highlights yet") {
		t.Errorf("unexpected empty-state message when highlights are present")
	}
	// Each card must be anchored by its highlight ID for scroll-to and delete.
	for _, id := range []string{"h1", "h2"} {
		if !strings.Contains(got, `data-scroll-to-highlight="`+id+`"`) {
			t.Errorf("expected data-scroll-to-highlight=%q, got: %s", id, got)
		}
		if !strings.Contains(got, `data-delete-highlight="`+id+`"`) {
			t.Errorf("expected data-delete-highlight=%q, got: %s", id, got)
		}
	}
	// Text, tag, and note of the first highlight must be visible.
	for _, want := range []string{"selected text", "important", "a note"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in rendered output, got: %s", want, got)
		}
	}
}
