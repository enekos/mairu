package contextsrv

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(dsn string) (*PostgresRepository, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(12)
	db.SetMaxIdleConns(12)
	db.SetConnMaxLifetime(30 * time.Minute)

	repo := &PostgresRepository{db: db}
	if err := repo.Migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return repo, nil
}

func (r *PostgresRepository) Close() error {
	return r.db.Close()
}

func (r *PostgresRepository) Migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS memories (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL,
			category TEXT NOT NULL,
			owner TEXT NOT NULL,
			importance INT NOT NULL,
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
			moderation_status TEXT NOT NULL,
			moderation_reasons JSONB NOT NULL DEFAULT '[]'::jsonb,
			review_required BOOLEAN NOT NULL DEFAULT false,
			version BIGINT NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS skills (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL,
			description TEXT NOT NULL,
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
			moderation_status TEXT NOT NULL,
			moderation_reasons JSONB NOT NULL DEFAULT '[]'::jsonb,
			review_required BOOLEAN NOT NULL DEFAULT false,
			version BIGINT NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS context_nodes (
			uri TEXT PRIMARY KEY,
			project TEXT NOT NULL DEFAULT '',
			parent_uri TEXT NULL,
			name TEXT NOT NULL,
			abstract TEXT NOT NULL,
			overview TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL DEFAULT '',
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
			moderation_status TEXT NOT NULL,
			moderation_reasons JSONB NOT NULL DEFAULT '[]'::jsonb,
			review_required BOOLEAN NOT NULL DEFAULT false,
			version BIGINT NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS moderation_events (
			id BIGSERIAL PRIMARY KEY,
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			project TEXT NOT NULL DEFAULT '',
			decision TEXT NOT NULL,
			reasons JSONB NOT NULL DEFAULT '[]'::jsonb,
			review_status TEXT NOT NULL DEFAULT 'pending',
			reviewer_decision TEXT NOT NULL DEFAULT '',
			reviewer TEXT NOT NULL DEFAULT '',
			notes TEXT NOT NULL DEFAULT '',
			policy_version TEXT NOT NULL DEFAULT 'v1',
			review_required BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			reviewed_at TIMESTAMPTZ NULL
		)`,
		`CREATE TABLE IF NOT EXISTS audit_log (
			id BIGSERIAL PRIMARY KEY,
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			action TEXT NOT NULL,
			actor TEXT NOT NULL DEFAULT 'system',
			details JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS search_outbox (
			id BIGSERIAL PRIMARY KEY,
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			op_type TEXT NOT NULL,
			payload JSONB NOT NULL,
			payload_hash TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending',
			retry_count INT NOT NULL DEFAULT 0,
			last_error TEXT NOT NULL DEFAULT '',
			next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_memories_project_created ON memories(project, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_skills_project_created ON skills(project, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_nodes_project_parent ON context_nodes(project, parent_uri)`,
		`CREATE INDEX IF NOT EXISTS idx_moderation_pending ON moderation_events(review_status, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_outbox_pending ON search_outbox(status, next_attempt_at, id)`,
	}
	for _, stmt := range stmts {
		if _, err := r.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
