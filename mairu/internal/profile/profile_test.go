package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withMairuHome points MAIRU_HOME at a fresh temp dir for the duration of
// the test, so Dir/Root/List don't touch the real ~/.config/mairu.
func withMairuHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("MAIRU_HOME", tmp)
	return tmp
}

func writeFile(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// stageLocalDist builds a complete distribution source tree on disk and
// returns its root path. The shape mirrors what an author would push to git.
func stageLocalDist(t *testing.T, name string) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ManifestFilename), `name: `+name+`
version: 0.1.0
description: "test distribution"
mairu_requires: ">=0.0.1"
env_requires:
  - name: TEST_KEY
    description: "test key"
    required: true
`)
	writeFile(t, filepath.Join(root, "agent_system.md"), "# test prompt\n")
	writeFile(t, filepath.Join(root, "prompts", "extra.md"), "# extra\n")
	writeFile(t, filepath.Join(root, "config.toml"), "[api]\ngemini_api_key = ''\n")
	return root
}

func TestManifestRoundtrip(t *testing.T) {
	tmp := t.TempDir()
	src := &Manifest{
		Name:          "demo",
		Version:       "1.2.3",
		MairuRequires: ">=0.0.1",
		EnvRequires: []EnvRequirement{
			{Name: "FOO", Description: "foo key", Required: true},
			{Name: "BAR", Required: false, Default: "x"},
		},
		DistributionOwned: []string{"agent_system.md", "prompts/"},
		Source:            "github.com/me/demo",
	}
	if err := WriteManifest(tmp, src); err != nil {
		t.Fatal(err)
	}
	got, err := ReadManifest(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != src.Name || got.Version != src.Version || got.Source != src.Source {
		t.Fatalf("roundtrip mismatch: got=%+v want=%+v", got, src)
	}
	owned := got.OwnedPaths()
	if len(owned) != 2 || owned[0] != "agent_system.md" || owned[1] != "prompts" {
		t.Fatalf("owned paths not stripped of trailing slash: %v", owned)
	}
}

func TestCheckMairuRequires(t *testing.T) {
	cases := []struct {
		spec, cur string
		wantErr   bool
	}{
		{"", "0.0.1", false},
		{">=0.1.0", "0.5.0", false},
		{">=0.1.0", "0.0.9", true},
		{"==0.5.0", "0.5.0", false},
		{"==0.5.0", "0.5.1", true},
		{"0.4.0", "0.4.0", false}, // bare → >=
		{"0.4.0", "0.3.9", true},
		{">v1.0", "1.0.1", false},
	}
	for _, c := range cases {
		err := CheckMairuRequires(c.spec, c.cur)
		if (err != nil) != c.wantErr {
			t.Errorf("CheckMairuRequires(%q, %q) err=%v wantErr=%v", c.spec, c.cur, err, c.wantErr)
		}
	}
}

func TestNameValidation(t *testing.T) {
	// Inputs that should normalize-then-validate cleanly. Casing is part of
	// the normalize step (Demo → demo), so it's accepted here by design.
	good := []string{"demo", "demo-1", "a_b", "x", "Demo"}
	// Inputs that should fail at normalize OR validate.
	bad := []string{"demo!", "", "mairu", "sudo", "-leading", "name with space"}
	for _, n := range good {
		canon, err := NormalizeName(n)
		if err != nil {
			t.Fatalf("normalize %q: %v", n, err)
		}
		if err := ValidateName(canon); err != nil {
			t.Errorf("expected %q to validate, got: %v", n, err)
		}
	}
	for _, n := range bad {
		canon, err := NormalizeName(n)
		if err != nil {
			continue // empty etc. fails normalize, fine
		}
		if err := ValidateName(canon); err == nil {
			t.Errorf("expected %q to fail validate", n)
		}
	}
}

func TestInstallFromLocalDir(t *testing.T) {
	home := withMairuHome(t)
	src := stageLocalDist(t, "demo")

	plan, err := Install(src, InstallOptions{MairuVersion: "0.1.0"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Existing {
		t.Fatal("fresh install should not report Existing")
	}
	if plan.Manifest.Source != src {
		t.Errorf("Source = %q want %q", plan.Manifest.Source, src)
	}

	target := filepath.Join(home, "profiles", "demo")
	for _, must := range []string{ManifestFilename, "agent_system.md", "config.toml",
		filepath.Join("prompts", "extra.md"), EnvExampleFilename} {
		if _, err := os.Stat(filepath.Join(target, must)); err != nil {
			t.Errorf("expected %s in target: %v", must, err)
		}
	}
	// Bootstrap dirs created.
	for _, d := range []string{"logs", "sessions", "local"} {
		if info, err := os.Stat(filepath.Join(target, d)); err != nil || !info.IsDir() {
			t.Errorf("expected bootstrap dir %s", d)
		}
	}
}

func TestInstallRejectsExistingWithoutForce(t *testing.T) {
	withMairuHome(t)
	src := stageLocalDist(t, "demo")
	if _, err := Install(src, InstallOptions{MairuVersion: "0.1.0"}); err != nil {
		t.Fatal(err)
	}
	_, err := Install(src, InstallOptions{MairuVersion: "0.1.0"})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected already-exists error, got: %v", err)
	}
	if _, err := Install(src, InstallOptions{MairuVersion: "0.1.0", Force: true}); err != nil {
		t.Fatalf("force install failed: %v", err)
	}
}

func TestUpdatePreservesConfigAndUserData(t *testing.T) {
	home := withMairuHome(t)
	src := stageLocalDist(t, "demo")
	if _, err := Install(src, InstallOptions{MairuVersion: "0.1.0"}); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, "profiles", "demo")

	// User edits config.toml; creates user-owned data + .env.
	writeFile(t, filepath.Join(target, "config.toml"), "[api]\ngemini_api_key = 'USER_VALUE'\n")
	writeFile(t, filepath.Join(target, ".env"), "TEST_KEY=user_value\n")
	writeFile(t, filepath.Join(target, "sessions", "s1.json"), "{}")

	// Author bumps the distribution (change config.toml to detect overwrite).
	writeFile(t, filepath.Join(src, "config.toml"), "[api]\ngemini_api_key = 'AUTHOR_NEW'\n")
	writeFile(t, filepath.Join(src, "agent_system.md"), "# v2 prompt\n")

	if _, err := Update("demo", UpdateOptions{MairuVersion: "0.1.0"}); err != nil {
		t.Fatal(err)
	}
	// User config preserved.
	cfg, _ := os.ReadFile(filepath.Join(target, "config.toml"))
	if !strings.Contains(string(cfg), "USER_VALUE") {
		t.Errorf("expected user's config.toml preserved on update, got: %s", cfg)
	}
	// Distribution-owned files replaced.
	prompt, _ := os.ReadFile(filepath.Join(target, "agent_system.md"))
	if !strings.Contains(string(prompt), "v2 prompt") {
		t.Errorf("expected prompt updated, got: %s", prompt)
	}
	// User data left alone.
	if _, err := os.Stat(filepath.Join(target, ".env")); err != nil {
		t.Errorf("user .env clobbered: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "sessions", "s1.json")); err != nil {
		t.Errorf("user session clobbered: %v", err)
	}

	// ForceConfig pulls the new config in.
	if _, err := Update("demo", UpdateOptions{MairuVersion: "0.1.0", ForceConfig: true}); err != nil {
		t.Fatal(err)
	}
	cfg, _ = os.ReadFile(filepath.Join(target, "config.toml"))
	if !strings.Contains(string(cfg), "AUTHOR_NEW") {
		t.Errorf("expected --force-config to overwrite config.toml, got: %s", cfg)
	}
}

func TestInstallRejectsSymlinks(t *testing.T) {
	withMairuHome(t)
	src := stageLocalDist(t, "demo")
	if err := os.Symlink("/etc/passwd", filepath.Join(src, "naughty")); err != nil {
		t.Skip("symlink unsupported on this fs:", err)
	}
	_, err := Install(src, InstallOptions{MairuVersion: "0.1.0"})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink rejection, got: %v", err)
	}
}

func TestInstallRejectsDefaultName(t *testing.T) {
	withMairuHome(t)
	src := stageLocalDist(t, "default")
	_, err := Install(src, InstallOptions{MairuVersion: "0.1.0"})
	if err == nil || !strings.Contains(err.Error(), "default") {
		t.Fatalf("expected refusal to install as default, got: %v", err)
	}
}

func TestVersionGateRejectsOldMairu(t *testing.T) {
	withMairuHome(t)
	src := t.TempDir()
	writeFile(t, filepath.Join(src, ManifestFilename), `name: demo
mairu_requires: ">=1.0.0"
`)
	writeFile(t, filepath.Join(src, "agent_system.md"), "# x\n")
	_, err := Install(src, InstallOptions{MairuVersion: "0.5.0"})
	if err == nil || !strings.Contains(err.Error(), "requires mairu") {
		t.Fatalf("expected version-gate error, got: %v", err)
	}
}
