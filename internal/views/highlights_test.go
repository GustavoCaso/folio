package views_test

import (
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/domain"
	"github.com/GustavoCaso/folio/ui/internal/views"
)

func TestChapterTitle(t *testing.T) {
	toc := []views.EpubTOCEntry{
		{Title: "Part One", ChapterIdx: 0, Items: []views.EpubTOCEntry{
			{Title: "Nested", ChapterIdx: 1},
		}},
		{Title: "Top level", ChapterIdx: 1},
	}
	cases := []struct {
		name    string
		chapter int
		want    string
	}{
		{"top-level entry", 0, "Part One"},
		{"top-level wins over nested", 1, "Top level"},
		{"missing falls back to 1-based number", 4, "Chapter 5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := views.ChapterTitle(toc, tc.chapter); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestChapterTitle_NestedOnly(t *testing.T) {
	toc := []views.EpubTOCEntry{
		{Title: "Part One", ChapterIdx: 0, Items: []views.EpubTOCEntry{{Title: "Nested", ChapterIdx: 3}}},
	}
	if got := views.ChapterTitle(toc, 3); got != "Nested" {
		t.Errorf("got %q, want Nested", got)
	}
}

func TestGroupHighlightsByChapter(t *testing.T) {
	toc := []views.EpubTOCEntry{{Title: "Intro", ChapterIdx: 0}, {Title: "Middle", ChapterIdx: 2}}
	groups := views.GroupHighlightsByChapter([]domain.Highlight{
		{ID: "a", Chapter: 2},
		{ID: "b", Chapter: 0},
		{ID: "c", Chapter: 2},
	}, toc)

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[0].Title != "Intro" || groups[1].Title != "Middle" {
		t.Errorf("expected chapters in reading order, got %q, %q", groups[0].Title, groups[1].Title)
	}
	if got := groups[1].Highlights; len(got) != 2 || got[0].ID != "a" || got[1].ID != "c" {
		t.Errorf("expected input order kept within chapter, got %+v", got)
	}
}

func TestHighlightHref(t *testing.T) {
	got := views.HighlightHref("doc-1", 3, "hl 1")
	if want := "/read/doc-1?chapter=3&highlight=hl+1"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGroupExportsByDocument(t *testing.T) {
	groups := views.GroupExportsByDocument([]domain.ExportRecord{
		{ID: "1", DocumentID: "a", DocumentTitle: "Book A"},
		{ID: "2", DocumentID: "a", DocumentTitle: "Book A"},
		{ID: "3", DocumentID: "b", DocumentFilename: "b.epub"},
	})

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[0].Title != "Book A" || len(groups[0].Records) != 2 {
		t.Errorf("unexpected first group %+v", groups[0])
	}
	if groups[1].Title != "b.epub" {
		t.Errorf("expected filename fallback, got %q", groups[1].Title)
	}
}

func TestHighlightCountLabel(t *testing.T) {
	if got := views.HighlightCountLabel(1); got != "1 highlight" {
		t.Errorf("got %q", got)
	}
	if got := views.HighlightCountLabel(4); got != "4 highlights" {
		t.Errorf("got %q", got)
	}
}
