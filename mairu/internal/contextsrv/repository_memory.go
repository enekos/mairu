package contextsrv

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

func (r *SQLiteRepository) CreateMemory(ctx context.Context, input MemoryCreateInput) (Memory, error) {
	id := fmt.Sprintf("mem_%d", time.Now().UnixNano())
	now := time.Now().UTC()
	reasonsJSON, err := json.Marshal(input.ModerationReasons)
	if err != nil {
		return Memory{}, fmt.Errorf("marshal moderation reasons: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Memory{}, err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO memories (id, project, content, category, owner, importance, metadata, moderation_status, moderation_reasons, review_required, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$11)
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

func (r *SQLiteRepository) ListMemories(ctx context.Context, project string, limit int) ([]Memory, error) {
	if limit <= 0 {
		limit = 200
	}
	q := `SELECT id, project, content, category, owner, importance, retrieval_count, feedback_count, last_retrieved_at, moderation_status, moderation_reasons, review_required, created_at, updated_at FROM memories`
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
		if err := rows.Scan(&m.ID, &m.Project, &m.Content, &m.Category, &m.Owner, &m.Importance, &m.RetrievalCount, &m.FeedbackCount, &m.LastRetrievedAt, &m.ModerationStatus, &reasonsRaw, &m.ReviewRequired, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		if err := unmarshalJSONField(reasonsRaw, &m.ModerationReasons); err != nil {
			return nil, fmt.Errorf("unmarshal moderation_reasons for memory %s: %w", m.ID, err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *SQLiteRepository) GetMemory(ctx context.Context, id string) (Memory, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, project, content, category, owner, importance, retrieval_count, feedback_count, last_retrieved_at, moderation_status, moderation_reasons, review_required, created_at, updated_at
		FROM memories WHERE id = $1
	`, id)
	var m Memory
	var reasonsRaw []byte
	if err := row.Scan(&m.ID, &m.Project, &m.Content, &m.Category, &m.Owner, &m.Importance, &m.RetrievalCount, &m.FeedbackCount, &m.LastRetrievedAt, &m.ModerationStatus, &reasonsRaw, &m.ReviewRequired, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return Memory{}, err
	}
	if err := unmarshalJSONField(reasonsRaw, &m.ModerationReasons); err != nil {
		return Memory{}, fmt.Errorf("unmarshal moderation_reasons for memory %s: %w", m.ID, err)
	}
	return m, nil
}

func (r *SQLiteRepository) UpdateMemory(ctx context.Context, input MemoryUpdateInput) (Memory, error) {
	if input.ID == "" {
		return Memory{}, fmt.Errorf("id is required")
	}

	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE memories
		SET content = COALESCE(NULLIF($2, ''), content),
		    category = COALESCE(NULLIF($3, ''), category),
		    owner = COALESCE(NULLIF($4, ''), owner),
		    importance = CASE WHEN $5 > 0 THEN $5 ELSE importance END,
		    updated_at = $6
		WHERE id = $1
	`, input.ID, input.Content, input.Category, input.Owner, input.Importance, now)
	if err != nil {
		return Memory{}, err
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, project, content, category, owner, importance, retrieval_count, feedback_count, last_retrieved_at, moderation_status, moderation_reasons, review_required, created_at, updated_at
		FROM memories WHERE id = $1
	`, input.ID)
	var m Memory
	var reasonsRaw []byte
	if err := row.Scan(&m.ID, &m.Project, &m.Content, &m.Category, &m.Owner, &m.Importance, &m.RetrievalCount, &m.FeedbackCount, &m.LastRetrievedAt, &m.ModerationStatus, &reasonsRaw, &m.ReviewRequired, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return Memory{}, err
	}
	if err := unmarshalJSONField(reasonsRaw, &m.ModerationReasons); err != nil {
		return Memory{}, fmt.Errorf("unmarshal moderation_reasons for memory %s: %w", m.ID, err)
	}
	return m, nil
}

func (r *SQLiteRepository) DeleteMemory(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM memories WHERE id = $1`, id)
	return err
}

// implicitDecayBaseline is the importance level unrewarded memories drift toward.
const implicitDecayBaseline = 3

// implicitDecayAlpha is the learning rate for each decay step.
const implicitDecayAlpha = 0.1

// implicitDecayInterval is how many unrewarded retrievals trigger a decay step.
// "Unrewarded retrievals" = retrieval_count - feedback_count * implicitDecayInterval.
const implicitDecayInterval = 10

// RecordRetrievals bumps retrieval_count and last_retrieved_at for each memory id.
// When a memory accumulates implicitDecayInterval retrievals without feedback, its
// importance is nudged toward implicitDecayBaseline (only downward — low-importance
// memories are left alone).
//
// The decay is expressed as a single atomic SQL UPDATE to avoid read-modify-write
// races and keep the operation lightweight.
func (r *SQLiteRepository) RecordRetrievals(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	now := time.Now().UTC()

	// Decay formula (integer arithmetic, applied when a new unrewarded interval is crossed):
	//   unrewarded = retrieval_count + 1 - feedback_count * INTERVAL
	//   fires when unrewarded > 0 AND unrewarded % INTERVAL == 0 AND importance > BASELINE
	//   new_importance = ROUND(importance + ALPHA * (BASELINE - importance))
	//
	// ROUND via CAST(x + 0.5 AS INT) is equivalent to floor(x + 0.5).
	const query = `
		UPDATE memories
		SET
			retrieval_count    = retrieval_count + 1,
			last_retrieved_at  = $2,
			importance = CASE
				WHEN importance > $3
				 AND (retrieval_count + 1 - feedback_count * $4) > 0
				 AND (retrieval_count + 1 - feedback_count * $4) % $4 = 0
				THEN MAX(1, CAST(importance + $5 * ($3 - importance) + 0.5 AS INT))
				ELSE importance
			END
		WHERE id = $1`

	for _, id := range ids {
		if _, err := r.db.ExecContext(ctx, query,
			id,
			now,
			implicitDecayBaseline,
			implicitDecayInterval,
			implicitDecayAlpha,
		); err != nil {
			return err
		}
	}
	return nil
}

// IncrementFeedbackCount records that explicit feedback was given for a memory.
func (r *SQLiteRepository) IncrementFeedbackCount(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE memories SET feedback_count = feedback_count + 1 WHERE id = $1
	`, id)
	return err
}
