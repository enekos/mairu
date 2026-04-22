package pipeline

import (
	"strings"
	"testing"
)

func run(in string, kind Kind) Result {
	return Run(in, kind, DefaultOptions())
}

func contains(h, n string) bool { return n != "" && strings.Contains(h, n) }

func TestEmptyInputIsSafe(t *testing.T) {
	got := run("", KindText)
	if got.Redacted != "" {
		t.Errorf("Redacted = %q; want empty", got.Redacted)
	}
	if !got.EmbeddingSafe {
		t.Error("empty input must be embedding-safe")
	}
	if got.Dropped {
		t.Error("empty input must not be dropped")
	}
}

func TestPlainTextPassesThrough(t *testing.T) {
	got := run("hello world this is fine", KindText)
	if got.Redacted != "hello world this is fine" {
		t.Errorf("Redacted = %q; want pass-through", got.Redacted)
	}
}

func TestLayer0DotenvPair(t *testing.T) {
	in := "DATABASE_PASSWORD=hunter2supersecret"
	got := run(in, KindText)
	if contains(got.Redacted, "hunter2supersecret") {
		t.Errorf(".env value leaked: %q", got.Redacted)
	}
}

func TestLayer0YamlPair(t *testing.T) {
	in := "api_key: superSecretHardcoded123"
	got := run(in, KindText)
	if contains(got.Redacted, "superSecretHardcoded123") {
		t.Errorf("yaml value leaked: %q", got.Redacted)
	}
}

func TestLayer0ConnURI(t *testing.T) {
	in := "postgres://admin:hunter2supersecret@db.internal:5432/app"
	got := run(in, KindText)
	if contains(got.Redacted, "hunter2supersecret") {
		t.Errorf("URI password leaked: %q", got.Redacted)
	}
}

func TestLayer1GitHubPAT(t *testing.T) {
	in := "token=ghp_1234567890abcdefghijklmnopqrstuvwxyz"
	got := run(in, KindText)
	if contains(got.Redacted, "ghp_1234567890") {
		t.Errorf("raw PAT leaked: %q", got.Redacted)
	}
	if got.EmbeddingSafe {
		t.Error("Layer 1 hit must set EmbeddingSafe=false")
	}
}

func TestLayer1AWSAccessKey(t *testing.T) {
	in := "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE"
	got := run(in, KindText)
	if contains(got.Redacted, "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("AWS key leaked: %q", got.Redacted)
	}
}

func TestLayer1Anthropic(t *testing.T) {
	in := "ANTHROPIC_API_KEY=sk-ant-" + strings.Repeat("a", 95)
	got := run(in, KindText)
	if contains(got.Redacted, strings.Repeat("a", 95)) {
		t.Errorf("anthropic key leaked: %q", got.Redacted)
	}
}

func TestLayer1OpenAIProject(t *testing.T) {
	in := "OPENAI_API_KEY=sk-proj-" + strings.Repeat("X", 50)
	got := run(in, KindText)
	if contains(got.Redacted, strings.Repeat("X", 50)) {
		t.Errorf("openai key leaked: %q", got.Redacted)
	}
}

func TestLayer1JWT(t *testing.T) {
	in := "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0In0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	got := run(in, KindText)
	if contains(got.Redacted, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9") {
		t.Errorf("JWT leaked: %q", got.Redacted)
	}
}

func TestLayer1PEM(t *testing.T) {
	in := "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA\n-----END RSA PRIVATE KEY-----"
	got := run(in, KindText)
	if contains(got.Redacted, "MIIEpAIBAAKCAQEA") {
		t.Errorf("PEM body leaked: %q", got.Redacted)
	}
}

func TestLayer2AuthorizationHeader(t *testing.T) {
	in := `curl -H "Authorization: Bearer abc123xyz" https://api.example.com`
	got := run(in, KindCommand)
	if contains(got.Redacted, "abc123xyz") {
		t.Errorf("bearer token leaked: %q", got.Redacted)
	}
}

func TestLayer2CurlUserPass(t *testing.T) {
	in := `curl -u admin:hunter2 https://api.example.com`
	got := run(in, KindCommand)
	if contains(got.Redacted, "hunter2") {
		t.Errorf("curl user:pass leaked: %q", got.Redacted)
	}
}

func TestLayer2TokenFlag(t *testing.T) {
	in := `deploy --token=abcdefg12345 --env=prod`
	got := run(in, KindCommand)
	if contains(got.Redacted, "abcdefg12345") {
		t.Errorf("--token= value leaked: %q", got.Redacted)
	}
	if !contains(got.Redacted, "prod") {
		t.Errorf("non-sensitive flag should be preserved: %q", got.Redacted)
	}
}

func TestLayer2InlineEnvPrefix(t *testing.T) {
	in := `API_KEY=leakme_12345 ./deploy.sh production us-east-1 release rollout staging`
	got := run(in, KindCommand)
	if contains(got.Redacted, "leakme_12345") {
		t.Errorf("inline env value leaked: %q", got.Redacted)
	}
}

func TestLayer2DoesNotTouchNonSensitive(t *testing.T) {
	in := `deploy --env=prod --region=us-east-1 --replicas=3`
	got := run(in, KindCommand)
	if got.Redacted != in {
		t.Errorf("non-sensitive flags were modified: %q", got.Redacted)
	}
}

func TestLayer3Email(t *testing.T) {
	in := "Support: john.doe@acme.io for questions"
	got := run(in, KindText)
	if contains(got.Redacted, "john.doe@acme.io") {
		t.Errorf("email leaked: %q", got.Redacted)
	}
	if !contains(got.Redacted, "Support:") {
		t.Errorf("surrounding text lost: %q", got.Redacted)
	}
}

func TestLayer3PublicIPv4Redacted(t *testing.T) {
	in := "peer 8.8.8.8 responded in 12ms"
	got := run(in, KindText)
	if contains(got.Redacted, "8.8.8.8") {
		t.Errorf("public IP leaked: %q", got.Redacted)
	}
}

func TestLayer3PrivateIPv4Preserved(t *testing.T) {
	in := "listening on 10.0.0.5 and 192.168.1.100 and 127.0.0.1"
	got := run(in, KindText)
	if !contains(got.Redacted, "10.0.0.5") {
		t.Errorf("private IP was redacted: %q", got.Redacted)
	}
	if !contains(got.Redacted, "192.168.1.100") {
		t.Errorf("rfc1918 IP was redacted: %q", got.Redacted)
	}
	if !contains(got.Redacted, "127.0.0.1") {
		t.Errorf("loopback was redacted: %q", got.Redacted)
	}
}

// Note: `1.2.3.4` (semver-looking public IPv4) gets redacted — this is a
// known false-positive we accept in favor of erring on the side of
// scrubbing. Callers that need semver preservation should use an explicit
// allowlist in their config.

func TestLayer3LuhnCreditCard(t *testing.T) {
	in := "card 4111 1111 1111 1111 expires 12/29"
	got := run(in, KindText)
	if contains(got.Redacted, "4111 1111 1111 1111") {
		t.Errorf("CC leaked: %q", got.Redacted)
	}
}

func TestLayer3RejectsNonLuhn(t *testing.T) {
	in := "order 1234 5678 9012 3456 placed"
	got := run(in, KindText)
	if !contains(got.Redacted, "1234 5678 9012 3456") {
		t.Errorf("non-Luhn redacted as CC: %q", got.Redacted)
	}
}

func TestLayer4HighEntropyBase64(t *testing.T) {
	in := "secret blob: Zk9Qb1hVazRWdnM5RjE3bUNOZ2hLcw=="
	got := run(in, KindText)
	if contains(got.Redacted, "Zk9Qb1hVazRWdnM5RjE3bUNOZ2hLcw") {
		t.Errorf("high-entropy blob leaked: %q", got.Redacted)
	}
}

func TestLayer4AllowsFullGitSHA(t *testing.T) {
	in := "commit c0f9f3f7952529deadbeefc0ffee123456789abc landed in master"
	got := run(in, KindText)
	if !contains(got.Redacted, "c0f9f3f7952529deadbeefc0ffee123456789abc") {
		t.Errorf("full git SHA was wrongly redacted: %q", got.Redacted)
	}
}

func TestLayer4AllowsUUID(t *testing.T) {
	in := "request_id=550e8400-e29b-41d4-a716-446655440000"
	got := run(in, KindText)
	if !contains(got.Redacted, "550e8400-e29b-41d4-a716-446655440000") {
		t.Errorf("UUID was wrongly redacted: %q", got.Redacted)
	}
}

func TestLayer5VaultCommand(t *testing.T) {
	in := "vault kv get -format=json secret/stripe/prod"
	got := run(in, KindCommand)
	if contains(got.Redacted, "secret/stripe/prod") {
		t.Errorf("vault arg leaked: %q", got.Redacted)
	}
	if !contains(got.Redacted, "vault") {
		t.Errorf("program name should be preserved: %q", got.Redacted)
	}
}

func TestLayer5PreservesBenignAWS(t *testing.T) {
	in := "aws s3 ls s3://public-bucket/"
	got := run(in, KindCommand)
	if got.Redacted != in {
		t.Errorf("benign aws command was redacted: %q", got.Redacted)
	}
}

func TestLayer5DoesNotApplyToText(t *testing.T) {
	in := "the vault kv get example from docs shows how to read secrets"
	got := run(in, KindText)
	if got.Redacted != in {
		t.Errorf("text input was treated as command: %q", got.Redacted)
	}
}

func TestDamageCapOnCommand(t *testing.T) {
	in := `curl -H "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhYmMifQ.sig" https://user:hunter2supersecret@api.example.com/path?token=ghp_1234567890abcdefghijklmnopqrstuvwxyz`
	got := run(in, KindCommand)
	if !got.Dropped {
		t.Errorf("expected Dropped=true; got Redacted=%q", got.Redacted)
	}
	if !strings.HasPrefix(got.Redacted, "curl ") {
		t.Errorf("Redacted should keep program name: %q", got.Redacted)
	}
}

func TestPanicRecovery(t *testing.T) {
	// Run a huge input to exercise the defer-recover (not an actual panic
	// trigger, but ensures the function returns cleanly under stress).
	in := strings.Repeat("A", 200000)
	got := run(in, KindText)
	if got.Redacted == "" {
		t.Error("expected non-empty output for non-panic path")
	}
}
