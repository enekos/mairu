package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"mairu/internal/contextsrv"
	"mairu/internal/eval"
)

var (
	evalDatasetPath string
	evalTopK        int
	failBelowMRR    float64
	failBelowRecall float64
	verbose         bool
)

var evalCmd = &cobra.Command{
	Use:   "eval:retrieval",
	Short: "Run retrieval evaluation suite",
	RunE: func(cmd *cobra.Command, args []string) error {
		raw, err := os.ReadFile(evalDatasetPath)
		if err != nil {
			return fmt.Errorf("failed to read dataset: %w", err)
		}

		var dataset eval.EvalDataset
		if err := json.Unmarshal(raw, &dataset); err != nil {
			return fmt.Errorf("failed to parse dataset: %w", err)
		}

		ctx := context.Background()
		meiliURL := os.Getenv("MEILI_URL")
		if meiliURL == "" {
			meiliURL = "http://localhost:7700"
		}
		meiliKey := os.Getenv("MEILI_API_KEY")

		// embedder logic
		// we don't strictly need embedder if we just search, but meili needs it.
		meili := contextsrv.NewMeiliIndexer(meiliURL, meiliKey, nil)
		svc := contextsrv.NewServiceWithSearch(nil, meili, nil)

		_ = eval.SeedFixtures(ctx, svc, dataset.Fixtures)
		defer eval.CleanupFixtures(ctx, svc, dataset.Fixtures)

		searchFunc := func(ctx context.Context, domain, query string, topK int) ([]eval.RetrievalResult, error) {
			store := ""
			switch strings.ToLower(domain) {
			case "memory":
				store = "memory"
			case "skill":
				store = "skill"
			case "context":
				store = "node"
			default:
				store = "memory"
			}
			res, err := svc.Search(contextsrv.SearchOptions{
				Query: query,
				Store: store,
				TopK:  topK,
			})
			if err != nil {
				return nil, err
			}
			var results []eval.RetrievalResult
			items := res["memories"]
			if store == "skill" {
				items = res["skills"]
			} else if store == "node" {
				items = res["contextNodes"]
			}
			if items == nil {
				return nil, nil
			}
			for _, item := range items.([]map[string]any) {
				id := ""
				if store == "node" {
					id, _ = item["uri"].(string)
				} else {
					id, _ = item["id"].(string)
				}
				score, _ := item["_score"].(float64)
				results = append(results, eval.RetrievalResult{ID: id, Score: score})
			}
			return results, nil
		}

		metrics, err := eval.EvaluateDataset(ctx, dataset, evalTopK, verbose, searchFunc)
		if err != nil {
			return err
		}

		fmt.Printf("Evaluation complete.\nMRR: %.3f\nRecall: %.3f\n", metrics.MRR, metrics.Recall)

		if metrics.MRR < failBelowMRR {
			return fmt.Errorf("MRR %.3f is below threshold %.3f", metrics.MRR, failBelowMRR)
		}
		if metrics.Recall < failBelowRecall {
			return fmt.Errorf("Recall %.3f is below threshold %.3f", metrics.Recall, failBelowRecall)
		}

		return nil
	},
}

func init() {
	evalCmd.Flags().StringVarP(&evalDatasetPath, "dataset", "d", "eval/dataset.json", "Path to dataset JSON")
	evalCmd.Flags().IntVarP(&evalTopK, "topK", "k", 5, "Number of results to retrieve")
	evalCmd.Flags().Float64Var(&failBelowMRR, "fail-below-mrr", 0.0, "Fail if MRR is below this threshold")
	evalCmd.Flags().Float64Var(&failBelowRecall, "fail-below-recall", 0.0, "Fail if Recall is below this threshold")
	evalCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Print verbose output")
	rootCmd.AddCommand(evalCmd)
}
