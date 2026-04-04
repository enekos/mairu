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

func TestRenderVibeQueryPlanner(t *testing.T) {
	out := Render("vibe_query_planner", nil)
	if !strings.Contains(out, "You are a search planner") {
		t.Errorf("vibe_query_planner prompt missing expected string, got:\n%s", out)
	}
}

func TestRenderVibeMutationPlanner(t *testing.T) {
	out := Render("vibe_mutation_planner", nil)
	if !strings.Contains(out, "You are a mutation planner") {
		t.Errorf("vibe_mutation_planner prompt missing expected string, got:\n%s", out)
	}
}

func TestRenderVibeMutationPlannerCompact(t *testing.T) {
	data := struct {
		Project                string
		ExistingEntriesSummary string
	}{
		Project:                "demo",
		ExistingEntriesSummary: `[{ "id": "mem_1" }]`,
	}
	out := Render("vibe_mutation_planner_compact", data)
	if !strings.Contains(out, "You are a JSON mutation planner") {
		t.Errorf("vibe_mutation_planner_compact missing expected heading")
	}
	if !strings.Contains(out, `Use project: "demo"`) {
		t.Errorf("vibe_mutation_planner_compact missing project directive")
	}
	if !strings.Contains(out, `{ "id": "mem_1" }`) {
		t.Errorf("vibe_mutation_planner_compact missing entries summary")
	}
}
