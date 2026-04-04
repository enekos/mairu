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
}

type scoredDoc struct {
	score float64
	doc   map[string]any
}

var tokenSplitRegex = regexp.MustCompile(`[^a-z0-9]+`)

func defaultMemoryWeights() hybridWeights {
	return hybridWeights{vector: 0.6, keyword: 0.2, recency: 0.05, importance: 0.15}
}

func defaultSkillWeights() hybridWeights {
	return hybridWeights{vector: 0.7, keyword: 0.3, recency: 0, importance: 0}
}

func defaultContextWeights() hybridWeights {
	return hybridWeights{vector: 0.65, keyword: 0.3, recency: 0.05, importance: 0}
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

func scoreHybrid(fields map[string]string, queryTokens []string, createdAt time.Time, importance int, opts SearchOptions, defaults hybridWeights) float64 {
	weights := effectiveWeights(opts, defaults)
	keywordScore := scoreKeyword(fields, queryTokens, opts.FieldBoosts)
	recencyScore := scoreRecency(createdAt, opts.RecencyScale, opts.RecencyDecay)
	importanceScore := 0.0
	if importance > 0 {
		importanceScore = float64(importance) / 10.0
	}

	return keywordScore*weights.keyword + recencyScore*weights.recency + importanceScore*weights.importance
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
	total := w.vector + w.keyword + w.recency + w.importance
	if total <= 0 {
		return defaults
	}
	return hybridWeights{
		vector:     w.vector / total,
		keyword:    w.keyword / total,
		recency:    w.recency / total,
		importance: w.importance / total,
	}
}

func scoreWithMeiliRanking(rankingScore float64, createdAt time.Time, importance int, opts SearchOptions, defaults hybridWeights) float64 {
	weights := effectiveWeights(opts, defaults)

	// rankingScore is the Meilisearch combined vector+keyword score, already normalized.
	// But it only represents the "vector + keyword" portion of our weights.
	// So we add recency and importance on top.
	score := rankingScore

	recencyScore := scoreRecency(createdAt, opts.RecencyScale, opts.RecencyDecay)
	importanceScore := 0.0
	if importance > 0 {
		importanceScore = float64(importance) / 10.0
	}

	if weights.recency > 0 {
		score += recencyScore * weights.recency
	}
	if weights.importance > 0 {
		score += importanceScore * weights.importance
	}
	// Add AI quality function boost if we ever index ai_quality_score

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
