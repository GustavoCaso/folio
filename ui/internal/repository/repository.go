package repository

import (
	"context"
	"time"
)

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

type JobRepository interface {
	CreateJob(ctx context.Context, filename string, content []byte, requestID string) (Job, error)
	GetJob(ctx context.Context, id string) (Job, error)
	GetPendingJobs(ctx context.Context) ([]Job, error)
	ListJobs(ctx context.Context) ([]Job, error)
	UpdateJobStatus(ctx context.Context, id, status string) error
	UpdateReadingProgress(ctx context.Context, id, blockID string) error
	RetryJob(ctx context.Context, id string) error
	MarkJobDone(ctx context.Context, id, outputPath string) error
	DeleteJob(ctx context.Context, id string) error
	MarkJobFailed(ctx context.Context, id, errMsg string) error
}

type HighlightRepository interface {
	CreateHighlight(ctx context.Context, h Highlight) (Highlight, error)
	ListHighlights(ctx context.Context, jobID string) ([]Highlight, error)
	DeleteHighlight(ctx context.Context, id string) error
}

type ExportRepository interface {
	ListUnexportedHighlights(ctx context.Context, backendName string) ([]ExportRecord, error)
	CreateHighlightExport(ctx context.Context, highlightID, backendName string) error
	MarkHighlightExported(ctx context.Context, exportID, externalID string) error
	MarkHighlightExportFailed(ctx context.Context, exportID, errMsg string) error
	ListExportsByHighlight(ctx context.Context, highlightID string) ([]ExportRecord, error)
	ListAllExports(ctx context.Context) ([]ExportRecord, error)
}

type Store interface {
	JobRepository
	HighlightRepository
	ExportRepository
	Close() error
}
