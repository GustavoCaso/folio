package db

import (
	"context"
	"encoding/json"
	"time"

	"github.com/GustavoCaso/folio/ui/internal/domain"
	"github.com/google/uuid"
)

func (d *db) CreateDocument(ctx context.Context, filename string, content []byte, requestID string, format domain.DocumentFormat) (domain.Document, error) {
	now := time.Now().UTC()
	id := uuid.NewString()
	_, err := d.conn.ExecContext(ctx,
		`INSERT INTO documents (id, filename, request_id, content, status, format, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 'PENDING', ?, ?, ?)`,
		id, filename, requestID, content, format, now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return domain.Document{}, err
	}
	return domain.Document{ID: id, Filename: filename, RequestID: requestID, Content: content, Status: "PENDING", Format: domain.DocumentFormat(format), CreatedAt: now, UpdatedAt: now}, nil
}

func (d *db) GetDocument(ctx context.Context, id string) (domain.Document, error) {
	row := d.conn.QueryRowContext(ctx,
		`SELECT id, filename, request_id, content, retry_count, status, reading_progress, error, output_path, title, author, cover, source_url, tags, format, created_at, updated_at
		 FROM documents WHERE id = ?`, id)
	return scanDocument(row)
}

func (d *db) GetPendingDocuments(ctx context.Context) ([]domain.Document, error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT id, filename, request_id, NULL, retry_count, status, reading_progress, error, output_path, title, author, cover, source_url, tags, format, created_at, updated_at
		 FROM documents WHERE status = 'PENDING'`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var documents []domain.Document
	for rows.Next() {
		j, err := scanDocument(rows)
		if err != nil {
			return nil, err
		}
		documents = append(documents, j)
	}
	return documents, rows.Err()
}

func (d *db) ListDocuments(ctx context.Context) ([]domain.Document, error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT id, filename, request_id, NULL, retry_count, status, reading_progress, error, output_path, title, author, cover, source_url, tags, format, created_at, updated_at
		 FROM documents ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var documents []domain.Document
	for rows.Next() {
		j, err := scanDocument(rows)
		if err != nil {
			return nil, err
		}
		documents = append(documents, j)
	}
	return documents, rows.Err()
}

func (d *db) UpdateDocumentStatus(ctx context.Context, id, status string) error {
	_, err := d.conn.ExecContext(ctx,
		`UPDATE documents SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

func (d *db) UpdateReadingProgress(ctx context.Context, id, blockID string) error {
	_, err := d.conn.ExecContext(ctx,
		`UPDATE documents SET reading_progress = ?, updated_at = ? WHERE id = ?`,
		blockID, time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

func (d *db) RetryDocument(ctx context.Context, id string) error {
	_, err := d.conn.ExecContext(ctx,
		`UPDATE documents SET status = 'PENDING', retry_count = retry_count + 1, updated_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

func (d *db) MarkDocumentDone(ctx context.Context, id, outputPath, title, author string, cover []byte) error {
	_, err := d.conn.ExecContext(ctx,
		`UPDATE documents SET status = 'DONE', content = NULL, output_path = ?,
		 title = ?, author = ?, cover = ?, updated_at = ? WHERE id = ?`,
		outputPath, title, author, cover, time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

func (d *db) DeleteDocument(ctx context.Context, id string) error {
	_, err := d.conn.ExecContext(ctx, `DELETE FROM documents WHERE id = ?`, id)
	return err
}

func (d *db) UpdateDocument(ctx context.Context, id, title, author string, tags []string, cover []byte) error {
	if tags == nil {
		tags = []string{}
	}
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return err
	}
	_, err = d.conn.ExecContext(ctx,
		`UPDATE documents SET title = ?, author = ?, tags = ?, cover = ?, updated_at = ? WHERE id = ?`,
		title, author, string(tagsJSON), cover, time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

func (d *db) MarkDocumentFailed(ctx context.Context, id, errMsg string) error {
	_, err := d.conn.ExecContext(ctx,
		`UPDATE documents SET status = 'FAILED', error = ?, updated_at = ? WHERE id = ?`,
		errMsg, time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanDocument(s scanner) (domain.Document, error) {
	var j domain.Document
	var createdAt, updatedAt string
	var format string
	var tagsStr string
	err := s.Scan(
		&j.ID, &j.Filename, &j.RequestID, &j.Content, &j.RetryCount, &j.Status,
		&j.ReadingProgress, &j.Error, &j.OutputPath, &j.Title, &j.Author, &j.Cover,
		&j.SourceURL, &tagsStr, &format, &createdAt, &updatedAt,
	)
	if err != nil {
		return domain.Document{}, err
	}
	j.Tags = []string{}
	if tagsStr != "" {
		if jsonErr := json.Unmarshal([]byte(tagsStr), &j.Tags); jsonErr != nil {
			return domain.Document{}, jsonErr
		}
	}
	j.Format = domain.DocumentFormat(format)
	j.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	j.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return j, nil
}
