package eval

import (
	"context"
	"fmt"
)

type EvalDataset struct {
	Description string      `json:"description"`
	Fixtures    FixtureSpec `json:"fixtures"`
	Cases       []Case      `json:"cases"`
}

type Case struct {
	ID       string   `json:"id"`
	Domain   string   `json:"domain"`
	Query    string   `json:"query"`
	Expected []string `json:"expected"`
	Got      []RetrievalResult
}

type Metrics struct {
	MRR       float64
	Recall    float64
	Precision float64
	NDCG      float64
	MAP       float64
}

type SearchFunc func(ctx context.Context, domain, query string, topK int) ([]RetrievalResult, error)

func EvaluateDataset(ctx context.Context, dataset *EvalDataset, k int, verbose bool, search SearchFunc) (Metrics, error) {
	for i, c := range dataset.Cases {
		results, err := search(ctx, c.Domain, c.Query, k)
		if err != nil {
			return Metrics{}, err
		}
		dataset.Cases[i].Got = results

		if verbose {
			fmt.Printf("Query: %s\n", c.Query)
			fmt.Printf("Expected: %v\n", c.Expected)
			fmt.Printf("Got:\n")
			for j, r := range results {
				fmt.Printf("  %d: %s (score: %.3f)\n", j+1, r.ID, r.Score)
			}
			fmt.Println()
		}
	}
	return EvaluateCases(dataset.Cases, k), nil
}

func EvaluateCases(cases []Case, k int) Metrics {
	if len(cases) == 0 {
		return Metrics{}
	}
	var mrr, recall, precision, ndcg, ap float64
	for _, c := range cases {
		mrr += MeanReciprocalRank(c.Expected, c.Got)
		recall += RecallAtK(c.Expected, c.Got, k)
		precision += PrecisionAtK(c.Expected, c.Got, k)
		ndcg += NDCGAtK(c.Expected, c.Got, k)
		ap += AveragePrecision(c.Expected, c.Got)
	}
	n := float64(len(cases))
	return Metrics{
		MRR:       mrr / n,
		Recall:    recall / n,
		Precision: precision / n,
		NDCG:      ndcg / n,
		MAP:       ap / n,
	}
}
