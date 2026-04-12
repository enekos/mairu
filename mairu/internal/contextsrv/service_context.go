package contextsrv

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"mairu/internal/llm"
)

func (s *AppService) CreateContextNode(input ContextCreateInput) (ContextNode, error) {
	if strings.TrimSpace(input.URI) == "" || strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Abstract) == "" {
		return ContextNode{}, fmt.Errorf("uri, name, and abstract are required")
	}

	// Router logic for deduplication
	if s.searchBackend != nil && s.llmClient != nil {
		searchRes, err := s.searchBackend.Search(SearchOptions{
			Query:        input.Abstract,
			Project:      input.Project,
			Store:        StoreNode,
			TopK:         5,
			WeightVector: 1.0,
		})
		if err == nil && searchRes[StoreContextNodes] != nil {
			items := toAnyMapSlice(searchRes[StoreContextNodes])
			var candidates []llm.RouterCandidate
			for _, item := range items {
				uri, _ := item["uri"].(string)
				abstract, _ := item["abstract"].(string)
				score, _ := item["_score"].(float64)
				if uri != "" && abstract != "" {
					candidates = append(candidates, llm.RouterCandidate{ID: uri, Content: abstract, Score: score})
				}
			}
			action, err := llm.DecideContextAction(context.Background(), s.llmClient, input.URI, input.Name, input.Abstract, candidates)
			if err == nil {
				if action.Action == "skip" {
					return ContextNode{URI: "skipped"}, fmt.Errorf("skipped: %s", action.Reason)
				}
				if action.Action == "update" && action.TargetID != "" {
					updated, err := s.UpdateContextNode(ContextUpdateInput{
						URI:      action.TargetID,
						Abstract: action.MergedContent,
					})
					if err == nil {
						return updated, nil
					}
				}
			}
		}
	}

	m := ModerateContent(input.Name+": "+input.Abstract+"\n"+input.Content, s.moderationEnabled)
	input.ModerationStatus = m.Status
	input.ModerationReasons = m.Reasons
	input.ReviewRequired = m.Status == ModerationStatusFlaggedSoft
	if m.Status == ModerationStatusRejectHard {
		return ContextNode{}, fmt.Errorf("%w: %s", ErrModerationRejected, strings.Join(m.Reasons, ", "))
	}
	if len(input.Metadata) == 0 {
		input.Metadata = json.RawMessage(`{}`)
	}

	if s.repo == nil {
		out := ContextNode{
			URI:               input.URI,
			Project:           input.Project,
			ParentURI:         input.ParentURI,
			Name:              input.Name,
			Abstract:          input.Abstract,
			Overview:          input.Overview,
			Content:           input.Content,
			ModerationStatus:  input.ModerationStatus,
			ModerationReasons: input.ModerationReasons,
			ReviewRequired:    input.ReviewRequired,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		if s.searchBackend != nil {
			if mIdx, ok := s.searchBackend.(*MeiliIndexer); ok {
				payload := map[string]any{
					"id":         out.URI,
					"uri":        out.URI,
					"project":    out.Project,
					"name":       out.Name,
					"abstract":   out.Abstract,
					"overview":   out.Overview,
					"content":    out.Content,
					"created_at": out.CreatedAt.Unix(),
				}
				if out.ParentURI != nil {
					payload["parent_uri"] = *out.ParentURI
				}

				// Promote enrichment fields from metadata to top-level Meili fields
				// so they're searchable/filterable.
				if len(input.Metadata) > 0 {
					var meta map[string]any
					if err := json.Unmarshal(input.Metadata, &meta); err == nil {
						if intent, ok := meta["enrichment_intent"].(string); ok && intent != "" {
							payload["intent"] = intent
						}
						if score, ok := meta["enrichment_churn_score"].(float64); ok {
							payload["churn_score"] = score
						}
						if label, ok := meta["enrichment_churn_label"].(string); ok && label != "" {
							payload["churn_label"] = label
						}
					}
				}

				payload["_vectors"] = map[string]any{"default": nil}
				if s.embedder != nil {
					textToEmbed := out.Name + "\n" + out.Abstract + "\n" + out.Content
					vec, err := s.embedder.GetEmbedding(context.Background(), textToEmbed)
					if err == nil && len(vec) > 0 {
						payload["_vectors"] = map[string]any{"default": vec}
					}
				}
				if err := mIdx.Upsert("context_node", payload); err != nil {
					slog.Error("Meilisearch Upsert error", "error", err)
				}
			}
		}
		return out, nil
	}

	out, err := s.repo.CreateContextNode(context.Background(), input)
	if err != nil {
		return ContextNode{}, err
	}
	_ = s.repo.EnqueueOutbox(context.Background(), "context_node", out.URI, "upsert", out)
	return out, nil
}

func (s *AppService) ListContextNodes(project string, parentURI *string, limit int) ([]ContextNode, error) {
	if s.repo == nil {
		if s.searchBackend != nil {
			opts := SearchOptions{
				Project: project,
				Store:   StoreNode,
				TopK:    limit,
				Query:   "",
			}
			res, err := s.searchBackend.Search(opts)
			if err != nil {
				return nil, err
			}
			items := toAnyMapSlice(res[StoreContextNodes])
			var out []ContextNode
			for _, item := range items {
				var n ContextNode
				if uri, ok := item["uri"].(string); ok {
					n.URI = uri
				}
				if p, ok := item["project"].(string); ok {
					n.Project = p
				}
				if name, ok := item["name"].(string); ok {
					n.Name = name
				}
				if abs, ok := item["abstract"].(string); ok {
					n.Abstract = abs
				}
				if over, ok := item["overview"].(string); ok {
					n.Overview = over
				}
				if cnt, ok := item["content"].(string); ok {
					n.Content = cnt
				}
				out = append(out, n)
			}
			return out, nil
		}
		return []ContextNode{}, nil
	}
	return s.repo.ListContextNodes(context.Background(), project, parentURI, limit)
}

func (s *AppService) UpdateContextNode(input ContextUpdateInput) (ContextNode, error) {
	if s.repo == nil {
		// Minimal fallback
		return ContextNode{URI: input.URI, Abstract: input.Abstract}, nil
	}
	return s.repo.UpdateContextNode(context.Background(), input)
}

func (s *AppService) DeleteContextNode(uri string) error {
	if strings.TrimSpace(uri) == "" {
		return fmt.Errorf("uri is required")
	}
	if s.repo == nil {
		if s.searchBackend != nil {
			if mIdx, ok := s.searchBackend.(*MeiliIndexer); ok {
				return mIdx.Delete("context_node", uri)
			}
		}
		return nil
	}
	return s.repo.DeleteContextNode(context.Background(), uri)
}
