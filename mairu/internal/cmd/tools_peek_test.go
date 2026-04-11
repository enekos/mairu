package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestPeekCmd(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "testpeek*.txt")
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("line 1\nfunc myTestFunc() {\n  return 1\n}\nline 5\n")
	tmpFile.Close()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := NewPeekCmd()
	peekLines = ""
	peekSymbol = "myTestFunc"

	cmd.Run(cmd, []string{tmpFile.Name()})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if out == "" {
		t.Errorf("peekCmd output is empty")
	}
}

func TestPeekMultiSymbol(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "testpeekmulti*.go")
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("package main\n\nfunc Alpha() {\n  return\n}\n\nfunc Beta() {\n  return\n}\n")
	tmpFile.Close()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := NewPeekCmd()
	peekLines = ""
	peekSymbol = "Alpha,Beta"
	peekNumbered = false

	cmd.Run(cmd, []string{tmpFile.Name()})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var results []peekResult
	if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
		t.Fatalf("expected JSON array for multi-symbol, got: %s", buf.String())
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestPeekPythonIndent(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "testpeek*.py")
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("def greet(name):\n    print(f'Hello {name}')\n    return True\n\ndef other():\n    pass\n")
	tmpFile.Close()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := NewPeekCmd()
	peekLines = ""
	peekSymbol = "greet"
	peekNumbered = false

	cmd.Run(cmd, []string{tmpFile.Name()})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var res peekResult
	json.Unmarshal(buf.Bytes(), &res)

	if !strings.Contains(res.Content, "greet") {
		t.Errorf("expected content to contain 'greet', got: %s", res.Content)
	}
	if strings.Contains(res.Content, "def other") {
		t.Errorf("Python indent extraction leaked into next function: %s", res.Content)
	}
}
