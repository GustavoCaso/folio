package domain

import "time"

type Job struct {
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
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Highlight struct {
	ID           string
	JobID        string
	StartBlockID string
	EndBlockID   string
	StartPos     int
	EndPos       int
	Text         string
	Tag          string
	Note         string
	CreatedAt    time.Time
}

type ExportRecord struct {
	ID            string
	HighlightID   string
	HighlightText string
	HighlightNote string
	HighlightTag  string
	JobFilename   string
	BackendName   string
	ExternalID    string
	Status        string
	Error         string
	ExportedAt    time.Time
}
