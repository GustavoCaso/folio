package db

import (
	"context"
	"time"

	"github.com/GustavoCaso/folio/ui/internal/repository"
	"github.com/google/uuid"
)

func (r *Repository) ListUnexportedHighlights(ctx context.Context, backendName string) ([]repository.ExportRecord, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT he.id, he.highlight_id, he.backend_name, COALESCE(he.external_id, ''), he.status, COALESCE(he.error, ''), COALESCE(he.exported_at, ''),
		       h.text, COALESCE(h.note, ''), COALESCE(h.tag, ''), j.filename
		FROM highlight_exports he
		JOIN highlights h ON he.highlight_id = h.id
		JOIN jobs j ON h.job_id = j.id
		WHERE he.backend_name = ? AND he.status = 'PENDING'
		ORDER BY he.created_at ASC`, backendName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []repository.ExportRecord
	for rows.Next() {
		var rec repository.ExportRecord
		var exportedAt string
		if err := rows.Scan(
			&rec.ID, &rec.HighlightID, &rec.BackendName, &rec.ExternalID,
			&rec.Status, &rec.Error, &exportedAt,
			&rec.HighlightText, &rec.HighlightNote, &rec.HighlightTag, &rec.JobFilename,
		); err != nil {
			return nil, err
		}
		rec.ExportedAt, _ = time.Parse(time.RFC3339, exportedAt)
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *Repository) CreateHighlightExport(ctx context.Context, highlightID, backendName string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.db.ExecContext(ctx, `
		INSERT INTO highlight_exports (id, highlight_id, status, backend_name, created_at, updated_at)
		VALUES (?, ?, 'PENDING', ?, ?, ?)`, uuid.NewString(), highlightID, backendName, now, now,
	)
	return err
}

func (r *Repository) MarkHighlightExported(ctx context.Context, exportID, externalID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.db.ExecContext(ctx, `
		UPDATE highlight_exports
		SET external_id = ?, status = 'EXPORTED', error = '', exported_at = ?, updated_at = ?
		WHERE id = ?`,
		externalID, now, now, exportID,
	)
	return err
}

func (r *Repository) MarkHighlightExportFailed(ctx context.Context, exportID, errMsg string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.db.ExecContext(ctx, `
		UPDATE highlight_exports
		SET status = 'FAILED', error = ?, exported_at = ?, updated_at = ?
		WHERE id = ?`,
		errMsg, now, now, exportID,
	)
	return err
}

func (r *Repository) ListExportsByHighlight(ctx context.Context, highlightID string) ([]repository.ExportRecord, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, highlight_id, backend_name, COALESCE(external_id, ''), status, COALESCE(error, ''), COALESCE(exported_at, '')
		FROM highlight_exports
		WHERE highlight_id = ?
		ORDER BY exported_at DESC`, highlightID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []repository.ExportRecord
	for rows.Next() {
		var rec repository.ExportRecord
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

func (r *Repository) ListAllExports(ctx context.Context) ([]repository.ExportRecord, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT he.id, he.highlight_id, he.backend_name, COALESCE(he.external_id, ''), he.status, COALESCE(he.error, ''), COALESCE(he.exported_at, ''),
		       h.text, COALESCE(h.note, ''), COALESCE(h.tag, ''), j.filename
		FROM highlight_exports he
		JOIN highlights h ON he.highlight_id = h.id
		JOIN jobs j ON h.job_id = j.id
		ORDER BY he.exported_at DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []repository.ExportRecord
	for rows.Next() {
		var rec repository.ExportRecord
		var exportedAt string
		if err := rows.Scan(
			&rec.ID, &rec.HighlightID, &rec.BackendName, &rec.ExternalID,
			&rec.Status, &rec.Error, &exportedAt,
			&rec.HighlightText, &rec.HighlightNote, &rec.HighlightTag, &rec.JobFilename,
		); err != nil {
			return nil, err
		}
		rec.ExportedAt, _ = time.Parse(time.RFC3339, exportedAt)
		out = append(out, rec)
	}
	return out, rows.Err()
}
