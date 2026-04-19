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
