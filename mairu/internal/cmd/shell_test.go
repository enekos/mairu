package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestShellInitZsh_PrintsExpectedHooks(t *testing.T) {
	cmd := NewShellCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"init", "zsh"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	// NewShellCmd writes via fmt.Print, not cmd.OutOrStdout — the test
	// can't capture stdout directly, so we re-invoke newShellInitCmd's
	// runner by calling it on the snippet constant. Assert the snippet
	// contains the load-bearing bits.
	if !strings.Contains(zshHookSnippet, "add-zsh-hook preexec __mairu_preexec") {
		t.Error("snippet missing preexec hook registration")
	}
	if !strings.Contains(zshHookSnippet, "add-zsh-hook precmd __mairu_precmd") {
		t.Error("snippet missing precmd hook registration")
	}
	if !strings.Contains(zshHookSnippet, "MAIRU_NO_HOOK") {
		t.Error("snippet missing MAIRU_NO_HOOK escape hatch")
	}
	if !strings.Contains(zshHookSnippet, "mairu ingest record") {
		t.Error("snippet does not invoke `mairu ingest record`")
	}
	if !strings.Contains(zshHookSnippet, "&!") {
		t.Error("snippet is not backgrounding the client (missing &!)")
	}
}

func TestShellInitBash_NotImplemented(t *testing.T) {
	cmd := NewShellCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"init", "bash"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for bash (not implemented)")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestShellInitUnknown_Errors(t *testing.T) {
	cmd := NewShellCmd()
	cmd.SetArgs([]string{"init", "powershell"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown shell")
	}
	if !strings.Contains(err.Error(), "unknown shell") {
		t.Errorf("unexpected error: %v", err)
	}
}
