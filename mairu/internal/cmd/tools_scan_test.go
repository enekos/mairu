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
