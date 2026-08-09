package templates_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/domain"
	"github.com/GustavoCaso/folio/ui/internal/templates"
)

func renderReader(t *testing.T, job domain.Job, html string, hls []domain.Highlight) string {
	t.Helper()
	var buf bytes.Buffer
	if err := templates.Reader(job, html, hls, nil, 0, 0, false).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func renderReaderEpub(t *testing.T, job domain.Job, html string, hls []domain.Highlight, toc []templates.EpubTOCEntry, currentChapterIdx, lastChapterIdx int, full bool) string {
	t.Helper()
	var buf bytes.Buffer
	if err := templates.Reader(job, html, hls, toc, currentChapterIdx, lastChapterIdx, full).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func TestReaderEmbedsHighlightsJSON(t *testing.T) {
	hls := []domain.Highlight{{ID: "h1", StartBlockID: "paragraph-1", EndBlockID: "paragraph-1"}}
	got := renderReader(t, domain.Job{ID: "j1", Filename: "doc.pdf"}, "<p>x</p>", hls)

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
	got := renderReader(t, domain.Job{ID: "j1"}, "", nil)

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
	got := renderReader(t, domain.Job{ID: "abc", Filename: "doc.pdf"}, "<h1>Title</h1>", nil)

	if !strings.Contains(got, `data-job-id="abc"`) {
		t.Errorf("expected job ID data attr, got: %s", got)
	}
	if !strings.Contains(got, "<h1>Title</h1>") {
		t.Errorf("expected raw rendered HTML, got: %s", got)
	}
}

func TestReaderRendersPopover(t *testing.T) {
	got := renderReader(t, domain.Job{ID: "j1"}, "", nil)

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
	got := renderReader(t, domain.Job{ID: "j1"}, "", nil)

	if !strings.Contains(got, `id="hl-error" class="hidden`) {
		t.Errorf("expected error element id, got: %s", got)
	}
}
func TestReaderRendersTooltip(t *testing.T) {
	got := renderReader(t, domain.Job{ID: "j1"}, "", nil)

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
	got := renderReader(t, domain.Job{ID: "j1"}, "", nil)

	if !strings.Contains(got, `id="highlights-panel"`) {
		t.Errorf("expected highlights panel, got: %s", got)
	}
	if !strings.Contains(got, "No highlights yet") {
		t.Errorf("expected empty-state message, got: %s", got)
	}
}

func TestReaderEpubSidebarRendersTOC(t *testing.T) {
	toc := []templates.EpubTOCEntry{
		{Title: "Chapter One", ChapterIdx: 0, Items: []templates.EpubTOCEntry{
			{Title: "Subsection", ChapterIdx: 0, Anchor: "sub1"},
		}},
		{Title: "Chapter Two", ChapterIdx: 1},
	}
	got := renderReaderEpub(t, domain.Job{ID: "j1", Filename: "book.epub"}, "<p>chapter content</p>", nil, toc, 0, 1, false)

	if !strings.Contains(got, "data-tui-sidebar-layout") {
		t.Errorf("expected sidebar layout wrapper for epub job, got: %s", got)
	}
	if !strings.Contains(got, "Chapter One") || !strings.Contains(got, "Chapter Two") {
		t.Errorf("expected chapter titles in sidebar, got: %s", got)
	}
	if !strings.Contains(got, "Subsection") {
		t.Errorf("expected subsection title in sidebar, got: %s", got)
	}
	if !strings.Contains(got, `href="#sub1"`) {
		t.Errorf("expected same-chapter subsection anchor link, got: %s", got)
	}
}

// TestReaderEpubSidebarTriggerTargetsSidebarID is a regression test for a bug
// where the sidebar toggle button did nothing: headerActions (which renders
// sidebar.Trigger()) is evaluated and passed into Layout(...) as an argument
// before Layout's body block runs, so it lives in a separate templ
// component-tree branch from the sidebar.Layout()/sidebar.Sidebar() call
// inside Reader's Layout body. sidebar.Sidebar() sets its ID via context
// inside that branch, which never reaches sidebar.Trigger() in the sibling
// branch — so without an explicit shared ID, the trigger rendered with an
// empty data-tui-sidebar-target and clicking it matched no sidebar element.
// This test extracts both rendered attribute values and asserts they match.
func TestReaderEpubSidebarTriggerTargetsSidebarID(t *testing.T) {
	toc := []templates.EpubTOCEntry{{Title: "Chapter One", ChapterIdx: 0}}
	got := renderReaderEpub(t, domain.Job{ID: "j1", Filename: "book.epub"}, "<p>chapter content</p>", nil, toc, 0, 0, false)

	target := extractAttr(t, got, "data-tui-sidebar-target")
	sidebarID := extractAttr(t, got, "data-tui-sidebar-id")

	if target == "" {
		t.Fatalf("expected non-empty data-tui-sidebar-target, got: %s", got)
	}
	if target != sidebarID {
		t.Errorf("trigger target %q does not match sidebar id %q; toggle button would silently do nothing", target, sidebarID)
	}
}

// extractAttr returns the value of the first occurrence of attr="..." in html.
func extractAttr(t *testing.T, html, attr string) string {
	t.Helper()
	needle := attr + `="`
	idx := strings.Index(html, needle)
	if idx == -1 {
		t.Fatalf("attribute %s not found in: %s", attr, html)
	}
	start := idx + len(needle)
	end := strings.Index(html[start:], `"`)
	if end == -1 {
		t.Fatalf("unterminated attribute %s in: %s", attr, html)
	}
	return html[start : start+end]
}

func TestReaderNonEpubHasNoSidebar(t *testing.T) {
	got := renderReader(t, domain.Job{ID: "j1", Filename: "doc.pdf"}, "<p>x</p>", nil)

	if strings.Contains(got, "data-tui-sidebar-layout") {
		t.Errorf("did not expect sidebar layout wrapper for non-epub job, got: %s", got)
	}
	// Regression: sidebar.Script() must not be loaded in <head> for pages
	// that render no sidebar DOM at all (e.g. PDF/markdown reader pages).
	if strings.Contains(got, "/templui/js/sidebar") {
		t.Errorf("did not expect sidebar.Script() tag for non-epub job, got: %s", got)
	}
}

func TestReaderEpubHasSidebarScript(t *testing.T) {
	toc := []templates.EpubTOCEntry{{Title: "Chapter One", ChapterIdx: 0}}
	got := renderReaderEpub(t, domain.Job{ID: "j1", Filename: "book.epub"}, "<p>chapter content</p>", nil, toc, 0, 0, false)

	if !strings.Contains(got, "/templui/js/sidebar") {
		t.Errorf("expected sidebar.Script() tag for epub job, got: %s", got)
	}
}

func TestReaderHighlightsPanelWithHighlights(t *testing.T) {
	hls := []domain.Highlight{
		{ID: "h1", Text: "selected text", Tag: "important", Note: "a note"},
		{ID: "h2", Text: "another selection", Tag: "", Note: ""},
	}
	got := renderReader(t, domain.Job{ID: "j1"}, "", hls)

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
