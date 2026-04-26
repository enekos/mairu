package agent

import (
	"strings"
	"testing"
)

func TestTruncateHead_NoTruncationNeeded(t *testing.T) {
	in := "a\nb\nc"
	r := TruncateHead(in, 100, 1024)
	if r.Truncated {
		t.Fatalf("expected no truncation, got truncated=%v by=%s", r.Truncated, r.TruncatedBy)
	}
	if r.Content != in {
		t.Fatalf("content mutated: %q", r.Content)
	}
	if r.OutputLines != 3 || r.TotalLines != 3 {
		t.Fatalf("line counts off: out=%d total=%d", r.OutputLines, r.TotalLines)
	}
}

func TestTruncateHead_LineLimit(t *testing.T) {
	in := strings.Repeat("x\n", 50) // 51 lines (final empty)
	r := TruncateHead(in, 10, 1024)
	if !r.Truncated || r.TruncatedBy != "lines" {
		t.Fatalf("expected lines truncation, got truncated=%v by=%s", r.Truncated, r.TruncatedBy)
	}
	if r.OutputLines != 10 {
		t.Fatalf("expected 10 output lines, got %d", r.OutputLines)
	}
	// Result must not contain partial lines: every kept line is "x".
	for i, line := range strings.Split(r.Content, "\n") {
		if line != "x" {
			t.Fatalf("line %d not whole: %q", i, line)
		}
	}
}

func TestTruncateHead_ByteLimit(t *testing.T) {
	long := strings.Repeat("a", 200)
	in := long + "\n" + long + "\n" + long
	r := TruncateHead(in, 100, 250)
	if !r.Truncated || r.TruncatedBy != "bytes" {
		t.Fatalf("expected bytes truncation, got truncated=%v by=%s", r.Truncated, r.TruncatedBy)
	}
	if r.OutputBytes > 250 {
		t.Fatalf("output exceeds budget: %d", r.OutputBytes)
	}
}

func TestTruncateTail_KeepsEnd(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 100; i++ {
		b.WriteString("line\n")
	}
	b.WriteString("FINAL")
	r := TruncateTail(b.String(), 5, 1024)
	if !r.Truncated {
		t.Fatalf("expected truncation")
	}
	if !strings.HasSuffix(r.Content, "FINAL") {
		t.Fatalf("tail truncation must keep the last line, got %q", r.Content)
	}
}

func TestTruncateTail_SingleHugeLine(t *testing.T) {
	in := strings.Repeat("z", 5000)
	r := TruncateTail(in, 100, 100)
	if !r.Truncated || r.TruncatedBy != "bytes" {
		t.Fatalf("expected bytes truncation, got %s", r.TruncatedBy)
	}
	if len(r.Content) > 100 {
		t.Fatalf("partial line exceeds byte budget: %d", len(r.Content))
	}
}

func TestTruncateLine(t *testing.T) {
	short, was := TruncateLine("hello", 100)
	if was || short != "hello" {
		t.Fatalf("short line should pass through: %q was=%v", short, was)
	}
	long := strings.Repeat("a", 600)
	out, was := TruncateLine(long, 500)
	if !was {
		t.Fatalf("expected truncation flag")
	}
	if !strings.HasSuffix(out, "[truncated]") {
		t.Fatalf("missing truncation suffix: %q", out)
	}
}

func TestFormatSize(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{500, "500B"},
		{2048, "2.0KB"},
		{5 * 1024 * 1024, "5.0MB"},
	}
	for _, tc := range cases {
		if got := FormatSize(tc.in); got != tc.want {
			t.Errorf("FormatSize(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatTruncationNote(t *testing.T) {
	r := TruncateResult{Truncated: false}
	if FormatTruncationNote(r, "head") != "" {
		t.Fatal("no note expected when not truncated")
	}
	r = TruncateResult{
		Truncated: true, TruncatedBy: "lines",
		OutputLines: 10, TotalLines: 100,
		OutputBytes: 100, TotalBytes: 1000,
	}
	note := FormatTruncationNote(r, "head")
	if !strings.Contains(note, "10/100 lines") || !strings.Contains(note, "Use offset/limit") {
		t.Fatalf("unexpected head note: %q", note)
	}
	tailNote := FormatTruncationNote(r, "tail")
	if !strings.Contains(tailNote, "Earlier output was dropped") {
		t.Fatalf("unexpected tail note: %q", tailNote)
	}
}
