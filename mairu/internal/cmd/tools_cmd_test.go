package cmd

import (
	"bytes"
	"os"
	"testing"
)

func TestMapCmd(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	mapCmd.Run(mapCmd, []string{"."})

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

	sysCmd.Run(sysCmd, []string{})

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

	infoCmd.Run(infoCmd, []string{"."})

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

	tmpFile.WriteString("API_KEY=secret\nexport PORT=8080\n# comment\n")
	tmpFile.Close()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	envCmd.Run(envCmd, []string{tmpFile.Name()})

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
}
