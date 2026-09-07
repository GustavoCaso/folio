package parser

import "testing"

func TestResolveChapters_NoBookmarksFallsBackToSingleChapter(t *testing.T) {
	chapters, entries := resolveChapters(nil, 10)

	if len(chapters) != 1 {
		t.Fatalf("expected 1 fallback chapter, got %d: %+v", len(chapters), chapters)
	}
	if chapters[0].StartPage != 0 || chapters[0].EndPage != 9 {
		t.Errorf("expected page range [0,9], got [%d,%d]", chapters[0].StartPage, chapters[0].EndPage)
	}
	if len(entries) != 1 || entries[0].ChapterIdx != 0 || entries[0].Title != "" {
		t.Errorf("expected 1 synthetic toc entry pointing at chapter 0, got %+v", entries)
	}
}

func TestResolveChapters_FlatBookmarksBecomeSiblingChapters(t *testing.T) {
	bookmarks := []bookmark{
		{Title: "Chapter 1", PageIndex: 0},
		{Title: "Chapter 2", PageIndex: 5},
		{Title: "Chapter 3", PageIndex: 8},
	}
	chapters, entries := resolveChapters(bookmarks, 10)

	if len(chapters) != 3 {
		t.Fatalf("expected 3 chapters, got %d: %+v", len(chapters), chapters)
	}
	wantRanges := [][2]int{{0, 4}, {5, 7}, {8, 9}}
	for i, want := range wantRanges {
		if chapters[i].StartPage != want[0] || chapters[i].EndPage != want[1] {
			t.Errorf("chapter %d: expected [%d,%d], got [%d,%d]", i, want[0], want[1], chapters[i].StartPage, chapters[i].EndPage)
		}
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 toc entries, got %d", len(entries))
	}
	for i, want := range []string{"Chapter 1", "Chapter 2", "Chapter 3"} {
		if entries[i].Title != want {
			t.Errorf("entry %d: expected title %q, got %q", i, want, entries[i].Title)
		}
		if entries[i].ChapterIdx != i {
			t.Errorf("entry %d: expected ChapterIdx %d, got %d", i, i, entries[i].ChapterIdx)
		}
	}
}

func TestResolveChapters_NestedBookmarksProduceSubsectionsWithinParentPageRange(t *testing.T) {
	bookmarks := []bookmark{
		{Title: "Chapter 1", PageIndex: 0, Children: []bookmark{
			{Title: "Section 1.1", PageIndex: 2},
		}},
		{Title: "Chapter 2", PageIndex: 5},
	}
	chapters, entries := resolveChapters(bookmarks, 10)

	if len(chapters) != 2 {
		t.Fatalf("expected 2 top-level chapters, got %d: %+v", len(chapters), chapters)
	}
	if chapters[0].StartPage != 0 || chapters[0].EndPage != 4 {
		t.Errorf("expected chapter 0 range [0,4], got [%d,%d]", chapters[0].StartPage, chapters[0].EndPage)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 top-level entries, got %d", len(entries))
	}
	if len(entries[0].Items) != 1 || entries[0].Items[0].Title != "Section 1.1" {
		t.Errorf("expected nested subsection entry, got %+v", entries[0])
	}
	// Subsection shares its parent chapter's index — nested bookmarks are
	// anchors within the parent's chapter page range, not separate chapters.
	if entries[0].Items[0].ChapterIdx != 0 {
		t.Errorf("expected subsection ChapterIdx 0, got %d", entries[0].Items[0].ChapterIdx)
	}
}

func TestResolveChapters_LastChapterExtendsToLastPage(t *testing.T) {
	bookmarks := []bookmark{
		{Title: "Only Chapter", PageIndex: 3},
	}
	chapters, _ := resolveChapters(bookmarks, 20)

	if len(chapters) != 1 {
		t.Fatalf("expected 1 chapter, got %d", len(chapters))
	}
	if chapters[0].StartPage != 3 || chapters[0].EndPage != 19 {
		t.Errorf("expected range [3,19], got [%d,%d]", chapters[0].StartPage, chapters[0].EndPage)
	}
}
