package redact

import (
	"bytes"
	"strings"
	"testing"

	"github.com/join-com/pii-redact/internal/patterns"
)

func TestLines_RedactsEachLine(t *testing.T) {
	set, _ := patterns.Compile(map[string]string{
		"email": `[\w.+-]+@[\w-]+\.[\w.-]+`,
		"ipv4":  `\b(?:\d{1,3}\.){3}\d{1,3}\b`,
	})
	in := strings.NewReader("login from 10.0.0.5\nemail john@acme.io\nok\n")
	var out bytes.Buffer
	stats, err := Lines(in, &out, set)
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if strings.Contains(got, "10.0.0.5") {
		t.Errorf("ip not redacted: %q", got)
	}
	if strings.Contains(got, "john@acme.io") {
		t.Errorf("email not redacted: %q", got)
	}
	if !strings.Contains(got, "ok") {
		t.Errorf("clean line dropped: %q", got)
	}
	if stats["email"] != 1 || stats["ipv4"] != 1 {
		t.Errorf("bad stats: %v", stats)
	}
}

func TestLines_EmptyInput(t *testing.T) {
	set, _ := patterns.Compile(nil)
	var out bytes.Buffer
	stats, err := Lines(strings.NewReader(""), &out, set)
	if err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Errorf("expected empty output, got %q", out.String())
	}
	if len(stats) != 0 {
		t.Error("expected empty stats")
	}
}
