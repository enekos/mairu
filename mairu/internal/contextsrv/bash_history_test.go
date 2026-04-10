package contextsrv

import (
	"context"
	"testing"
)

func TestBashHistoryStats(t *testing.T) {
	dbPath := t.TempDir() + "/test_bash.db"
	repo, err := NewSQLiteRepository("file:" + dbPath + "?cache=shared&mode=rwc")
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()

	// Insert some history
	err = repo.InsertBashHistory(ctx, "proj1", "go test ./...", 0, 100, "ok")
	if err != nil {
		t.Fatalf("failed to insert history: %v", err)
	}

	repo.InsertBashHistory(ctx, "proj1", "go test ./...", 1, 200, "fail")

	repo.InsertBashHistory(ctx, "proj1", "npm run dev", 0, 50, "starting")

	stats, err := repo.GetBashStats(ctx, "proj1", 10)
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}

	if len(stats) != 2 {
		t.Fatalf("expected 2 grouped commands, got %d", len(stats))
	}

	// Should be ordered by count DESC
	if stats[0].Command != "go test ./..." {
		t.Errorf("expected go test ./... to be first, got %s", stats[0].Command)
	}
	if stats[0].Count != 2 {
		t.Errorf("expected count 2, got %d", stats[0].Count)
	}
	if stats[0].SuccessRatio != 0.5 {
		t.Errorf("expected success ratio 0.5, got %f", stats[0].SuccessRatio)
	}
	if stats[0].AvgDuration != 150 {
		t.Errorf("expected avg duration 150, got %d", stats[0].AvgDuration)
	}

	if stats[1].Command != "npm run dev" {
		t.Errorf("expected npm run dev to be second, got %s", stats[1].Command)
	}
	if stats[1].Count != 1 {
		t.Errorf("expected count 1, got %d", stats[1].Count)
	}
	if stats[1].SuccessRatio != 1.0 {
		t.Errorf("expected success ratio 1.0, got %f", stats[1].SuccessRatio)
	}
}
