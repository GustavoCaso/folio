package views

import (
	"net/url"
	"slices"
	"strconv"

	"github.com/GustavoCaso/folio/ui/internal/domain"
)

// ChapterHighlights is one chapter's section on the highlights page.
type ChapterHighlights struct {
	Chapter    int
	Title      string
	Highlights []domain.Highlight
}

// GroupHighlightsByChapter buckets highlights by chapter in reading order.
// Within a chapter the input order (created_at from the store) is kept.
func GroupHighlightsByChapter(highlights []domain.Highlight, toc []EpubTOCEntry) []ChapterHighlights {
	sorted := slices.Clone(highlights)
	slices.SortStableFunc(sorted, func(a, b domain.Highlight) int { return a.Chapter - b.Chapter })

	var groups []ChapterHighlights
	for _, h := range sorted {
		if len(groups) == 0 || groups[len(groups)-1].Chapter != h.Chapter {
			groups = append(groups, ChapterHighlights{Chapter: h.Chapter, Title: ChapterTitle(toc, h.Chapter)})
		}
		last := &groups[len(groups)-1]
		last.Highlights = append(last.Highlights, h)
	}
	return groups
}

// ChapterTitle returns the TOC title for a chapter index, preferring a
// top-level entry over a nested one. Spine items without a TOC entry fall
// back to "Chapter N" (1-based).
func ChapterTitle(toc []EpubTOCEntry, chapter int) string {
	if title, ok := findChapterTitle(toc, chapter); ok {
		return title
	}
	return "Chapter " + strconv.Itoa(chapter+1)
}

func findChapterTitle(entries []EpubTOCEntry, chapter int) (string, bool) {
	for _, e := range entries {
		if e.ChapterIdx == chapter && e.Title != "" {
			return e.Title, true
		}
	}
	for _, e := range entries {
		if title, ok := findChapterTitle(e.Items, chapter); ok {
			return title, true
		}
	}
	return "", false
}

// HighlightHref links into the reader at a highlight's chapter. The server
// only reads chapter; reader.js reads highlight and scrolls to its mark.
func HighlightHref(documentID string, chapter int, highlightID string) string {
	q := url.Values{}
	q.Set("chapter", strconv.Itoa(chapter))
	q.Set("highlight", highlightID)
	return "/read/" + url.PathEscape(documentID) + "?" + q.Encode()
}

func HighlightsHref(documentID string) string {
	return "/documents/" + url.PathEscape(documentID) + "/highlights"
}

// DocumentExports is one book's section on the exports page.
type DocumentExports struct {
	DocumentID string
	Title      string
	Records    []domain.ExportRecord
}

// GroupExportsByDocument splits export records into one group per document,
// starting a new group wherever DocumentID changes. It relies on the store
// returning each document's rows next to each other.
func GroupExportsByDocument(records []domain.ExportRecord) []DocumentExports {
	var groups []DocumentExports
	for _, rec := range records {
		if len(groups) == 0 || groups[len(groups)-1].DocumentID != rec.DocumentID {
			title := rec.DocumentTitle
			if title == "" {
				title = rec.DocumentFilename
			}
			groups = append(groups, DocumentExports{DocumentID: rec.DocumentID, Title: title})
		}
		last := &groups[len(groups)-1]
		last.Records = append(last.Records, rec)
	}
	return groups
}

func HighlightCountLabel(n int) string {
	if n == 1 {
		return "1 highlight"
	}
	return strconv.Itoa(n) + " highlights"
}

// ChapterSectionID is the id of a chapter heading on the highlights page.
func ChapterSectionID(chapter int) string {
	return "hl-ch-" + strconv.Itoa(chapter)
}
