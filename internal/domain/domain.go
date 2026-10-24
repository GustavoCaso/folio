package domain

import "time"

type DocumentFormat string

const EpubFormat DocumentFormat = "epub"

type Document struct {
	ID              string
	Filename        string
	RequestID       string
	Content         []byte
	RetryCount      int
	Status          string
	ReadingProgress string
	Error           string
	OutputPath      string
	Title           string
	Author          string
	Cover           []byte
	SourceURL       string
	Format          DocumentFormat
	Tags            []string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// DocumentFilter narrows the library listing. Empty fields match everything;
// set fields combine with AND.
type DocumentFilter struct {
	Query  string
	Author string
	Tag    string
}

func (f DocumentFilter) Active() bool {
	return f.Query != "" || f.Author != "" || f.Tag != ""
}

type FacetCount struct {
	Value string
	Count int
}

// Facets are the author and tag counts across a filtered result set.
type Facets struct {
	Authors []FacetCount
	Tags    []FacetCount
}

type Highlight struct {
	ID           string    `json:"id"`
	DocumentID   string    `json:"document_id"`
	StartBlockID string    `json:"start_block_id"`
	EndBlockID   string    `json:"end_block_id"`
	Chapter      int       `json:"chapter"`
	StartPos     int       `json:"start_pos"`
	EndPos       int       `json:"end_pos"`
	Text         string    `json:"text"`
	Tag          string    `json:"tag"`
	Note         string    `json:"note"`
	CreatedAt    time.Time `json:"created_at"`
}

type ExportRecord struct {
	ID               string
	HighlightID      string
	HighlightText    string
	HighlightNote    string
	HighlightTag     string
	DocumentFilename string
	DocumentTags     []string
	BackendName      string
	ExternalID       string
	Status           string
	Error            string
	ExportedAt       time.Time
}
