package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
)

func (r *Repository) CreateJob(ctx context.Context, filename string, content []byte, requestID string) (Job, error) {
	now := time.Now().UTC()
	id := uuid.NewString()
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO jobs (id, filename, request_id, content, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 'PENDING', ?, ?)`,
		id, filename, requestID, content, now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return Job{}, err
	}
	return Job{ID: id, Filename: filename, RequestID: requestID, Content: content, Status: "PENDING", CreatedAt: now, UpdatedAt: now}, nil
}

func (r *Repository) GetJob(ctx context.Context, id string) (Job, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, filename, request_id, content, retry_count, status, reading_progress, error, output_path, created_at, updated_at
		 FROM jobs WHERE id = ?`, id)
	return scanJob(row)
}

func (r *Repository) GetPendingJobs(ctx context.Context) ([]Job, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, filename, request_id, NULL, retry_count, status, reading_progress, error, output_path, created_at, updated_at
		 FROM jobs WHERE status = 'PENDING'`)

	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var jobs []Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func (r *Repository) ListJobs(ctx context.Context) ([]Job, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, filename, request_id, NULL, retry_count, status, reading_progress, error, output_path, created_at, updated_at
		 FROM jobs ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var jobs []Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func (r *Repository) UpdateJobStatus(ctx context.Context, id, status string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE jobs SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

func (r *Repository) UpdateReadingProgress(ctx context.Context, id, blockID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE jobs SET reading_progress = ?, updated_at = ? WHERE id = ?`,
		blockID, time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

func (r *Repository) RetryJob(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE jobs SET status = 'PENDING', retry_count = retry_count + 1, updated_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

func (r *Repository) MarkJobDone(ctx context.Context, id, outputPath string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE jobs SET status = 'DONE', content = NULL, output_path = ?, updated_at = ? WHERE id = ?`,
		outputPath, time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

func (r *Repository) DeleteJob(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM jobs WHERE id = ?`, id)
	return err
}

func (r *Repository) MarkJobFailed(ctx context.Context, id, errMsg string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE jobs SET status = 'FAILED', error = ?, updated_at = ? WHERE id = ?`,
		errMsg, time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

func scanJob(s Row) (Job, error) {
	var j Job
	var createdAt, updatedAt string
	err := s.Scan(
		&j.ID, &j.Filename, &j.RequestID, &j.Content, &j.RetryCount, &j.Status,
		&j.ReadingProgress,
		&j.Error, &j.OutputPath,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return Job{}, err
	}
	j.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	j.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return j, nil
}
