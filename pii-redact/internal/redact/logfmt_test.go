package redact

import (
	"bytes"
	"strings"
	"testing"

	"github.com/join-com/pii-redact/internal/config"
	"github.com/join-com/pii-redact/internal/mask"
	"github.com/join-com/pii-redact/internal/patterns"
)

func newTestOpts(t *testing.T, reveal bool) Options {
	t.Helper()
	rules, err := config.Load(config.LoadOptions{
		ConfigDirs: []string{"../../testdata/configs/join"},
	})
	if err != nil {
		t.Fatal(err)
	}
	m := mask.NewMasker(reveal)
	set, err := patterns.Compile(rules.ContentPatterns)
	if err != nil {
		t.Fatal(err)
	}
	set = set.WithMasker(m)
	return Options{Rules: rules, Set: set, Masker: m, Strict: true}
}

func TestLogfmt_Reveal(t *testing.T) {
	opts := newTestOpts(t, true)
	in := `ts=2026-04-21 level=error email=jane@acme.io ip=10.0.0.5 msg="login from john@acme.io at 10.0.0.5" auth="Bearer abcdefghij"`
	var buf bytes.Buffer
	if _, err := Logfmt(strings.NewReader(in+"\n"), &buf, opts); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, leak := range []string{"jane@acme.io", "john@acme.io", "10.0.0.5", "abcdefghij"} {
		if strings.Contains(out, leak) {
			t.Errorf("raw %q leaked: %s", leak, out)
		}
	}
	for _, want := range []string{"ts=2026-04-21", "level=error", "Bearer ****"} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in %s", want, out)
		}
	}
}

func TestLogfmt_OpaqueRedactKeys(t *testing.T) {
	opts := newTestOpts(t, false)
	in := `email=jane@acme.io status=ok`
	var buf bytes.Buffer
	if _, err := Logfmt(strings.NewReader(in+"\n"), &buf, opts); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "email=[REDACTED:KEY]") {
		t.Errorf("expected opaque KEY redaction; got %s", out)
	}
	if !strings.Contains(out, "status=ok") {
		t.Errorf("safe key dropped: %s", out)
	}
}

func TestLogfmt_QuotedEscapes(t *testing.T) {
	opts := newTestOpts(t, true)
	in := `msg="user said \"hi\" to jane@acme.io"`
	var buf bytes.Buffer
	if _, err := Logfmt(strings.NewReader(in+"\n"), &buf, opts); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "jane@acme.io") {
		t.Errorf("leak: %s", out)
	}
	if !strings.Contains(out, `user said`) {
		t.Errorf("context lost: %s", out)
	}
}
