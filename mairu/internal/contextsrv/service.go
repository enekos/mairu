package contextsrv

import (
	"context"
	"errors"

	"mairu/internal/llm"
)

var ErrModerationRejected = errors.New("content rejected by moderation policy")

// ---- domain-scoped sub-interfaces ----
// Callers that only need one domain can depend on the narrower type instead
// of the full Service or Repository, making dependencies explicit and tests
// easier to stub.

// MemoryService covers the full memory lifecycle.
type MemoryService interface {
	CreateMemory(input MemoryCreateInput) (Memory, error)
	ListMemories(project string, limit int) ([]Memory, error)
	GetMemory(id string) (Memory, error)
	UpdateMemory(input MemoryUpdateInput) (Memory, error)
	DeleteMemory(id string) error
	ApplyMemoryFeedback(id string, reward int) (Memory, error)
}

// SkillService covers the full skill lifecycle.
type SkillService interface {
	CreateSkill(input SkillCreateInput) (Skill, error)
	ListSkills(project string, limit int) ([]Skill, error)
	UpdateSkill(input SkillUpdateInput) (Skill, error)
	DeleteSkill(id string) error
}

// NodeService covers the full context-node lifecycle.
type NodeService interface {
	CreateContextNode(input ContextCreateInput) (ContextNode, error)
	ListContextNodes(project string, parentURI *string, limit int) ([]ContextNode, error)
	UpdateContextNode(input ContextUpdateInput) (ContextNode, error)
	DeleteContextNode(uri string) error
}

// ModerationService covers the moderation review workflow.
type ModerationService interface {
	ListModerationQueue(limit int) ([]ModerationEvent, error)
	ReviewModeration(input ModerationReviewInput) error
}

// VibeService covers natural-language planning and mutation.
type VibeService interface {
	VibeQuery(prompt, project string, topK int) (VibeQueryResult, error)
	PlanVibeMutation(prompt, project string, topK int) (VibeMutationPlan, error)
	ExecuteVibeMutation(ops []VibeMutationOp, project string) ([]map[string]any, error)
}

// Service defines the interface for core functionality, including memories, skills,
// context nodes, vibe querying and mutation, and moderation.
// It composes the domain-scoped sub-interfaces so that callers that only need
// one domain can depend on the narrower type.
type Service interface {
	MemoryService
	SkillService
	NodeService
	ModerationService
	VibeService
	Health() map[string]any
	Search(opts SearchOptions) (map[string]any, error)
	Dashboard(limit int, project string) (map[string]any, error)
	ClusterStats() map[string]any
	Ingest(text, baseURI string) ([]llm.ProposedContextNode, error)
}

// ---- repository sub-interfaces ----

// MemoryRepository covers memory persistence.
type MemoryRepository interface {
	CreateMemory(ctx context.Context, input MemoryCreateInput) (Memory, error)
	ListMemories(ctx context.Context, project string, limit int) ([]Memory, error)
	GetMemory(ctx context.Context, id string) (Memory, error)
	UpdateMemory(ctx context.Context, input MemoryUpdateInput) (Memory, error)
	DeleteMemory(ctx context.Context, id string) error
}

// SkillRepository covers skill persistence.
type SkillRepository interface {
	CreateSkill(ctx context.Context, input SkillCreateInput) (Skill, error)
	ListSkills(ctx context.Context, project string, limit int) ([]Skill, error)
	UpdateSkill(ctx context.Context, input SkillUpdateInput) (Skill, error)
	DeleteSkill(ctx context.Context, id string) error
}

// NodeRepository covers context-node persistence.
type NodeRepository interface {
	CreateContextNode(ctx context.Context, input ContextCreateInput) (ContextNode, error)
	ListContextNodes(ctx context.Context, project string, parentURI *string, limit int) ([]ContextNode, error)
	UpdateContextNode(ctx context.Context, input ContextUpdateInput) (ContextNode, error)
	DeleteContextNode(ctx context.Context, uri string) error
}

// Repository encapsulates data access logic, usually persisting to a database.
// It composes the domain-scoped repository sub-interfaces.
type Repository interface {
	MemoryRepository
	SkillRepository
	NodeRepository
	SearchText(ctx context.Context, opts SearchOptions) (map[string]any, error)
	ListModerationQueue(ctx context.Context, limit int) ([]ModerationEvent, error)
	ReviewModeration(ctx context.Context, input ModerationReviewInput) error
	EnqueueOutbox(ctx context.Context, entityType, entityID, opType string, payload any) error
}

// AppService is the default implementation of the Service interface.
type AppService struct {
	repo              Repository
	searchBackend     SearchBackend
	llmClient         LLMClient
	moderationEnabled bool
}

// NewService creates a new AppService with the given repository.
func NewService(repo Repository) *AppService {
	return &AppService{repo: repo}
}

// SearchBackend defines the capability for advanced text or hybrid searches.
type SearchBackend interface {
	Search(opts SearchOptions) (map[string]any, error)
	ClusterStats() map[string]any
}

// NewServiceWithSearch creates an AppService with both a repository and search backend.
func NewServiceWithSearch(repo Repository, backend SearchBackend, llmClient LLMClient, moderationEnabled bool) *AppService {
	return &AppService{repo: repo, searchBackend: backend, llmClient: llmClient, moderationEnabled: moderationEnabled}
}

// Health returns basic service health status.
func (s *AppService) Health() map[string]any {
	return map[string]any{"ok": true, "service": "contextsrv"}
}

// Search performs a search across memories, skills, and/or context nodes.
func (s *AppService) Search(opts SearchOptions) (map[string]any, error) {
	if s.searchBackend != nil {
		out, err := s.searchBackend.Search(opts)
		if err == nil {
			return out, nil
		}
		// Log the underlying search error but fall through to DB search.
		// (In a real app, use a logger; here we can return it if repo is nil)
		if s.repo == nil {
			return nil, err
		}
	}
	if s.repo == nil {
		return nil, errors.New("no search backend and no repository configured")
	}
	return s.repo.SearchText(context.Background(), opts)
}

// Dashboard returns a summary of the current project state.
func (s *AppService) Dashboard(limit int, project string) (map[string]any, error) {
	if s.repo == nil {
		return map[string]any{
			"counts": map[string]int{
				StoreSkills:       0,
				StoreMemories:     0,
				StoreContextNodes: 0,
			},
			StoreSkills:       []Skill{},
			StoreMemories:     []Memory{},
			StoreContextNodes: []ContextNode{},
			"warning":         "SQLite repository not configured",
		}, nil
	}
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

// ClusterStats retrieves search cluster indexing metadata.
func (s *AppService) ClusterStats() map[string]any {
	if s.searchBackend != nil {
		return s.searchBackend.ClusterStats()
	}
	return map[string]any{
		"ok":      true,
		"service": "contextsrv",
		"indexes": []string{IndexMemories, IndexSkills, IndexNodes, IndexSymbols},
	}
}

// Ingest converts raw text into structured ContextNodes via an LLM.
func (s *AppService) Ingest(text, baseURI string) ([]llm.ProposedContextNode, error) {
	if s.llmClient == nil {
		return nil, errors.New("no llm client configured")
	}
	return llm.ParseTextIntoContextNodes(context.Background(), s.llmClient, "gemini-2.5-flash", text, baseURI)
}
