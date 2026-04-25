package db

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Job struct {
	ID         string
	Filename   string
	RequestID  string
	Status     string
	PagesDone  int
	PagesTotal int
	Error      string
	OutputPath string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (s *Store) CreateJob(ctx context.Context, filename, requestID string) (Job, error) {
	now := time.Now().UTC()
	id := uuid.NewString()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO jobs (id, filename, request_id, status, created_at, updated_at)
		 VALUES (?, ?, ?, 'PENDING', ?, ?)`,
		id, filename, requestID, now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return Job{}, err
	}
	return Job{ID: id, Filename: filename, RequestID: requestID, Status: "PENDING", CreatedAt: now, UpdatedAt: now}, nil
}

func (s *Store) GetJob(ctx context.Context, id string) (Job, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, filename, request_id, status, pages_done, pages_total, error, output_path, created_at, updated_at
		 FROM jobs WHERE id = ?`, id)
	return scanJob(row)
}

func (s *Store) ListJobs(ctx context.Context) ([]Job, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, filename, request_id, status, pages_done, pages_total, error, output_path, created_at, updated_at
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

func (s *Store) UpdateJobProgress(ctx context.Context, id, status string, pagesDone, pagesTotal int) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE jobs SET status = ?, pages_done = ?, pages_total = ?, updated_at = ? WHERE id = ?`,
		status, pagesDone, pagesTotal, time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

func (s *Store) MarkJobDone(ctx context.Context, id, outputPath string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE jobs SET status = 'DONE', output_path = ?, updated_at = ? WHERE id = ?`,
		outputPath, time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

func (s *Store) MarkJobFailed(ctx context.Context, id, errMsg string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE jobs SET status = 'FAILED', error = ?, updated_at = ? WHERE id = ?`,
		errMsg, time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanJob(s scanner) (Job, error) {
	var j Job
	var createdAt, updatedAt string
	err := s.Scan(
		&j.ID, &j.Filename, &j.RequestID, &j.Status,
		&j.PagesDone, &j.PagesTotal,
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
