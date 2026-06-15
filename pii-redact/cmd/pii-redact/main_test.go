package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Build the binary once per test run and reuse.
func buildBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "pii-redact")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

func runCLI(t *testing.T, bin string, args []string, stdin string) (stdout, stderr string, exit int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Stdin = strings.NewReader(stdin)
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("run: %v", err)
		}
	}
	return so.String(), se.String(), code
}

func TestCLI_AutoDetectsJSON(t *testing.T) {
	bin := buildBinary(t)
	testCfg := filepath.Join(projectRoot(t), "testdata/configs/default")
	args := []string{"--config-dir", testCfg}
	stdout, _, code := runCLI(t, bin, args, `{"id": "abc", "email": "john@acme.io"}`)
	if code != 0 {
		t.Fatalf("exit %d; stdout=%s", code, stdout)
	}
	if strings.Contains(stdout, "john@acme.io") {
		t.Errorf("raw email leaked; got %s", stdout)
	}
	if !strings.Contains(stdout, "@a") || !strings.Contains(stdout, ".io") {
		t.Errorf("expected partial-reveal email with domain hint; got %s", stdout)
	}
	// Output must be valid JSON.
	var parsed map[string]any
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Errorf("output not valid JSON: %v", err)
	}
}

func TestCLI_AutoDetectsLines(t *testing.T) {
	bin := buildBinary(t)
	testCfg := filepath.Join(projectRoot(t), "testdata/configs/default")
	args := []string{"--config-dir", testCfg}
	stdout, _, code := runCLI(t, bin, args, "login from 10.0.0.5\nok\n")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if strings.Contains(stdout, "10.0.0.5") {
		t.Errorf("ip not redacted: %s", stdout)
	}
	if !strings.Contains(stdout, "ok") {
		t.Errorf("clean line dropped: %s", stdout)
	}
}

func TestCLI_MalformedJSON_FailsClosed(t *testing.T) {
	bin := buildBinary(t)
	testCfg := filepath.Join(projectRoot(t), "testdata/configs/default")
	args := []string{"--mode", "json", "--config-dir", testCfg}
	stdout, stderr, code := runCLI(t, bin, args, `{not json`)
	if code != 2 {
		t.Fatalf("expected exit 2 for parse error, got %d", code)
	}
	if stdout != "" {
		t.Errorf("fail-closed violated: stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "pii-redact:") {
		t.Errorf("expected diagnostic on stderr, got %q", stderr)
	}
}

func TestCLI_BadConfigDir_FailsClosed(t *testing.T) {
	bin := buildBinary(t)
	args := []string{"--config-dir", "/definitely/does/not/exist"}
	// ConfigDirs with missing dir => mergeConfigDir returns nil (no global),
	// so this should succeed with empty ruleset. That's a permissive load.
	// We specifically test bad file content though.
	badDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(badDir, "global.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	args = []string{"--config-dir", badDir}
	stdout, stderr, code := runCLI(t, bin, args, `{"a":"b"}`)
	if code != 1 {
		t.Fatalf("expected exit 1 for config error, got %d", code)
	}
	if stdout != "" {
		t.Errorf("fail-closed: expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "pii-redact:") {
		t.Errorf("expected diagnostic on stderr, got %q", stderr)
	}
}

func TestCLI_Stats_GoesToStderr(t *testing.T) {
	bin := buildBinary(t)
	testCfg := filepath.Join(projectRoot(t), "testdata/configs/default")
	args := []string{"--config-dir", testCfg, "--stats", "--mode", "line"}
	_, stderr, code := runCLI(t, bin, args, "login from 10.0.0.5\n")
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "pii-redact stats:") {
		t.Errorf("stats not written to stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "ipv4") {
		t.Errorf("expected ipv4 in stats histogram: %q", stderr)
	}
}

func TestCLI_RedTeamFixture_E2E(t *testing.T) {
	bin := buildBinary(t)
	testCfg := filepath.Join(projectRoot(t), "testdata/configs/default")
	fixture, err := os.ReadFile(filepath.Join(projectRoot(t), "testdata/fixtures/red_team.json"))
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"--config-dir", testCfg, "--mode", "json"}
	stdout, _, code := runCLI(t, bin, args, string(fixture))
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	piiNeedles := []string{
		"john.doe@acme.io", "leak@nope.io", "10.0.0.5", "Jane", "Doe",
		"+14155551234", "Hauptstr 5", "DE89370400440532013000",
		"4111 1111 1111 1111", "eyJhbGciOiJIUzI1NiJ9",
	}
	for _, n := range piiNeedles {
		if strings.Contains(stdout, n) {
			t.Errorf("PII leaked via CLI: %q", n)
		}
	}
}

func TestCLI_KeepIDs_UUIDPassesThrough(t *testing.T) {
	// `message` is a safe key in the default test config; `candidateId` is
	// neither safe nor in id_keys. With --keep-ids, the broadened id_keys
	// catches *Id and the uuid content pattern is dropped, so UUIDs in both
	// keyed and free-text contexts survive intact.
	bin := buildBinary(t)
	testCfg := filepath.Join(projectRoot(t), "testdata/configs/default")
	uuid := "550e8400-e29b-41d4-a716-446655440000"
	in := `{"candidateId": "` + uuid + `", "message": "ref ` + uuid + `", "email": "j@a.io"}`
	args := []string{"--config-dir", testCfg, "--keep-ids", "--mode", "json"}
	stdout, _, code := runCLI(t, bin, args, in)
	if code != 0 {
		t.Fatalf("exit %d; stdout=%s", code, stdout)
	}
	if strings.Count(stdout, uuid) < 2 {
		t.Errorf("--keep-ids: expected UUID to survive in both id-keyed and free-text contexts; got %s", stdout)
	}
	if strings.Contains(stdout, "j@a.io") {
		t.Errorf("--keep-ids: email must still be redacted; got %s", stdout)
	}
}

func TestCLI_KeepIDs_DoesNotAffectDefaultRun(t *testing.T) {
	// Without --keep-ids, UUIDs in safe-key free text are still redacted.
	bin := buildBinary(t)
	testCfg := filepath.Join(projectRoot(t), "testdata/configs/default")
	uuid := "550e8400-e29b-41d4-a716-446655440000"
	in := `{"message": "ref ` + uuid + `"}`
	args := []string{"--config-dir", testCfg, "--mode", "json"}
	stdout, _, code := runCLI(t, bin, args, in)
	if code != 0 {
		t.Fatalf("exit %d; stdout=%s", code, stdout)
	}
	if strings.Contains(stdout, uuid) {
		t.Errorf("baseline: UUID in free text should be masked without --keep-ids; got %s", stdout)
	}
}

func projectRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// test runs inside cmd/pii-redact — go two levels up.
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}
