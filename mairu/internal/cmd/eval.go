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
	evalVerbose     bool
)

func NewEvalCmd() *cobra.Command {
	cmd := &cobra.Command{
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

		sqliteDSN := os.Getenv("CONTEXT_SERVER_SQLITE_DSN")
		if sqliteDSN == "" {
			sqliteDSN = "file:mairu_eval.db?cache=shared&mode=rwc"
		}
		repo, err := contextsrv.NewSQLiteRepository(sqliteDSN)
		if err != nil {
			return fmt.Errorf("failed to connect to db: %w", err)
		}

		meili := contextsrv.NewMeiliIndexer(meiliURL, meiliKey, nil)
		svc := contextsrv.NewServiceWithSearch(repo, meili, nil, false)

		_ = eval.SeedFixtures(ctx, svc, dataset.Fixtures)

		// Run projector once to sync fixtures to Meilisearch
		projector := contextsrv.NewProjector(repo, meili, nil)
		_, _ = projector.RunOnce(ctx, 100)

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

		metrics, err := eval.EvaluateDataset(ctx, &dataset, evalTopK, evalVerbose, searchFunc)
		if err != nil {
			return err
		}

		fmt.Printf("Evaluation complete.\nMRR:       %.3f\nRecall@%d: %.3f\nPrecision@%d: %.3f\nNDCG@%d:   %.3f\nMAP:       %.3f\n",
			metrics.MRR, evalTopK, metrics.Recall, evalTopK, metrics.Precision, evalTopK, metrics.NDCG, metrics.MAP)

		if metrics.MRR < failBelowMRR {
			return fmt.Errorf("MRR %.3f is below threshold %.3f", metrics.MRR, failBelowMRR)
		}
		if metrics.Recall < failBelowRecall {
			return fmt.Errorf("recall %.3f is below threshold %.3f", metrics.Recall, failBelowRecall)
		}

		return nil
	},
}
	cmd.Flags().StringVarP(&evalDatasetPath, "dataset", "d", "eval/dataset.json", "Path to dataset JSON")
	cmd.Flags().IntVarP(&evalTopK, "topK", "k", 5, "Number of results to retrieve")
	cmd.Flags().Float64Var(&failBelowMRR, "fail-below-mrr", 0.0, "Fail if MRR is below this threshold")
	cmd.Flags().Float64Var(&failBelowRecall, "fail-below-recall", 0.0, "Fail if Recall is below this threshold")
	cmd.Flags().BoolVarP(&evalVerbose, "verbose", "v", false, "Print verbose output")
	return cmd
}


