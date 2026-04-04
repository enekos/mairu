package contextsrv

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/meilisearch/meilisearch-go"
)

type MeiliIndexer struct {
	client   meilisearch.ServiceManager
	embedder Embedder
}

func NewMeiliIndexer(host, apiKey string, embedder Embedder) *MeiliIndexer {
	return &MeiliIndexer{
		client:   meilisearch.New(host, meilisearch.WithAPIKey(apiKey)),
		embedder: embedder,
	}
}

func (m *MeiliIndexer) EnsureIndexes() error {
	indexes := []string{IndexMemories, IndexSkills, IndexNodes, IndexSymbols}
	for _, idx := range indexes {
		_, _ = m.client.CreateIndex(&meilisearch.IndexConfig{
			Uid:        idx,
			PrimaryKey: "id",
		})
	}
	return nil
}

func (m *MeiliIndexer) Upsert(entityType string, payload map[string]any) error {
	idx, err := indexFromEntity(entityType)
	if err != nil {
		return err
	}
	switch entityType {
	case "context_node":
		if _, ok := payload["id"]; !ok {
			if uri, ok := payload["uri"].(string); ok && uri != "" {
				payload["id"] = uri
			}
		}
	}
	_, err = m.client.Index(idx).AddDocuments([]map[string]any{payload}, nil)
	return err
}

func (m *MeiliIndexer) Delete(entityType, id string) error {
	idx, err := indexFromEntity(entityType)
	if err != nil {
		return err
	}
	_, err = m.client.Index(idx).DeleteDocument(id, nil)
	return err
}

func decodePayload(raw []byte) (map[string]any, error) {
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func indexFromEntity(entityType string) (string, error) {
	switch entityType {
	case "memory":
		return IndexMemories, nil
	case "skill":
		return IndexSkills, nil
	case "context_node":
		return IndexNodes, nil
	case "symbol":
		return IndexSymbols, nil
	default:
		return "", fmt.Errorf("unsupported entity type %q", entityType)
	}
}

func (m *MeiliIndexer) Search(opts SearchOptions) (map[string]any, error) {
	topK := opts.TopK
	if topK <= 0 {
		topK = 10
	}
	store := normalizeStoreName(opts.Store)
	out := map[string]any{
		"memories":     []map[string]any{},
		"skills":       []map[string]any{},
		"contextNodes": []map[string]any{},
	}
	if strings.TrimSpace(opts.Query) == "" {
		return out, nil
	}

	var queryEmbedding []float32
	if m.embedder != nil {
		emb, _ := m.embedder.GetEmbedding(context.Background(), opts.Query)
		queryEmbedding = emb
	}

	if store == "all" || store == "memories" {
		res, err := m.searchIndex(IndexMemories, opts, []string{"content"}, topK, queryEmbedding)
		if err != nil {
			return nil, err
		}
		out["memories"] = res
	}
	if store == "all" || store == "skills" {
		res, err := m.searchIndex(IndexSkills, opts, []string{"name", "description"}, topK, queryEmbedding)
		if err != nil {
			return nil, err
		}
		out["skills"] = res
	}
	if store == "all" || store == "context" {
		res, err := m.searchIndex(IndexNodes, opts, []string{"name", "abstract", "content"}, topK, queryEmbedding)
		if err != nil {
			return nil, err
		}
		out["contextNodes"] = res
	}
	return out, nil
}

func (m *MeiliIndexer) searchIndex(index string, opts SearchOptions, fields []string, limit int, queryEmbedding []float32) ([]map[string]any, error) {
	fetchLimit := limit * 2 // CANDIDATE_MULTIPLIER equivalent
	if fetchLimit < 20 {
		fetchLimit = 20
	}

	defaults := defaultsForIndex(index)
	weights := effectiveWeights(opts, defaults)

	semanticRatio := 0.5
	if weights.vector+weights.keyword > 0 {
		semanticRatio = weights.vector / (weights.vector + weights.keyword)
	}

	req := &meilisearch.SearchRequest{
		Limit:            int64(fetchLimit),
		ShowRankingScore: true,
	}

	if opts.Project != "" {
		req.Filter = fmt.Sprintf(`project = "%s"`, escapeFilterValue(opts.Project))
	}

	if len(queryEmbedding) > 0 {
		req.Vector = queryEmbedding
		req.Hybrid = &meilisearch.SearchRequestHybrid{
			SemanticRatio: float64(semanticRatio),
			Embedder:      "default",
		}
	}

	resp, err := m.client.Index(index).Search(opts.Query, req)
	if err != nil {
		return nil, err
	}

	queryTokens := tokenizeForSearch(opts.Query)
	items := []scoredDoc{}
	for _, hit := range resp.Hits {
		raw, _ := json.Marshal(hit)
		doc := map[string]any{}
		if err := json.Unmarshal(raw, &doc); err != nil {
			continue
		}

		createdAt := parseDocTime(doc["created_at"])
		importance := parseDocInt(doc["importance"])

		var rankingScore float64
		if rs, ok := doc["_rankingScore"].(float64); ok {
			rankingScore = rs
		}

		score := scoreWithMeiliRanking(rankingScore, createdAt, importance, opts, defaults)
		doc["_score"] = score

		if opts.Highlight {
			fieldValues := map[string]string{}
			for _, field := range fields {
				if v, ok := doc[field].(string); ok {
					fieldValues[field] = v
				}
			}
			if h := highlightsForFields(fieldValues, queryTokens); len(h) > 0 {
				doc["_highlight"] = h
			}
		}

		items = append(items, scoredDoc{score: score, doc: doc})
	}
	return finalizeScoredDocs(items, limit, opts.MinScore), nil
}

func defaultsForIndex(index string) hybridWeights {
	switch index {
	case IndexMemories:
		return defaultMemoryWeights()
	case IndexSkills:
		return defaultSkillWeights()
	default:
		return defaultContextWeights()
	}
}

func (m *MeiliIndexer) ClusterStats() map[string]any {
	stats, err := m.client.GetStats()
	if err != nil {
		return map[string]any{
			"ok":      false,
			"service": "contextsrv",
			"error":   err.Error(),
		}
	}
	indexes := map[string]any{}
	for _, uid := range []string{IndexMemories, IndexSkills, IndexNodes, IndexSymbols} {
		docCount := 0
		if idx, ok := stats.Indexes[uid]; ok {
			docCount = int(idx.NumberOfDocuments)
		}
		indexes[uid] = map[string]any{
			"docs": docCount,
		}
	}
	return map[string]any{
		"ok":      true,
		"service": "contextsrv",
		"indexes": indexes,
	}
}

func parseDocTime(raw any) (outTimeValue time.Time) {
	str, ok := raw.(string)
	if !ok || strings.TrimSpace(str) == "" {
		return outTimeValue
	}
	t, err := time.Parse(time.RFC3339, str)
	if err != nil {
		return outTimeValue
	}
	return t
}

func parseDocInt(raw any) int {
	switch n := raw.(type) {
	case int:
		return n
	case float64:
		return int(n)
	default:
		return 0
	}
}

func escapeFilterValue(v string) string {
	return strings.ReplaceAll(v, `"`, `\"`)
}
