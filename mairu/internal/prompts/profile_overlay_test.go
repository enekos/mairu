package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetForProject_ProfileOverridesUser(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MAIRU_HOME", home)
	t.Setenv("HOME", home)
	t.Setenv("MAIRU_PROFILE", "research")

	// User-global override
	userPath := filepath.Join(home, ".config", "mairu", "prompts", "session_summarize.md")
	if err := os.MkdirAll(filepath.Dir(userPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userPath, []byte("USER OVERRIDE"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Profile override
	profilePath := filepath.Join(home, "profiles", "research", "prompts", "session_summarize.md")
	if err := os.MkdirAll(filepath.Dir(profilePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profilePath, []byte("PROFILE OVERRIDE"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := GetForProject("session_summarize", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "PROFILE OVERRIDE") {
		t.Errorf("expected profile override to win, got: %q", out)
	}
}

func TestGetForProject_AgentSystemAtProfileRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MAIRU_HOME", home)
	t.Setenv("HOME", home)
	t.Setenv("MAIRU_PROFILE", "research")

	// hermes-style: agent_system.md at profile root (not under prompts/)
	rootPath := filepath.Join(home, "profiles", "research", "agent_system.md")
	if err := os.MkdirAll(filepath.Dir(rootPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rootPath, []byte("CUSTOM AGENT PROMPT"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := GetForProject("agent_system", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "CUSTOM AGENT PROMPT") {
		t.Errorf("expected root-level agent_system.md to win, got: %q", out)
	}
}

func TestGetForProject_NoProfileFallsThroughToEmbedded(t *testing.T) {
	t.Setenv("MAIRU_PROFILE", "")
	t.Setenv("MAIRU_HOME", t.TempDir()) // empty profile dir
	out, err := GetForProject("agent_system", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "You are Mairu") {
		t.Errorf("expected embedded prompt to render when no profile/user/project override exists, got:\n%s", out)
	}
}
