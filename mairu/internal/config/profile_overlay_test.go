package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Profile config.toml should override user-level config when MAIRU_PROFILE is set,
// and lose to project-level config when both are present.
func TestNewViper_ProfileLayer(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MAIRU_HOME", home)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config")) // honored by os.UserConfigDir
	t.Setenv("MAIRU_PROFILE", "research")

	// User config: model = "user-default"
	userDir := filepath.Join(home, ".config", "mairu")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "config.toml"),
		[]byte(`[llm]
model = "user-default"
provider = "user-provider"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Profile config: model = "profile-override" (should win over user)
	profileDir := filepath.Join(home, "profiles", "research")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "config.toml"),
		[]byte(`[llm]
model = "profile-override"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	v := NewViper(t.TempDir())
	if got := v.GetString("llm.model"); got != "profile-override" {
		t.Errorf("llm.model = %q, want %q (profile should beat user)", got, "profile-override")
	}
	if got := v.GetString("llm.provider"); got != "user-provider" {
		t.Errorf("llm.provider = %q, want %q (user value should survive when profile doesn't set it)", got, "user-provider")
	}
}

func TestNewViper_NoProfileFallsBackToUser(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MAIRU_HOME", home)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("MAIRU_PROFILE", "")

	userDir := filepath.Join(home, ".config", "mairu")
	os.MkdirAll(userDir, 0o755)
	os.WriteFile(filepath.Join(userDir, "config.toml"),
		[]byte(`[llm]
model = "user-only"
`), 0o644)

	v := NewViper(t.TempDir())
	if got := v.GetString("llm.model"); got != "user-only" {
		t.Errorf("llm.model = %q, want %q (with no profile, user wins)", got, "user-only")
	}
}
