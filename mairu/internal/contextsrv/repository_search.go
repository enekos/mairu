package contextsrv

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (r *PostgresRepository) SearchText(ctx context.Context, opts SearchOptions) (map[string]any, error) {
	project := opts.Project
	store := normalizeStoreName(opts.Store)
	topK := opts.TopK
	if topK <= 0 {
		topK = 10
	}
	candidateLimit := topK * 5
	if candidateLimit < 25 {
		candidateLimit = 25
	}

	queryTokens := tokenizeForSearch(opts.Query)
	if len(queryTokens) == 0 {
		return map[string]any{
			StoreMemories:     []map[string]any{},
			StoreSkills:       []map[string]any{},
			StoreContextNodes: []map[string]any{},
		}, nil
	}

	q := "%" + strings.ToLower(opts.Query) + "%"
	out := map[string]any{}

	if store == StoreAll || store == StoreMemories {
		rows, err := r.db.QueryContext(ctx, `
			SELECT id, content, importance, created_at
			FROM memories
			WHERE ($1 = '' OR project = $1) AND LOWER(content) LIKE $2
			ORDER BY created_at DESC LIMIT $3
		`, project, q, candidateLimit)
		if err != nil {
			return nil, err
		}
		items := []scoredDoc{}
		for rows.Next() {
			var id, content string
			var importance int
			var createdAt time.Time
			if err := rows.Scan(&id, &content, &importance, &createdAt); err != nil {
				rows.Close()
				return nil, err
			}
			fields := map[string]string{"content": content}
			score := scoreHybrid(fields, queryTokens, createdAt, importance, opts, defaultMemoryWeights())
			doc := map[string]any{"id": id, "content": content, "_score": score}
			if opts.Highlight {
				if h := highlightsForFields(fields, queryTokens); len(h) > 0 {
					doc["_highlight"] = h
				}
			}
			items = append(items, scoredDoc{score: score, doc: doc})
		}
		rows.Close()
		out[StoreMemories] = finalizeScoredDocs(items, topK, opts.MinScore)
	}

	if store == StoreAll || store == StoreSkills {
		rows, err := r.db.QueryContext(ctx, `
			SELECT id, name, description, created_at
			FROM skills
			WHERE ($1 = '' OR project = $1) AND (LOWER(name) LIKE $2 OR LOWER(description) LIKE $2)
			ORDER BY created_at DESC LIMIT $3
		`, project, q, candidateLimit)
		if err != nil {
			return nil, err
		}
		items := []scoredDoc{}
		for rows.Next() {
			var id, name, description string
			var createdAt time.Time
			if err := rows.Scan(&id, &name, &description, &createdAt); err != nil {
				rows.Close()
				return nil, err
			}
			fields := map[string]string{
				"name":        name,
				"description": description,
			}
			score := scoreHybrid(fields, queryTokens, createdAt, 0, opts, defaultSkillWeights())
			doc := map[string]any{"id": id, "name": name, "description": description, "_score": score}
			if opts.Highlight {
				if h := highlightsForFields(fields, queryTokens); len(h) > 0 {
					doc["_highlight"] = h
				}
			}
			items = append(items, scoredDoc{score: score, doc: doc})
		}
		rows.Close()
		out[StoreSkills] = finalizeScoredDocs(items, topK, opts.MinScore)
	}

	if store == StoreAll || store == StoreContext {
		rows, err := r.db.QueryContext(ctx, `
			SELECT uri, name, abstract, content, created_at
			FROM context_nodes
			WHERE ($1 = '' OR project = $1) AND (LOWER(name) LIKE $2 OR LOWER(abstract) LIKE $2 OR LOWER(content) LIKE $2)
			ORDER BY created_at DESC LIMIT $3
		`, project, q, candidateLimit)
		if err != nil {
			return nil, err
		}
		items := []scoredDoc{}
		for rows.Next() {
			var uri, name, abstract, content string
			var createdAt time.Time
			if err := rows.Scan(&uri, &name, &abstract, &content, &createdAt); err != nil {
				rows.Close()
				return nil, err
			}
			fields := map[string]string{
				"name":     name,
				"abstract": abstract,
				"content":  content,
			}
			score := scoreHybrid(fields, queryTokens, createdAt, 0, opts, defaultContextWeights())
			doc := map[string]any{"uri": uri, "name": name, "abstract": abstract, "_score": score}
			if opts.Highlight {
				if h := highlightsForFields(fields, queryTokens); len(h) > 0 {
					doc["_highlight"] = h
				}
			}
			items = append(items, scoredDoc{score: score, doc: doc})
		}
		rows.Close()
		out[StoreContextNodes] = finalizeScoredDocs(items, topK, opts.MinScore)
	}
	return out, nil
}

func (r *PostgresRepository) ListModerationQueue(ctx context.Context, limit int) ([]ModerationEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, entity_type, entity_id, project, decision, reasons, review_status, reviewer_decision, review_required, policy_version, created_at, COALESCE(reviewed_at, '0001-01-01'::timestamptz), reviewer
		FROM moderation_events
		WHERE review_status = 'pending'
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ModerationEvent
	for rows.Next() {
		var ev ModerationEvent
		var reasonsRaw []byte
		if err := rows.Scan(&ev.ID, &ev.EntityType, &ev.EntityID, &ev.Project, &ev.Decision, &reasonsRaw, &ev.ReviewStatus, &ev.ReviewerDecision, &ev.ReviewRequired, &ev.PolicyVersion, &ev.CreatedAt, &ev.ReviewedAt, &ev.Reviewer); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(reasonsRaw, &ev.Reasons)
		out = append(out, ev)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) ReviewModeration(ctx context.Context, input ModerationReviewInput) error {
	if input.EventID == 0 {
		return fmt.Errorf("event_id is required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `
		UPDATE moderation_events
		SET review_status = 'reviewed',
			reviewer_decision = $2,
			reviewer = $3,
			notes = $4,
			reviewed_at = NOW()
		WHERE id = $1
	`, input.EventID, input.Decision, input.Reviewer, input.Notes)
	if err != nil {
		return err
	}
	if err := r.insertAuditTx(ctx, tx, "moderation_event", fmt.Sprintf("%d", input.EventID), "review", input.Reviewer, map[string]any{"decision": input.Decision}); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *PostgresRepository) EnqueueOutbox(ctx context.Context, entityType, entityID, opType string, payload any) error {
	payloadBytes, _ := json.Marshal(payload)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO search_outbox (entity_type, entity_id, op_type, payload, payload_hash, status, retry_count, next_attempt_at, updated_at)
		VALUES ($1, $2, $3, $4::jsonb, md5($4::text), 'pending', 0, NOW(), NOW())
	`, entityType, entityID, opType, string(payloadBytes))
	return err
}

func (r *PostgresRepository) insertModerationEventTx(ctx context.Context, tx *sql.Tx, entityType, entityID, project, decision string, reasons []string, reviewRequired bool) error {
	reasonsJSON, _ := json.Marshal(reasons)
	_, err := tx.ExecContext(ctx, `
		INSERT INTO moderation_events (entity_type, entity_id, project, decision, reasons, review_status, review_required, policy_version)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, 'v1')
	`, entityType, entityID, project, decision, string(reasonsJSON), reviewState(reviewRequired), reviewRequired)
	return err
}

func (r *PostgresRepository) insertAuditTx(ctx context.Context, tx *sql.Tx, entityType, entityID, action, actor string, details map[string]any) error {
	detailsJSON, _ := json.Marshal(details)
	_, err := tx.ExecContext(ctx, `
		INSERT INTO audit_log (entity_type, entity_id, action, actor, details)
		VALUES ($1, $2, $3, $4, $5::jsonb)
	`, entityType, entityID, action, actor, string(detailsJSON))
	return err
}

func reviewState(reviewRequired bool) string {
	if reviewRequired {
		return "pending"
	}
	return "auto_approved"
}

func jsonString(raw json.RawMessage, fallback string) string {
	if len(raw) == 0 {
		return fallback
	}
	return string(raw)
}
