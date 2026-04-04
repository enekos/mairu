package prompts

import (
	"strings"
	"testing"
)

func TestRenderAgentSystem(t *testing.T) {
	out := Render("agent_system", nil)
	if !strings.Contains(out, "You are Mairu") {
		t.Errorf("agent_system prompt missing expected string, got:\n%s", out)
	}
}

func TestRenderSessionSummarize(t *testing.T) {
	data := struct {
		Conversation string
	}{
		Conversation: "Hello, this is a test.",
	}
	out := Render("session_summarize", data)
	if !strings.Contains(out, "Please summarize") {
		t.Errorf("missing string in session_summarize")
	}
	if !strings.Contains(out, "Hello, this is a test.") {
		t.Errorf("missing conversation in session_summarize")
	}
}

func TestRenderDelegateTask(t *testing.T) {
	data := struct {
		Context string
		Task    string
	}{
		Context: "Some Context",
		Task:    "Some Task",
	}
	out := Render("delegate_task", data)
	if !strings.Contains(out, "Main Agent Context:") {
		t.Errorf("missing string in delegate_task")
	}
	if !strings.Contains(out, "Some Context") {
		t.Errorf("missing context in delegate_task")
	}
	if !strings.Contains(out, "Some Task") {
		t.Errorf("missing task in delegate_task")
	}
}

func TestGetUnknownPrompt(t *testing.T) {
	_, err := Get("unknown_prompt_xyz", nil)
	if err == nil {
		t.Errorf("expected error for unknown prompt, got nil")
	}
}
