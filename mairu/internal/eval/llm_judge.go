package eval

import (
	"context"
	"fmt"
	"strings"

	"mairu/internal/llm"
)

const defaultJudgeThreshold = 0.7

// judgeResponse asks a judge LLM whether `actual` satisfies the expectation in
// case.Expected. The judge returns a 0.0–1.0 score; the case passes when
// score >= case.JudgeThreshold (defaulting to 0.7).
func judgeResponse(ctx context.Context, judge LLMRunner, c LLMCase, actual string) (bool, string) {
	threshold := c.JudgeThreshold
	if threshold <= 0 {
		threshold = defaultJudgeThreshold
	}

	system := strings.TrimSpace(`You are an impartial evaluator. You will receive an EXPECTATION (what an ideal response should satisfy) and an ACTUAL response from a model under test. Score how well the actual response satisfies the expectation on a 0.0–1.0 scale, then return strictly JSON of the form {"score": 0.0, "reason": "..."}.

Use:
- 1.0 = fully satisfies the expectation
- 0.5 = partially satisfies (right idea, wrong detail)
- 0.0 = does not satisfy

Be strict but fair. Do not penalize for stylistic differences if the substance matches.`)

	user := fmt.Sprintf("EXPECTATION:\n%s\n\nACTUAL:\n%s", c.Expected, actual)

	schema := &llm.JSONSchema{
		Type: llm.TypeObject,
		Properties: map[string]*llm.JSONSchema{
			"score":  {Type: llm.TypeNumber, Description: "Score from 0.0 to 1.0"},
			"reason": {Type: llm.TypeString, Description: "One-sentence justification"},
		},
		Required: []string{"score", "reason"},
	}

	var verdict struct {
		Score  float64 `json:"score"`
		Reason string  `json:"reason"`
	}
	if err := judge.GenerateJSON(ctx, system, user, schema, &verdict); err != nil {
		return false, fmt.Sprintf("judge call failed: %v", err)
	}

	if verdict.Score >= threshold {
		return true, fmt.Sprintf("judge score %.2f >= %.2f (%s)", verdict.Score, threshold, verdict.Reason)
	}
	return false, fmt.Sprintf("judge score %.2f < %.2f (%s)", verdict.Score, threshold, verdict.Reason)
}
