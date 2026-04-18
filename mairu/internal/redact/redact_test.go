package redact

import (
	"strings"
	"testing"
)

// contains is a local helper so test failure messages read naturally.
func contains(haystack, needle string) bool {
	return len(needle) > 0 && strings.Contains(haystack, needle)
}

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

func TestLayer1RedactsGitHubPAT(t *testing.T) {
	in := "token=ghp_1234567890abcdefghijklmnopqrstuvwxyz"
	got := New().Redact(in, KindText)
	if contains(got.Redacted, "ghp_1234567890") {
		t.Errorf("raw PAT leaked: %q", got.Redacted)
	}
	if got.EmbeddingSafe {
		t.Error("Layer 1 hit must set EmbeddingSafe=false")
	}
	if len(got.Findings) == 0 || got.Findings[0].Layer != LayerKnownToken {
		t.Errorf("expected LayerKnownToken finding; got %+v", got.Findings)
	}
}

func TestLayer1RedactsAWSAccessKey(t *testing.T) {
	in := "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE"
	got := New().Redact(in, KindText)
	if contains(got.Redacted, "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("AWS key leaked: %q", got.Redacted)
	}
	if got.EmbeddingSafe {
		t.Error("AWS key must set EmbeddingSafe=false")
	}
}

func TestLayer1RedactsStripeLiveKey(t *testing.T) {
	in := "sk_live_4eC39HqLyjWDarjtT1zdp7dcABCDEFGH"
	got := New().Redact(in, KindText)
	if contains(got.Redacted, "sk_live_4eC39HqLyjWDarjtT1zdp7dc") {
		t.Errorf("stripe key leaked: %q", got.Redacted)
	}
	if got.EmbeddingSafe {
		t.Error("stripe key must set EmbeddingSafe=false")
	}
}

func TestLayer1RedactsSlackToken(t *testing.T) {
	in := "xoxb-1234567890-0987654321-AbCdEfGhIjKlMnOpQrStUvWx"
	got := New().Redact(in, KindText)
	if contains(got.Redacted, "xoxb-1234567890") {
		t.Errorf("slack token leaked: %q", got.Redacted)
	}
}

func TestLayer1RedactsJWT(t *testing.T) {
	in := "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0In0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	got := New().Redact(in, KindText)
	if contains(got.Redacted, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9") {
		t.Errorf("JWT leaked: %q", got.Redacted)
	}
}

func TestLayer1RedactsURICredentials(t *testing.T) {
	in := "postgres://admin:hunter2supersecret@db.internal:5432/app"
	got := New().Redact(in, KindText)
	if contains(got.Redacted, "hunter2supersecret") {
		t.Errorf("URI password leaked: %q", got.Redacted)
	}
}

func TestLayer1RedactsPEMPrivateKey(t *testing.T) {
	in := "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA\n-----END RSA PRIVATE KEY-----"
	got := New().Redact(in, KindText)
	if contains(got.Redacted, "MIIEpAIBAAKCAQEA") {
		t.Errorf("PEM body leaked: %q", got.Redacted)
	}
}

func TestLayer2RedactsAuthorizationHeader(t *testing.T) {
	in := `curl -H "Authorization: Bearer abc123xyz" https://api.example.com`
	got := New().Redact(in, KindCommand)
	if contains(got.Redacted, "abc123xyz") {
		t.Errorf("bearer token leaked: %q", got.Redacted)
	}
}

func TestLayer2RedactsAPIKeyHeader(t *testing.T) {
	in := `curl -H "X-Api-Key: supersecretkey123" https://api.example.com`
	got := New().Redact(in, KindCommand)
	if contains(got.Redacted, "supersecretkey123") {
		t.Errorf("api key leaked: %q", got.Redacted)
	}
}

func TestLayer2RedactsCurlUserPass(t *testing.T) {
	in := `curl -u admin:hunter2 https://api.example.com`
	got := New().Redact(in, KindCommand)
	if contains(got.Redacted, "hunter2") {
		t.Errorf("curl user:pass leaked: %q", got.Redacted)
	}
}

func TestLayer2RedactsTokenFlag(t *testing.T) {
	in := `deploy --token=abcdefg12345 --env=prod`
	got := New().Redact(in, KindCommand)
	if contains(got.Redacted, "abcdefg12345") {
		t.Errorf("--token= value leaked: %q", got.Redacted)
	}
	if !contains(got.Redacted, "prod") {
		t.Errorf("non-sensitive flag should be preserved: %q", got.Redacted)
	}
}

func TestLayer2RedactsSpaceSeparatedPasswordFlag(t *testing.T) {
	in := `login --password sekret`
	got := New().Redact(in, KindCommand)
	if contains(got.Redacted, "sekret") {
		t.Errorf("--password value leaked: %q", got.Redacted)
	}
}

func TestLayer2RedactsInlineEnvPrefix(t *testing.T) {
	in := `API_KEY=leakme_12345 ./deploy.sh production us-east-1 release rollout staging`
	got := New().Redact(in, KindCommand)
	if contains(got.Redacted, "leakme_12345") {
		t.Errorf("inline env value leaked: %q", got.Redacted)
	}
	if !contains(got.Redacted, "./deploy.sh") {
		t.Errorf("command body should be preserved: %q", got.Redacted)
	}
}

func TestLayer2DoesNotTouchNonSensitiveFlags(t *testing.T) {
	in := `deploy --env=prod --region=us-east-1 --replicas=3`
	got := New().Redact(in, KindCommand)
	if got.Redacted != in {
		t.Errorf("non-sensitive flags were modified: %q", got.Redacted)
	}
}

func TestLayer3RedactsHighEntropyBase64(t *testing.T) {
	in := "secret blob: Zk9Qb1hVazRWdnM5RjE3bUNOZ2hLcw=="
	got := New().Redact(in, KindText)
	if contains(got.Redacted, "Zk9Qb1hVazRWdnM5RjE3bUNOZ2hLcw") {
		t.Errorf("high-entropy blob leaked: %q", got.Redacted)
	}
}

func TestLayer3AllowsFullGitSHA(t *testing.T) {
	in := "commit c0f9f3f7952529deadbeefc0ffee123456789abc landed in master"
	got := New().Redact(in, KindText)
	if !contains(got.Redacted, "c0f9f3f7952529deadbeefc0ffee123456789abc") {
		t.Errorf("full git SHA was wrongly redacted: %q", got.Redacted)
	}
}

func TestLayer3AllowsUUID(t *testing.T) {
	in := "request_id=550e8400-e29b-41d4-a716-446655440000"
	got := New().Redact(in, KindText)
	if !contains(got.Redacted, "550e8400-e29b-41d4-a716-446655440000") {
		t.Errorf("UUID was wrongly redacted: %q", got.Redacted)
	}
}

func TestLayer3LeavesLowEntropyAlone(t *testing.T) {
	in := "deploying version 1.2.3 to production cluster us-east-1"
	got := New().Redact(in, KindText)
	if got.Redacted != in {
		t.Errorf("low-entropy text was modified: %q", got.Redacted)
	}
}

func TestLayer3RespectsMinLength(t *testing.T) {
	in := "token=A7x9K2mQp4Zr"
	got := New().Redact(in, KindText)
	if !contains(got.Redacted, "A7x9K2mQp4Zr") {
		t.Errorf("short high-entropy token was redacted by Layer 3 (should require length >= minEntropyLen): %q", got.Redacted)
	}
}

func TestLayer4RedactsVaultCommand(t *testing.T) {
	in := "vault kv get -format=json secret/stripe/prod"
	got := New().Redact(in, KindCommand)
	if contains(got.Redacted, "secret/stripe/prod") {
		t.Errorf("vault arg leaked: %q", got.Redacted)
	}
	if !contains(got.Redacted, "vault") {
		t.Errorf("program name should be preserved: %q", got.Redacted)
	}
}

func TestLayer4RedactsGPGDecrypt(t *testing.T) {
	in := "gpg --decrypt --output plaintext.txt ciphertext.gpg"
	got := New().Redact(in, KindCommand)
	if contains(got.Redacted, "plaintext.txt") || contains(got.Redacted, "ciphertext.gpg") {
		t.Errorf("gpg decrypt args leaked: %q", got.Redacted)
	}
}

func TestLayer4RedactsAWSConfigure(t *testing.T) {
	in := "aws configure --profile prod"
	got := New().Redact(in, KindCommand)
	if contains(got.Redacted, "--profile prod") {
		t.Errorf("aws configure args leaked: %q", got.Redacted)
	}
}

func TestLayer4PreservesBenignAWSCommand(t *testing.T) {
	in := "aws s3 ls s3://public-bucket/"
	got := New().Redact(in, KindCommand)
	if got.Redacted != in {
		t.Errorf("benign aws command was redacted: %q", got.Redacted)
	}
}

func TestLayer4DoesNotApplyToKindText(t *testing.T) {
	in := "the vault kv get example from docs shows how to read secrets"
	got := New().Redact(in, KindText)
	if got.Redacted != in {
		t.Errorf("text input was treated as command: %q", got.Redacted)
	}
}

func TestLayer5DamageCapTriggersOnCommand(t *testing.T) {
	in := `curl -H "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhYmMifQ.sig" https://user:hunter2supersecret@api.example.com/path?token=ghp_1234567890abcdefghijklmnopqrstuvwxyz`
	got := New().Redact(in, KindCommand)
	if !got.Dropped {
		t.Errorf("expected Dropped=true; got Redacted=%q", got.Redacted)
	}
	if got.Redacted != "curl [REDACTED:damage_cap]" {
		t.Errorf("Redacted = %q; want 'curl [REDACTED:damage_cap]'", got.Redacted)
	}
}

func TestLayer5DamageCapTriggersOnText(t *testing.T) {
	in := "ghp_1234567890abcdefghijklmnopqrstuvwxyz ghp_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	got := New().Redact(in, KindText)
	if !got.Dropped {
		t.Errorf("expected Dropped=true; got Redacted=%q", got.Redacted)
	}
	if got.Redacted != "[REDACTED:damage_cap]" {
		t.Errorf("Redacted = %q; want '[REDACTED:damage_cap]'", got.Redacted)
	}
}

func TestLayer5DoesNotTriggerOnSingleRedaction(t *testing.T) {
	in := "the deployment uses stripe secret key sk_live_4eC39HqLyjWDarjtT1zdp7dcABCDEFGH as documented in internal ops runbook chapter four"
	got := New().Redact(in, KindText)
	if got.Dropped {
		t.Errorf("unexpected Dropped; got %q", got.Redacted)
	}
	if !contains(got.Redacted, "deployment uses stripe") {
		t.Errorf("benign context was dropped: %q", got.Redacted)
	}
}
