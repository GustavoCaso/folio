package epubconvert

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raitucarp/epub"
)

type fakeStore struct {
	doneID, outputPath, title, author string
	cover                             []byte
	failedID, failedErr               string
}

func (f *fakeStore) MarkJobDone(ctx context.Context, id, outputPath, title, author string, cover []byte) error {
	f.doneID, f.outputPath, f.title, f.author, f.cover = id, outputPath, title, author, cover
	return nil
}
func (f *fakeStore) MarkJobFailed(ctx context.Context, id, errMsg string) error {
	f.failedID, f.failedErr = id, errMsg
	return nil
}

// buildTestEpub constructs a minimal valid epub in memory using the
// raitucarp/epub writer API: one XHTML chapter, a 1x1 PNG cover, and a
// table of contents (all required by the writer's internal guardCheck).
func buildTestEpub(t *testing.T) []byte {
	t.Helper()

	w := epub.New("urn:uuid:test-book-0001")
	w.Title("Test Book")
	w.Author("Test Author")
	w.Languages("en")

	chapterHTML := `<html><body><h1>Chapter One</h1><p>Hello world.</p></body></html>`
	w.AddContent("chapter1.xhtml", []byte(chapterHTML))

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.White)
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	if err := w.CoverPNG(img); err != nil {
		t.Fatalf("CoverPNG: %v", err)
	}

	if err := w.TableOfContents("toc", epub.TOC{
		Title: "Test Book",
		Items: []epub.TOC{
			{Title: "Chapter One", Href: "chapter1.xhtml"},
		},
	}); err != nil {
		t.Fatalf("TableOfContents: %v", err)
	}

	outPath := filepath.Join(t.TempDir(), "test.epub")
	if err := w.Write(outPath); err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return data
}

func TestRun_WritesChaptersAndMarksDone(t *testing.T) {
	epubBytes := buildTestEpub(t)
	dataDir := t.TempDir()
	store := &fakeStore{}
	r := New(store, nil, dataDir)

	r.Run(context.Background(), "job-1", epubBytes)

	if store.doneID != "job-1" {
		t.Fatalf("job not marked done: %+v", store)
	}
	entries, err := os.ReadDir(filepath.Join(dataDir, "job-1"))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var sawChapter, sawTOC bool
	for _, e := range entries {
		if e.Name() == "toc.json" {
			sawTOC = true
		}
		if filepath.Ext(e.Name()) == ".html" {
			sawChapter = true
		}
	}
	if !sawChapter || !sawTOC {
		t.Errorf("expected chapter html + toc.json, got %v", entries)
	}
}

func TestRun_TocEntryUsesRealChapterTitleNotHref(t *testing.T) {
	epubBytes := buildTestEpub(t)
	dataDir := t.TempDir()
	store := &fakeStore{}
	r := New(store, nil, dataDir)

	r.Run(context.Background(), "job-toc", epubBytes)

	if store.doneID != "job-toc" {
		t.Fatalf("job not marked done: %+v", store)
	}

	tocBytes, err := os.ReadFile(filepath.Join(dataDir, "job-toc", "toc.json"))
	if err != nil {
		t.Fatalf("ReadFile toc.json: %v", err)
	}
	var entries []tocEntry
	if err := json.Unmarshal(tocBytes, &entries); err != nil {
		t.Fatalf("Unmarshal toc.json: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 toc entry, got %d: %+v", len(entries), entries)
	}
	if entries[0].Title != "Chapter One" {
		t.Errorf("expected toc title %q (real chapter title), got %q", "Chapter One", entries[0].Title)
	}
	if entries[0].Title == "chapter1.xhtml" {
		t.Errorf("toc title fell back to raw href %q instead of using TOC title", entries[0].Title)
	}
	if entries[0].Anchor != "" {
		t.Errorf("expected empty anchor for chapter-level entry, got %q", entries[0].Anchor)
	}
	if len(entries[0].Items) != 0 {
		t.Errorf("expected no nested items, got %+v", entries[0].Items)
	}
}

// buildTestEpubNoMatchingTOC builds an epub whose TOC references a href
// that does not match any spine item, so title resolution must fall back to
// a title derived from the chapter's own content. The chapter document has
// an h1, so the fallback should use its text rather than the spine href.
func buildTestEpubNoMatchingTOC(t *testing.T) []byte {
	t.Helper()
	return buildEpubNoMatchingTOCWithChapter(t, "test-book-0002", "Test Book No Match",
		`<html><body><h1>Chapter One</h1><p>Hello world.</p></body></html>`)
}

// buildTestEpubNoMatchingTOCNoHeading is like buildTestEpubNoMatchingTOC but
// the chapter document has no heading element at all, so title resolution
// must fall further back to a generic "Chapter {i+1}" placeholder.
func buildTestEpubNoMatchingTOCNoHeading(t *testing.T) []byte {
	t.Helper()
	return buildEpubNoMatchingTOCWithChapter(t, "test-book-0003", "Test Book No Heading",
		`<html><body><p>Just a paragraph, no heading here.</p></body></html>`)
}

// buildEpubNoMatchingTOCWithChapter builds a minimal epub whose single TOC
// entry points at an href that does not correspond to any spine item, using
// the given chapter HTML as the sole spine item's content.
func buildEpubNoMatchingTOCWithChapter(t *testing.T, uuidSuffix, bookTitle, chapterHTML string) []byte {
	t.Helper()

	w := epub.New("urn:uuid:" + uuidSuffix)
	w.Title(bookTitle)
	w.Author("Test Author")
	w.Languages("en")

	w.AddContent("chapter1.xhtml", []byte(chapterHTML))

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.White)
	if err := w.CoverPNG(img); err != nil {
		t.Fatalf("CoverPNG: %v", err)
	}

	// TOC points at an href that doesn't correspond to any spine item.
	if err := w.TableOfContents("toc", epub.TOC{
		Title: bookTitle,
		Items: []epub.TOC{
			{Title: "Nonexistent Chapter", Href: "does-not-exist.xhtml"},
		},
	}); err != nil {
		t.Fatalf("TableOfContents: %v", err)
	}

	outPath := filepath.Join(t.TempDir(), uuidSuffix+".epub")
	if err := w.Write(outPath); err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return data
}

// runAndReadTOC runs the conversion and returns the parsed toc.json entries.
func runAndReadTOC(t *testing.T, epubBytes []byte, jobID string) []tocEntry {
	t.Helper()
	dataDir := t.TempDir()
	store := &fakeStore{}
	r := New(store, nil, dataDir)

	r.Run(context.Background(), jobID, epubBytes)

	if store.doneID != jobID {
		t.Fatalf("job not marked done: %+v", store)
	}

	tocBytes, err := os.ReadFile(filepath.Join(dataDir, jobID, "toc.json"))
	if err != nil {
		t.Fatalf("ReadFile toc.json: %v", err)
	}
	var entries []tocEntry
	if err := json.Unmarshal(tocBytes, &entries); err != nil {
		t.Fatalf("Unmarshal toc.json: %v", err)
	}
	return entries
}

func TestRun_TocEntryFallsBackToHeadingWhenNoTOCMatch(t *testing.T) {
	epubBytes := buildTestEpubNoMatchingTOC(t)
	entries := runAndReadTOC(t, epubBytes, "job-no-toc-match")

	if len(entries) != 1 {
		t.Fatalf("expected 1 toc entry, got %d: %+v", len(entries), entries)
	}
	if entries[0].Title != "Chapter One" {
		t.Errorf("expected fallback title to be heading text %q, got %q", "Chapter One", entries[0].Title)
	}
	if entries[0].Title == "chapter1.xhtml" {
		t.Errorf("fallback title should not be the raw spine href, got %q", entries[0].Title)
	}
}

func TestRun_TocEntryFallsBackToChapterNWhenNoTOCMatchAndNoHeading(t *testing.T) {
	epubBytes := buildTestEpubNoMatchingTOCNoHeading(t)
	entries := runAndReadTOC(t, epubBytes, "job-no-toc-no-heading")

	if len(entries) != 1 {
		t.Fatalf("expected 1 toc entry, got %d: %+v", len(entries), entries)
	}
	if entries[0].Title != "Chapter 1" {
		t.Errorf("expected fallback title %q, got %q", "Chapter 1", entries[0].Title)
	}
	if entries[0].Title == "chapter1.xhtml" {
		t.Errorf("fallback title should not be the raw spine href, got %q", entries[0].Title)
	}
}

// buildTestEpubWithSubsectionTOC builds an epub whose TOC has a top-level
// chapter entry ("chapter1.xhtml" -> "Real Chapter Title") plus a nested
// subsection entry pointing at the same file with a fragment
// ("chapter1.xhtml#section1" -> "Wrapping Up"). This reproduces the
// real-world bug where a flat href->title map lets a later-processed
// subsection entry silently overwrite the chapter-level title.
//
// The raitucarp/epub Writer.TableOfContents has a bug: its NCX-generation
// path panics (index out of range) when given TOC.Items nested two levels
// deep (see writer.go's visitTOC-based NavPoint placement, which only
// handles depth 0-1 correctly). This is a bug in the epub-writing library,
// not in code under test — the reader side (both ncx.Parse and the EPUB3
// nav.xhtml parser) fully supports nested navigation. Reader.TableOfContents
// prefers the EPUB3 nav.xhtml resource over the NCX when both exist (see
// toc.go's resourceWithNavIndex check), so to work around the writer bug
// this builds the epub with a flat depth-1 TOC via the writer, then
// rewrites the generated nav.xhtml entry inside the resulting zip directly
// with hand-authored nested markup.
func buildTestEpubWithSubsectionTOC(t *testing.T) []byte {
	t.Helper()

	w := epub.New("urn:uuid:test-book-0004")
	w.Title("Test Book")
	w.Author("Test Author")
	w.Languages("en")

	chapterHTML := `<html><body><h1>Real Chapter Title</h1><p>Intro.</p><h2 id="section1">Wrapping Up</h2><p>Recap.</p></body></html>`
	w.AddContent("chapter1.xhtml", []byte(chapterHTML))

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.White)
	if err := w.CoverPNG(img); err != nil {
		t.Fatalf("CoverPNG: %v", err)
	}

	// Flat depth-1 TOC — avoids the writer's depth-2 nesting panic. We
	// overwrite the .ncx content below with the real nested structure.
	if err := w.TableOfContents("toc", epub.TOC{
		Title: "Test Book",
		Items: []epub.TOC{
			{Title: "Real Chapter Title", Href: "chapter1.xhtml"},
		},
	}); err != nil {
		t.Fatalf("TableOfContents: %v", err)
	}

	outPath := filepath.Join(t.TempDir(), "test4.epub")
	if err := w.Write(outPath); err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	return rewriteNavXHTMLWithNestedTOC(t, data)
}

// nestedNavXHTML is a hand-authored EPUB3 nav document with a top-level
// <li><a> ("Real Chapter Title" -> chapter1.xhtml) containing a nested
// <ol><li><a> ("Wrapping Up" -> chapter1.xhtml#section1), matching the
// shape the raitucarp/epub Writer cannot itself produce for its NCX (see
// buildTestEpubWithSubsectionTOC). The reader's TOC parser (toc.go
// parseNav/parseList/parseListItem) only requires: a <nav> whose some
// attribute value is "toc", a leading element used as the title, and an
// <ol>/<ul> of <li><a href>Title</a>[<ol>...</ol>]</li>.
const nestedNavXHTML = `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
<head><title>Test Book</title></head>
<body>
<nav epub:type="toc" id="toc">
<h1>Test Book</h1>
<ol>
<li><a href="chapter1.xhtml">Real Chapter Title</a>
<ol>
<li><a href="chapter1.xhtml#section1">Wrapping Up</a></li>
</ol>
</li>
</ol>
</nav>
</body>
</html>`

// rewriteNavXHTMLWithNestedTOC re-zips epubBytes with the writer-generated
// EPUB3 nav document (the resource with manifest property "nav", located by
// filename convention "toc.xhtml" matching the name passed to
// Writer.TableOfContents in buildTestEpubWithSubsectionTOC) replaced by
// nestedNavXHTML, leaving every other entry untouched.
func rewriteNavXHTMLWithNestedTOC(t *testing.T, epubBytes []byte) []byte {
	t.Helper()

	zr, err := zip.NewReader(bytes.NewReader(epubBytes), int64(len(epubBytes)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	sawNav := false
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open zip entry %q: %v", f.Name, err)
		}
		content, err := io.ReadAll(rc)
		if closeErr := rc.Close(); closeErr != nil {
			t.Fatalf("close zip entry %q: %v", f.Name, closeErr)
		}
		if err != nil {
			t.Fatalf("read zip entry %q: %v", f.Name, err)
		}

		if strings.HasSuffix(f.Name, "toc.xhtml") {
			content = []byte(nestedNavXHTML)
			sawNav = true
		}

		// The container's first entry ("mimetype") must be stored, not
		// deflated, and uncompressed, per the EPUB OCF spec.
		var w io.Writer
		if f.Name == "mimetype" {
			hdr := &zip.FileHeader{Name: f.Name, Method: zip.Store}
			w, err = zw.CreateHeader(hdr)
		} else {
			w, err = zw.Create(f.Name)
		}
		if err != nil {
			t.Fatalf("create zip entry %q: %v", f.Name, err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatalf("write zip entry %q: %v", f.Name, err)
		}
	}
	if !sawNav {
		t.Fatalf("no nav (toc.xhtml) entry found in epub zip to rewrite")
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip.Writer.Close: %v", err)
	}

	return buf.Bytes()
}

func TestRun_TocSubsectionDoesNotOverwriteChapterTitle(t *testing.T) {
	epubBytes := buildTestEpubWithSubsectionTOC(t)
	entries := runAndReadTOC(t, epubBytes, "job-subsection-toc")

	if len(entries) != 1 {
		t.Fatalf("expected 1 top-level toc entry, got %d: %+v", len(entries), entries)
	}
	top := entries[0]
	if top.Title != "Real Chapter Title" {
		t.Errorf("expected top-level title %q, got %q (subsection collision bug)", "Real Chapter Title", top.Title)
	}
	if top.Anchor != "" {
		t.Errorf("expected empty anchor on chapter-level entry, got %q", top.Anchor)
	}
	if top.ChapterIdx != 0 {
		t.Errorf("expected chapter_idx 0, got %d", top.ChapterIdx)
	}

	if len(top.Items) != 1 {
		t.Fatalf("expected 1 nested subsection item, got %d: %+v", len(top.Items), top.Items)
	}
	sub := top.Items[0]
	if sub.Title != "Wrapping Up" {
		t.Errorf("expected nested item title %q, got %q", "Wrapping Up", sub.Title)
	}
	if sub.Anchor != "section1" {
		t.Errorf("expected nested item anchor %q, got %q", "section1", sub.Anchor)
	}
	if sub.ChapterIdx != 0 {
		t.Errorf("expected nested item chapter_idx 0, got %d", sub.ChapterIdx)
	}
}

func TestRun_MarksFailedOnInvalidEpub(t *testing.T) {
	store := &fakeStore{}
	r := New(store, nil, t.TempDir())

	r.Run(context.Background(), "job-2", []byte("not an epub"))

	if store.failedID != "job-2" || store.failedErr == "" {
		t.Errorf("expected job-2 marked failed with an error, got %+v", store)
	}
}
