package redact

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/join-com/pii-redact/internal/config"
	"github.com/join-com/pii-redact/internal/mask"
	"github.com/join-com/pii-redact/internal/patterns"
)

// TestRedTeam_NoRawPIISurvives is the CI hard gate. Any PII value listed
// below MUST NOT appear verbatim in the redacted output.
func TestRedTeam_NoRawPIISurvives(t *testing.T) {
	rules, err := config.Load(config.LoadOptions{
		ConfigDirs: []string{"../../testdata/configs/join"},
	})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	set, err := patterns.Compile(rules.ContentPatterns)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	input, err := os.ReadFile("../../testdata/fixtures/red_team.json")
	if err != nil {
		t.Fatal(err)
	}

	opts := Options{
		Rules:  rules,
		Set:    set,
		Strict: true,
		ServiceOf: func(e any) string {
			return ExtractByPath(e, rules.ServiceField)
		},
	}
	var out bytes.Buffer
	if _, err := JSON(bytes.NewReader(input), &out, opts); err != nil {
		t.Fatal(err)
	}
	got := out.String()

	// ensure reveal-mode red-team is also safe (PII must still never appear raw).
	t.Run("reveal_mode", func(t *testing.T) {
		optsR := opts
		optsR.Masker = mask.NewMasker(true)
		optsR.Set = optsR.Set.WithMasker(optsR.Masker)
		var outR bytes.Buffer
		if _, err := JSON(bytes.NewReader(input), &outR, optsR); err != nil {
			t.Fatal(err)
		}
		raw := outR.String()
		for _, n := range rawNeedles() {
			if strings.Contains(raw, n) {
				t.Errorf("reveal mode leaked: %q", n)
			}
		}
	})

	piiNeedles := []string{
		// emails, phones, basic PII
		"john.doe@acme.io", "leak@nope.io", "+14155551234",
		// names / address / demographics
		"Jane", "Doe", "Hauptstr 5", "German", "female",
		// DOB
		"1985-03-14",
		// national / government IDs
		"123-45-6789", "P123456789", "D9876543210", "AB123456C",
		// banking
		"DE89370400440532013000", "4111 1111 1111 1111", "DEUTDEFF", "DE123456789",
		// network
		"10.0.0.5", "2001:db8:85a3:8d3:1319:8a2e:370:7348", "aa:bb:cc:dd:ee:ff",
		// geo
		"52.5200", "13.4050",
		// comp
		"75000",
		// tokens / secrets
		"eyJhbGciOiJIUzI1NiJ9", "at_live_abc", "rt_live_abc",
		"AIzaSyABCDEFGHIJKLMNOPQRSTUVWXYZ1234567",
		"AKIAIOSFODNN7EXAMPLE",
		"ghp_abcdefghijklmnopqrstuvwxyz0123456789",
		"xoxb-1234567890-abcdefghij",
		"sk_live_abcdefghijklmnopqrstuv",
		"hunter2", "admin:hunter2",
		"BEGIN RSA PRIVATE KEY",
	}

	for _, needle := range piiNeedles {
		if strings.Contains(got, needle) {
			t.Errorf("PII leaked through redactor: %q\n\noutput:\n%s", needle, got)
		}
	}

	// Sanity: structural safe fields should survive.
	mustKeep := []string{"timestamp", "traceId", "abc123", "ERROR", "container_name", "ats"}
	for _, keep := range mustKeep {
		if !strings.Contains(got, keep) {
			t.Errorf("expected structural field %q to survive, missing from output", keep)
		}
	}
}

// rawNeedles is the superset of PII strings that must never survive
// verbatim, regardless of mask mode.
func rawNeedles() []string {
	return []string{
		"john.doe@acme.io", "leak@nope.io", "+14155551234",
		"Jane", "Doe", "Hauptstr 5", "German", "female",
		"1985-03-14",
		"123-45-6789", "P123456789", "D9876543210", "AB123456C",
		"DE89370400440532013000", "4111 1111 1111 1111", "DEUTDEFF", "DE123456789",
		"10.0.0.5", "2001:db8:85a3:8d3:1319:8a2e:370:7348", "aa:bb:cc:dd:ee:ff",
		"52.5200", "13.4050",
		"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.signature_here",
		"AIzaSyABCDEFGHIJKLMNOPQRSTUVWXYZ1234567",
		"AKIAIOSFODNN7EXAMPLE",
		"ghp_abcdefghijklmnopqrstuvwxyz0123456789",
		"xoxb-1234567890-abcdefghij",
		"sk_live_abcdefghijklmnopqrstuv",
		"hunter2", "admin:hunter2",
		"BEGIN RSA PRIVATE KEY",
		"550e8400-e29b-41d4-a716-446655440000",
		"0xAbCdef0123456789abcdef0123456789abcdef01",
		"ya29.aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789",
		"AccountKey=abcdef1234567890",
	}
}
