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
		defaultMemoryWeights(),
	)
	oldHighImportance := scoreHybrid(
		map[string]string{"content": "auth token rotation"},
		tokens,
		time.Now().Add(-60*24*time.Hour),
		10,
		opts,
		defaultMemoryWeights(),
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
