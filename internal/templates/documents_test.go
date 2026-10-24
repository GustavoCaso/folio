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

func renderDocuments(t *testing.T, documents []domain.Document, watchDocumentIDs ...string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := templates.Documents(views.LibraryPage{Documents: documents, WatchDocumentIDs: watchDocumentIDs}).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func renderDocumentList(t *testing.T, documents []domain.Document) string {
	t.Helper()
	var buf bytes.Buffer
	if err := templates.DocumentList(documents, domain.DocumentFilter{}).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func TestDocumentList_RendersIDAndFilename(t *testing.T) {
	documents := []domain.Document{
		{ID: "j1", Filename: "alpha.pdf", Status: "PENDING"},
		{ID: "j2", Filename: "beta.pdf", Status: "PENDING"},
	}
	got := renderDocumentList(t, documents)

	for _, want := range []string{`id="document-j1"`, "alpha.pdf", `id="document-j2"`, "beta.pdf"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got: %s", want, got)
		}
	}
}

func TestDocumentList_ProcessingOmitsPageCountWhenZero(t *testing.T) {
	got := renderDocumentList(t, []domain.Document{
		{ID: "j1", Filename: "doc.pdf", Status: "PROCESSING"},
	})

	if strings.Contains(got, "pages") {
		t.Errorf("expected no page count when PagesTotal is 0, got: %s", got)
	}
}

func TestDocumentList_DoneShowsReadLink(t *testing.T) {
	got := renderDocumentList(t, []domain.Document{
		{ID: "j2", Filename: "beta.pdf", Status: "DONE"},
	})

	if !strings.Contains(got, `href="/read/j2"`) {
		t.Errorf("expected read link in output, got: %s", got)
	}
}

func TestDocumentList_FailedShowsErrorMessage(t *testing.T) {
	got := renderDocumentList(t, []domain.Document{
		{ID: "j3", Filename: "bad.pdf", Status: "FAILED", Error: "parser timeout"},
	})

	if !strings.Contains(got, "parser timeout") {
		t.Errorf("expected error message in output, got: %s", got)
	}
}

func TestDocumentList_EmptyRendersNothing(t *testing.T) {
	got := renderDocumentList(t, nil)

	if strings.Contains(got, "<ul>") {
		t.Errorf("expected no <ul> for empty document list, got: %s", got)
	}
}

func TestDocumentsEmbedsDocumentIDInDataAttribute(t *testing.T) {
	got := renderDocuments(t, nil, "document-abc-123")

	if !strings.Contains(got, `data-document-ids="document-abc-123"`) {
		t.Errorf("expected data-document-ids attribute with document id, got: %s", got)
	}
}

func TestDocumentsScriptHasNoUnrenderedTemplExpression(t *testing.T) {
	got := renderDocuments(t, nil, "document-1")

	// Regression: templ does NOT interpolate { ... } inside raw <script>.
	// Previous code shipped `const documentID = { templ.JSONString(watchDocumentID) };`
	// to the browser, causing a JS SyntaxError on the `.`.
	if strings.Contains(got, "templ.JSONString") {
		t.Errorf("script contains unrendered templ expression: %s", got)
	}
}

func TestDocumentsOmitsScriptWhenNoWatchDocumentID(t *testing.T) {
	got := renderDocuments(t, nil)

	if strings.Contains(got, "EventSource") {
		t.Errorf("expected no EventSource script when watchDocumentIDs empty, got: %s", got)
	}
}

func TestDocumentsIncludesDocumentList(t *testing.T) {
	documents := []domain.Document{
		{ID: "j1", Filename: "alpha.pdf", Status: "DONE"},
	}
	got := renderDocuments(t, documents)

	if !strings.Contains(got, `id="document-j1"`) {
		t.Errorf("expected DocumentList output inside Documents, got: %s", got)
	}
}

func TestDocumentsShowsErrorBanner(t *testing.T) {
	var buf bytes.Buffer
	if err := templates.Documents(views.LibraryPage{ErrMsg: "Something went wrong"}).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	got := buf.String()

	if !strings.Contains(got, `role="alert"`) {
		t.Errorf("expected alert element in output, got: %s", got)
	}
	if !strings.Contains(got, "Something went wrong") {
		t.Errorf("expected error message in output, got: %s", got)
	}
}

func TestDocumentsOmitsErrorBannerWhenNoError(t *testing.T) {
	got := renderDocuments(t, nil)

	if strings.Contains(got, `role="alert"`) {
		t.Errorf("expected no alert element when errMsg empty, got: %s", got)
	}
}

func TestDocumentsLoadsScript(t *testing.T) {
	got := renderDocuments(t, nil, "document-1")
	if !strings.Contains(got, `<script type="module" src="/static/js/documents.js">`) {
		t.Errorf("expected documents.js script tag, got:\n%s", got)
	}
}

func TestDocumentList_ShowsDeleteForDone(t *testing.T) {
	body := renderDocumentList(t, []domain.Document{
		{ID: "abc123", Filename: "done.pdf", Status: "DONE"},
	})
	if !strings.Contains(body, `data-delete-document="abc123"`) {
		t.Errorf("expected delete button for DONE document, got:\n%s", body)
	}
}

func TestDocumentList_ShowsDeleteForFailed(t *testing.T) {
	body := renderDocumentList(t, []domain.Document{
		{ID: "def456", Filename: "failed.pdf", Status: "FAILED", Error: "parse error"},
	})
	if !strings.Contains(body, `data-delete-document="def456"`) {
		t.Errorf("expected delete button for FAILED document, got:\n%s", body)
	}
}

func TestDocumentList_ShowsRetryForFailed(t *testing.T) {
	body := renderDocumentList(t, []domain.Document{
		{ID: "def456", Filename: "failed.pdf", Status: "FAILED", Error: "parse error"},
	})
	if !strings.Contains(body, `/documents/def456/retry`) {
		t.Errorf("expected retry link for FAILED document, got:\n%s", body)
	}
}

func TestDocumentList_ShowsCancelForPending(t *testing.T) {
	body := renderDocumentList(t, []domain.Document{
		{ID: "ghi789", Filename: "pending.pdf", Status: "PENDING"},
	})
	if !strings.Contains(body, `data-cancel-document="ghi789"`) {
		t.Errorf("expected cancel button for PENDING document, got:\n%s", body)
	}
}

func TestDocumentList_NoCancelForDone(t *testing.T) {
	body := renderDocumentList(t, []domain.Document{
		{ID: "abc123", Filename: "done.pdf", Status: "DONE"},
	})
	if strings.Contains(body, `data-cancel-document=`) {
		t.Errorf("expected no cancel button for DONE document, got:\n%s", body)
	}
}

func TestDocumentList_NoCancelForFailed(t *testing.T) {
	body := renderDocumentList(t, []domain.Document{
		{ID: "def456", Filename: "failed.pdf", Status: "FAILED", Error: "parse error"},
	})
	if strings.Contains(body, `data-cancel-document=`) {
		t.Errorf("expected no cancel button for FAILED document, got:\n%s", body)
	}
}

func TestDocumentList_NoDeleteForPending(t *testing.T) {
	body := renderDocumentList(t, []domain.Document{
		{ID: "ghi789", Filename: "pending.pdf", Status: "PENDING"},
	})
	if strings.Contains(body, `/documents/ghi789/delete`) {
		t.Errorf("expected no delete link for PENDING document, got:\n%s", body)
	}
}

func TestDocumentList_NoDeleteForProcessing(t *testing.T) {
	body := renderDocumentList(t, []domain.Document{
		{ID: "jkl012", Filename: "processing.pdf", Status: "PROCESSING"},
	})
	if strings.Contains(body, `/documents/jkl012/delete`) {
		t.Errorf("expected no delete link for PROCESSING document, got:\n%s", body)
	}
}

func TestDocumentsEmbedsMultipleDocumentIDs(t *testing.T) {
	got := renderDocuments(t, nil, "document-1", "document-2")

	if !strings.Contains(got, `data-document-ids="document-1,document-2"`) {
		t.Errorf("expected joined data-document-ids attribute, got: %s", got)
	}
}

func TestDocuments_UploadButtonStartsDisabled(t *testing.T) {
	got := renderDocuments(t, nil)
	if !strings.Contains(got, `id="upload-btn"`) {
		t.Errorf("expected upload-btn id on button, got:\n%s", got)
	}
	if !strings.Contains(got, `disabled`) {
		t.Errorf("expected upload button to be disabled initially, got:\n%s", got)
	}
}

func renderLibrary(t *testing.T, page views.LibraryPage) string {
	t.Helper()
	var buf bytes.Buffer
	if err := templates.Documents(page).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func TestDocuments_ActiveFacetLinkRemovesItself(t *testing.T) {
	got := renderLibrary(t, views.LibraryPage{
		Filter: domain.DocumentFilter{Query: "dune", Author: "Frank Herbert"},
		Facets: domain.Facets{Authors: []domain.FacetCount{{Value: "Frank Herbert", Count: 2}}},
	})

	// Rendered twice: desktop sidebar and mobile sheet.
	if n := strings.Count(got, `href="/?q=dune" class="facet" aria-current="true"`); n != 2 {
		t.Errorf("expected active author facet linking back to the search only (x2), found %d in:\n%s", n, got)
	}
}

func TestDocuments_FilterChipsAndClearAll(t *testing.T) {
	got := renderLibrary(t, views.LibraryPage{
		Filter: domain.DocumentFilter{Query: "dune", Tag: "sci-fi"},
	})

	for _, want := range []string{
		`href="/?tag=sci-fi" class="filter-chip"`, // removing the query keeps the tag
		`href="/?q=dune" class="filter-chip"`,     // removing the tag keeps the query
		`<input type="hidden" name="tag" value="sci-fi">`,
		"Clear all",
		"No books match these filters.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output", want)
		}
	}
}

func TestDocuments_NoFilterHidesClearAll(t *testing.T) {
	got := renderLibrary(t, views.LibraryPage{})
	if strings.Contains(got, "Clear all") {
		t.Error("did not expect Clear all without filters")
	}
	if !strings.Contains(got, "Your library is empty.") {
		t.Error("expected empty library state")
	}
}

func TestDocuments_ViewTogglePressedState(t *testing.T) {
	got := renderLibrary(t, views.LibraryPage{View: views.ViewList})
	if !strings.Contains(got, `data-view-toggle="list" aria-pressed="true"`) {
		t.Errorf("expected list toggle pressed, got:\n%s", got)
	}
	if !strings.Contains(got, `data-view-toggle="grid" aria-pressed="false"`) {
		t.Errorf("expected grid toggle not pressed")
	}
}

func TestDocumentList_AuthorAndTagLinksKeepFilter(t *testing.T) {
	var buf bytes.Buffer
	err := templates.DocumentList(
		[]domain.Document{{ID: "j1", Filename: "dune.epub", Status: "DONE", Author: "Frank Herbert", Tags: []string{"sci-fi"}}},
		domain.DocumentFilter{Query: "dune"},
	).Render(context.Background(), &buf)
	if err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	for _, want := range []string{
		`href="/?author=Frank+Herbert&amp;q=dune"`,
		`href="/?q=dune&amp;tag=sci-fi"`,
		`data-cover-placeholder`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output:\n%s", want, got)
		}
	}
}
