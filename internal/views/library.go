// Package views holds the data each page template renders and the pure
// helpers templates use to format it.
package views

import (
	"hash/fnv"
	"net/url"
	"strconv"

	"github.com/GustavoCaso/folio/ui/internal/domain"
)

type LibraryView string

const (
	ViewGrid LibraryView = "grid"
	ViewList LibraryView = "list"
)

// LibraryPage is everything the documents page renders.
type LibraryPage struct {
	Documents        []domain.Document
	Facets           domain.Facets
	Filter           domain.DocumentFilter
	View             LibraryView
	WatchDocumentIDs []string
	ErrMsg           string
}

// LibraryURL encodes filter as a "/" query string, omitting empty fields so
// facet links stay short and stable.
func LibraryURL(filter domain.DocumentFilter) string {
	q := url.Values{}
	if filter.Query != "" {
		q.Set("q", filter.Query)
	}
	if filter.Author != "" {
		q.Set("author", filter.Author)
	}
	if filter.Tag != "" {
		q.Set("tag", filter.Tag)
	}
	if len(q) == 0 {
		return "/"
	}
	return "/?" + q.Encode()
}

// ToggleAuthorURL selects author, or clears it when it's already selected,
// keeping the other active filters.
func ToggleAuthorURL(filter domain.DocumentFilter, author string) string {
	if filter.Author == author {
		author = ""
	}
	filter.Author = author
	return LibraryURL(filter)
}

func ToggleTagURL(filter domain.DocumentFilter, tag string) string {
	if filter.Tag == tag {
		tag = ""
	}
	filter.Tag = tag
	return LibraryURL(filter)
}

func BookCount(n int) string {
	if n == 1 {
		return "1 book"
	}
	return strconv.Itoa(n) + " books"
}

func DisplayTitle(d domain.Document) string {
	if d.Title != "" {
		return d.Title
	}
	return d.Filename
}

// coverTones is the number of .cover-tone-N classes in library.css.
const coverTones = 6

// CoverTone picks a stable background for books without a cover image, so the
// same book always gets the same colour.
func CoverTone(d domain.Document) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(d.ID))
	return "cover-tone-" + strconv.Itoa(int(h.Sum32()%coverTones))
}
