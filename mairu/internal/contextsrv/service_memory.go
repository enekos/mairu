package contextsrv

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
			Store:        "memory",
			TopK:         5,
			WeightVector: 1.0,
		})
		if err == nil && searchRes["memories"] != nil {
			items := toAnyMapSlice(searchRes["memories"])
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

	m := ModerateContent(input.Content)
	input.ModerationStatus = m.Status
	input.ModerationReasons = m.Reasons
	input.ReviewRequired = m.Status == ModerationStatusFlaggedSoft
	if m.Status == ModerationStatusRejectHard {
		return Memory{}, fmt.Errorf("%w: %s", ErrModerationRejected, strings.Join(m.Reasons, ", "))
	}
	if len(input.Metadata) == 0 {
		input.Metadata = json.RawMessage(`{}`)
	}
	out, err := s.repo.CreateMemory(context.Background(), input)
	if err != nil {
		return Memory{}, err
	}
	_ = s.repo.EnqueueOutbox(context.Background(), "memory", out.ID, "upsert", out)
	return out, nil
}

func (s *AppService) ListMemories(project string, limit int) ([]Memory, error) {
	return s.repo.ListMemories(context.Background(), project, limit)
}

func (s *AppService) UpdateMemory(input MemoryUpdateInput) (Memory, error) {
	return s.repo.UpdateMemory(context.Background(), input)
}

func (s *AppService) DeleteMemory(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("id is required")
	}
	return s.repo.DeleteMemory(context.Background(), id)
}
