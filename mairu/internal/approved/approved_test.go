package approved

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssertString_UpdateMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.approved.txt")

	t.Setenv("UPDATE_APPROVED", "1")
	// Should write the file instead of comparing
	AssertString(t, path, "hello world\n")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("approved file not written: %v", err)
	}
	if string(data) != "hello world\n" {
		t.Errorf("wrong content: %q", string(data))
	}
}

func TestAssertString_Match(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.approved.txt")
	os.WriteFile(path, []byte("hello\n"), 0644)

	// Should pass without error
	AssertString(t, path, "hello\n")
}

func TestAssertString_Mismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.approved.txt")
	os.WriteFile(path, []byte("expected\n"), 0644)

	mt := &mockT{}
	AssertString(mt, path, "actual\n")

	if !mt.failed {
		t.Error("expected test to fail on mismatch")
	}
	if mt.errorMsg == "" {
		t.Error("expected error message with diff")
	}
}

func TestAssertJSON_NormalizesNils(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.approved.json")

	type S struct {
		Items []string `json:"items"`
	}

	// Write approved file with empty array
	os.WriteFile(path, []byte("{\n  \"items\": []\n}\n"), 0644)

	// Pass struct with nil slice — should match after normalization
	AssertJSON(t, path, S{Items: nil})
}

func TestAssertJSON_FieldDiffOnMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.approved.json")
	os.WriteFile(path, []byte("{\n  \"name\": \"old\"\n}\n"), 0644)

	mt := &mockT{}
	AssertJSON(mt, path, map[string]string{"name": "new"})

	if !mt.failed {
		t.Error("expected test to fail on mismatch")
	}
	// Should contain field-level diff
	if !strings.Contains(mt.errorMsg, "name") {
		t.Errorf("expected field-level diff mentioning 'name', got:\n%s", mt.errorMsg)
	}
}

func TestAssert_MissingApprovedFile(t *testing.T) {
	mt := &mockT{}
	Assert(mt, "/nonexistent/file.txt", []byte("data"))

	if !mt.fataled {
		t.Error("expected fatal on missing approved file")
	}
}

func TestAssert_WithCustomUpdateEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.approved.txt")

	t.Setenv("CUSTOM_UPDATE", "1")
	AssertString(t, path, "content\n", WithUpdateEnv("CUSTOM_UPDATE"))

	data, _ := os.ReadFile(path)
	if string(data) != "content\n" {
		t.Errorf("custom env var not honored: %q", string(data))
	}
}

// mockT captures test failures without actually failing the parent test.
type mockT struct {
	testing.TB
	failed   bool
	fataled  bool
	errorMsg string
}

func (m *mockT) Helper() {}
func (m *mockT) Errorf(format string, args ...any) {
	m.failed = true
	m.errorMsg = fmt.Sprintf(format, args...)
}
func (m *mockT) Fatalf(format string, args ...any) {
	m.fataled = true
	m.failed = true
	m.errorMsg = fmt.Sprintf(format, args...)
}
func (m *mockT) Logf(format string, args ...any) {}
func (m *mockT) Setenv(key, value string)        { os.Setenv(key, value) }
