package templates_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/templates"
)

func renderDocuments(t *testing.T, jobs []db.Job, watchJobID string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := templates.Documents(jobs, watchJobID).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func TestDocumentsEmbedsJobIDInDataAttribute(t *testing.T) {
	got := renderDocuments(t, nil, "job-abc-123")

	if !strings.Contains(got, `data-job-id="job-abc-123"`) {
		t.Errorf("expected data-job-id attribute with job id, got: %s", got)
	}
}

func TestDocumentsScriptHasNoUnrenderedTemplExpression(t *testing.T) {
	got := renderDocuments(t, nil, "job-1")

	// Regression: templ does NOT interpolate { ... } inside raw <script>.
	// Previous code shipped `const jobID = { templ.JSONString(watchJobID) };`
	// to the browser, causing a JS SyntaxError on the `.`.
	if strings.Contains(got, "templ.JSONString") {
		t.Errorf("script contains unrendered templ expression: %s", got)
	}
}

func TestDocumentsOmitsScriptWhenNoWatchJobID(t *testing.T) {
	got := renderDocuments(t, nil, "")

	if strings.Contains(got, "EventSource") {
		t.Errorf("expected no EventSource script when watchJobID empty, got: %s", got)
	}
}

func TestDocumentsRendersJobList(t *testing.T) {
	jobs := []db.Job{
		{ID: "j1", Filename: "alpha.pdf", Status: "PROCESSING", PagesDone: 2, PagesTotal: 5},
		{ID: "j2", Filename: "beta.pdf", Status: "DONE"},
	}
	got := renderDocuments(t, jobs, "")

	for _, want := range []string{
		`id="job-j1"`,
		"alpha.pdf",
		"2/5 pages",
		`id="job-j2"`,
		"beta.pdf",
		`href="/read/j2"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected output to contain %q, got: %s", want, got)
		}
	}
}
