package views_test

import (
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/views"
)

func TestTruncate(t *testing.T) {
	cases := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{"shorter than limit", "abc", 5, "abc"},
		{"exactly the limit", "abcde", 5, "abcde"},
		{"longer gets ellipsis", "abcdef", 5, "abcde…"},
		{"counts runes, not bytes", "ñandú ñandú", 5, "ñandú…"},
		{"empty", "", 3, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := views.Truncate(tc.s, tc.n); got != tc.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tc.s, tc.n, got, tc.want)
			}
		})
	}
}
