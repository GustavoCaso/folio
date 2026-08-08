package db

import (
	"context"
	"encoding/json"
	"time"

	"github.com/GustavoCaso/folio/ui/internal/domain"
	"github.com/google/uuid"
)

func (d *db) CreateJob(ctx context.Context, filename string, content []byte, requestID, format string) (domain.Job, error) {
	now := time.Now().UTC()
	id := uuid.NewString()
	_, err := d.conn.ExecContext(ctx,
		`INSERT INTO jobs (id, filename, request_id, content, status, format, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 'PENDING', ?, ?, ?)`,
		id, filename, requestID, content, format, now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return domain.Job{}, err
	}
	return domain.Job{ID: id, Filename: filename, RequestID: requestID, Content: content, Status: "PENDING", Format: format, CreatedAt: now, UpdatedAt: now}, nil
}

func (d *db) CreateJobFromURL(ctx context.Context, sourceURL, filename, requestID string) (domain.Job, error) {
	now := time.Now().UTC()
	id := uuid.NewString()
	_, err := d.conn.ExecContext(ctx,
		`INSERT INTO jobs (id, filename, request_id, source_url, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 'PENDING', ?, ?)`,
		id, filename, requestID, sourceURL, now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return domain.Job{}, err
	}
	return domain.Job{ID: id, Filename: filename, RequestID: requestID, SourceURL: sourceURL, Status: "PENDING", CreatedAt: now, UpdatedAt: now}, nil
}

func (d *db) GetJob(ctx context.Context, id string) (domain.Job, error) {
	row := d.conn.QueryRowContext(ctx,
		`SELECT id, filename, request_id, content, retry_count, status, reading_progress, error, output_path, title, author, cover, source_url, tags, format, created_at, updated_at
		 FROM jobs WHERE id = ?`, id)
	return scanJob(row)
}

func (d *db) GetPendingJobs(ctx context.Context) ([]domain.Job, error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT id, filename, request_id, NULL, retry_count, status, reading_progress, error, output_path, title, author, cover, source_url, tags, format, created_at, updated_at
		 FROM jobs WHERE status = 'PENDING'`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var jobs []domain.Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func (d *db) ListJobs(ctx context.Context) ([]domain.Job, error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT id, filename, request_id, NULL, retry_count, status, reading_progress, error, output_path, title, author, cover, source_url, tags, format, created_at, updated_at
		 FROM jobs ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var jobs []domain.Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func (d *db) UpdateJobStatus(ctx context.Context, id, status string) error {
	_, err := d.conn.ExecContext(ctx,
		`UPDATE jobs SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

func (d *db) UpdateReadingProgress(ctx context.Context, id, blockID string) error {
	_, err := d.conn.ExecContext(ctx,
		`UPDATE jobs SET reading_progress = ?, updated_at = ? WHERE id = ?`,
		blockID, time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

func (d *db) RetryJob(ctx context.Context, id string) error {
	_, err := d.conn.ExecContext(ctx,
		`UPDATE jobs SET status = 'PENDING', retry_count = retry_count + 1, updated_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

func (d *db) MarkJobDone(ctx context.Context, id, outputPath, title, author string, cover []byte) error {
	_, err := d.conn.ExecContext(ctx,
		`UPDATE jobs SET status = 'DONE', content = NULL, output_path = ?,
		 title = ?, author = ?, cover = ?, updated_at = ? WHERE id = ?`,
		outputPath, title, author, cover, time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

func (d *db) DeleteJob(ctx context.Context, id string) error {
	_, err := d.conn.ExecContext(ctx, `DELETE FROM jobs WHERE id = ?`, id)
	return err
}

func (d *db) UpdateJob(ctx context.Context, id, title, author string, tags []string, cover []byte) error {
	if tags == nil {
		tags = []string{}
	}
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return err
	}
	_, err = d.conn.ExecContext(ctx,
		`UPDATE jobs SET title = ?, author = ?, tags = ?, cover = ?, updated_at = ? WHERE id = ?`,
		title, author, string(tagsJSON), cover, time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

func (d *db) MarkJobFailed(ctx context.Context, id, errMsg string) error {
	_, err := d.conn.ExecContext(ctx,
		`UPDATE jobs SET status = 'FAILED', error = ?, updated_at = ? WHERE id = ?`,
		errMsg, time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanJob(s scanner) (domain.Job, error) {
	var j domain.Job
	var createdAt, updatedAt string
	var tagsStr string
	err := s.Scan(
		&j.ID, &j.Filename, &j.RequestID, &j.Content, &j.RetryCount, &j.Status,
		&j.ReadingProgress, &j.Error, &j.OutputPath, &j.Title, &j.Author, &j.Cover,
		&j.SourceURL, &tagsStr, &j.Format, &createdAt, &updatedAt,
	)
	if err != nil {
		return domain.Job{}, err
	}
	j.Tags = []string{}
	if tagsStr != "" {
		if jsonErr := json.Unmarshal([]byte(tagsStr), &j.Tags); jsonErr != nil {
			return domain.Job{}, jsonErr
		}
	}
	j.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	j.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return j, nil
}
