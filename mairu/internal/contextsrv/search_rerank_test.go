package contextsrv

import (
	"testing"
	"time"
)

func TestScoreHybrid_RecencyAndImportanceAffectOrdering(t *testing.T) {
	tokens := tokenizeForSearch("auth token")
	opts := SearchOptions{
		WeightRecency: 0.8,
		WeightImp:     0.05,
		WeightKeyword: 0.15,
	}
	newLowImportance := scoreHybrid(
		map[string]string{"content": "auth token rotation"},
		tokens,
		time.Now().Add(-1*time.Hour),
		1,
		opts,
		defaultMemoryWeights(nil),
	)
	oldHighImportance := scoreHybrid(
		map[string]string{"content": "auth token rotation"},
		tokens,
		time.Now().Add(-60*24*time.Hour),
		10,
		opts,
		defaultMemoryWeights(nil),
	)

	if newLowImportance <= oldHighImportance {
		t.Fatalf("expected fresher item score > older one, got fresh=%f old=%f", newLowImportance, oldHighImportance)
	}
}

func TestScoreKeyword_RespectsFieldBoosts(t *testing.T) {
	fields := map[string]string{
		"name":        "Authentication",
		"description": "Token refresh handling",
	}
	tokens := tokenizeForSearch("authentication token")

	noBoost := scoreKeyword(fields, tokens, nil)
	withBoost := scoreKeyword(fields, tokens, map[string]float64{"name": 4})
	if withBoost <= noBoost {
		t.Fatalf("expected boosted score > baseline, got boosted=%f baseline=%f", withBoost, noBoost)
	}
}

func TestScoreWithMeiliRanking_TotalNeverExceedsOne(t *testing.T) {
	// A perfect Meilisearch score (1.0) combined with maximum recency and
	// importance should still produce a final score ≤ 1.0.
	opts := SearchOptions{}
	score := scoreWithMeiliRanking(1.0, time.Now(), 10, opts, defaultMemoryWeights(nil))
	if score > 1.0 {
		t.Fatalf("expected score ≤ 1.0 with perfect inputs, got %f", score)
	}
}

func TestScoreWithMeiliRanking_WeightBudgetSplit(t *testing.T) {
	// With only vector+keyword weights (no recency/importance), the Meilisearch
	// score should map linearly through those weights.
	opts := SearchOptions{WeightVector: 0.7, WeightKeyword: 0.3, WeightRecency: 0, WeightImp: 0}
	score := scoreWithMeiliRanking(0.8, time.Time{}, 0, opts, defaultSkillWeights(nil))
	expected := 0.8 // vector+keyword fraction = 1.0, so score = 0.8 * 1.0
	if score < expected-0.001 || score > expected+0.001 {
		t.Fatalf("expected score ≈ %f, got %f", expected, score)
	}
}

func TestNormalizeStoreName(t *testing.T) {
	cases := map[string]string{
		"":             "all",
		"all":          "all",
		"memory":       "memories",
		"memories":     "memories",
		"skill":        "skills",
		"skills":       "skills",
		"node":         "context",
		"contextnodes": "context",
		"unknown":      "all",
	}
	for in, expected := range cases {
		got := normalizeStoreName(in)
		if got != expected {
			t.Fatalf("normalizeStoreName(%q): expected %q, got %q", in, expected, got)
		}
	}
}
