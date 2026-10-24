package views

import "unicode/utf8"

// Truncate shortens s to n runes (not bytes, so multi-byte text never splits
// mid-character) and appends an ellipsis when it cut anything.
func Truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return string(runes[:n]) + "…"
}
