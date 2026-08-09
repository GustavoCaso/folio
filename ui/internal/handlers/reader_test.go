package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/domain"
)

func TestReadDocument_NotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/read/no-such-id", nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, newTestStore(t))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Document not found") {
		t.Errorf("expected 'Document not found' in response, got:\n%s", rec.Body.String())
	}
}

func TestReadDocument_JobPending_NotReady(t *testing.T) {
	store := newTestStore(t)
	job, err := store.CreateJob(context.Background(), "pending.pdf", []byte{}, "", domain.PdfFormat)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/read/"+job.ID, nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "not ready") {
		t.Errorf("expected 'not ready' in response, got:\n%s", rec.Body.String())
	}
}

func TestReadDocument_JobFailed(t *testing.T) {
	store := newTestStore(t)
	job, err := store.CreateJob(context.Background(), "broken.pdf", []byte{}, "", domain.PdfFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkJobFailed(context.Background(), job.ID, "parser crashed"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/read/"+job.ID, nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "conversion failed") {
		t.Errorf("expected 'conversion failed' in response, got:\n%s", rec.Body.String())
	}
}

func TestReadDocument_MarkdownFileMissing(t *testing.T) {
	store := newTestStore(t)
	job, err := store.CreateJob(context.Background(), "gone.pdf", []byte{}, "", domain.PdfFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkJobDone(context.Background(), job.ID, "/nonexistent/path/gone.md", "", "", nil); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/read/"+job.ID, nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Failed to read document") {
		t.Errorf("expected 'Failed to read document' in response, got:\n%s", rec.Body.String())
	}
}

func TestReadDocument_HappyPath(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(mdPath, []byte("# Hello\n\nWorld paragraph."), 0644); err != nil {
		t.Fatal(err)
	}

	store := newTestStore(t)
	job, err := store.CreateJob(context.Background(), "doc.pdf", []byte{}, "", domain.PdfFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkJobDone(context.Background(), job.ID, mdPath, "", "", nil); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/read/"+job.ID, nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	got := rec.Body.String()
	if !strings.Contains(got, "Hello") {
		t.Errorf("expected rendered heading in response, got:\n%s", got)
	}
	if !strings.Contains(got, "World paragraph") {
		t.Errorf("expected rendered paragraph in response, got:\n%s", got)
	}
	if strings.Contains(got, `class="error-banner"`) {
		t.Errorf("unexpected error banner in successful reader response")
	}
}

func TestReadDocument_EPUB_RendersRequestedChapter(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "chapter-0.html"), []byte(`<p data-block-id="ch0-p-1">Chapter zero content</p>`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "chapter-1.html"), []byte(`<p data-block-id="ch1-p-1">Chapter one content marker XYZ</p>`), 0644); err != nil {
		t.Fatal(err)
	}
	tocJSON := `[{"title":"Chapter Zero","chapter_idx":0,"anchor":"","items":[]},{"title":"Chapter One","chapter_idx":1,"anchor":"","items":[]}]`
	if err := os.WriteFile(filepath.Join(dir, "toc.json"), []byte(tocJSON), 0644); err != nil {
		t.Fatal(err)
	}

	store := newTestStore(t)
	job, err := store.CreateJob(context.Background(), "book.epub", []byte{}, "", "epub")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkJobDone(context.Background(), job.ID, dir, "Test Book", "Test Author", nil); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/read/"+job.ID+"?chapter=1", nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Chapter one content marker XYZ") {
		t.Errorf("expected chapter 1 content in response, got:\n%s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "Chapter zero content") {
		t.Errorf("chapter 0 content should NOT appear when chapter=1 requested, got:\n%s", rec.Body.String())
	}
}

func TestReadDocument_EPUB_FullConcatenatesAllChapters(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "chapter-0.html"), []byte(`<p data-block-id="ch0-p-1">Chapter zero content</p>`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "chapter-1.html"), []byte(`<p data-block-id="ch1-p-1">Chapter one content marker XYZ</p>`), 0644); err != nil {
		t.Fatal(err)
	}
	tocJSON := `[{"title":"Chapter Zero","chapter_idx":0,"anchor":"","items":[]},{"title":"Chapter One","chapter_idx":1,"anchor":"","items":[]}]`
	if err := os.WriteFile(filepath.Join(dir, "toc.json"), []byte(tocJSON), 0644); err != nil {
		t.Fatal(err)
	}

	store := newTestStore(t)
	job, err := store.CreateJob(context.Background(), "book.epub", []byte{}, "", "epub")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkJobDone(context.Background(), job.ID, dir, "Test Book", "Test Author", nil); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/read/"+job.ID+"?full=1", nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Chapter zero content") {
		t.Errorf("expected chapter 0 content in response, got:\n%s", body)
	}
	if !strings.Contains(body, "Chapter one content marker XYZ") {
		t.Errorf("expected chapter 1 content in response, got:\n%s", body)
	}

	// Full view: there is no single "current chapter", so no TOC menu item is
	// marked active, and prev/next links are replaced by a static indicator.
	if !strings.Contains(body, `<span>Full view</span>`) {
		t.Errorf("expected static 'Full view' indicator span, got:\n%s", body)
	}
	if strings.Contains(body, `rel="prev"`) || strings.Contains(body, `rel="next"`) {
		t.Errorf("did not expect prev/next chapter links in full view, got:\n%s", body)
	}
	if strings.Contains(body, `data-tui-sidebar-active="true"`) {
		t.Errorf("did not expect any active sidebar menu item in full view, got:\n%s", body)
	}
}

func TestReadDocument_EPUB_ChapterNav(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "chapter-0.html"), []byte(`<p data-block-id="ch0-p-1">Chapter zero content</p>`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "chapter-1.html"), []byte(`<p data-block-id="ch1-p-1">Chapter one content</p>`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "chapter-2.html"), []byte(`<p data-block-id="ch2-p-1">Chapter two content</p>`), 0644); err != nil {
		t.Fatal(err)
	}
	// Chapter 1's title contains HTML-special characters to verify escaping.
	tocJSON := `[{"title":"Chapter Zero","chapter_idx":0,"anchor":"","items":[]},{"title":"<script>alert(1)</script> & Chapter One","chapter_idx":1,"anchor":"","items":[]},{"title":"Chapter Two","chapter_idx":2,"anchor":"","items":[]}]`
	if err := os.WriteFile(filepath.Join(dir, "toc.json"), []byte(tocJSON), 0644); err != nil {
		t.Fatal(err)
	}

	store := newTestStore(t)
	job, err := store.CreateJob(context.Background(), "book.epub", []byte{}, "", "epub")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkJobDone(context.Background(), job.ID, dir, "Test Book", "Test Author", nil); err != nil {
		t.Fatal(err)
	}

	mux, _ := newTestMux(t, store)

	// First chapter: no "previous" link, but a "next" link to chapter 1.
	req := httptest.NewRequest(http.MethodGet, "/read/"+job.ID+"?chapter=0", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `href="?chapter=1"`) {
		t.Errorf("expected next link to chapter 1, got:\n%s", body)
	}
	if strings.Contains(body, `href="?chapter=-1"`) {
		t.Errorf("did not expect a previous link at chapter 0, got:\n%s", body)
	}
	if !strings.Contains(body, `rel="next"`) {
		t.Errorf("expected rel=\"next\" link at chapter 0, got:\n%s", body)
	}
	if strings.Contains(body, `rel="prev"`) {
		t.Errorf("did not expect a rel=\"prev\" link at chapter 0, got:\n%s", body)
	}

	// Last chapter: "previous" link to chapter 1, but no "next" link.
	req = httptest.NewRequest(http.MethodGet, "/read/"+job.ID+"?chapter=2", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", rec.Code, rec.Body.String())
	}
	body = rec.Body.String()
	if !strings.Contains(body, `href="?chapter=1"`) {
		t.Errorf("expected previous link to chapter 1, got:\n%s", body)
	}
	if strings.Contains(body, `href="?chapter=3"`) {
		t.Errorf("did not expect a next link at last chapter, got:\n%s", body)
	}
	if !strings.Contains(body, `rel="prev"`) {
		t.Errorf("expected rel=\"prev\" link at last chapter, got:\n%s", body)
	}
	if strings.Contains(body, `rel="next"`) {
		t.Errorf("did not expect a rel=\"next\" link at last chapter, got:\n%s", body)
	}

	// TOC chapter list renders titles, and HTML-special characters are escaped.
	if !strings.Contains(body, "Chapter Zero") {
		t.Errorf("expected TOC title 'Chapter Zero' in response, got:\n%s", body)
	}
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Errorf("chapter title should be HTML-escaped, not injected as raw script, got:\n%s", body)
	}
	if !strings.Contains(body, "&lt;script&gt;") || !strings.Contains(body, "&amp;") {
		t.Errorf("expected escaped chapter title markup, got:\n%s", body)
	}
}

func TestReadDocument_EPUB_ChapterNav_NestedSubsectionLinks(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "chapter-0.html"), []byte(`<h2 id="h1-sect">Wrapping Up</h2><p data-block-id="ch0-p-1">Chapter zero content</p>`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "chapter-1.html"), []byte(`<p data-block-id="ch1-p-1">Chapter one content</p>`), 0644); err != nil {
		t.Fatal(err)
	}
	// Chapter 0 has a nested subsection ("Wrapping Up", anchor "h1-sect").
	tocJSON := `[
		{"title":"Chapter Zero","chapter_idx":0,"anchor":"","items":[
			{"title":"Wrapping Up","chapter_idx":0,"anchor":"h1-sect","items":[]}
		]},
		{"title":"Chapter One","chapter_idx":1,"anchor":"","items":[]}
	]`
	if err := os.WriteFile(filepath.Join(dir, "toc.json"), []byte(tocJSON), 0644); err != nil {
		t.Fatal(err)
	}

	store := newTestStore(t)
	job, err := store.CreateJob(context.Background(), "book.epub", []byte{}, "", "epub")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkJobDone(context.Background(), job.ID, dir, "Test Book", "Test Author", nil); err != nil {
		t.Fatal(err)
	}

	mux, _ := newTestMux(t, store)

	// Viewing chapter 0: the subsection belongs to the current chapter, so
	// its link should be a same-page "#anchor" jump, not a "?chapter=" nav.
	req := httptest.NewRequest(http.MethodGet, "/read/"+job.ID+"?chapter=0", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `href="#h1-sect"`) {
		t.Errorf("expected same-page anchor link '#h1-sect' for subsection in current chapter, got:\n%s", body)
	}
	if strings.Contains(body, `href="?chapter=0#h1-sect"`) {
		t.Errorf("did not expect cross-chapter style link for subsection in current chapter, got:\n%s", body)
	}

	// Viewing chapter 1: the subsection belongs to a different chapter, so
	// its link should be "?chapter=0#h1-sect" (load chapter then jump).
	req = httptest.NewRequest(http.MethodGet, "/read/"+job.ID+"?chapter=1", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", rec.Code, rec.Body.String())
	}
	body = rec.Body.String()
	if !strings.Contains(body, `href="?chapter=0#h1-sect"`) {
		t.Errorf("expected cross-chapter anchor link '?chapter=0#h1-sect' for subsection in a different chapter, got:\n%s", body)
	}
}

func TestReadDocument_EPUB_MissingChapterFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "chapter-0.html"), []byte(`<p data-block-id="ch0-p-1">Chapter zero content</p>`), 0644); err != nil {
		t.Fatal(err)
	}
	tocJSON := `[{"title":"Chapter Zero","chapter_idx":0,"anchor":"","items":[]}]`
	if err := os.WriteFile(filepath.Join(dir, "toc.json"), []byte(tocJSON), 0644); err != nil {
		t.Fatal(err)
	}

	store := newTestStore(t)
	job, err := store.CreateJob(context.Background(), "book.epub", []byte{}, "", "epub")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkJobDone(context.Background(), job.ID, dir, "Test Book", "Test Author", nil); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/read/"+job.ID+"?chapter=5", nil)
	rec := httptest.NewRecorder()
	mux, _ := newTestMux(t, store)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d, body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Failed to read document") {
		t.Errorf("expected 'Failed to read document' in response, got:\n%s", rec.Body.String())
	}
}
