package contextsrv

import (
	"path/filepath"
	"testing"
)

func TestNewApp_NoGeminiKey_LeavesProjectorEmbedderNil(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "context.db")
	app, err := NewApp(Config{
		SQLiteDSN: dsn,
	})
	if err != nil {
		t.Fatalf("NewApp failed: %v", err)
	}
	defer app.repo.Close()

	if app.projector == nil {
		t.Fatal("expected projector to be initialized when sqlite repo exists")
	}
	if app.projector.embedder != nil {
		t.Fatal("expected projector embedder to be nil when GEMINI key is not configured")
	}
}
