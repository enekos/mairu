package contextsrv

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/meilisearch/meilisearch-go"
	"mairu/internal/llm"
)

// embeddingCacheSize is the number of query embeddings kept in the LRU cache.
// Each entry holds a float32 slice (~3 KB for 768-dim models), so 256
// entries cost at most ~0.75 MB.
const embeddingCacheSize = 256

// embeddingCacher is the minimal interface MeiliIndexer needs from the LRU
// cache, making it easy to swap in a no-op in tests.
type embeddingCacher interface {
	Get(key string) ([]float32, bool)
	Put(key string, value []float32)
}

type MeiliIndexer struct {
	client   meilisearch.ServiceManager
	embedder Embedder
	cache    embeddingCacher
}

func NewMeiliIndexer(host, apiKey string, embedder Embedder) *MeiliIndexer {
	return &MeiliIndexer{
		client:   meilisearch.New(host, meilisearch.WithAPIKey(apiKey)),
		embedder: embedder,
		cache:    llm.NewEmbeddingCache(embeddingCacheSize),
	}
}

func (m *MeiliIndexer) EnsureIndexes() error {
	indexes := []string{IndexMemories, IndexSkills, IndexNodes, IndexSymbols, IndexBashHistory}
	for _, idx := range indexes {
		task, err := m.client.CreateIndex(&meilisearch.IndexConfig{
			Uid:        idx,
			PrimaryKey: "id",
		})
		if err != nil {
			return fmt.Errorf("create index %q: %w", idx, err)
		}
		if task != nil {
			_, _ = m.client.WaitForTask(task.TaskUID, 100*time.Millisecond)
		}
		filterable := []interface{}{"project", "uri", "category", "owner"}
		task, err = m.client.Index(idx).UpdateFilterableAttributes(&filterable)
		if err != nil {
			return fmt.Errorf("update filterable attributes for %q: %w", idx, err)
		}
		if task != nil {
			_, _ = m.client.WaitForTask(task.TaskUID, 100*time.Millisecond)
		}

		dim := 768
		if m.embedder != nil {
			dim = m.embedder.GetEmbeddingDimension()
		}
		task, err = m.client.Index(idx).UpdateEmbedders(map[string]meilisearch.Embedder{
			"default": {
				Source:     meilisearch.UserProvidedEmbedderSource,
				Dimensions: dim,
			},
		})
		if err != nil {
			return fmt.Errorf("update embedders for %q: %w", idx, err)
		}
		if task != nil {
			_, _ = m.client.WaitForTask(task.TaskUID, 100*time.Millisecond)
		}
	}
	return nil
}

func sanitizeMeiliID(raw string) string {
	var sb strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			sb.WriteRune(r)
		} else {
			sb.WriteString(fmt.Sprintf("_%x_", r))
		}
	}
	return sb.String()
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
				payload["id"] = sanitizeMeiliID(uri)
			}
		} else {
			if idStr, ok := payload["id"].(string); ok {
				payload["id"] = sanitizeMeiliID(idStr)
			}
		}
	default:
		if idStr, ok := payload["id"].(string); ok {
			payload["id"] = sanitizeMeiliID(idStr)
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
	_, err = m.client.Index(idx).DeleteDocument(sanitizeMeiliID(id), nil)
	return err
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
	case "bash_history":
		return IndexBashHistory, nil
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
		"bashHistory":  []map[string]any{},
	}

	var queryEmbedding []float32
	if strings.TrimSpace(opts.Query) != "" && m.embedder != nil {
		q := strings.TrimSpace(opts.Query)
		if cached, ok := m.cache.Get(q); ok {
			queryEmbedding = cached
		} else if emb, err := m.embedder.GetEmbedding(context.Background(), q); err == nil {
			// Embedding failure is non-fatal: degrade gracefully to keyword-only.
			m.cache.Put(q, emb)
			queryEmbedding = emb
		}
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
	if store == "all" || store == "bash_history" {
		res, err := m.searchIndex(IndexBashHistory, opts, []string{"command", "output"}, topK, queryEmbedding)
		if err != nil {
			return nil, err
		}
		out["bashHistory"] = res
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

		// Extract enrichment data for scoring (churn_score is promoted to top-level by the service layer)
		var enrichmentData map[string]any
		if cs, ok := doc["churn_score"].(float64); ok {
			enrichmentData = map[string]any{"enrichment_churn_score": cs}
		}
		score := scoreWithMeiliRanking(rankingScore, createdAt, importance, opts, defaults, enrichmentData)
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
		return defaultMemoryWeights(nil)
	case IndexBashHistory:
		return defaultMemoryWeights(nil)
	case IndexSkills:
		return defaultSkillWeights(nil)
	default:
		return defaultContextWeights(nil)
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
