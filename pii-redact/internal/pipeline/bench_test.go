package pipeline

import (
	"strings"
	"testing"
)

// BenchmarkPipelineText exercises the full pipeline on a realistic 4 KiB
// log-like payload. Target: <200 µs/op on modern hardware.
func BenchmarkPipelineText(b *testing.B) {
	const chunk = `2026-04-22T08:15:42Z INFO request=550e8400-e29b-41d4-a716-446655440000 ` +
		`user=john.doe@acme.io ip=8.8.8.8 endpoint=/api/v1/accounts/42 ` +
		`referrer=https://example.com/path method=GET status=200 duration_ms=123 ` +
		`token=ghp_1234567890abcdefghijklmnopqrstuvwxyz session=abc ` +
		`db=postgres://app:s3cret@db.internal:5432/prod cache_hit=true `

	payload := strings.Repeat(chunk, 4096/len(chunk)+1)[:4096]
	opts := DefaultOptions()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Run(payload, KindText, opts)
	}
}

func BenchmarkPipelineCommand(b *testing.B) {
	cmd := `curl -H "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhYmMifQ.sig" ` +
		`-u admin:hunter2 https://api.example.com/accounts?email=jane@acme.io`
	opts := DefaultOptions()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Run(cmd, KindCommand, opts)
	}
}
