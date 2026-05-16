package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mairu/internal/llm"
)

// LLMRunner is the slice of an LLM provider the evaluator needs. Stays small
// so tests can supply a fake. Signature matches llm.Provider so a real
// provider can be passed in directly.
type LLMRunner interface {
	GenerateJSON(ctx context.Context, system, user string, schema *llm.JSONSchema, out any) error
	GenerateContent(ctx context.Context, model, prompt string) (string, error)
}

// EvaluateLLMDataset runs every case in ds against runner and returns
// per-case results plus aggregate metrics. If judge is non-nil, cases with
// Match="judge" use it to score responses; otherwise judge cases fail with a
// helpful error.
func EvaluateLLMDataset(ctx context.Context, ds *LLMDataset, runner LLMRunner, judge LLMRunner) ([]LLMResult, LLMMetrics) {
	results := make([]LLMResult, 0, len(ds.Cases))
	var passed, failed, errored int
	for _, c := range ds.Cases {
		res := runCase(ctx, c, runner, judge)
		switch {
		case res.Error != "":
			errored++
		case res.Passed:
			passed++
		default:
			failed++
		}
		results = append(results, res)
	}
	total := len(ds.Cases)
	var pr float64
	if total > 0 {
		pr = float64(passed) / float64(total)
	}
	return results, LLMMetrics{
		Total:  total,
		Passed: passed,
		Failed: failed,
		Errors: errored,
		Pass:   pr,
	}
}

func runCase(ctx context.Context, c LLMCase, runner LLMRunner, judge LLMRunner) LLMResult {
	out := LLMResult{ID: c.ID}
	switch strings.ToLower(c.Mode) {
	case "", "text":
		resp, err := runner.GenerateContent(ctx, c.Model, c.User)
		if err != nil {
			out.Error = err.Error()
			return out
		}
		out.Response = resp
	case "json":
		// GenerateJSON unmarshals into the `out` param. We want the raw JSON
		// text so we can apply json_field comparisons — use a generic map.
		var raw map[string]any
		if err := runner.GenerateJSON(ctx, c.System, c.User, nil, &raw); err != nil {
			out.Error = err.Error()
			return out
		}
		b, _ := json.Marshal(raw)
		out.Response = string(b)
	default:
		out.Error = fmt.Sprintf("unknown mode %q", c.Mode)
		return out
	}

	pass, reason := compareResponse(ctx, c, out.Response, judge)
	out.Passed = pass
	out.Reason = reason
	return out
}

// compareResponse returns (pass, reason). Reason is populated on failure to
// help debugging.
func compareResponse(ctx context.Context, c LLMCase, actual string, judge LLMRunner) (bool, string) {
	switch strings.ToLower(c.Match) {
	case "", "exact":
		ok := strings.TrimSpace(c.Expected) == strings.TrimSpace(actual)
		if !ok {
			return false, fmt.Sprintf("exact mismatch: want %q, got %q", c.Expected, actual)
		}
		return true, ""
	case "contains":
		ok := strings.Contains(strings.ToLower(actual), strings.ToLower(c.Expected))
		if !ok {
			return false, fmt.Sprintf("expected substring %q not in response", c.Expected)
		}
		return true, ""
	case "json_field":
		return compareJSONSubset(c.Expected, actual)
	case "judge":
		if judge == nil {
			return false, "judge mode requires a judge LLM (none configured)"
		}
		return judgeResponse(ctx, judge, c, actual)
	default:
		return false, fmt.Sprintf("unknown match strategy %q", c.Match)
	}
}

func compareJSONSubset(expected, actual string) (bool, string) {
	var want, got map[string]any
	if err := json.Unmarshal([]byte(expected), &want); err != nil {
		return false, fmt.Sprintf("expected is not valid JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(actual), &got); err != nil {
		return false, fmt.Sprintf("response is not valid JSON: %v", err)
	}
	for k, wv := range want {
		gv, present := got[k]
		if !present {
			return false, fmt.Sprintf("missing field %q in response", k)
		}
		if !jsonEqual(wv, gv) {
			return false, fmt.Sprintf("field %q: want %v, got %v", k, wv, gv)
		}
	}
	return true, ""
}

func jsonEqual(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}
