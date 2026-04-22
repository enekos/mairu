package walkers

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enekos/mairu/pii-redact/internal/approved"
	"github.com/enekos/mairu/pii-redact/internal/config"
	"github.com/enekos/mairu/pii-redact/internal/mask"
	"github.com/enekos/mairu/pii-redact/internal/patterns"
)

// These tests lock the exact redacted output of each scenario to an
// approved file in testdata/approved/. Regenerate with:
//
//	UPDATE_APPROVED=1 go test ./internal/walkers/ -run Approved
//
// Every intentional change to redaction behavior should show up as a
// reviewable diff in the approved files.

func TestApproved_JSON_StrictMode(t *testing.T) {
	out := redactFixture(t, "../../testdata/fixtures/red_team.json", true)
	approved.Assert(t,
		"../../testdata/approved/red_team_strict.approved.json",
		out,
	)
}

func TestApproved_JSON_PermissiveMode(t *testing.T) {
	out := redactFixture(t, "../../testdata/fixtures/red_team.json", false)
	approved.Assert(t,
		"../../testdata/approved/red_team_permissive.approved.json",
		out,
	)
}

func TestApproved_JSON_RevealMode(t *testing.T) {
	out := redactFixtureReveal(t, "../../testdata/fixtures/red_team.json", true)
	approved.Assert(t,
		"../../testdata/approved/red_team_reveal.approved.json",
		out,
	)
}

func TestApproved_LineMode_ValueFormat(t *testing.T) {
	set, err := patterns.Compile(loadJOINPatterns(t))
	if err != nil {
		t.Fatal(err)
	}
	in := strings.NewReader(mustRead(t, "../../testdata/fixtures/value_format.txt"))
	var buf bytes.Buffer
	if _, err := Lines(in, &buf, set); err != nil {
		t.Fatal(err)
	}
	approved.Assert(t,
		"../../testdata/approved/value_format.approved.txt",
		buf.Bytes(),
	)
}

// CheckFixtures ensures every input fixture has a matching approved file.
func TestApproved_FixtureCoverage(t *testing.T) {
	cwd, _ := os.Getwd()
	root := filepath.Clean(filepath.Join(cwd, "..", ".."))
	rule := approved.FixtureRule{
		InputGlob: filepath.Join(root, "testdata/fixtures/*.json"),
		MapFunc: func(in string) string {
			// red_team.json has two approved files (strict + permissive).
			// Require at least the strict variant.
			base := filepath.Base(in)
			name := strings.TrimSuffix(base, ".json")
			return filepath.Join(root, "testdata/approved", name+"_strict.approved.json")
		},
	}
	missing, err := approved.CheckFixtures(rule)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) > 0 {
		t.Errorf("fixtures missing approved files:\n  %s\n\n"+
			"Regenerate with UPDATE_APPROVED=1 go test ./internal/walkers/",
			strings.Join(missing, "\n  "))
	}
}

// helpers

func redactFixtureReveal(t *testing.T, fixturePath string, strict bool) []byte {
	t.Helper()
	rules, err := config.Load(config.LoadOptions{
		ConfigDirs: []string{"../../testdata/configs/join"},
	})
	if err != nil {
		t.Fatal(err)
	}
	set, err := patterns.Compile(rules.ContentPatterns)
	if err != nil {
		t.Fatal(err)
	}
	m := mask.NewMasker(true)
	set = set.WithMasker(m)
	opts := Options{
		Rules:  rules,
		Set:    set,
		Masker: m,
		Strict: strict,
		ServiceOf: func(e any) string {
			return ExtractByPath(e, rules.ServiceField)
		},
	}
	in := strings.NewReader(mustRead(t, fixturePath))
	var buf bytes.Buffer
	if _, err := JSON(in, &buf, opts); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func redactFixture(t *testing.T, fixturePath string, strict bool) []byte {
	t.Helper()
	rules, err := config.Load(config.LoadOptions{
		ConfigDirs: []string{"../../testdata/configs/join"},
	})
	if err != nil {
		t.Fatal(err)
	}
	set, err := patterns.Compile(rules.ContentPatterns)
	if err != nil {
		t.Fatal(err)
	}
	opts := Options{
		Rules:  rules,
		Set:    set,
		Strict: strict,
		ServiceOf: func(e any) string {
			return ExtractByPath(e, rules.ServiceField)
		},
	}
	in := strings.NewReader(mustRead(t, fixturePath))
	var buf bytes.Buffer
	if _, err := JSON(in, &buf, opts); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func loadJOINPatterns(t *testing.T) map[string]string {
	t.Helper()
	rules, err := config.Load(config.LoadOptions{
		ConfigDirs: []string{"../../testdata/configs/join"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return rules.ContentPatterns
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
