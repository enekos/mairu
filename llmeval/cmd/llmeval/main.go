package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/user/llmeval/pkg/evaluator"
	"github.com/user/llmeval/pkg/llm"
	"github.com/user/llmeval/pkg/runner"
)

func main() {
	datasetPath := flag.String("dataset", "", "Path to the JSON dataset file")
	model := flag.String("model", "kimi-k2-0711-preview", "Model to evaluate")
	judgeModel := flag.String("judge-model", "kimi-k2-0711-preview", "Model to use for llm_judge evaluations")
	concurrency := flag.Int("concurrency", 4, "Number of parallel evaluations")
	baseURL := flag.String("base-url", "", "Kimi API Base URL (optional)")
	apiKey := flag.String("api-key", os.Getenv("KIMI_API_KEY"), "API Key (defaults to KIMI_API_KEY env var)")
	outputFormat := flag.String("format", "text", "Output format: text, json, csv")
	outputFile := flag.String("output-file", "", "File to write results to (defaults to stdout if empty)")

	flag.Parse()

	if *datasetPath == "" {
		fmt.Fprintln(os.Stderr, "Error: --dataset flag is required")
		flag.Usage()
		os.Exit(1)
	}

	data, err := os.ReadFile(*datasetPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading dataset: %v\n", err)
		os.Exit(1)
	}

	var testCases []runner.TestCase
	if err := json.Unmarshal(data, &testCases); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing dataset JSON: %v\n", err)
		os.Exit(1)
	}

	if len(testCases) == 0 {
		fmt.Fprintln(os.Stderr, "Dataset is empty.")
		os.Exit(0)
	}

	if *outputFormat == "text" {
		fmt.Printf("Loaded %d test cases from %s\n", len(testCases), *datasetPath)
		fmt.Printf("Evaluating model: %s (Concurrency: %d)\n\n", *model, *concurrency)
	}

	client := llm.NewClient(*baseURL, *apiKey)
	eval := evaluator.NewEvaluator(client, *judgeModel)
	run := runner.NewRunner(client, eval, *model, *concurrency)

	ctx := context.Background()
	startTime := time.Now()

	results := run.Run(ctx, testCases)

	totalDuration := time.Since(startTime)

	var outWriter *os.File
	if *outputFile != "" {
		outWriter, err = os.Create(*outputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create output file: %v\n", err)
			os.Exit(1)
		}
		defer outWriter.Close()
	} else {
		outWriter = os.Stdout
	}

	switch *outputFormat {
	case "json":
		writeJSON(outWriter, results, totalDuration)
	case "csv":
		writeCSV(outWriter, results)
	case "text":
		fallthrough
	default:
		writeText(outWriter, results, totalDuration)
	}
}

func writeText(out *os.File, results []runner.TestResult, totalDuration time.Duration) {
	fmt.Fprintln(out, "========================================")
	fmt.Fprintln(out, "EVALUATION RESULTS")
	fmt.Fprintln(out, "========================================")

	passes := 0
	var totalCost float64
	var totalTokens int

	for _, res := range results {
		totalCost += res.Cost + res.JudgeCost
		totalTokens += res.Usage.TotalTokens + res.JudgeUsage.TotalTokens

		status := "❌ FAIL"
		if res.Error != nil {
			status = "⚠️ ERROR"
			fmt.Fprintf(out, "[%s] %s (%v, $%.6f, %dtoks)\n", status, res.TestCase.ID, res.Duration.Round(time.Millisecond), res.Cost+res.JudgeCost, res.Usage.TotalTokens+res.JudgeUsage.TotalTokens)
			fmt.Fprintf(out, "  Error: %v\n\n", res.Error)
			continue
		}

		if res.Pass {
			status = "✅ PASS"
			passes++
		}

		fmt.Fprintf(out, "[%s] %s (%v, $%.6f, %dtoks)\n", status, res.TestCase.ID, res.Duration.Round(time.Millisecond), res.Cost+res.JudgeCost, res.Usage.TotalTokens+res.JudgeUsage.TotalTokens)
		fmt.Fprintf(out, "  Type:     %s\n", res.TestCase.EvalType)
		if !res.Pass {
			if res.TestCase.Expected != "" {
				fmt.Fprintf(out, "  Expected: %s\n", res.TestCase.Expected)
			}
			fmt.Fprintf(out, "  Actual:   %s\n", res.Actual)
			fmt.Fprintf(out, "  Reason:   %s\n", res.Reason)
		}
		fmt.Fprintln(out)
	}

	fmt.Fprintln(out, "========================================")
	fmt.Fprintf(out, "Total Tests: %d\n", len(results))
	fmt.Fprintf(out, "Passed:      %d\n", passes)
	fmt.Fprintf(out, "Failed:      %d\n", len(results)-passes)
	fmt.Fprintf(out, "Pass Rate:   %.2f%%\n", float64(passes)/float64(len(results))*100)
	fmt.Fprintf(out, "Duration:    %v\n", totalDuration.Round(time.Millisecond))
	fmt.Fprintf(out, "Total Cost:  $%.6f\n", totalCost)
	fmt.Fprintf(out, "Total Tokens:%d\n", totalTokens)
}

func writeJSON(out *os.File, results []runner.TestResult, totalDuration time.Duration) {
	passes := 0
	var totalCost float64
	var totalTokens int
	for _, res := range results {
		if res.Pass {
			passes++
		}
		totalCost += res.Cost + res.JudgeCost
		totalTokens += res.Usage.TotalTokens + res.JudgeUsage.TotalTokens
	}

	report := map[string]interface{}{
		"summary": map[string]interface{}{
			"total":        len(results),
			"passed":       passes,
			"failed":       len(results) - passes,
			"pass_rate":    float64(passes) / float64(len(results)) * 100,
			"duration_s":   totalDuration.Seconds(),
			"total_cost":   totalCost,
			"total_tokens": totalTokens,
		},
		"results": results,
	}

	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing JSON: %v\n", err)
	}
}

func writeCSV(out *os.File, results []runner.TestResult) {
	writer := csv.NewWriter(out)
	defer writer.Flush()

	writer.Write([]string{"ID", "Pass", "Type", "Expected", "Actual", "Reason", "Error", "Duration_ms", "Cost", "Tokens", "JudgeCost", "JudgeTokens"})

	for _, res := range results {
		passStr := strconv.FormatBool(res.Pass)
		errorStr := ""
		if res.Error != nil {
			errorStr = res.Error.Error()
			passStr = "ERROR"
		}

		writer.Write([]string{
			res.TestCase.ID,
			passStr,
			string(res.TestCase.EvalType),
			res.TestCase.Expected,
			res.Actual,
			res.Reason,
			errorStr,
			fmt.Sprintf("%d", res.Duration.Milliseconds()),
			fmt.Sprintf("%f", res.Cost),
			strconv.Itoa(res.Usage.TotalTokens),
			fmt.Sprintf("%f", res.JudgeCost),
			strconv.Itoa(res.JudgeUsage.TotalTokens),
		})
	}
}
