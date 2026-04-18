package ingest

import (
	"bufio"
	"bytes"
	"io"
	"testing"
	"time"
)

func TestEncodeDecodeRoundtrip(t *testing.T) {
	ts := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	want := Record{
		Command:    "go test ./...",
		ExitCode:   0,
		DurationMs: 1234,
		Cwd:        "/home/user/project",
		Timestamp:  ts,
		Project:    "my-project",
		Output:     "ok      mairu/internal/foo       0.123s\n",
	}

	var buf bytes.Buffer
	if err := Encode(&buf, want); err != nil {
		t.Fatalf("Encode: %v", err)
	}

	got, err := Decode(bufio.NewReader(&buf))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if got.Command != want.Command {
		t.Errorf("Command: got %q, want %q", got.Command, want.Command)
	}
	if got.ExitCode != want.ExitCode {
		t.Errorf("ExitCode: got %d, want %d", got.ExitCode, want.ExitCode)
	}
	if got.DurationMs != want.DurationMs {
		t.Errorf("DurationMs: got %d, want %d", got.DurationMs, want.DurationMs)
	}
	if got.Cwd != want.Cwd {
		t.Errorf("Cwd: got %q, want %q", got.Cwd, want.Cwd)
	}
	if !got.Timestamp.Equal(want.Timestamp) {
		t.Errorf("Timestamp: got %v, want %v", got.Timestamp, want.Timestamp)
	}
	if got.Project != want.Project {
		t.Errorf("Project: got %q, want %q", got.Project, want.Project)
	}
	if got.Output != want.Output {
		t.Errorf("Output: got %q, want %q", got.Output, want.Output)
	}
}

func TestDecodeMultipleRecordsInSameStream(t *testing.T) {
	ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	records := []Record{
		{Command: "ls -la", Timestamp: ts, Project: "proj"},
		{Command: "go build .", Timestamp: ts, Project: "proj"},
		{Command: "make test", Timestamp: ts, Project: "proj"},
	}

	var buf bytes.Buffer
	for _, r := range records {
		if err := Encode(&buf, r); err != nil {
			t.Fatalf("Encode: %v", err)
		}
	}

	reader := bufio.NewReader(&buf)

	for i, want := range records {
		got, err := Decode(reader)
		if err != nil {
			t.Fatalf("Decode record %d: %v", i, err)
		}
		if got.Command != want.Command {
			t.Errorf("record %d Command: got %q, want %q", i, got.Command, want.Command)
		}
	}

	// Fourth decode must return io.EOF.
	_, err := Decode(reader)
	if err != io.EOF {
		t.Errorf("fourth Decode: got %v, want io.EOF", err)
	}
}

func TestEncodeEmitsCompactSingleLine(t *testing.T) {
	ts := time.Date(2024, 3, 10, 8, 30, 0, 0, time.UTC)
	rec := Record{
		Command:   "echo hello",
		Timestamp: ts,
	}

	var buf bytes.Buffer
	if err := Encode(&buf, rec); err != nil {
		t.Fatalf("Encode: %v", err)
	}

	b := buf.Bytes()
	if len(b) == 0 {
		t.Fatal("expected non-empty output")
	}

	newlineCount := 0
	for _, c := range b {
		if c == '\n' {
			newlineCount++
		}
	}
	if newlineCount != 1 {
		t.Errorf("expected exactly 1 newline, got %d; output: %q", newlineCount, b)
	}
}

func TestDecodeRejectsMalformedLine(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("not json\n")

	_, err := Decode(bufio.NewReader(&buf))
	if err == nil {
		t.Fatal("expected non-nil error for malformed JSON, got nil")
	}
	if err == io.EOF {
		t.Fatal("expected non-EOF error for malformed JSON, got io.EOF")
	}
}
