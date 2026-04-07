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
