package contextsrv

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

func (r *PostgresRepository) CreateMemory(ctx context.Context, input MemoryCreateInput) (Memory, error) {
	id := fmt.Sprintf("mem_%d", time.Now().UnixNano())
	now := time.Now().UTC()
	reasonsJSON, _ := json.Marshal(input.ModerationReasons)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Memory{}, err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO memories (id, project, content, category, owner, importance, metadata, moderation_status, moderation_reasons, review_required, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8,$9::jsonb,$10,$11,$11)
	`, id, input.Project, input.Content, input.Category, input.Owner, input.Importance, jsonString(input.Metadata, `{}`), input.ModerationStatus, string(reasonsJSON), input.ReviewRequired, now)
	if err != nil {
		return Memory{}, err
	}
	if err := r.insertModerationEventTx(ctx, tx, "memory", id, input.Project, input.ModerationStatus, input.ModerationReasons, input.ReviewRequired); err != nil {
		return Memory{}, err
	}
	if err := r.insertAuditTx(ctx, tx, "memory", id, "create", "contextsrv", map[string]any{"project": input.Project}); err != nil {
		return Memory{}, err
	}
	if err := tx.Commit(); err != nil {
		return Memory{}, err
	}
	return Memory{
		ID:                id,
		Project:           input.Project,
		Content:           input.Content,
		Category:          input.Category,
		Owner:             input.Owner,
		Importance:        input.Importance,
		ModerationStatus:  input.ModerationStatus,
		ModerationReasons: input.ModerationReasons,
		ReviewRequired:    input.ReviewRequired,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

func (r *PostgresRepository) ListMemories(ctx context.Context, project string, limit int) ([]Memory, error) {
	if limit <= 0 {
		limit = 200
	}
	q := `SELECT id, project, content, category, owner, importance, moderation_status, moderation_reasons, review_required, created_at, updated_at FROM memories`
	var args []any
	if project != "" {
		q += ` WHERE project = $1`
		args = append(args, project)
	}
	q += ` ORDER BY created_at DESC LIMIT `
	q += fmt.Sprintf("%d", limit)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Memory
	for rows.Next() {
		var m Memory
		var reasonsRaw []byte
		if err := rows.Scan(&m.ID, &m.Project, &m.Content, &m.Category, &m.Owner, &m.Importance, &m.ModerationStatus, &reasonsRaw, &m.ReviewRequired, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(reasonsRaw, &m.ModerationReasons)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) UpdateMemory(ctx context.Context, input MemoryUpdateInput) (Memory, error) {
	if input.ID == "" {
		return Memory{}, fmt.Errorf("id is required")
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE memories
		SET content = COALESCE(NULLIF($2, ''), content),
		    category = COALESCE(NULLIF($3, ''), category),
		    owner = COALESCE(NULLIF($4, ''), owner),
		    importance = CASE WHEN $5 > 0 THEN $5 ELSE importance END,
		    updated_at = NOW()
		WHERE id = $1
	`, input.ID, input.Content, input.Category, input.Owner, input.Importance)
	if err != nil {
		return Memory{}, err
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, project, content, category, owner, importance, moderation_status, moderation_reasons, review_required, created_at, updated_at
		FROM memories WHERE id = $1
	`, input.ID)
	var m Memory
	var reasonsRaw []byte
	if err := row.Scan(&m.ID, &m.Project, &m.Content, &m.Category, &m.Owner, &m.Importance, &m.ModerationStatus, &reasonsRaw, &m.ReviewRequired, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return Memory{}, err
	}
	_ = json.Unmarshal(reasonsRaw, &m.ModerationReasons)
	return m, nil
}

func (r *PostgresRepository) DeleteMemory(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM memories WHERE id = $1`, id)
	return err
}
