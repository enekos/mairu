package contextsrv

import (
	"context"
	"errors"

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
	Ingest(text, baseURI string) ([]llm.ProposedContextNode, error)
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
			StoreSkills:       len(skills),
			StoreMemories:     len(memories),
			StoreContextNodes: len(contextNodes),
		},
		StoreSkills:       skills,
		StoreMemories:     memories,
		StoreContextNodes: contextNodes,
	}, nil
}

func (s *AppService) ClusterStats() map[string]any {
	if s.searchBackend != nil {
		return s.searchBackend.ClusterStats()
	}
	return map[string]any{
		"ok":      true,
		"service": "contextsrv",
		"indexes": []string{IndexMemories, IndexSkills, IndexNodes},
	}
}

func (s *AppService) Ingest(text, baseURI string) ([]llm.ProposedContextNode, error) {
	if s.llmClient == nil {
		return nil, errors.New("no llm client configured")
	}
	return llm.ParseTextIntoContextNodes(context.Background(), s.llmClient, "gemini-2.5-flash", text, baseURI)
}
