package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMapCmd(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputFormat = "json"
	cmd := NewMapCmd()
	cmd.Run(cmd, []string{"."})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if out == "" {
		t.Errorf("mapCmd output is empty")
	}
}

func TestSysCmd(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputFormat = "json"
	cmd := NewSysCmd()
	cmd.Run(cmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if out == "" {
		t.Errorf("sysCmd output is empty")
	}
}

func TestInfoCmd(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputFormat = "json"
	cmd := NewInfoCmd()
	cmd.Run(cmd, []string{"."})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if out == "" {
		t.Errorf("infoCmd output is empty")
	}
}

func TestEnvCmd(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "testenv*.env")
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("API_KEY=secret_hash_here\nexport PORT=8080\nDEBUG=true\nURL=http://localhost:3000\n# comment\n")
	tmpFile.Close()

	// Run 1: Normal mode (just keys)
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := NewEnvCmd()
	envReveal = false
	envPattern = ""
	cmd.Run(cmd, []string{tmpFile.Name()})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if out == "" {
		t.Errorf("envCmd output is empty")
	}
	if !bytes.Contains(buf.Bytes(), []byte("API_KEY")) || !bytes.Contains(buf.Bytes(), []byte("PORT")) {
		t.Errorf("envCmd output missing keys: %s", out)
	}

	// Run 2: Reveal mode
	r, w, _ = os.Pipe()
	os.Stdout = w
	cmd = NewEnvCmd()
	envReveal = true
	cmd.Run(cmd, []string{tmpFile.Name()})
	w.Close()
	os.Stdout = oldStdout
	var buf2 bytes.Buffer
	buf2.ReadFrom(r)
	out2 := buf2.String()

	if !bytes.Contains(buf2.Bytes(), []byte(`"val":"true"`)) {
		t.Errorf("envCmd reveal failed for safe boolean: %s", out2)
	}
	if !bytes.Contains(buf2.Bytes(), []byte(`"val":"8080"`)) {
		t.Errorf("envCmd reveal failed for safe short string: %s", out2)
	}
	if bytes.Contains(buf2.Bytes(), []byte("secret_hash_here")) {
		t.Errorf("envCmd LEAKED A SECRET: %s", out2)
	}
}

func TestInfoStructuredOutput(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n\nfunc main() {\n}\n"), 0644)
	os.WriteFile(filepath.Join(dir, "b.ts"), []byte("export function hello() {\n  return 1\n}\n"), 0644)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := NewInfoCmd()
	infoTop = 0
	infoExtensions = ""
	outputFormat = "json"
	cmd.Run(cmd, []string{dir})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var raw map[string]interface{}
	json.Unmarshal(buf.Bytes(), &raw)

	if _, ok := raw["lines"]; !ok {
		t.Errorf("expected 'lines' field in output, got: %s", buf.String())
	}

	langs, ok := raw["languages"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected languages map, got: %s", buf.String())
	}
	goLang, ok := langs[".go"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected .go to be an object, got: %v", langs[".go"])
	}
	if _, ok := goLang["files"]; !ok {
		t.Errorf("expected 'files' in language entry")
	}
	if _, ok := goLang["pct"]; !ok {
		t.Errorf("expected 'pct' in language entry")
	}
}

func TestInfoTop(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "small.go"), []byte("x\n"), 0644)
	os.WriteFile(filepath.Join(dir, "big.go"), []byte(strings.Repeat("x\n", 1000)), 0644)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := NewInfoCmd()
	infoTop = 1
	infoExtensions = ""
	outputFormat = "json"
	cmd.Run(cmd, []string{dir})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var raw map[string]interface{}
	json.Unmarshal(buf.Bytes(), &raw)

	top, ok := raw["top"].([]interface{})
	if !ok || len(top) != 1 {
		t.Fatalf("expected top array with 1 entry, got: %s", buf.String())
	}
	entry := top[0].(map[string]interface{})
	if entry["p"] != "big.go" {
		t.Errorf("expected big.go as top file, got %v", entry["p"])
	}
}

func TestMapExtFilter(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0644)
	os.WriteFile(filepath.Join(dir, "b.ts"), []byte("export {}\n"), 0644)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := NewMapCmd()
	mapDepth = 0
	mapExtensions = ".go"
	mapMin = 0
	mapSort = ""
	mapDirs = false
	outputFormat = "json"
	cmd.Run(cmd, []string{dir})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var entries []mapEntry
	json.Unmarshal(buf.Bytes(), &entries)

	if len(entries) != 1 {
		t.Errorf("expected 1 .go file, got %d", len(entries))
	}
	if len(entries) > 0 && !strings.HasSuffix(entries[0].P, ".go") {
		t.Errorf("expected .go file, got %s", entries[0].P)
	}
}

func TestMapSortSize(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "small.go"), []byte("x\n"), 0644)
	os.WriteFile(filepath.Join(dir, "big.go"), []byte(strings.Repeat("x", 1000)), 0644)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := NewMapCmd()
	mapDepth = 0
	mapExtensions = ""
	mapMin = 0
	mapSort = "size"
	mapDirs = false
	outputFormat = "json"
	cmd.Run(cmd, []string{dir})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var entries []mapEntry
	json.Unmarshal(buf.Bytes(), &entries)

	if len(entries) < 2 {
		t.Fatalf("expected at least 2 entries, got %d", len(entries))
	}
	if entries[0].T < entries[1].T {
		t.Errorf("expected descending sort by size, got %d then %d", entries[0].T, entries[1].T)
	}
}

func TestMapDirs(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(sub, "a.go"), []byte("package main\n"), 0644)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := NewMapCmd()
	mapDepth = 0
	mapExtensions = ""
	mapMin = 0
	mapSort = ""
	mapDirs = true
	outputFormat = "json"
	cmd.Run(cmd, []string{dir})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var entries []map[string]interface{}
	json.Unmarshal(buf.Bytes(), &entries)

	hasDirEntry := false
	for _, e := range entries {
		if d, ok := e["d"]; ok && d == true {
			hasDirEntry = true
		}
	}
	if !hasDirEntry {
		t.Errorf("expected directory entry with 'd' flag, got: %s", buf.String())
	}
}

func TestEnvDiff(t *testing.T) {
	f1, _ := os.CreateTemp("", "env1*.env")
	defer os.Remove(f1.Name())
	f1.WriteString("PORT=3000\nDEBUG=true\nOLD_KEY=val\n")
	f1.Close()

	f2, _ := os.CreateTemp("", "env2*.env")
	defer os.Remove(f2.Name())
	f2.WriteString("PORT=8080\nDEBUG=true\nNEW_KEY=val\n")
	f2.Close()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := NewEnvCmd()
	envReveal = false
	envPattern = ""
	envDiff = f2.Name()
	envRequired = ""
	cmd.Run(cmd, []string{f1.Name()})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var raw map[string]interface{}
	json.Unmarshal(buf.Bytes(), &raw)

	if _, ok := raw["added"]; !ok {
		t.Errorf("expected 'added' field in diff output, got: %s", buf.String())
	}
	if _, ok := raw["removed"]; !ok {
		t.Errorf("expected 'removed' field in diff output")
	}

	added := raw["added"].([]interface{})
	if len(added) != 1 || added[0] != "NEW_KEY" {
		t.Errorf("expected [NEW_KEY] added, got %v", added)
	}
}

func TestEnvRequired(t *testing.T) {
	f, _ := os.CreateTemp("", "envreq*.env")
	defer os.Remove(f.Name())
	f.WriteString("PORT=3000\nDEBUG=true\n")
	f.Close()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := NewEnvCmd()
	envReveal = false
	envPattern = ""
	envDiff = ""
	envRequired = "PORT,DEBUG"

	cmd.Run(cmd, []string{f.Name()})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var raw map[string]interface{}
	json.Unmarshal(buf.Bytes(), &raw)

	if ok, exists := raw["ok"]; !exists || ok != true {
		t.Errorf("expected ok=true when all required keys present, got: %s", buf.String())
	}
}

func TestEnvMultiFile(t *testing.T) {
	f1, _ := os.CreateTemp("", "env1*.env")
	defer os.Remove(f1.Name())
	f1.WriteString("PORT=3000\nDEBUG=false\n")
	f1.Close()

	f2, _ := os.CreateTemp("", "env2*.env")
	defer os.Remove(f2.Name())
	f2.WriteString("DEBUG=true\nNEW_VAR=hello\n")
	f2.Close()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := NewEnvCmd()
	envReveal = true
	envPattern = ""
	envDiff = ""
	envRequired = ""
	cmd.Run(cmd, []string{f1.Name(), f2.Name()})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var res envResult
	json.Unmarshal(buf.Bytes(), &res)

	// DEBUG should be overridden to "true" from f2
	found := false
	for _, v := range res.Vars {
		if v.Key == "DEBUG" && v.Value == "true" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected DEBUG=true (overridden by second file), got: %s", buf.String())
	}

	// Should have 3 unique keys
	if len(res.Vars) != 3 {
		t.Errorf("expected 3 merged vars, got %d: %s", len(res.Vars), buf.String())
	}
}

func TestSysEnhanced(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputFormat = "json"
	cmd := NewSysCmd()
	cmd.Run(cmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var raw map[string]interface{}
	json.Unmarshal(buf.Bytes(), &raw)

	if _, ok := raw["go_version"]; !ok {
		t.Errorf("expected 'go_version' field, got: %s", buf.String())
	}
	if _, ok := raw["disk_free_gb"]; !ok {
		t.Errorf("expected 'disk_free_gb' field, got: %s", buf.String())
	}
	if _, ok := raw["docker"]; !ok {
		t.Errorf("expected 'docker' field, got: %s", buf.String())
	}
}
