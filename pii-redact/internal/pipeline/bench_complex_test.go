package pipeline

import (
	"bytes"
	"strings"
	"testing"

	"github.com/enekos/mairu/pii-redact/internal/config"
	"github.com/enekos/mairu/pii-redact/internal/mask"
	"github.com/enekos/mairu/pii-redact/internal/patterns"
	"github.com/enekos/mairu/pii-redact/internal/walkers"
)

// benchChunk is a realistic log-like line with mixed PII.
const benchChunk = `2026-04-22T08:15:42Z INFO request=550e8400-e29b-41d4-a716-446655440000 ` +
	`user=john.doe@acme.io ip=8.8.8.8 endpoint=/api/v1/accounts/42 ` +
	`referrer=https://example.com/path method=GET status=200 duration_ms=123 ` +
	`token=ghp_1234567890abcdefghijklmnopqrstuvwxyz session=abc ` +
	`db=postgres://app:s3cret@db.internal:5432/prod cache_hit=true ` +
	`card=4111 1111 1111 1111 iban=DE89370400440532013000 ` +
	`ssn=123-45-6789 phone=+14155551234 jwt=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhYmMifQ.sig ` +
	`aws=AKIAIOSFODNN7EXAMPLE gcp=AIzaSyABCDEFGHIJKLMNOPQRSTUVWXYZ1234567 ` +
	`slack=xoxb-1234567890-abcdefghij stripe=sk_live_abcdefghijklmnopqrstuv `

func makePayload(size int) string {
	return strings.Repeat(benchChunk, size/len(benchChunk)+1)[:size]
}

func BenchmarkPipelineText_512B(b *testing.B) {
	payload := makePayload(512)
	opts := DefaultOptions()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Run(payload, KindText, opts)
	}
}

func BenchmarkPipelineText_4KB(b *testing.B) {
	payload := makePayload(4096)
	opts := DefaultOptions()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Run(payload, KindText, opts)
	}
}

func BenchmarkPipelineText_64KB(b *testing.B) {
	payload := makePayload(64 * 1024)
	opts := DefaultOptions()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Run(payload, KindText, opts)
	}
}

func BenchmarkPipelineText_1MB(b *testing.B) {
	payload := makePayload(1024 * 1024)
	opts := DefaultOptions()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Run(payload, KindText, opts)
	}
}

// BenchmarkPipelineText_Clean measures overhead on text with zero PII matches.
func BenchmarkPipelineText_Clean(b *testing.B) {
	payload := strings.Repeat("hello world this is a clean log message with no secrets at all ", 64)
	opts := DefaultOptions()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Run(payload, KindText, opts)
	}
}

// BenchmarkPipelineText_DensePII measures worst-case with many matches per KB.
func BenchmarkPipelineText_DensePII(b *testing.B) {
	chunk := "email=alice@ex.io ip=9.9.9.9 card=4111 1111 1111 1111 jwt=eyJhb.a.b ssn=123-45-6789 "
	payload := strings.Repeat(chunk, 4096/len(chunk)+1)[:4096]
	opts := DefaultOptions()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Run(payload, KindText, opts)
	}
}

func BenchmarkPipelineCommand_Simple(b *testing.B) {
	cmd := `curl -H "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhYmMifQ.sig" ` +
		`-u admin:hunter2 https://api.example.com/accounts?email=jane@acme.io`
	opts := DefaultOptions()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Run(cmd, KindCommand, opts)
	}
}

func BenchmarkPipelineCommand_Complex(b *testing.B) {
	cmd := `docker run -e DATABASE_PASSWORD=supersecret123 -e API_KEY=sk-live-abc ` +
		`-v /host:/container --token=ghp_1234567890abcdefghijklmnopqrstuvwxyz ` +
		`myimage /bin/sh -c "curl -u admin:hunter2 https://api.example.com"`
	opts := DefaultOptions()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Run(cmd, KindCommand, opts)
	}
}

// BenchmarkJSONWalker_1KB exercises the JSON walker on a small object.
func BenchmarkJSONWalker_1KB(b *testing.B) {
	input := []byte(`{
  "timestamp": "2026-04-22T08:15:42Z",
  "traceId": "550e8400-e29b-41d4-a716-446655440000",
  "user": "john.doe@acme.io",
  "ip": "8.8.8.8",
  "token": "ghp_1234567890abcdefghijklmnopqrstuvwxyz",
  "message": "request completed"
}`)
	rules, _ := config.Load(config.LoadOptions{ConfigDirs: []string{"../../testdata/configs/default"}})
	set, _ := patterns.Compile(rules.ContentPatterns)
	opts := walkers.Options{
		Rules:     rules,
		Set:       set,
		Strict:    true,
		Masker:    mask.NewMasker(true),
		ServiceOf: func(e any) string { return "" },
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out bytes.Buffer
		walkers.JSON(bytes.NewReader(input), &out, opts)
	}
}

// BenchmarkJSONWalker_100KB exercises the JSON walker on a larger array.
func BenchmarkJSONWalker_100KB(b *testing.B) {
	entry := `{"timestamp":"2026-04-22T08:15:42Z","traceId":"550e8400-e29b-41d4-a716-446655440000","user":"john.doe@acme.io","ip":"8.8.8.8","token":"ghp_1234567890abcdefghijklmnopqrstuvwxyz","message":"request completed","status":"200"}`
	arr := "[" + strings.Repeat(entry+",", 99) + entry + "]"
	input := []byte(arr)
	rules, _ := config.Load(config.LoadOptions{ConfigDirs: []string{"../../testdata/configs/default"}})
	set, _ := patterns.Compile(rules.ContentPatterns)
	opts := walkers.Options{
		Rules:     rules,
		Set:       set,
		Strict:    true,
		Masker:    mask.NewMasker(true),
		ServiceOf: func(e any) string { return "" },
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out bytes.Buffer
		walkers.JSON(bytes.NewReader(input), &out, opts)
	}
}
