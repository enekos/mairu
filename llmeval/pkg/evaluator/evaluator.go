package evaluator

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/tidwall/gjson"
	"github.com/user/llmeval/pkg/llm"
)

type EvalType string

const (
	Exact      EvalType = "exact"
	Includes   EvalType = "includes"
	LLMJudge   EvalType = "llm_judge"
	ValidJSON  EvalType = "valid_json"
	JSONSchema EvalType = "json_schema"
	Regex      EvalType = "regex"
	JSONPath   EvalType = "json_path"
)

type Evaluator struct {
	LLMClient  *llm.Client
	JudgeModel string
}

func NewEvaluator(client *llm.Client, judgeModel string) *Evaluator {
	return &Evaluator{
		LLMClient:  client,
		JudgeModel: judgeModel,
	}
}

type EvalResult struct {
	Pass       bool
	Reason     string
	JudgeUsage llm.Usage
	JudgeCost  float64
}

func (e *Evaluator) Evaluate(ctx context.Context, evalType EvalType, prompt, expected, actual string) (EvalResult, error) {
	switch evalType {
	case Exact:
		return e.evaluateExact(expected, actual), nil
	case Includes:
		return e.evaluateIncludes(expected, actual), nil
	case LLMJudge:
		return e.evaluateLLMJudge(ctx, prompt, expected, actual)
	case ValidJSON:
		return e.evaluateValidJSON(actual), nil
	case JSONSchema:
		return e.evaluateJSONSchema(expected, actual), nil
	case Regex:
		return e.evaluateRegex(expected, actual), nil
	case JSONPath:
		return e.evaluateJSONPath(expected, actual), nil
	default:
		return EvalResult{}, fmt.Errorf("unknown evaluation type: %s", evalType)
	}
}

func (e *Evaluator) evaluateExact(expected, actual string) EvalResult {
	normExpected := strings.Trim(strings.TrimSpace(strings.ToLower(expected)), "`")
	normActual := strings.Trim(strings.TrimSpace(strings.ToLower(actual)), "`")

	pass := normExpected == normActual
	reason := "Exact match successful"
	if !pass {
		reason = fmt.Sprintf("Expected '%s' but got '%s'", normExpected, normActual)
	}

	return EvalResult{Pass: pass, Reason: reason}
}

func (e *Evaluator) evaluateIncludes(expected, actual string) EvalResult {
	normExpected := strings.TrimSpace(strings.ToLower(expected))
	normActual := strings.ToLower(actual)

	pass := strings.Contains(normActual, normExpected)
	reason := "Output included the expected string"
	if !pass {
		reason = fmt.Sprintf("Output did not contain '%s'", normExpected)
	}

	return EvalResult{Pass: pass, Reason: reason}
}

func (e *Evaluator) evaluateRegex(expected, actual string) EvalResult {
	re, err := regexp.Compile(expected)
	if err != nil {
		return EvalResult{Pass: false, Reason: fmt.Sprintf("Invalid expected regex: %v", err)}
	}
	pass := re.MatchString(actual)
	reason := "Regex matched"
	if !pass {
		reason = fmt.Sprintf("Output did not match regex: %s", expected)
	}
	return EvalResult{Pass: pass, Reason: reason}
}

func (e *Evaluator) evaluateJSONPath(expected, actual string) EvalResult {
	// expected should be in format "path=value"
	parts := strings.SplitN(expected, "=", 2)
	if len(parts) != 2 {
		return EvalResult{Pass: false, Reason: "Expected format for json_path is 'path=value'"}
	}

	path := strings.TrimSpace(parts[0])
	expectedVal := strings.TrimSpace(parts[1])

	cleanedActual := extractJSON(actual)
	if !gjson.Valid(cleanedActual) {
		return EvalResult{Pass: false, Reason: "Actual output is not valid JSON"}
	}

	result := gjson.Get(cleanedActual, path)
	if !result.Exists() {
		return EvalResult{Pass: false, Reason: fmt.Sprintf("JSON path '%s' not found", path)}
	}

	actualVal := result.String()
	if actualVal != expectedVal {
		return EvalResult{Pass: false, Reason: fmt.Sprintf("Path '%s' matched '%s', expected '%s'", path, actualVal, expectedVal)}
	}

	return EvalResult{Pass: true, Reason: "JSON path matched expected value"}
}

func (e *Evaluator) evaluateLLMJudge(ctx context.Context, prompt, expected, actual string) (EvalResult, error) {
	if e.LLMClient == nil || e.JudgeModel == "" {
		return EvalResult{}, fmt.Errorf("LLM client and JudgeModel must be configured for llm_judge evaluation")
	}

	judgePrompt := fmt.Sprintf(`You are an impartial evaluator grading an LLM's response.
Compare the ACTUAL OUTPUT to the EXPECTED OUTPUT for the given PROMPT.

PROMPT:
%s

EXPECTED OUTPUT:
%s

ACTUAL OUTPUT:
%s

Does the ACTUAL OUTPUT fulfill the core requirements of the EXPECTED OUTPUT?
If it does, reply strictly with the word "PASS".
If it does not, reply strictly with the word "FAIL", followed by a brief reason on a new line.`, prompt, expected, actual)

	response, judgeUsage, err := e.LLMClient.Generate(ctx, e.JudgeModel, "", judgePrompt, 0.0)
	if err != nil {
		return EvalResult{}, fmt.Errorf("failed to call judge LLM: %w", err)
	}

	var judgeCost float64
	// Kimi pricing approx
	switch e.JudgeModel {
	case "kimi-k2-0711-preview":
		judgeCost = float64(judgeUsage.PromptTokens)*0.50/1000000.0 + float64(judgeUsage.CompletionTokens)*2.00/1000000.0
	case "kimi-latest":
		judgeCost = float64(judgeUsage.PromptTokens)*0.50/1000000.0 + float64(judgeUsage.CompletionTokens)*2.00/1000000.0
	default:
		judgeCost = float64(judgeUsage.TotalTokens) * 0.1 / 1000000.0
	}

	respTrimmed := strings.TrimSpace(response)
	if strings.HasPrefix(strings.ToUpper(respTrimmed), "PASS") {
		return EvalResult{Pass: true, Reason: "Judge LLM approved", JudgeUsage: judgeUsage, JudgeCost: judgeCost}, nil
	}

	return EvalResult{Pass: false, Reason: "Judge LLM rejected: " + respTrimmed, JudgeUsage: judgeUsage, JudgeCost: judgeCost}, nil
}

func (e *Evaluator) evaluateValidJSON(actual string) EvalResult {
	cleanedActual := extractJSON(actual)
	var js interface{}
	err := json.Unmarshal([]byte(cleanedActual), &js)
	if err != nil {
		return EvalResult{Pass: false, Reason: fmt.Sprintf("Invalid JSON: %v", err)}
	}
	return EvalResult{Pass: true, Reason: "Output is valid JSON"}
}

func (e *Evaluator) evaluateJSONSchema(schemaStr, actual string) EvalResult {
	cleanedActual := extractJSON(actual)

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", strings.NewReader(schemaStr)); err != nil {
		return EvalResult{Pass: false, Reason: fmt.Sprintf("Failed to load expected schema: %v", err)}
	}

	schema, err := compiler.Compile("schema.json")
	if err != nil {
		return EvalResult{Pass: false, Reason: fmt.Sprintf("Invalid expected JSON schema: %v", err)}
	}

	var parsedActual interface{}
	if err := json.Unmarshal([]byte(cleanedActual), &parsedActual); err != nil {
		return EvalResult{Pass: false, Reason: fmt.Sprintf("Actual output is not valid JSON: %v", err)}
	}

	if err := schema.Validate(parsedActual); err != nil {
		return EvalResult{Pass: false, Reason: fmt.Sprintf("Schema validation failed: %v", err)}
	}

	return EvalResult{Pass: true, Reason: "JSON matches expected schema"}
}

// extractJSON attempts to strip Markdown code block formatting if present
func extractJSON(input string) string {
	input = strings.TrimSpace(input)
	// Match first ```json ... ``` or ``` ... ```
	re := regexp.MustCompile("(?s)```(?:json)?\n?(.*?)\n?```")
	match := re.FindStringSubmatch(input)
	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return input
}
