package cmd

import (
	"bytes"
	"testing"
)

func TestCappedWriter_BelowLimit(t *testing.T) {
	var buf bytes.Buffer
	c := &cappedWriter{buf: &buf, limit: 100}
	n, err := c.Write([]byte("hello world"))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if n != 11 {
		t.Errorf("n = %d; want 11", n)
	}
	if buf.String() != "hello world" {
		t.Errorf("buf = %q; want 'hello world'", buf.String())
	}
	if c.capped {
		t.Error("capped should be false below limit")
	}
}

func TestCappedWriter_AtLimit_Truncates(t *testing.T) {
	var buf bytes.Buffer
	c := &cappedWriter{buf: &buf, limit: 5}
	n, err := c.Write([]byte("hello world"))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	// Writer MUST report the full length — upstream is io.MultiWriter
	// fanning out to the real stdout/stderr too, and a short write would
	// abort the fanout.
	if n != 11 {
		t.Errorf("n = %d; want 11 (must swallow the overflow)", n)
	}
	if buf.String() != "hello" {
		t.Errorf("buf = %q; want 'hello'", buf.String())
	}
	if !c.capped {
		t.Error("capped should be true after hitting limit")
	}
}

func TestCappedWriter_AfterCap_SwallowsSilently(t *testing.T) {
	var buf bytes.Buffer
	c := &cappedWriter{buf: &buf, limit: 3}
	c.Write([]byte("abc")) // fills the buffer exactly
	n, err := c.Write([]byte("def"))
	if err != nil {
		t.Fatalf("second write: %v", err)
	}
	if n != 3 {
		t.Errorf("n = %d; want 3", n)
	}
	if buf.String() != "abc" {
		t.Errorf("buf = %q; want 'abc' (no new content after cap)", buf.String())
	}
}

func TestCappedWriter_ZeroLimit_IsNoOp(t *testing.T) {
	// A limit of 0 means "cap immediately"; every write is swallowed.
	var buf bytes.Buffer
	c := &cappedWriter{buf: &buf, limit: 0}
	n, err := c.Write([]byte("anything"))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if n != 8 {
		t.Errorf("n = %d; want 8", n)
	}
	if buf.Len() != 0 {
		t.Errorf("buf = %q; want empty", buf.String())
	}
}

func TestJoinArgs_SimpleArgv(t *testing.T) {
	got := joinArgs([]string{"go", "test", "./..."})
	want := "go test ./..."
	if got != want {
		t.Errorf("joinArgs = %q; want %q", got, want)
	}
}

func TestJoinArgs_EmptySliceReturnsEmpty(t *testing.T) {
	if got := joinArgs(nil); got != "" {
		t.Errorf("joinArgs(nil) = %q; want empty", got)
	}
}
