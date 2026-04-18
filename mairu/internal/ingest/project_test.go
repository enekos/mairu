package ingest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveProject_FindsMairuMarker(t *testing.T) {
	tmp := t.TempDir()
	projectDir := filepath.Join(tmp, "project-a")
	deepDir := filepath.Join(projectDir, "subdir", "deep")
	if err := os.MkdirAll(deepDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Place .mairu as a file at project-a/
	markerPath := filepath.Join(projectDir, ".mairu")
	if err := os.WriteFile(markerPath, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	got := ResolveProject(deepDir)
	if got != "project-a" {
		t.Errorf("expected %q, got %q", "project-a", got)
	}
}

func TestResolveProject_MairuMarkerCanBeDirectory(t *testing.T) {
	tmp := t.TempDir()
	projectDir := filepath.Join(tmp, "project-a")
	deepDir := filepath.Join(projectDir, "subdir", "deep")
	if err := os.MkdirAll(deepDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Place .mairu as a directory at project-a/
	markerPath := filepath.Join(projectDir, ".mairu")
	if err := os.Mkdir(markerPath, 0o755); err != nil {
		t.Fatal(err)
	}

	got := ResolveProject(deepDir)
	if got != "project-a" {
		t.Errorf("expected %q, got %q", "project-a", got)
	}
}

func TestResolveProject_FallsBackToGit(t *testing.T) {
	tmp := t.TempDir()
	projectDir := filepath.Join(tmp, "proj-b")
	srcDir := filepath.Join(projectDir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Place .git directory at proj-b/ — no .mairu anywhere
	gitDir := filepath.Join(projectDir, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	got := ResolveProject(srcDir)
	if got != "proj-b" {
		t.Errorf("expected %q, got %q", "proj-b", got)
	}
}

func TestResolveProject_MairuBeatsGit(t *testing.T) {
	tmp := t.TempDir()
	outerDir := filepath.Join(tmp, "outer")
	innerDir := filepath.Join(outerDir, "inner")
	subdirDir := filepath.Join(innerDir, "subdir")
	if err := os.MkdirAll(subdirDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// .git at outer/ — further from cwd
	gitDir := filepath.Join(outerDir, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// .mairu at outer/inner/ — closer to cwd
	mairuDir := filepath.Join(innerDir, ".mairu")
	if err := os.Mkdir(mairuDir, 0o755); err != nil {
		t.Fatal(err)
	}

	got := ResolveProject(subdirDir)
	if got != "inner" {
		t.Errorf("expected %q, got %q", "inner", got)
	}
}

func TestResolveProject_NoMarkerReturnsBasename(t *testing.T) {
	tmp := t.TempDir()
	lonelyDir := filepath.Join(tmp, "lonely", "dir")
	if err := os.MkdirAll(lonelyDir, 0o755); err != nil {
		t.Fatal(err)
	}

	got := ResolveProject(lonelyDir)
	if got != "dir" {
		t.Errorf("expected %q, got %q", "dir", got)
	}
}

func TestResolveProject_EmptyInput(t *testing.T) {
	got := ResolveProject("")
	if got != "" {
		t.Errorf("expected %q, got %q", "", got)
	}
}

// TestResolveProject_StopsAtRoot passes "/" directly.
// Choice: return "" because the filesystem root has no meaningful project name.
// filepath.Base("/") returns "/" which is not a useful project name, so we
// special-case it in ResolveProject to return "".
func TestResolveProject_StopsAtRoot(t *testing.T) {
	got := ResolveProject("/")
	if got != "" {
		t.Errorf("expected %q, got %q", "", got)
	}
}
