package approved

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type config struct {
	diffContext  int
	updateEnv    string
	ignoreFields []string
	fuzzyFields  map[string]string
}

func defaultConfig() config {
	return config{
		diffContext: 3,
		updateEnv:   "UPDATE_APPROVED",
	}
}

// Option configures assertion behavior.
type Option func(*config)

// WithDiffContext sets the number of unified diff context lines (default 3).
func WithDiffContext(n int) Option {
	return func(c *config) { c.diffContext = n }
}

// WithIgnoreFields skips these JSON keys at any depth during comparison.
func WithIgnoreFields(fields ...string) Option {
	return func(c *config) { c.ignoreFields = fields }
}

// WithFuzzyFields replaces exact comparison for specific JSON paths with regex.
func WithFuzzyFields(rules map[string]string) Option {
	return func(c *config) { c.fuzzyFields = rules }
}

// WithUpdateEnv overrides the env var name that triggers update mode.
func WithUpdateEnv(envVar string) Option {
	return func(c *config) { c.updateEnv = envVar }
}

// Assert compares actual bytes against an approved file.
// In update mode (env var set), writes actual to the file instead.
// On mismatch, fails with a unified diff.
func Assert(t testing.TB, approvedPath string, actual []byte, opts ...Option) {
	t.Helper()
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}

	if os.Getenv(cfg.updateEnv) != "" {
		if err := os.MkdirAll(filepath.Dir(approvedPath), 0755); err != nil {
			t.Fatalf("creating directory for approved file: %v", err)
		}
		if err := os.WriteFile(approvedPath, actual, 0644); err != nil {
			t.Fatalf("writing approved file: %v", err)
		}
		t.Logf("updated %s", approvedPath)
		return
	}

	expected, err := os.ReadFile(approvedPath)
	if err != nil {
		t.Fatalf("reading approved file %s (run with %s=1 to generate): %v",
			approvedPath, cfg.updateEnv, err)
		return
	}

	if string(expected) == string(actual) {
		return
	}

	diff := unifiedDiff(string(expected), string(actual),
		"approved: "+approvedPath, "actual", cfg.diffContext)
	t.Errorf("output differs from approved %s\n\n%s", approvedPath, diff)
}

// AssertString is a convenience wrapper for string content.
func AssertString(t testing.TB, approvedPath string, actual string, opts ...Option) {
	t.Helper()
	Assert(t, approvedPath, []byte(actual), opts...)
}

// AssertJSON normalizes actual to JSON and compares against an approved .json file.
// On mismatch, shows both a field-level structured diff and a unified text diff.
func AssertJSON(t testing.TB, approvedPath string, actual any, opts ...Option) {
	t.Helper()
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}

	actualBytes, err := normalizeJSON(actual)
	if err != nil {
		t.Fatalf("normalizing JSON: %v", err)
		return
	}

	if os.Getenv(cfg.updateEnv) != "" {
		if err := os.MkdirAll(filepath.Dir(approvedPath), 0755); err != nil {
			t.Fatalf("creating directory for approved file: %v", err)
		}
		if err := os.WriteFile(approvedPath, actualBytes, 0644); err != nil {
			t.Fatalf("writing approved file: %v", err)
		}
		t.Logf("updated %s", approvedPath)
		return
	}

	expectedBytes, err := os.ReadFile(approvedPath)
	if err != nil {
		t.Fatalf("reading approved file %s (run with %s=1 to generate): %v",
			approvedPath, cfg.updateEnv, err)
		return
	}

	if string(expectedBytes) == string(actualBytes) {
		return
	}

	// Unmarshal both for field-level diff
	var expectedTree, actualTree any
	json.Unmarshal(expectedBytes, &expectedTree)
	json.Unmarshal(actualBytes, &actualTree)

	// Apply ignore/fuzzy if configured
	if len(cfg.ignoreFields) > 0 {
		fieldSet := make(map[string]bool, len(cfg.ignoreFields))
		for _, f := range cfg.ignoreFields {
			fieldSet[f] = true
		}
		expectedTree = removeFields(expectedTree, fieldSet)
		actualTree = removeFields(actualTree, fieldSet)
		// Re-check after field removal
		ej, _ := json.MarshalIndent(expectedTree, "", "  ")
		aj, _ := json.MarshalIndent(actualTree, "", "  ")
		if string(ej) == string(aj) {
			return
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("JSON differs from approved %s\n", approvedPath))

	fieldDiff := jsonFieldDiff(expectedTree, actualTree)
	if fieldDiff != "" {
		sb.WriteString("\nField-level changes:\n")
		sb.WriteString(fieldDiff)
		sb.WriteString("\n")
	}

	textDiff := unifiedDiff(string(expectedBytes), string(actualBytes),
		"approved: "+approvedPath, "actual", cfg.diffContext)
	if textDiff != "" {
		sb.WriteString("\nFull diff:\n")
		sb.WriteString(textDiff)
	}

	t.Errorf("%s", sb.String())
}
