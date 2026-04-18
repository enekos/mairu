package redact

import "testing"

func TestNewAppliesDefaults(t *testing.T) {
	r := New()
	if r.entropyThreshold == 0 {
		t.Fatal("expected non-zero default entropyThreshold")
	}
	if r.damageCapRatio == 0 {
		t.Fatal("expected non-zero default damageCapRatio")
	}
	if r.minEntropyLen == 0 {
		t.Fatal("expected non-zero default minEntropyLen")
	}
}

func TestNewAcceptsOptions(t *testing.T) {
	r := New(
		WithEntropyThreshold(5.0),
		WithDamageCapRatio(0.75),
		WithMinEntropyLen(32),
		WithDenylistCommands([]string{"vault"}),
	)
	if r.entropyThreshold != 5.0 {
		t.Errorf("entropyThreshold = %v; want 5.0", r.entropyThreshold)
	}
	if r.damageCapRatio != 0.75 {
		t.Errorf("damageCapRatio = %v; want 0.75", r.damageCapRatio)
	}
	if r.minEntropyLen != 32 {
		t.Errorf("minEntropyLen = %v; want 32", r.minEntropyLen)
	}
	if len(r.denylistCommands) != 1 || r.denylistCommands[0] != "vault" {
		t.Errorf("denylistCommands = %v; want [vault]", r.denylistCommands)
	}
}

func TestRedactEmptyInputIsSafe(t *testing.T) {
	got := New().Redact("", KindText)
	if got.Redacted != "" {
		t.Errorf("Redacted = %q; want empty", got.Redacted)
	}
	if !got.EmbeddingSafe {
		t.Error("empty input must be embedding-safe")
	}
	if got.Dropped {
		t.Error("empty input must not be dropped")
	}
	if len(got.Findings) != 0 {
		t.Errorf("len(Findings) = %d; want 0", len(got.Findings))
	}
}

func TestRedactPlainTextPassesThrough(t *testing.T) {
	got := New().Redact("hello world this is fine", KindText)
	if got.Redacted != "hello world this is fine" {
		t.Errorf("Redacted = %q; want pass-through", got.Redacted)
	}
	if !got.EmbeddingSafe {
		t.Error("plain text must be embedding-safe")
	}
}
