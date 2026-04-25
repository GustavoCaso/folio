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
	if err := templates.Documents(jobs, watchJobID, "").Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func renderDocumentList(t *testing.T, jobs []db.Job) string {
	t.Helper()
	var buf bytes.Buffer
	if err := templates.DocumentList(jobs).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func TestDocumentList_RendersIDAndFilename(t *testing.T) {
	jobs := []db.Job{
		{ID: "j1", Filename: "alpha.pdf", Status: "PENDING"},
		{ID: "j2", Filename: "beta.pdf", Status: "PENDING"},
	}
	got := renderDocumentList(t, jobs)

	for _, want := range []string{`id="job-j1"`, "alpha.pdf", `id="job-j2"`, "beta.pdf"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got: %s", want, got)
		}
	}
}

func TestDocumentList_ProcessingShowsPageCount(t *testing.T) {
	got := renderDocumentList(t, []db.Job{
		{ID: "j1", Filename: "doc.pdf", Status: "PROCESSING", PagesDone: 2, PagesTotal: 5},
	})

	if !strings.Contains(got, "2/5 pages") {
		t.Errorf("expected '2/5 pages' in output, got: %s", got)
	}
}

func TestDocumentList_ProcessingOmitsPageCountWhenZero(t *testing.T) {
	got := renderDocumentList(t, []db.Job{
		{ID: "j1", Filename: "doc.pdf", Status: "PROCESSING", PagesDone: 0, PagesTotal: 0},
	})

	if strings.Contains(got, "pages") {
		t.Errorf("expected no page count when PagesTotal is 0, got: %s", got)
	}
}

func TestDocumentList_DoneShowsReadLink(t *testing.T) {
	got := renderDocumentList(t, []db.Job{
		{ID: "j2", Filename: "beta.pdf", Status: "DONE"},
	})

	if !strings.Contains(got, `href="/read/j2"`) {
		t.Errorf("expected read link in output, got: %s", got)
	}
}

func TestDocumentList_FailedShowsErrorMessage(t *testing.T) {
	got := renderDocumentList(t, []db.Job{
		{ID: "j3", Filename: "bad.pdf", Status: "FAILED", Error: "parser timeout"},
	})

	if !strings.Contains(got, "parser timeout") {
		t.Errorf("expected error message in output, got: %s", got)
	}
}

func TestDocumentList_EmptyRendersNothing(t *testing.T) {
	got := renderDocumentList(t, nil)

	if strings.Contains(got, "<ul>") {
		t.Errorf("expected no <ul> for empty job list, got: %s", got)
	}
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

func TestDocumentsIncludesDocumentList(t *testing.T) {
	jobs := []db.Job{
		{ID: "j1", Filename: "alpha.pdf", Status: "DONE"},
	}
	got := renderDocuments(t, jobs, "")

	if !strings.Contains(got, `id="job-j1"`) {
		t.Errorf("expected DocumentList output inside Documents, got: %s", got)
	}
}

func TestDocumentsShowsErrorBanner(t *testing.T) {
	var buf bytes.Buffer
	if err := templates.Documents(nil, "", "Something went wrong").Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	got := buf.String()

	if !strings.Contains(got, `class="error-banner"`) {
		t.Errorf("expected error-banner element in output, got: %s", got)
	}
	if !strings.Contains(got, "Something went wrong") {
		t.Errorf("expected error message in output, got: %s", got)
	}
}

func TestDocumentsOmitsErrorBannerWhenNoError(t *testing.T) {
	got := renderDocuments(t, nil, "")

	if strings.Contains(got, `class="error-banner"`) {
		t.Errorf("expected no error-banner element when errMsg empty, got: %s", got)
	}
}
