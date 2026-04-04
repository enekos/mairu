package eval

import (
	"context"
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
	MRR    float64
	Recall float64
}

type SearchFunc func(ctx context.Context, domain, query string, topK int) ([]RetrievalResult, error)

func EvaluateDataset(ctx context.Context, dataset EvalDataset, k int, verbose bool, search SearchFunc) (Metrics, error) {
	for i, c := range dataset.Cases {
		results, err := search(ctx, c.Domain, c.Query, k)
		if err != nil {
			return Metrics{}, err
		}
		dataset.Cases[i].Got = results
	}
	return EvaluateCases(dataset.Cases, k), nil
}

func EvaluateCases(cases []Case, k int) Metrics {
	if len(cases) == 0 {
		return Metrics{}
	}
	var mrr, recall float64
	for _, c := range cases {
		mrr += MeanReciprocalRank(c.Expected, c.Got)
		recall += RecallAtK(c.Expected, c.Got, k)
	}
	return Metrics{MRR: mrr / float64(len(cases)), Recall: recall / float64(len(cases))}
}
