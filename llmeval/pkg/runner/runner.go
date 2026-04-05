package runner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/user/llmeval/pkg/evaluator"
	"github.com/user/llmeval/pkg/llm"
)

type TestCase struct {
	ID       string             `json:"id"`
	Prompt   string             `json:"prompt"`
	Expected string             `json:"expected"`
	EvalType evaluator.EvalType `json:"eval_type"`
}

type TestResult struct {
	TestCase   TestCase
	Actual     string
	Pass       bool
	Reason     string
	Error      error
	Duration   time.Duration
	Usage      llm.Usage
	Cost       float64
	JudgeUsage llm.Usage
	JudgeCost  float64
}

type Runner struct {
	TestModel  string
	LLMClient  *llm.Client
	Evaluator  *evaluator.Evaluator
	MaxWorkers int
}

func NewRunner(client *llm.Client, eval *evaluator.Evaluator, testModel string, concurrency int) *Runner {
	if concurrency < 1 {
		concurrency = 1
	}
	return &Runner{
		TestModel:  testModel,
		LLMClient:  client,
		Evaluator:  eval,
		MaxWorkers: concurrency,
	}
}

func (r *Runner) Run(ctx context.Context, cases []TestCase) []TestResult {
	type job struct {
		index int
		tc    TestCase
	}

	jobs := make(chan job, len(cases))
	allResults := make([]TestResult, len(cases))

	var wg sync.WaitGroup

	for i := 0; i < r.MaxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				allResults[j.index] = r.runSingle(ctx, j.tc)
			}
		}()
	}

	for i, tc := range cases {
		jobs <- job{index: i, tc: tc}
	}
	close(jobs)

	wg.Wait()

	return allResults
}

func (r *Runner) runSingle(ctx context.Context, tc TestCase) TestResult {
	start := time.Now()

	actual, usage, err := r.LLMClient.Generate(ctx, r.TestModel, tc.Prompt, 0.0)
	duration := time.Since(start)

	var cost float64
	// Simple cost calculation based on model (approximations)
	switch r.TestModel {
	case "gpt-4":
		cost = float64(usage.PromptTokens)*0.03/1000.0 + float64(usage.CompletionTokens)*0.06/1000.0
	case "gpt-3.5-turbo":
		cost = float64(usage.PromptTokens)*0.0005/1000.0 + float64(usage.CompletionTokens)*0.0015/1000.0
	case "gpt-4o":
		cost = float64(usage.PromptTokens)*0.005/1000.0 + float64(usage.CompletionTokens)*0.015/1000.0
	case "gpt-4o-mini":
		cost = float64(usage.PromptTokens)*0.00015/1000.0 + float64(usage.CompletionTokens)*0.0006/1000.0
	default:
		// Default to some generic low cost
		cost = float64(usage.TotalTokens) * 0.001 / 1000.0
	}

	if err != nil {
		return TestResult{
			TestCase: tc,
			Error:    fmt.Errorf("failed to generate response: %w", err),
			Duration: duration,
			Usage:    usage,
			Cost:     cost,
		}
	}

	evalRes, err := r.Evaluator.Evaluate(ctx, tc.EvalType, tc.Prompt, tc.Expected, actual)
	if err != nil {
		return TestResult{
			TestCase: tc,
			Actual:   actual,
			Error:    fmt.Errorf("evaluation failed: %w", err),
			Duration: duration,
			Usage:    usage,
			Cost:     cost,
		}
	}

	return TestResult{
		TestCase:   tc,
		Actual:     actual,
		Pass:       evalRes.Pass,
		Reason:     evalRes.Reason,
		Duration:   duration,
		Usage:      usage,
		Cost:       cost,
		JudgeUsage: evalRes.JudgeUsage,
		JudgeCost:  evalRes.JudgeCost,
	}
}
