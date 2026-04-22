package contextsrv

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
	db.SetMaxOpenConns(4) // WAL mode allows concurrent readers; writers still serialize automatically
	db.SetMaxIdleConns(4)
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

func (r *SQLiteRepository) InsertBashHistory(ctx context.Context, project string, command string, exitCode int, durationMs int, output string) error {
	id := fmt.Sprintf("bash_%d", time.Now().UnixNano())
	h := BashHistory{
		ID:            id,
		Project:       project,
		Command:       command,
		ExitCode:      exitCode,
		DurationMs:    durationMs,
		Output:        output,
		Importance:    5,
		FeedbackCount: 0,
		CreatedAt:     time.Now().UTC(),
	}

	query := `INSERT INTO bash_history (id, project, command, exit_code, duration_ms, output, importance, feedback_count, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query,
		h.ID,
		h.Project,
		h.Command,
		h.ExitCode,
		h.DurationMs,
		h.Output,
		h.Importance,
		h.FeedbackCount,
		h.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return err
	}

	// Also enqueue to search_outbox for indexing
	return r.EnqueueOutbox(ctx, "bash_history", h.ID, "upsert", h)
}

func (r *SQLiteRepository) GetBashHistory(ctx context.Context, id string) (BashHistory, error) {
	var h BashHistory
	var createdAtStr string
	query := `SELECT id, project, command, exit_code, duration_ms, output, importance, feedback_count, created_at FROM bash_history WHERE id = ?`
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&h.ID, &h.Project, &h.Command, &h.ExitCode, &h.DurationMs, &h.Output, &h.Importance, &h.FeedbackCount, &createdAtStr,
	)
	if err == sql.ErrNoRows {
		return h, fmt.Errorf("bash history not found: %s", id)
	} else if err != nil {
		return h, err
	}
	h.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	return h, nil
}

func (r *SQLiteRepository) UpdateBashHistory(ctx context.Context, h BashHistory) error {
	query := `UPDATE bash_history SET importance = ?, feedback_count = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, h.Importance, h.FeedbackCount, h.ID)
	if err != nil {
		return err
	}
	return r.EnqueueOutbox(ctx, "bash_history", h.ID, "upsert", h)
}

func (r *SQLiteRepository) IncrementBashHistoryFeedbackCount(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE bash_history SET feedback_count = feedback_count + 1 WHERE id = ?`, id)
	return err
}

type BashStat struct {
	Command      string  `json:"command"`
	Count        int     `json:"count"`
	AvgDuration  int     `json:"avg_duration"`
	SuccessRatio float64 `json:"success_ratio"`
}

func (r *SQLiteRepository) GetBashStats(ctx context.Context, project string, limit int) ([]BashStat, error) {
	// Group by the full command
	query := `
		SELECT 
			command as cmd,
			COUNT(*) as cnt,
			CAST(AVG(duration_ms) AS INTEGER) as avg_dur,
			CAST(SUM(CASE WHEN exit_code = 0 THEN 1 ELSE 0 END) AS FLOAT) / COUNT(*) as success_ratio
		FROM bash_history
		WHERE project = ?
		GROUP BY cmd
		ORDER BY cnt DESC
		LIMIT ?`

	rows, err := r.db.QueryContext(ctx, query, project, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []BashStat
	for rows.Next() {
		var s BashStat
		if err := rows.Scan(&s.Command, &s.Count, &s.AvgDuration, &s.SuccessRatio); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}
