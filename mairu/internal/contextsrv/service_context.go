package contextsrv

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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

	m := ModerateContent(input.Name + ": " + input.Abstract + "\n" + input.Content)
	input.ModerationStatus = m.Status
	input.ModerationReasons = m.Reasons
	input.ReviewRequired = m.Status == ModerationStatusFlaggedSoft
	if m.Status == ModerationStatusRejectHard {
		return ContextNode{}, fmt.Errorf("%w: %s", ErrModerationRejected, strings.Join(m.Reasons, ", "))
	}
	if len(input.Metadata) == 0 {
		input.Metadata = json.RawMessage(`{}`)
	}
	out, err := s.repo.CreateContextNode(context.Background(), input)
	if err != nil {
		return ContextNode{}, err
	}
	_ = s.repo.EnqueueOutbox(context.Background(), "context_node", out.URI, "upsert", out)
	return out, nil
}

func (s *AppService) ListContextNodes(project string, parentURI *string, limit int) ([]ContextNode, error) {
	return s.repo.ListContextNodes(context.Background(), project, parentURI, limit)
}

func (s *AppService) UpdateContextNode(input ContextUpdateInput) (ContextNode, error) {
	return s.repo.UpdateContextNode(context.Background(), input)
}

func (s *AppService) DeleteContextNode(uri string) error {
	if strings.TrimSpace(uri) == "" {
		return fmt.Errorf("uri is required")
	}
	return s.repo.DeleteContextNode(context.Background(), uri)
}
