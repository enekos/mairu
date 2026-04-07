package contextsrv

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// unmarshalJSONField decodes a nullable JSON column into dst.
// Empty/null bytes are treated as a no-op rather than an error.
func unmarshalJSONField(raw []byte, dst any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dst)
}

type SQLiteRepository struct {
	db *sql.DB
}

func NewSQLiteRepository(dsn string) (*SQLiteRepository, error) {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite works best with 1 writer to avoid BUSY errors if not using WAL properly, but let's stick to 1 for safety or use WAL mode
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(30 * time.Minute)

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return nil, err
	}

	repo := &SQLiteRepository{db: db}
	if err := repo.Migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return repo, nil
}

func (r *SQLiteRepository) Close() error {
	return r.db.Close()
}

func (r *SQLiteRepository) Migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS memories (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL,
			category TEXT NOT NULL,
			owner TEXT NOT NULL,
			importance INT NOT NULL,
			metadata TEXT NOT NULL DEFAULT '{}',
			moderation_status TEXT NOT NULL,
			moderation_reasons TEXT NOT NULL DEFAULT '[]',
			review_required BOOLEAN NOT NULL DEFAULT 0,
			version BIGINT NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS skills (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL,
			description TEXT NOT NULL,
			metadata TEXT NOT NULL DEFAULT '{}',
			moderation_status TEXT NOT NULL,
			moderation_reasons TEXT NOT NULL DEFAULT '[]',
			review_required BOOLEAN NOT NULL DEFAULT 0,
			version BIGINT NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS context_nodes (
			uri TEXT PRIMARY KEY,
			project TEXT NOT NULL DEFAULT '',
			parent_uri TEXT NULL,
			name TEXT NOT NULL,
			abstract TEXT NOT NULL,
			overview TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL DEFAULT '',
			metadata TEXT NOT NULL DEFAULT '{}',
			moderation_status TEXT NOT NULL,
			moderation_reasons TEXT NOT NULL DEFAULT '[]',
			review_required BOOLEAN NOT NULL DEFAULT 0,
			version BIGINT NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS moderation_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			project TEXT NOT NULL DEFAULT '',
			decision TEXT NOT NULL,
			reasons TEXT NOT NULL DEFAULT '[]',
			review_status TEXT NOT NULL DEFAULT 'pending',
			reviewer_decision TEXT NOT NULL DEFAULT '',
			reviewer TEXT NOT NULL DEFAULT '',
			notes TEXT NOT NULL DEFAULT '',
			policy_version TEXT NOT NULL DEFAULT 'v1',
			review_required BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			reviewed_at DATETIME NULL
		)`,
		`CREATE TABLE IF NOT EXISTS audit_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			action TEXT NOT NULL,
			actor TEXT NOT NULL DEFAULT 'system',
			details TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS search_outbox (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			op_type TEXT NOT NULL,
			payload TEXT NOT NULL,
			payload_hash TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending',
			retry_count INT NOT NULL DEFAULT 0,
			last_error TEXT NOT NULL DEFAULT '',
			next_attempt_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
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

	// Retrieval tracking columns — ALTER TABLE is not idempotent in SQLite,
	// so we ignore "duplicate column name" errors for each.
	alterStmts := []string{
		`ALTER TABLE memories ADD COLUMN retrieval_count INT NOT NULL DEFAULT 0`,
		`ALTER TABLE memories ADD COLUMN feedback_count INT NOT NULL DEFAULT 0`,
		`ALTER TABLE memories ADD COLUMN last_retrieved_at DATETIME NULL`,
	}
	for _, stmt := range alterStmts {
		if _, err := r.db.ExecContext(ctx, stmt); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return err
		}
	}

	return nil
}
