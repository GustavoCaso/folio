package views_test

import (
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/domain"
	"github.com/GustavoCaso/folio/ui/internal/views"
)

func TestHighlightCount(t *testing.T) {
	if got := views.HighlightCount(nil); got != "0" {
		t.Errorf("got %q, want 0", got)
	}
	if got := views.HighlightCount(make([]domain.Highlight, 3)); got != "3" {
		t.Errorf("got %q, want 3", got)
	}
}

func TestSubsectionHref(t *testing.T) {
	item := views.EpubTOCEntry{ChapterIdx: 2, Anchor: "sec-1"}
	cases := []struct {
		name    string
		current int
		full    bool
		want    string
	}{
		{"same chapter jumps in page", 2, false, "#sec-1"},
		{"other chapter links across", 1, false, "?chapter=2#sec-1"},
		{"full view always links across", 2, true, "?chapter=2#sec-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := views.SubsectionHref(item, tc.current, tc.full); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRailLabel(t *testing.T) {
	if got := views.RailLabel("prev"); got != "Previous chapter" {
		t.Errorf("prev: got %q", got)
	}
	if got := views.RailLabel("next"); got != "Next chapter" {
		t.Errorf("next: got %q", got)
	}
}

func TestCurrentChapterAttr(t *testing.T) {
	if got := views.CurrentChapterAttr(4, false); got != "4" {
		t.Errorf("chapter view: got %q, want 4", got)
	}
	if got := views.CurrentChapterAttr(4, true); got != "-1" {
		t.Errorf("full view: got %q, want -1", got)
	}
}
