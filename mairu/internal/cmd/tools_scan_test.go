package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func resetScanFlags() {
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
