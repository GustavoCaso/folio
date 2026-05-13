package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func (r *Repository) CreateHighlight(ctx context.Context, h Highlight) (Highlight, error) {
	h.ID = uuid.NewString()
	h.CreatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO highlights (id, job_id, start_block_id, end_block_id, start_pos, end_pos, text, tag, note, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		h.ID, h.JobID, h.StartBlockID, h.EndBlockID, h.StartPos, h.EndPos, h.Text, h.Tag, h.Note,
		h.CreatedAt.Format(time.RFC3339),
	)
	return h, err
}

func (r *Repository) ListHighlights(ctx context.Context, jobID string) ([]Highlight, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, job_id, start_block_id, end_block_id, start_pos, end_pos, text, tag, note, created_at
		 FROM highlights WHERE job_id = ? ORDER BY created_at ASC`, jobID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []Highlight
	for rows.Next() {
		var h Highlight
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

func (r *Repository) DeleteHighlight(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM highlights WHERE id = ?`, id)
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
