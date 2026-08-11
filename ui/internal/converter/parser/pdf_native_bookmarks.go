package parser

// bookmark is a pdfium bookmark/outline node, decoupled from pdfium's
// response types so chapter resolution stays pure and testable without a
// pdfium instance. PageIndex is 0-based.
type bookmark struct {
	Title     string
	PageIndex int
	Children  []bookmark
}

// chapterRange is a 0-based, inclusive page range for one top-level chapter.
type chapterRange struct {
	StartPage int
	EndPage   int
}

// resolveChapters turns a pdfium bookmark tree into top-level chapter page
// ranges plus a matching tocEntry tree. Top-level bookmarks become chapters;
// their page range runs from their own start page to the page before the
// next top-level bookmark's start page (or the last page, for the final
// chapter). Nested bookmarks become subsection tocEntry items anchored
// within their parent's chapter (they share the parent's ChapterIdx — unlike
// epub, a PDF chapter is a page range, not a separate file, so there is
// nothing for a subsection to point at other than its parent chapter).
//
// No bookmarks at all falls back to a single synthetic chapter spanning the
// whole document, mirroring epub's fallback-entry behavior for spine items
// with zero TOC coverage.
func resolveChapters(bookmarks []bookmark, pageCount int) ([]chapterRange, []tocEntry) {
	if len(bookmarks) == 0 {
		return []chapterRange{{StartPage: 0, EndPage: pageCount - 1}},
			[]tocEntry{{ChapterIdx: 0}}
	}

	chapters := make([]chapterRange, len(bookmarks))
	entries := make([]tocEntry, len(bookmarks))
	for i, b := range bookmarks {
		start := b.PageIndex
		end := pageCount - 1
		if i+1 < len(bookmarks) {
			end = bookmarks[i+1].PageIndex - 1
		}
		chapters[i] = chapterRange{StartPage: start, EndPage: end}
		entries[i] = tocEntry{
			Title:      b.Title,
			ChapterIdx: i,
			Items:      subsectionEntries(b.Children, i),
		}
	}
	return chapters, entries
}

// subsectionEntries converts nested bookmarks into tocEntry items anchored
// to their parent's chapterIdx.
func subsectionEntries(children []bookmark, chapterIdx int) []tocEntry {
	if len(children) == 0 {
		return nil
	}
	entries := make([]tocEntry, len(children))
	for i, c := range children {
		entries[i] = tocEntry{
			Title:      c.Title,
			ChapterIdx: chapterIdx,
			Items:      subsectionEntries(c.Children, chapterIdx),
		}
	}
	return entries
}
