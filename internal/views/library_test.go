package views_test

import (
	"strings"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/domain"
	"github.com/GustavoCaso/folio/ui/internal/views"
)

func TestLibraryURL(t *testing.T) {
	cases := []struct {
		name   string
		filter domain.DocumentFilter
		want   string
	}{
		{"empty filter is the bare root", domain.DocumentFilter{}, "/"},
		{"query only", domain.DocumentFilter{Query: "dune"}, "/?q=dune"},
		{"all fields, keys sorted", domain.DocumentFilter{Query: "dune", Author: "Frank Herbert", Tag: "sci-fi"}, "/?author=Frank+Herbert&q=dune&tag=sci-fi"},
		{"values are escaped", domain.DocumentFilter{Tag: "a&b=c"}, "/?tag=a%26b%3Dc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := views.LibraryURL(tc.filter); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestToggleAuthorURL(t *testing.T) {
	filter := domain.DocumentFilter{Query: "dune", Author: "Frank Herbert", Tag: "sci-fi"}

	if got, want := views.ToggleAuthorURL(filter, "Frank Herbert"), "/?q=dune&tag=sci-fi"; got != want {
		t.Errorf("active author should clear: got %q, want %q", got, want)
	}
	if got, want := views.ToggleAuthorURL(filter, "Ursula K. Le Guin"), "/?author=Ursula+K.+Le+Guin&q=dune&tag=sci-fi"; got != want {
		t.Errorf("other author should replace: got %q, want %q", got, want)
	}
	if filter.Author != "Frank Herbert" {
		t.Errorf("caller's filter was mutated: %+v", filter)
	}
}

func TestToggleTagURL(t *testing.T) {
	filter := domain.DocumentFilter{Query: "dune", Tag: "sci-fi"}

	if got, want := views.ToggleTagURL(filter, "sci-fi"), "/?q=dune"; got != want {
		t.Errorf("active tag should clear: got %q, want %q", got, want)
	}
	if got, want := views.ToggleTagURL(filter, "classic"), "/?q=dune&tag=classic"; got != want {
		t.Errorf("other tag should replace: got %q, want %q", got, want)
	}
}

func TestBookCount(t *testing.T) {
	for n, want := range map[int]string{0: "0 books", 1: "1 book", 2: "2 books", 12: "12 books"} {
		if got := views.BookCount(n); got != want {
			t.Errorf("BookCount(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestDisplayTitle(t *testing.T) {
	if got := views.DisplayTitle(domain.Document{Title: "Dune", Filename: "dune.epub"}); got != "Dune" {
		t.Errorf("prefers title, got %q", got)
	}
	if got := views.DisplayTitle(domain.Document{Filename: "dune.epub"}); got != "dune.epub" {
		t.Errorf("falls back to filename, got %q", got)
	}
}

func TestCoverTone(t *testing.T) {
	doc := domain.Document{ID: "a1"}
	first := views.CoverTone(doc)
	if views.CoverTone(doc) != first {
		t.Error("expected the same tone for the same document")
	}

	// Every tone must map to a .cover-tone-0..5 rule in library.css.
	for i := range 200 {
		tone := views.CoverTone(domain.Document{ID: strings.Repeat("x", i)})
		if !strings.HasPrefix(tone, "cover-tone-") || len(tone) != len("cover-tone-0") || tone[len(tone)-1] < '0' || tone[len(tone)-1] > '5' {
			t.Fatalf("unexpected tone %q", tone)
		}
	}
}
