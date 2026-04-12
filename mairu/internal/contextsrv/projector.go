package contextsrv

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type OutboxJob struct {
	ID         int64
	EntityType string
	EntityID   string
	OpType     string
	Payload    []byte
	RetryCount int
}

func (r *SQLiteRepository) PullOutboxBatch(ctx context.Context, limit int) ([]OutboxJob, error) {
	if limit <= 0 {
		limit = 50
	}
	now := time.Now().UTC()
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, entity_type, entity_id, op_type, payload, retry_count
		FROM search_outbox
		WHERE status = 'pending' AND next_attempt_at <= $1
		ORDER BY id ASC
		LIMIT $2
	`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OutboxJob
	for rows.Next() {
		var j OutboxJob
		var payloadStr string
		if err := rows.Scan(&j.ID, &j.EntityType, &j.EntityID, &j.OpType, &payloadStr, &j.RetryCount); err != nil {
			return nil, err
		}
		j.Payload = []byte(payloadStr)
		out = append(out, j)
	}
	return out, rows.Err()
}

func (r *SQLiteRepository) MarkOutboxDone(ctx context.Context, id int64) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE search_outbox
		SET status = 'done', updated_at = $2
		WHERE id = $1
	`, id, now)
	return err
}

func (r *SQLiteRepository) MarkOutboxFailed(ctx context.Context, id int64, retryCount int, lastErr string) error {
	nextAttempt := time.Now().UTC().Add(time.Duration(1<<min(6, retryCount)) * time.Second)
	status := "pending"
	if retryCount >= 8 {
		status = "dead_letter"
	}
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE search_outbox
		SET status = $2,
			retry_count = $3,
			last_error = $4,
			next_attempt_at = $5,
			updated_at = $6
		WHERE id = $1
	`, id, status, retryCount, truncate(lastErr, 800), nextAttempt, now)
	return err
}

type Embedder interface {
	GetEmbedding(ctx context.Context, text string) ([]float32, error)
	GetEmbeddingDimension() int
}

type Projector struct {
	repo     *SQLiteRepository
	indexer  *MeiliIndexer
	embedder Embedder
}

func NewProjector(repo *SQLiteRepository, indexer *MeiliIndexer, embedder Embedder) *Projector {
	return &Projector{repo: repo, indexer: indexer, embedder: embedder}
}

func (p *Projector) RunOnce(ctx context.Context, batchSize int) (int, error) {
	jobs, err := p.repo.PullOutboxBatch(ctx, batchSize)
	if err != nil {
		return 0, err
	}
	done := 0
	for _, job := range jobs {
		if err := ctx.Err(); err != nil {
			break
		}
		if err := p.processJob(ctx, job); err != nil {
			if ctx.Err() != nil {
				// The context expired during processing. Don't mark the job as failed.
				break
			}
			_ = p.repo.MarkOutboxFailed(context.WithoutCancel(ctx), job.ID, job.RetryCount+1, err.Error())
			continue
		}
		if err := p.repo.MarkOutboxDone(context.WithoutCancel(ctx), job.ID); err != nil {
			return done, err
		}
		done++
	}
	return done, nil
}

func (p *Projector) processJob(ctx context.Context, job OutboxJob) error {
	switch job.OpType {
	case "delete":
		return p.indexer.Delete(job.EntityType, job.EntityID)
	case "upsert":
		payload := map[string]any{}
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return fmt.Errorf("bad payload: %w", err)
		}

		// Extract text to embed
		var textToEmbed string
		switch job.EntityType {
		case "memory":
			if content, ok := payload["content"].(string); ok {
				textToEmbed = content
			}
		case "skill":
			name, _ := payload["name"].(string)
			desc, _ := payload["description"].(string)
			textToEmbed = name + ": " + desc
		case "context_node":
			name, _ := payload["name"].(string)
			abstract, _ := payload["abstract"].(string)
			content, _ := payload["content"].(string)
			textToEmbed = name + "\n" + abstract + "\n" + content
		case "bash_history":
			cmd, _ := payload["command"].(string)
			out, _ := payload["output"].(string)
			textToEmbed = cmd + "\n" + truncate(out, 4000)
		}

		if textToEmbed != "" && p.embedder != nil {
			emb, err := p.embedder.GetEmbedding(ctx, textToEmbed)
			if err != nil {
				return fmt.Errorf("embedding failed: %w", err)
			}
			payload["_vectors"] = map[string]any{
				"default": emb,
			}
		}

		return p.indexer.Upsert(job.EntityType, payload)
	default:
		return fmt.Errorf("unsupported op_type %q", job.OpType)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func truncate(input string, n int) string {
	if len(input) <= n {
		return input
	}
	return input[:n]
}
