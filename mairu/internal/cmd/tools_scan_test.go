package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func resetScanFlags() {
	outputFormat = "json"
	scanBudget = 3000
	scanContext = 0
	scanExtensions = ""
	scanLimit = 0
	scanIgnoreCase = false
	scanFilesOnly = false
	scanHeading = false
	scanExclude = ""
	scanGroup = false
	scanInvert = false
	scanMulti = ""
	scanFixedStrings = false
	scanSmartCase = false
	scanWordRegexp = false
	scanOnlyMatching = false
}

func runScanCmd(t *testing.T, args ...string) scanResult {
	t.Helper()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	scanCmd.Run(scanCmd, args)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var res scanResult
	json.Unmarshal(buf.Bytes(), &res)
	return res
}

func TestScanExclude(t *testing.T) {
	resetScanFlags()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("func Hello() {}"), 0644)
	os.MkdirAll(filepath.Join(dir, "vendor"), 0755)
	os.WriteFile(filepath.Join(dir, "vendor", "lib.go"), []byte("func Hello() {}"), 0644)

	scanExclude = "vendor/*"
	res := runScanCmd(t, "Hello", dir)

	if res.Total != 1 {
		t.Errorf("expected 1 match (vendor excluded), got %d", res.Total)
	}
	for _, m := range res.Matches {
		if m.F == "vendor/lib.go" {
			t.Errorf("vendor file should have been excluded")
		}
	}
}

func TestScanInvert(t *testing.T) {
	resetScanFlags()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("alpha\nbeta\ngamma"), 0644)

	scanInvert = true
	res := runScanCmd(t, "beta", dir)

	// "alpha" and "gamma" should match (lines NOT containing "beta")
	if res.Total != 2 {
		t.Errorf("expected 2 inverted matches, got %d", res.Total)
	}
	for _, m := range res.Matches {
		if m.C == "beta" {
			t.Errorf("inverted match should not contain 'beta'")
		}
	}
}

func TestScanGroup(t *testing.T) {
	resetScanFlags()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("Hello\nWorld\n"), 0644)
	os.WriteFile(filepath.Join(dir, "b.go"), []byte("Hello\nBye\n"), 0644)

	scanGroup = true
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	scanCmd.Run(scanCmd, []string{"Hello", dir})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var raw map[string]interface{}
	json.Unmarshal(buf.Bytes(), &raw)

	grouped, ok := raw["grouped"]
	if !ok {
		t.Fatalf("expected 'grouped' key in output, got: %s", buf.String())
	}
	gmap, ok := grouped.(map[string]interface{})
	if !ok || len(gmap) != 2 {
		t.Errorf("expected 2 file groups, got %v", gmap)
	}
}

func TestScanMulti(t *testing.T) {
	resetScanFlags()
	dir := t.TempDir()
	// File with both patterns
	os.WriteFile(filepath.Join(dir, "auth.go"), []byte("func HandleAuth() {\n  token := validate()\n}\n"), 0644)
	// File with only one pattern
	os.WriteFile(filepath.Join(dir, "other.go"), []byte("func HandleOther() {\n  return\n}\n"), 0644)

	scanMulti = "validate"
	res := runScanCmd(t, "Handle", dir)

	// Only auth.go has both "Handle" and "validate"
	if res.Total != 1 {
		t.Errorf("expected 1 match (only file with both patterns), got %d", res.Total)
	}
	if len(res.Matches) > 0 && res.Matches[0].F != "auth.go" {
		t.Errorf("expected match in auth.go, got %s", res.Matches[0].F)
	}
}

func TestScanFixedStrings(t *testing.T) {
	resetScanFlags()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "regex.txt"), []byte("match .* this\nmatch all this\n"), 0644)

	scanFixedStrings = true
	res := runScanCmd(t, ".*", dir)

	if res.Total != 1 {
		t.Errorf("expected 1 fixed match, got %d", res.Total)
	}
	if len(res.Matches) > 0 && !bytes.Contains([]byte(res.Matches[0].C), []byte(".*")) {
		t.Errorf("expected match with literal .*, got %s", res.Matches[0].C)
	}
}

func TestScanSmartCase(t *testing.T) {
	resetScanFlags()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "case.txt"), []byte("HELLO\nhello\n"), 0644)

	// Lowercase query -> ignore case
	scanSmartCase = true
	res1 := runScanCmd(t, "hello", dir)
	if res1.Total != 2 {
		t.Errorf("smart-case lowercase: expected 2 matches, got %d", res1.Total)
	}

	// Uppercase query -> case sensitive
	resetScanFlags()
	scanSmartCase = true
	res2 := runScanCmd(t, "HELLO", dir)
	if res2.Total != 1 {
		t.Errorf("smart-case uppercase: expected 1 match, got %d", res2.Total)
	}
}

func TestScanWordRegexp(t *testing.T) {
	resetScanFlags()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "words.txt"), []byte("the dog\nthe underdog\n"), 0644)

	scanWordRegexp = true
	res := runScanCmd(t, "dog", dir)

	if res.Total != 1 {
		t.Errorf("word-regexp: expected 1 match, got %d", res.Total)
	}
	if len(res.Matches) > 0 && res.Matches[0].C != "the dog" {
		t.Errorf("word-regexp: expected 'the dog', got %s", res.Matches[0].C)
	}
}

func TestScanOnlyMatching(t *testing.T) {
	resetScanFlags()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "only.txt"), []byte("find apples and oranges\n"), 0644)

	scanOnlyMatching = true
	res := runScanCmd(t, "apples|oranges", dir)

	if res.Total != 1 {
		t.Errorf("only-matching: expected 1 match line, got %d", res.Total)
	}
	if len(res.Matches) > 0 && res.Matches[0].C != "apples\noranges" {
		t.Errorf("only-matching: expected 'apples\\noranges', got %q", res.Matches[0].C)
	}
}
