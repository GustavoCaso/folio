package db

import (
	"context"
	"fmt"
	"time"

	"github.com/GustavoCaso/folio/ui/internal/domain"
	"github.com/google/uuid"
)

func (d *db) CreateHighlight(ctx context.Context, h domain.Highlight) (domain.Highlight, error) {
	h.ID = uuid.NewString()
	h.CreatedAt = time.Now().UTC()
	_, err := d.conn.ExecContext(ctx,
		`INSERT INTO highlights (id, job_id, start_block_id, end_block_id, start_pos, end_pos, text, tag, note, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		h.ID, h.JobID, h.StartBlockID, h.EndBlockID, h.StartPos, h.EndPos, h.Text, h.Tag, h.Note,
		h.CreatedAt.Format(time.RFC3339),
	)
	return h, err
}

func (d *db) ListHighlights(ctx context.Context, jobID string) ([]domain.Highlight, error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT id, job_id, start_block_id, end_block_id, start_pos, end_pos, text, tag, note, created_at
		 FROM highlights WHERE job_id = ? ORDER BY created_at ASC`, jobID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.Highlight
	for rows.Next() {
		var h domain.Highlight
		var createdAt string
		if err := rows.Scan(&h.ID, &h.JobID, &h.StartBlockID, &h.EndBlockID, &h.StartPos, &h.EndPos,
			&h.Text, &h.Tag, &h.Note, &createdAt); err != nil {
			return nil, err
		}
		h.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		out = append(out, h)
	}
	return out, rows.Err()
}

func (d *db) DeleteHighlight(ctx context.Context, id string) error {
	res, err := d.conn.ExecContext(ctx, `DELETE FROM highlights WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("highlight %q not found", id)
	}
	return nil
}
