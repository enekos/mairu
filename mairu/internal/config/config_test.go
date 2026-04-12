package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindProjectConfig_Found(t *testing.T) {
	// Create temp dir with .git and .mairu.toml
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".git"), 0755)
	os.WriteFile(filepath.Join(root, ".mairu.toml"), []byte("[daemon]\nconcurrency = 4\n"), 0644)

	// Create nested subdir
	sub := filepath.Join(root, "src", "pkg")
	os.MkdirAll(sub, 0755)

	got := FindProjectConfig(sub)
	want := filepath.Join(root, ".mairu.toml")
	if got != want {
		t.Errorf("FindProjectConfig(%q) = %q, want %q", sub, got, want)
	}
}

func TestFindProjectConfig_NotFound(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".git"), 0755)
	// No .mairu.toml

	got := FindProjectConfig(root)
	if got != "" {
		t.Errorf("FindProjectConfig(%q) = %q, want empty", root, got)
	}
}

func TestFindProjectConfig_StopsAtGitBoundary(t *testing.T) {
	// outer/.mairu.toml exists but inner/.git boundary should stop search
	outer := t.TempDir()
	os.WriteFile(filepath.Join(outer, ".mairu.toml"), []byte("[daemon]\n"), 0644)

	inner := filepath.Join(outer, "inner")
	os.MkdirAll(filepath.Join(inner, ".git"), 0755)

	got := FindProjectConfig(inner)
	if got != "" {
		t.Errorf("FindProjectConfig should not cross .git boundary, got %q", got)
	}
}
func TestLoad_Defaults(t *testing.T) {
	// Point viper away from real config files
	t.Setenv("HOME", t.TempDir())

	cfg, err := Load(t.TempDir()) // no .mairu.toml in this dir
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Spot-check key defaults
	if cfg.API.MeiliURL != "http://localhost:7700" {
		t.Errorf("MeiliURL = %q, want http://localhost:7700", cfg.API.MeiliURL)
	}
	if cfg.Daemon.Concurrency != 8 {
		t.Errorf("Daemon.Concurrency = %d, want 8", cfg.Daemon.Concurrency)
	}
	if cfg.Search.Memories.Vector != 0.6 {
		t.Errorf("Search.Memories.Vector = %f, want 0.6", cfg.Search.Memories.Vector)
	}
	if cfg.Server.Port != 8788 {
		t.Errorf("Server.Port = %d, want 8788", cfg.Server.Port)
	}
	if cfg.Embedding.Model != "nomic-embed-text" {
		t.Errorf("Embedding.Model = %q, want nomic-embed-text", cfg.Embedding.Model)
	}
	if cfg.Output.Format != "table" {
		t.Errorf("Output.Format = %q, want table", cfg.Output.Format)
	}
}

func TestLoad_UserConfigOverridesDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDir := filepath.Join(home, ".config", "mairu")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(`
[daemon]
concurrency = 16

[server]
port = 9999
`), 0644)

	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Daemon.Concurrency != 16 {
		t.Errorf("Daemon.Concurrency = %d, want 16 (from user config)", cfg.Daemon.Concurrency)
	}
	if cfg.Server.Port != 9999 {
		t.Errorf("Server.Port = %d, want 9999 (from user config)", cfg.Server.Port)
	}
	// Unset values should still be defaults
	if cfg.Search.Memories.Vector != 0.6 {
		t.Errorf("Search.Memories.Vector = %f, want 0.6 (default)", cfg.Search.Memories.Vector)
	}
}

func TestLoad_ProjectConfigOverridesUser(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDir := filepath.Join(home, ".config", "mairu")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(`
[daemon]
concurrency = 16
`), 0644)

	projectDir := t.TempDir()
	os.MkdirAll(filepath.Join(projectDir, ".git"), 0755)
	os.WriteFile(filepath.Join(projectDir, ".mairu.toml"), []byte(`
[daemon]
concurrency = 4
`), 0644)

	cfg, err := Load(projectDir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Daemon.Concurrency != 4 {
		t.Errorf("Daemon.Concurrency = %d, want 4 (from project config)", cfg.Daemon.Concurrency)
	}
}

func TestLoad_EnvOverridesAll(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("MAIRU_DAEMON_CONCURRENCY", "32")

	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Daemon.Concurrency != 32 {
		t.Errorf("Daemon.Concurrency = %d, want 32 (from env)", cfg.Daemon.Concurrency)
	}
}

func TestLoad_LegacyEnvAliases(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GEMINI_API_KEY", "test-key-123")
	t.Setenv("MEILI_URL", "http://custom:7700")

	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.API.GeminiAPIKey != "test-key-123" {
		t.Errorf("GeminiAPIKey = %q, want test-key-123", cfg.API.GeminiAPIKey)
	}
	if cfg.API.MeiliURL != "http://custom:7700" {
		t.Errorf("MeiliURL = %q, want http://custom:7700", cfg.API.MeiliURL)
	}
}
