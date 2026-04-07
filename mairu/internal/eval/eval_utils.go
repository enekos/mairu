package eval

import "math"

type RetrievalResult struct {
	ID    string
	Score float64
}

// MeanReciprocalRank returns 1/(rank of first relevant result), or 0 if no
// relevant result appears in got.
func MeanReciprocalRank(expected []string, got []RetrievalResult) float64 {
	rel := toSet(expected)
	for i, r := range got {
		if rel[r.ID] {
			return 1.0 / float64(i+1)
		}
	}
	return 0
}

// RecallAtK returns the fraction of expected items that appear in the first k
// results.
func RecallAtK(expected []string, got []RetrievalResult, k int) float64 {
	if len(expected) == 0 || k <= 0 {
		return 0
	}
	rel := toSet(expected)
	hits := 0
	limit := min(k, len(got))
	for i := 0; i < limit; i++ {
		if rel[got[i].ID] {
			hits++
		}
	}
	return float64(hits) / float64(len(expected))
}

// PrecisionAtK returns the fraction of the first k results that are relevant.
func PrecisionAtK(expected []string, got []RetrievalResult, k int) float64 {
	if k <= 0 {
		return 0
	}
	rel := toSet(expected)
	hits := 0
	limit := min(k, len(got))
	for i := 0; i < limit; i++ {
		if rel[got[i].ID] {
			hits++
		}
	}
	return float64(hits) / float64(k)
}

// NDCGAtK computes Normalised Discounted Cumulative Gain at rank k.
// Binary relevance: a result is either relevant (gain=1) or not (gain=0).
func NDCGAtK(expected []string, got []RetrievalResult, k int) float64 {
	if len(expected) == 0 || k <= 0 {
		return 0
	}
	rel := toSet(expected)

	dcg := 0.0
	limit := min(k, len(got))
	for i := 0; i < limit; i++ {
		if rel[got[i].ID] {
			// Standard DCG formula: gain / log2(rank+1), rank is 1-based.
			dcg += 1.0 / math.Log2(float64(i+2))
		}
	}

	// Ideal DCG: place all relevant docs at the top k positions.
	idcg := 0.0
	idealHits := min(len(expected), k)
	for i := 0; i < idealHits; i++ {
		idcg += 1.0 / math.Log2(float64(i+2))
	}
	if idcg == 0 {
		return 0
	}
	return dcg / idcg
}

// AveragePrecision computes Average Precision for a single query.
// AP = (1/|relevant|) * Σ Precision@k for each k where result k is relevant.
func AveragePrecision(expected []string, got []RetrievalResult) float64 {
	if len(expected) == 0 {
		return 0
	}
	rel := toSet(expected)
	hits := 0
	sum := 0.0
	for i, r := range got {
		if rel[r.ID] {
			hits++
			sum += float64(hits) / float64(i+1)
		}
	}
	return sum / float64(len(expected))
}

func toSet(ids []string) map[string]bool {
	s := make(map[string]bool, len(ids))
	for _, id := range ids {
		s[id] = true
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
