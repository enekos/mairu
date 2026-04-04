package contextsrv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"mairu/internal/llm"
)

var ErrModerationRejected = errors.New("content rejected by moderation policy")

type Service interface {
	Health() map[string]any
	CreateMemory(input MemoryCreateInput) (Memory, error)
	ListMemories(project string, limit int) ([]Memory, error)
	UpdateMemory(input MemoryUpdateInput) (Memory, error)
	DeleteMemory(id string) error
	CreateSkill(input SkillCreateInput) (Skill, error)
	ListSkills(project string, limit int) ([]Skill, error)
	UpdateSkill(input SkillUpdateInput) (Skill, error)
	DeleteSkill(id string) error
	CreateContextNode(input ContextCreateInput) (ContextNode, error)
	ListContextNodes(project string, parentURI *string, limit int) ([]ContextNode, error)
	UpdateContextNode(input ContextUpdateInput) (ContextNode, error)
	DeleteContextNode(uri string) error
	Search(opts SearchOptions) (map[string]any, error)
	Dashboard(limit int, project string) (map[string]any, error)
	ClusterStats() map[string]any
	VibeQuery(prompt, project string, topK int) (VibeQueryResult, error)
	PlanVibeMutation(prompt, project string, topK int) (VibeMutationPlan, error)
	ExecuteVibeMutation(ops []VibeMutationOp, project string) ([]map[string]any, error)
	ListModerationQueue(limit int) ([]ModerationEvent, error)
	ReviewModeration(input ModerationReviewInput) error
}

type Repository interface {
	CreateMemory(ctx context.Context, input MemoryCreateInput) (Memory, error)
	ListMemories(ctx context.Context, project string, limit int) ([]Memory, error)
	UpdateMemory(ctx context.Context, input MemoryUpdateInput) (Memory, error)
	DeleteMemory(ctx context.Context, id string) error
	CreateSkill(ctx context.Context, input SkillCreateInput) (Skill, error)
	ListSkills(ctx context.Context, project string, limit int) ([]Skill, error)
	UpdateSkill(ctx context.Context, input SkillUpdateInput) (Skill, error)
	DeleteSkill(ctx context.Context, id string) error
	CreateContextNode(ctx context.Context, input ContextCreateInput) (ContextNode, error)
	ListContextNodes(ctx context.Context, project string, parentURI *string, limit int) ([]ContextNode, error)
	UpdateContextNode(ctx context.Context, input ContextUpdateInput) (ContextNode, error)
	DeleteContextNode(ctx context.Context, uri string) error
	SearchText(ctx context.Context, opts SearchOptions) (map[string]any, error)
	ListModerationQueue(ctx context.Context, limit int) ([]ModerationEvent, error)
	ReviewModeration(ctx context.Context, input ModerationReviewInput) error
	EnqueueOutbox(ctx context.Context, entityType, entityID, opType string, payload any) error
}

type AppService struct {
	repo          Repository
	searchBackend SearchBackend
	llmClient     LLMClient
}

func NewService(repo Repository) *AppService {
	return &AppService{repo: repo}
}

type SearchBackend interface {
	Search(opts SearchOptions) (map[string]any, error)
	ClusterStats() map[string]any
}

func NewServiceWithSearch(repo Repository, backend SearchBackend, llmClient LLMClient) *AppService {
	return &AppService{repo: repo, searchBackend: backend, llmClient: llmClient}
}

func (s *AppService) Health() map[string]any {
	return map[string]any{"ok": true, "service": "contextsrv"}
}

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

func (s *AppService) CreateSkill(input SkillCreateInput) (Skill, error) {
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Description) == "" {
		return Skill{}, fmt.Errorf("name and description are required")
	}
	m := ModerateContent(input.Name + ": " + input.Description)
	input.ModerationStatus = m.Status
	input.ModerationReasons = m.Reasons
	input.ReviewRequired = m.Status == ModerationStatusFlaggedSoft
	if m.Status == ModerationStatusRejectHard {
		return Skill{}, fmt.Errorf("%w: %s", ErrModerationRejected, strings.Join(m.Reasons, ", "))
	}
	if len(input.Metadata) == 0 {
		input.Metadata = json.RawMessage(`{}`)
	}
	out, err := s.repo.CreateSkill(context.Background(), input)
	if err != nil {
		return Skill{}, err
	}
	_ = s.repo.EnqueueOutbox(context.Background(), "skill", out.ID, "upsert", out)
	return out, nil
}

func (s *AppService) ListSkills(project string, limit int) ([]Skill, error) {
	return s.repo.ListSkills(context.Background(), project, limit)
}

func (s *AppService) UpdateSkill(input SkillUpdateInput) (Skill, error) {
	return s.repo.UpdateSkill(context.Background(), input)
}

func (s *AppService) DeleteSkill(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("id is required")
	}
	return s.repo.DeleteSkill(context.Background(), id)
}

func (s *AppService) CreateContextNode(input ContextCreateInput) (ContextNode, error) {
	if strings.TrimSpace(input.URI) == "" || strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Abstract) == "" {
		return ContextNode{}, fmt.Errorf("uri, name, and abstract are required")
	}

	// Router logic for deduplication
	if s.searchBackend != nil && s.llmClient != nil {
		searchRes, err := s.searchBackend.Search(SearchOptions{
			Query:        input.Abstract,
			Project:      input.Project,
			Store:        "node",
			TopK:         5,
			WeightVector: 1.0,
		})
		if err == nil && searchRes["contextNodes"] != nil {
			items := toAnyMapSlice(searchRes["contextNodes"])
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

func (s *AppService) Search(opts SearchOptions) (map[string]any, error) {
	if s.searchBackend != nil {
		if out, err := s.searchBackend.Search(opts); err == nil {
			return out, nil
		}
	}
	return s.repo.SearchText(context.Background(), opts)
}

func (s *AppService) Dashboard(limit int, project string) (map[string]any, error) {
	memories, err := s.repo.ListMemories(context.Background(), project, limit)
	if err != nil {
		return nil, err
	}
	skills, err := s.repo.ListSkills(context.Background(), project, limit)
	if err != nil {
		return nil, err
	}
	contextNodes, err := s.repo.ListContextNodes(context.Background(), project, nil, limit)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"counts": map[string]int{
			"skills":       len(skills),
			"memories":     len(memories),
			"contextNodes": len(contextNodes),
		},
		"skills":       skills,
		"memories":     memories,
		"contextNodes": contextNodes,
	}, nil
}

func (s *AppService) ClusterStats() map[string]any {
	if s.searchBackend != nil {
		return s.searchBackend.ClusterStats()
	}
	return map[string]any{
		"ok":      true,
		"service": "contextsrv",
		"indexes": []string{"contextfs_memories", "contextfs_skills", "contextfs_context_nodes"},
	}
}

func (s *AppService) ListModerationQueue(limit int) ([]ModerationEvent, error) {
	return s.repo.ListModerationQueue(context.Background(), limit)
}

func (s *AppService) ReviewModeration(input ModerationReviewInput) error {
	return s.repo.ReviewModeration(context.Background(), input)
}
