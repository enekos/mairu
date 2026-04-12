package contextsrv

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type hybridWeights struct {
	vector     float64
	keyword    float64
	recency    float64
	importance float64
	churn      float64
}

type scoredDoc struct {
	score float64
	doc   map[string]any
}

var tokenSplitRegex = regexp.MustCompile(`[^a-z0-9]+`)

// WeightOverrides allows injecting config-driven weight defaults.
type WeightOverrides struct {
	Vector     float64
	Keyword    float64
	Recency    float64
	Importance float64
	Churn      float64
}

func defaultMemoryWeights(overrides *WeightOverrides) hybridWeights {
	w := hybridWeights{vector: 0.6, keyword: 0.2, recency: 0.05, importance: 0.15, churn: 0.0}
	return applyOverrides(w, overrides)
}

func defaultSkillWeights(overrides *WeightOverrides) hybridWeights {
	w := hybridWeights{vector: 0.7, keyword: 0.3, recency: 0, importance: 0, churn: 0.0}
	return applyOverrides(w, overrides)
}

func defaultContextWeights(overrides *WeightOverrides) hybridWeights {
	w := hybridWeights{vector: 0.60, keyword: 0.30, recency: 0.05, importance: 0, churn: 0.05}
	return applyOverrides(w, overrides)
}

func applyOverrides(w hybridWeights, o *WeightOverrides) hybridWeights {
	if o == nil {
		return w
	}
	if o.Vector > 0 {
		w.vector = o.Vector
	}
	if o.Keyword > 0 {
		w.keyword = o.Keyword
	}
	if o.Recency > 0 {
		w.recency = o.Recency
	}
	if o.Importance > 0 {
		w.importance = o.Importance
	}
	if o.Churn > 0 {
		w.churn = o.Churn
	}
	return w
}

func normalizeStoreName(store string) string {
	switch strings.TrimSpace(strings.ToLower(store)) {
	case "", "all":
		return "all"
	case "memory", "memories":
		return "memories"
	case "skill", "skills":
		return "skills"
	case "node", "nodes", "context", "contextnodes":
		return "context"
	default:
		return "all"
	}
}

func tokenizeForSearch(query string) []string {
	raw := tokenSplitRegex.Split(strings.ToLower(query), -1)
	out := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, token := range raw {
		if token == "" {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	return out
}

func scoreHybrid(fields map[string]string, queryTokens []string, createdAt time.Time, importance int, churnScore float64, opts SearchOptions, defaults hybridWeights) float64 {
	weights := effectiveWeights(opts, defaults)
	keywordScore := scoreKeyword(fields, queryTokens, opts.FieldBoosts)
	recencyScore := scoreRecency(createdAt, opts.RecencyScale, opts.RecencyDecay)
	importanceScore := 0.0
	if importance > 0 {
		importanceScore = float64(importance) / 10.0
	}

	return keywordScore*weights.keyword + recencyScore*weights.recency + importanceScore*weights.importance + churnScore*weights.churn
}

func effectiveWeights(opts SearchOptions, defaults hybridWeights) hybridWeights {
	w := defaults
	if opts.WeightVector > 0 {
		w.vector = opts.WeightVector
	}
	if opts.WeightKeyword > 0 {
		w.keyword = opts.WeightKeyword
	}
	if opts.WeightRecency > 0 {
		w.recency = opts.WeightRecency
	}
	if opts.WeightImp > 0 {
		w.importance = opts.WeightImp
	}
	// Churn is not currently overridable per query, only via config defaults

	total := w.vector + w.keyword + w.recency + w.importance + w.churn
	if total <= 0 {
		return defaults
	}
	return hybridWeights{
		vector:     w.vector / total,
		keyword:    w.keyword / total,
		recency:    w.recency / total,
		importance: w.importance / total,
		churn:      w.churn / total,
	}
}

func scoreWithMeiliRanking(rankingScore float64, createdAt time.Time, importance int, opts SearchOptions, defaults hybridWeights, enrichmentData map[string]any) float64 {
	weights := effectiveWeights(opts, defaults)

	// rankingScore (0–1) comes from Meilisearch's hybrid vector+keyword
	// blend.  It represents the combined quality of those two signals, but
	// the weights budget is shared with recency and importance.  Scale it by
	// the vector+keyword fraction so that all four components sum to ≤ 1.
	vectorKeywordFraction := weights.vector + weights.keyword
	score := rankingScore * vectorKeywordFraction

	recencyScore := scoreRecency(createdAt, opts.RecencyScale, opts.RecencyDecay)
	importanceScore := 0.0
	if importance > 0 {
		importanceScore = float64(importance) / 10.0
	}

	churnScore := 0.0
	if enrichmentData != nil {
		if cs, ok := enrichmentData["enrichment_churn_score"].(float64); ok {
			churnScore = cs
		}
	}

	score += recencyScore * weights.recency
	score += importanceScore * weights.importance
	score += churnScore * weights.churn

	return score
}

func scoreKeyword(fields map[string]string, tokens []string, fieldBoosts map[string]float64) float64 {
	if len(tokens) == 0 {
		return 0
	}
	totalWeight := 0.0
	matchWeight := 0.0
	for _, token := range tokens {
		best := 0.0
		for field, value := range fields {
			boost := 1.0
			if fieldBoosts != nil {
				if b, ok := fieldBoosts[field]; ok && b > 0 {
					boost = b
				}
			}
			if strings.Contains(strings.ToLower(value), token) && boost > best {
				best = boost
			}
		}
		totalWeight += 1.0
		matchWeight += best
	}
	if totalWeight == 0 {
		return 0
	}
	return matchWeight / totalWeight
}

func scoreRecency(createdAt time.Time, recencyScale string, recencyDecay float64) float64 {
	if createdAt.IsZero() {
		return 0
	}
	scale := parseRecencyScale(recencyScale)
	if scale <= 0 {
		scale = 30 * 24 * time.Hour
	}
	decay := recencyDecay
	if decay <= 0 || decay >= 1 {
		decay = 0.5
	}
	age := time.Since(createdAt)
	if age < 0 {
		age = 0
	}
	return math.Pow(decay, float64(age)/float64(scale))
}

func parseRecencyScale(raw string) time.Duration {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return 30 * 24 * time.Hour
	}
	if strings.HasSuffix(raw, "d") {
		n := strings.TrimSuffix(raw, "d")
		if n == "" {
			return 30 * 24 * time.Hour
		}
		if dayCount, err := strconv.Atoi(n); err == nil && dayCount > 0 {
			return time.Duration(dayCount) * 24 * time.Hour
		}
	}
	if d, err := time.ParseDuration(raw); err == nil {
		return d
	}
	return 30 * 24 * time.Hour
}

func highlightsForFields(fields map[string]string, tokens []string) map[string][]string {
	out := map[string][]string{}
	for field, value := range fields {
		lower := strings.ToLower(value)
		for _, token := range tokens {
			idx := strings.Index(lower, token)
			if idx < 0 {
				continue
			}
			out[field] = []string{value[:idx] + "<mark>" + value[idx:idx+len(token)] + "</mark>" + value[idx+len(token):]}
			break
		}
	}
	return out
}

func finalizeScoredDocs(items []scoredDoc, topK int, minScore float64) []map[string]any {
	sort.Slice(items, func(i, j int) bool { return items[i].score > items[j].score })
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if minScore > 0 && item.score < minScore {
			continue
		}
		out = append(out, item.doc)
		if len(out) == topK {
			break
		}
	}
	return out
}
