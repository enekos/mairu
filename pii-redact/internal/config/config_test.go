package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_DefaultsOnly(t *testing.T) {
	r, err := Load(LoadOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := r.ContentPatterns["email"]; !ok {
		t.Fatal("expected default email pattern to be loaded")
	}
	if len(r.SafeKeys) != 0 {
		t.Fatalf("expected no safe keys without profile/config, got %d", len(r.SafeKeys))
	}
}

func TestLoad_ProfileGCP(t *testing.T) {
	r, err := Load(LoadOptions{Profile: "gcp-logging"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ServiceField != "resource.labels.container_name" {
		t.Fatalf("expected gcp profile to set service_field, got %q", r.ServiceField)
	}
	if r.MaxSafeStringLength == 0 {
		t.Fatal("expected gcp profile to set max_safe_string_length")
	}
}

func TestLoad_UnknownProfile(t *testing.T) {
	_, err := Load(LoadOptions{Profile: "does-not-exist"})
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
}

func TestLoad_ConfigDir_MergesGlobalAndServices(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, "global.json"), `{
		"safe_keys": ["id", "status"],
		"redact_keys": ["email"],
		"max_safe_string_length": 100
	}`)
	svcDir := filepath.Join(dir, "services")
	if err := os.Mkdir(svcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(svcDir, "ats.json"), `{
		"safe_keys": ["scorecardId"],
		"redact_keys": ["notes"]
	}`)

	r, err := Load(LoadOptions{ConfigDirs: []string{dir}})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, ok := r.SafeKeys["id"]; !ok {
		t.Error("expected id in safe_keys")
	}
	if _, ok := r.RedactKeys["email"]; !ok {
		t.Error("expected email in redact_keys")
	}
	if r.MaxSafeStringLength != 100 {
		t.Errorf("want max_safe_string_length=100, got %d", r.MaxSafeStringLength)
	}

	resolved := r.ResolveForService("ats")
	if _, ok := resolved.SafeKeys["scorecardId"]; !ok {
		t.Error("expected service override to add scorecardId")
	}
	if _, ok := resolved.RedactKeys["notes"]; !ok {
		t.Error("expected service override to add notes")
	}
	// Global list must still be present.
	if _, ok := resolved.RedactKeys["email"]; !ok {
		t.Error("service override must not erase global redact_keys")
	}
}

func TestResolveForService_CannotRemoveGlobalRedact(t *testing.T) {
	// The ruleset itself has no API to "remove" keys — ServiceRules only adds.
	// This test documents the invariant: resolving returns a superset.
	r := &Ruleset{
		SafeKeys:         map[string]struct{}{"a": {}},
		RedactKeys:       map[string]struct{}{"email": {}},
		ServiceOverrides: map[string]ServiceRules{"svc": {SafeKeys: []string{"b"}}},
	}
	resolved := r.ResolveForService("svc")
	if _, ok := resolved.RedactKeys["email"]; !ok {
		t.Fatal("email must stay redacted after service resolve")
	}
}

func TestLoad_MalformedJSON_FailsClosed(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, "global.json"), `{ not json`)
	_, err := Load(LoadOptions{ConfigDirs: []string{dir}})
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestLoad_UnknownFields_FailsClosed(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, "global.json"), `{"mystery_key": true}`)
	_, err := Load(LoadOptions{ConfigDirs: []string{dir}})
	if err == nil {
		t.Fatal("expected error for unknown field (typo-safety)")
	}
}

func TestLoad_ConfigFiles_LaterWins(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.json")
	b := filepath.Join(dir, "b.json")
	writeJSON(t, a, `{"max_safe_string_length": 50}`)
	writeJSON(t, b, `{"max_safe_string_length": 999}`)
	r, err := Load(LoadOptions{Configs: []string{a, b}})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if r.MaxSafeStringLength != 999 {
		t.Errorf("want 999, got %d", r.MaxSafeStringLength)
	}
}

func writeJSON(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
