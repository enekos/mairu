package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"mairu/internal/eval"
	"mairu/internal/llm"
)

// NewEvalLLMCmd is `mairu eval:llm` — replay a JSON dataset of LLM cases
// against the configured provider and report pass/fail metrics. Designed for
// CI gating with --fail-below-pass.
func NewEvalLLMCmd() *cobra.Command {
	var (
		datasetPath  string
		verbose      bool
		failBelow    float64
		outputJSON   bool
	)
	cmd := &cobra.Command{
		Use:   "eval:llm",
		Short: "Replay an LLM dataset and report pass/fail metrics",
		RunE: func(cmd *cobra.Command, _ []string) error {
			raw, err := os.ReadFile(datasetPath)
			if err != nil {
				return fmt.Errorf("read dataset: %w", err)
			}
			var ds eval.LLMDataset
			if err := json.Unmarshal(raw, &ds); err != nil {
				return fmt.Errorf("parse dataset: %w", err)
			}

			provider, err := llm.NewProviderFromEnv()
			if err != nil {
				return fmt.Errorf("init LLM provider: %w", err)
			}
			defer provider.Close()
			if ds.Model != "" {
				provider.SetModel(ds.Model)
			}

			// Judge defaults to the same provider — fine for "model judges
			// model" smoke tests; for production you'd want a different
			// (typically stronger) judge model.
			var judge eval.LLMRunner = provider
			results, metrics := eval.EvaluateLLMDataset(context.Background(), &ds, provider, judge)

			if outputJSON {
				out := map[string]any{
					"metrics": metrics,
					"results": results,
				}
				b, _ := json.MarshalIndent(out, "", "  ")
				fmt.Println(string(b))
			} else {
				for _, r := range results {
					status := "PASS"
					if r.Error != "" {
						status = "ERROR"
					} else if !r.Passed {
						status = "FAIL"
					}
					line := fmt.Sprintf("[%s] %s", status, r.ID)
					if verbose {
						if r.Reason != "" {
							line += "  " + r.Reason
						} else if r.Error != "" {
							line += "  " + r.Error
						}
					}
					fmt.Println(line)
				}
				fmt.Printf("\nTotal: %d  Passed: %d  Failed: %d  Errors: %d  Pass rate: %.2f%%\n",
					metrics.Total, metrics.Passed, metrics.Failed, metrics.Errors, metrics.Pass*100)
			}

			if metrics.Pass < failBelow {
				return fmt.Errorf("pass rate %.3f below threshold %.3f", metrics.Pass, failBelow)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&datasetPath, "dataset", "d", "llmeval/llm_dataset.json", "Path to LLM dataset JSON")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Print failure reasons")
	cmd.Flags().Float64Var(&failBelow, "fail-below-pass", 0.0, "Exit non-zero if pass rate is below this (0.0-1.0)")
	cmd.Flags().BoolVar(&outputJSON, "json", false, "Emit JSON output")
	return cmd
}
