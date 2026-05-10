package db

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type HighlightWithJob struct {
	Highlight
	JobFilename string
}

type ExportRecord struct {
	ID            string
	HighlightID   string
	HighlightText string
	JobFilename   string
	BackendName   string
	ExternalID    string
	Status        string // "exported" | "failed"
	Error         string
	ExportedAt    time.Time
}

func (s *Store) ListUnexportedHighlightsWithJob(ctx context.Context, backendName string) ([]HighlightWithJob, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT h.id, h.job_id, h.start_block_id, h.end_block_id, h.start_pos, h.end_pos,
		       h.text, h.tag, h.note, h.created_at, j.filename
		FROM highlights h
		JOIN jobs j ON h.job_id = j.id
		LEFT JOIN highlight_exports he
		       ON h.id = he.highlight_id AND he.backend_name = ? AND he.status = 'exported'
		WHERE he.id IS NULL
		ORDER BY h.created_at ASC`, backendName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []HighlightWithJob
	for rows.Next() {
		var h HighlightWithJob
		var createdAt string
		if err := rows.Scan(
			&h.ID, &h.JobID, &h.StartBlockID, &h.EndBlockID, &h.StartPos, &h.EndPos,
			&h.Text, &h.Tag, &h.Note, &createdAt, &h.JobFilename,
		); err != nil {
			return nil, err
		}
		h.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *Store) MarkHighlightExported(ctx context.Context, highlightID, backendName, externalID string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO highlight_exports (id, highlight_id, backend_name, external_id, status, error, exported_at)
		VALUES (?, ?, ?, ?, 'exported', '', ?)`,
		uuid.NewString(), highlightID, backendName, externalID,
		time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

func (s *Store) MarkHighlightExportFailed(ctx context.Context, highlightID, backendName, errMsg string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO highlight_exports (id, highlight_id, backend_name, external_id, status, error, exported_at)
		VALUES (?, ?, ?, '', 'failed', ?, ?)`,
		uuid.NewString(), highlightID, backendName, errMsg,
		time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

func (s *Store) ListExportsByHighlight(ctx context.Context, highlightID string) ([]ExportRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, highlight_id, backend_name, external_id, status, error, exported_at
		FROM highlight_exports
		WHERE highlight_id = ?
		ORDER BY exported_at DESC`, highlightID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []ExportRecord
	for rows.Next() {
		var rec ExportRecord
		var exportedAt string
		if err := rows.Scan(
			&rec.ID, &rec.HighlightID, &rec.BackendName, &rec.ExternalID,
			&rec.Status, &rec.Error, &exportedAt,
		); err != nil {
			return nil, err
		}
		rec.ExportedAt, _ = time.Parse(time.RFC3339, exportedAt)
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (s *Store) ListAllExports(ctx context.Context) ([]ExportRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT he.id, he.highlight_id, he.backend_name, he.external_id, he.status, he.error, he.exported_at,
		       h.text, j.filename
		FROM highlight_exports he
		JOIN highlights h ON he.highlight_id = h.id
		JOIN jobs j ON h.job_id = j.id
		ORDER BY he.exported_at DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []ExportRecord
	for rows.Next() {
		var rec ExportRecord
		var exportedAt string
		if err := rows.Scan(
			&rec.ID, &rec.HighlightID, &rec.BackendName, &rec.ExternalID,
			&rec.Status, &rec.Error, &exportedAt,
			&rec.HighlightText, &rec.JobFilename,
		); err != nil {
			return nil, err
		}
		rec.ExportedAt, _ = time.Parse(time.RFC3339, exportedAt)
		out = append(out, rec)
	}
	return out, rows.Err()
}
