package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestOutlineStructuredSymbols(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "testoutline*.go")
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("package main\n\nfunc Hello() {}\n\nfunc goodbye() {}\n")
	tmpFile.Close()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := NewOutlineCmd()
	outlineExports = false
	outlineTree = false
	outputFormat = "json"
	_ = cmd.RunE(cmd, []string{tmpFile.Name()})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var raw map[string]interface{}
	json.Unmarshal(buf.Bytes(), &raw)

	syms, ok := raw["symbols"].([]interface{})
	if !ok {
		t.Fatalf("expected symbols array, got: %s", buf.String())
	}
	if len(syms) < 1 {
		t.Fatalf("expected at least 1 symbol")
	}

	first := syms[0].(map[string]interface{})
	if _, ok := first["kind"]; !ok {
		t.Errorf("expected 'kind' field in symbol")
	}
	if _, ok := first["name"]; !ok {
		t.Errorf("expected 'name' field in symbol")
	}
	if _, ok := first["l"]; !ok {
		t.Errorf("expected 'l' (line) field in symbol")
	}
}

func TestOutlineExportsFilter(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "testoutline*.go")
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("package main\n\nfunc Hello() {}\n\nfunc goodbye() {}\n")
	tmpFile.Close()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := NewOutlineCmd()
	outlineExports = true
	outlineTree = false
	outputFormat = "json"
	_ = cmd.RunE(cmd, []string{tmpFile.Name()})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var raw map[string]interface{}
	json.Unmarshal(buf.Bytes(), &raw)

	syms := raw["symbols"].([]interface{})
	for _, s := range syms {
		sym := s.(map[string]interface{})
		if name, ok := sym["name"].(string); ok {
			if name == "goodbye" {
				t.Errorf("unexported symbol 'goodbye' should be filtered out")
			}
		}
	}
}
