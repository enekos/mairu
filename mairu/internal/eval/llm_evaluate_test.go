package eval

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mairu/internal/llm"
)

type fakeRunner struct {
	jsonResp map[string]any
	textResp string
	jsonErr  error
	textErr  error
}

func (f *fakeRunner) GenerateJSON(_ context.Context, _, _ string, _ *llm.JSONSchema, out any) error {
	if f.jsonErr != nil {
		return f.jsonErr
	}
	// emulate Provider behavior: unmarshal into out via pointer to map
	if m, ok := out.(*map[string]any); ok {
		*m = f.jsonResp
	}
	return nil
}

func (f *fakeRunner) GenerateContent(_ context.Context, _, _ string) (string, error) {
	return f.textResp, f.textErr
}

func TestEvaluateLLMDataset_JSONFieldMatch(t *testing.T) {
	ds := &LLMDataset{
		Cases: []LLMCase{
			{
				ID:       "skip",
				Mode:     "json",
				User:     "u",
				Expected: `{"action":"skip"}`,
				Match:    "json_field",
			},
		},
	}
	runner := &fakeRunner{jsonResp: map[string]any{"action": "skip", "reason": "dup"}}
	results, metrics := EvaluateLLMDataset(context.Background(), ds, runner, nil)
	if metrics.Passed != 1 || metrics.Total != 1 {
		t.Fatalf("expected 1/1 pass, got %+v", metrics)
	}
	if !results[0].Passed {
		t.Fatalf("expected pass, got %+v", results[0])
	}
}

func TestEvaluateLLMDataset_JSONFieldMismatch(t *testing.T) {
	ds := &LLMDataset{
		Cases: []LLMCase{
			{
				ID:       "wrong-action",
				Mode:     "json",
				User:     "u",
				Expected: `{"action":"skip"}`,
				Match:    "json_field",
			},
		},
	}
	runner := &fakeRunner{jsonResp: map[string]any{"action": "create"}}
	results, metrics := EvaluateLLMDataset(context.Background(), ds, runner, nil)
	if metrics.Failed != 1 {
		t.Fatalf("expected fail, got %+v", metrics)
	}
	if results[0].Passed {
		t.Fatalf("expected fail, got pass: %+v", results[0])
	}
	if results[0].Reason == "" {
		t.Fatalf("expected a reason string on failure")
	}
}

func TestEvaluateLLMDataset_TextContains(t *testing.T) {
	ds := &LLMDataset{
		Cases: []LLMCase{
			{ID: "contains", Mode: "text", User: "hi", Expected: "GREETING", Match: "contains"},
		},
	}
	runner := &fakeRunner{textResp: "Hello, that is a greeting"}
	results, metrics := EvaluateLLMDataset(context.Background(), ds, runner, nil)
	if metrics.Passed != 1 {
		t.Fatalf("expected pass, got %+v %+v", metrics, results)
	}
}

func TestEvaluateLLMDataset_ErrorClassification(t *testing.T) {
	ds := &LLMDataset{
		Cases: []LLMCase{
			{ID: "err", Mode: "json", User: "u", Expected: `{}`, Match: "json_field"},
		},
	}
	runner := &fakeRunner{jsonErr: errors.New("rate limited")}
	_, metrics := EvaluateLLMDataset(context.Background(), ds, runner, nil)
	if metrics.Errors != 1 || metrics.Passed != 0 {
		t.Fatalf("expected error classification, got %+v", metrics)
	}
}

func TestEvaluateLLMDataset_UnknownMode(t *testing.T) {
	ds := &LLMDataset{
		Cases: []LLMCase{
			{ID: "bad", Mode: "voice", User: "u", Expected: "", Match: "exact"},
		},
	}
	results, metrics := EvaluateLLMDataset(context.Background(), ds, &fakeRunner{}, nil)
	if metrics.Errors != 1 || results[0].Error == "" {
		t.Fatalf("expected error result, got %+v", results)
	}
}

// judgeRunner mocks an LLM-as-judge by returning a hard-coded score.
type judgeRunner struct {
	score  float64
	reason string
}

func (j *judgeRunner) GenerateJSON(_ context.Context, _, _ string, _ *llm.JSONSchema, out any) error {
	type verdict struct {
		Score  float64 `json:"score"`
		Reason string  `json:"reason"`
	}
	if v, ok := out.(*struct {
		Score  float64 `json:"score"`
		Reason string  `json:"reason"`
	}); ok {
		v.Score = j.score
		v.Reason = j.reason
	}
	_ = verdict{}
	return nil
}

func (j *judgeRunner) GenerateContent(_ context.Context, _, _ string) (string, error) {
	return "", nil
}

func TestEvaluateLLMDataset_JudgePass(t *testing.T) {
	ds := &LLMDataset{
		Cases: []LLMCase{
			{ID: "judge-pass", Mode: "text", User: "explain k8s", Expected: "a Kubernetes overview", Match: "judge", JudgeThreshold: 0.6},
		},
	}
	runner := &fakeRunner{textResp: "Kubernetes is a container orchestrator…"}
	judge := &judgeRunner{score: 0.85, reason: "covers main points"}
	results, metrics := EvaluateLLMDataset(context.Background(), ds, runner, judge)
	if metrics.Passed != 1 {
		t.Fatalf("expected pass, got %+v %+v", metrics, results)
	}
	if !strings.Contains(results[0].Reason, "0.85") {
		t.Fatalf("expected score in reason, got %q", results[0].Reason)
	}
}

func TestEvaluateLLMDataset_JudgeFailBelowThreshold(t *testing.T) {
	ds := &LLMDataset{
		Cases: []LLMCase{
			{ID: "judge-fail", Mode: "text", User: "explain k8s", Expected: "...", Match: "judge"},
		},
	}
	results, metrics := EvaluateLLMDataset(context.Background(), ds, &fakeRunner{textResp: "x"}, &judgeRunner{score: 0.3, reason: "off-topic"})
	if metrics.Failed != 1 {
		t.Fatalf("expected fail, got %+v %+v", metrics, results)
	}
}

func TestEvaluateLLMDataset_JudgeMissingJudge(t *testing.T) {
	ds := &LLMDataset{
		Cases: []LLMCase{
			{ID: "no-judge", Mode: "text", User: "u", Expected: "e", Match: "judge"},
		},
	}
	results, _ := EvaluateLLMDataset(context.Background(), ds, &fakeRunner{textResp: "x"}, nil)
	if results[0].Passed {
		t.Fatalf("expected fail when judge not configured")
	}
}
