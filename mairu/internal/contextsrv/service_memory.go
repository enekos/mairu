package contextsrv

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"mairu/internal/llm"
)

func (s *AppService) CreateMemory(input MemoryCreateInput) (Memory, error) {
	if strings.TrimSpace(input.Content) == "" {
		return Memory{}, fmt.Errorf("content is required")
	}
	if input.Importance <= 0 {
		input.Importance = 1
	}
	if input.Category == "" {
		input.Category = "observation"
	}
	if input.Owner == "" {
		input.Owner = "agent"
	}

	// Router logic for deduplication
	if s.searchBackend != nil && s.llmClient != nil {
		searchRes, err := s.searchBackend.Search(SearchOptions{
			Query:        input.Content,
			Project:      input.Project,
			Store:        StoreMemory,
			TopK:         5,
			WeightVector: 1.0,
		})
		if err == nil && searchRes[StoreMemories] != nil {
			items := toAnyMapSlice(searchRes[StoreMemories])
			var candidates []llm.RouterCandidate
			for _, item := range items {
				id, _ := item["id"].(string)
				content, _ := item["content"].(string)
				score, _ := item["_score"].(float64)
				if id != "" && content != "" {
					candidates = append(candidates, llm.RouterCandidate{ID: id, Content: content, Score: score})
				}
			}
			action, err := llm.DecideMemoryAction(context.Background(), s.llmClient, input.Content, candidates)
			if err == nil {
				if action.Action == "skip" {
					return Memory{ID: "skipped"}, fmt.Errorf("skipped: %s", action.Reason)
				}
				if action.Action == "update" && action.TargetID != "" {
					updated, err := s.UpdateMemory(MemoryUpdateInput{
						ID:         action.TargetID,
						Content:    action.MergedContent,
						Importance: input.Importance,
					})
					if err == nil {
						return updated, nil
					}
				}
			}
		}
	}

	m := ModerateContent(input.Content, s.moderationEnabled)
	input.ModerationStatus = m.Status
	input.ModerationReasons = m.Reasons
	input.ReviewRequired = m.Status == ModerationStatusFlaggedSoft
	if m.Status == ModerationStatusRejectHard {
		return Memory{}, fmt.Errorf("%w: %s", ErrModerationRejected, strings.Join(m.Reasons, ", "))
	}
	if len(input.Metadata) == 0 {
		input.Metadata = json.RawMessage(`{}`)
	}

	if s.repo == nil {
		out := Memory{
			ID:                fmt.Sprintf("mem_%d", time.Now().UnixNano()),
			Project:           input.Project,
			Content:           input.Content,
			Category:          input.Category,
			Owner:             input.Owner,
			Importance:        input.Importance,
			ModerationStatus:  input.ModerationStatus,
			ModerationReasons: input.ModerationReasons,
			ReviewRequired:    input.ReviewRequired,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		if s.searchBackend != nil {
			if mIdx, ok := s.searchBackend.(*MeiliIndexer); ok {
				payload := map[string]any{
					"id":         out.ID,
					"project":    out.Project,
					"content":    out.Content,
					"category":   out.Category,
					"owner":      out.Owner,
					"importance": out.Importance,
					"created_at": out.CreatedAt.Unix(),
				}
				payload["_vectors"] = map[string]any{"default": nil}
				if emb, ok := s.llmClient.(fallbackEmbedder); ok {
					vec, err := emb.GetEmbedding(context.Background(), out.Content)
					if err == nil && len(vec) > 0 {
						payload["_vectors"] = map[string]any{"default": vec}
					}
				}
				if err := mIdx.Upsert("memory", payload); err != nil {
					fmt.Printf("Meilisearch Upsert error: %v\n", err)
				}
			}
		}
		return out, nil
	}

	out, err := s.repo.CreateMemory(context.Background(), input)
	if err != nil {
		return Memory{}, err
	}
	_ = s.repo.EnqueueOutbox(context.Background(), "memory", out.ID, "upsert", out)
	return out, nil
}

func (s *AppService) ListMemories(project string, limit int) ([]Memory, error) {
	if s.repo == nil {
		if s.searchBackend != nil {
			opts := SearchOptions{Project: project, Store: StoreMemory, TopK: limit}
			res, err := s.searchBackend.Search(opts)
			if err != nil {
				return nil, err
			}
			var out []Memory
			for _, item := range toAnyMapSlice(res[StoreMemories]) {
				var m Memory
				if id, ok := item["id"].(string); ok {
					m.ID = id
				}
				if p, ok := item["project"].(string); ok {
					m.Project = p
				}
				if c, ok := item["content"].(string); ok {
					m.Content = c
				}
				if cat, ok := item["category"].(string); ok {
					m.Category = cat
				}
				if o, ok := item["owner"].(string); ok {
					m.Owner = o
				}
				if imp, ok := item["importance"].(float64); ok {
					m.Importance = int(imp)
				}
				out = append(out, m)
			}
			return out, nil
		}
		return []Memory{}, nil
	}
	return s.repo.ListMemories(context.Background(), project, limit)
}

func (s *AppService) UpdateMemory(input MemoryUpdateInput) (Memory, error) {
	if s.repo == nil {
		return Memory{ID: input.ID, Content: input.Content}, nil
	}
	return s.repo.UpdateMemory(context.Background(), input)
}

func (s *AppService) DeleteMemory(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("id is required")
	}
	if s.repo == nil {
		if s.searchBackend != nil {
			if mIdx, ok := s.searchBackend.(*MeiliIndexer); ok {
				return mIdx.Delete("memory", id)
			}
		}
		return nil
	}
	return s.repo.DeleteMemory(context.Background(), id)
}
