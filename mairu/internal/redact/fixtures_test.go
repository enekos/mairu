package redact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type fixture struct {
	Kind           string `yaml:"kind"`
	Input          string `yaml:"input"`
	MustNotContain string `yaml:"must_not_contain"`
	InputKind      string `yaml:"input_kind"`
}

func loadFixtures(t *testing.T, name string) []fixture {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	var fx []fixture
	if err := yaml.Unmarshal(b, &fx); err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}
	if len(fx) == 0 {
		t.Fatalf("fixture %s is empty", name)
	}
	return fx
}

func toKind(s string) Kind {
	if s == "command" {
		return KindCommand
	}
	return KindText
}

func TestSecretsFixturesAreFullyRedacted(t *testing.T) {
	r := New()
	for _, f := range loadFixtures(t, "secrets.yaml") {
		t.Run(f.Kind, func(t *testing.T) {
			got := r.Redact(f.Input, toKind(f.InputKind))
			if strings.Contains(got.Redacted, f.MustNotContain) {
				t.Fatalf("secret leaked through redactor\n  kind:     %s\n  input:    %q\n  redacted: %q\n  leaked:   %q",
					f.Kind, f.Input, got.Redacted, f.MustNotContain)
			}
		})
	}
}

func TestBenignFixturesArePreserved(t *testing.T) {
	r := New()
	for _, f := range loadFixtures(t, "benign.yaml") {
		t.Run(f.Kind, func(t *testing.T) {
			got := r.Redact(f.Input, toKind(f.InputKind))
			if got.Redacted != f.Input {
				t.Fatalf("benign input was modified\n  kind:     %s\n  input:    %q\n  redacted: %q",
					f.Kind, f.Input, got.Redacted)
			}
			if got.Dropped {
				t.Fatalf("benign input was dropped\n  kind:  %s\n  input: %q", f.Kind, f.Input)
			}
		})
	}
}

// layer1Kinds is the set of secret fixture kinds that represent Layer-1
// token patterns. Any hit in this set MUST set EmbeddingSafe=false — that's
// the single property most likely to regress silently, because a layer
// re-shuffle could redact correctly while forgetting to flip the flag, at
// which point we'd ship secrets to a remote embedder post-redaction.
var layer1Kinds = map[string]bool{
	"github_pat":          true,
	"github_fine_grained": true,
	"aws_access_key":      true,
	"stripe_live":         true,
	"slack_bot_token":     true,
	"google_api_key":      true,
	"jwt":                 true,
	"uri_credentials":     true,
	"pem_key":             true,
}

func TestLayer1FixturesSetEmbeddingUnsafe(t *testing.T) {
	r := New()
	for _, f := range loadFixtures(t, "secrets.yaml") {
		if !layer1Kinds[f.Kind] {
			continue
		}
		t.Run(f.Kind, func(t *testing.T) {
			got := r.Redact(f.Input, toKind(f.InputKind))
			if got.EmbeddingSafe {
				t.Fatalf("Layer-1 fixture did not flip EmbeddingSafe\n  kind:  %s\n  input: %q", f.Kind, f.Input)
			}
		})
	}
}
