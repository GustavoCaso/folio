package views

import (
	"strconv"

	"github.com/GustavoCaso/folio/ui/internal/domain"
)

func HighlightCount(highlights []domain.Highlight) string {
	return strconv.Itoa(len(highlights))
}

// EpubTOCEntry mirrors the shape parser.NewEPUB writes to toc.json.
// handlers/reader.go unmarshals toc.json directly into this type (the JSON
// tags match the parser's write-side tocEntry struct exactly), so there is no
// separate read-side struct in the handlers package.
type EpubTOCEntry struct {
	Title      string         `json:"title"`
	ChapterIdx int            `json:"chapter_idx"`
	Anchor     string         `json:"anchor"`
	Items      []EpubTOCEntry `json:"items"`
}

// SubsectionHref reproduces the link logic previously in reader.go's
// writeTOCList: a same-page "#anchor" jump when the subsection belongs to
// the chapter currently being viewed; otherwise a cross-chapter
// "?chapter=N#anchor" link.
func SubsectionHref(item EpubTOCEntry, currentChapterIdx int) string {
	if item.ChapterIdx == currentChapterIdx {
		return "#" + item.Anchor
	}
	return "?chapter=" + strconv.Itoa(item.ChapterIdx) + "#" + item.Anchor
}

func RailLabel(side string) string {
	if side == "prev" {
		return "Previous chapter"
	}
	return "Next chapter"
}
