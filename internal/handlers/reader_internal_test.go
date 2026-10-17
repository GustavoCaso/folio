package handlers

import "testing"

func TestParseChapterParam(t *testing.T) {
	tests := []struct {
		name            string
		chapterParam    string
		readingProgress string
		want            int
	}{
		{"chapter param wins", "2", "ch5-p-1", 2},
		{"chapter param zero", "0", "ch5-p-1", 0},
		{"negative chapter param falls back to progress", "-1", "ch3-p-1", 3},
		{"non-numeric chapter param falls back to progress", "abc", "ch3-p-1", 3},
		{"progress single digit", "", "ch0-p-1", 0},
		{"progress multi digit", "", "ch12-p-7", 12},
		{"progress heading tag", "", "ch4-h2-3", 4},
		{"progress tag with digits", "", "ch10-h1-22", 10},
		{"empty everything", "", "", 0},
		{"legacy markdown block id", "", "paragraph-42", 0},
		{"missing chapter digits", "", "ch-p-1", 0},
		{"missing element segment", "", "ch3", 0},
		{"garbage progress", "", "not-a-block", 0},
		{"overflowing chapter index", "", "ch99999999999999999999-p-1", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseChapterParam(tt.chapterParam, tt.readingProgress)
			if got != tt.want {
				t.Errorf("parseChapterParam(%q, %q) = %d, want %d", tt.chapterParam, tt.readingProgress, got, tt.want)
			}
		})
	}
}
