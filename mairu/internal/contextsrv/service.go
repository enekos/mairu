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

type BashHistoryService interface {
	ApplyBashHistoryFeedback(id string, reward int) (BashHistory, error)
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
	BashHistoryService
	Health() map[string]any
	Search(opts SearchOptions) (map[string]any, error)
	Dashboard(limit int, project string) (map[string]any, error)
	ClusterStats() map[string]any
	Ingest(text, baseURI string) ([]llm.ProposedContextNode, error)
	Autocomplete(req AutocompleteRequest) (AutocompleteResponse, error)
}

// ---- repository sub-interfaces ----

// MemoryRepository covers memory persistence.
type MemoryRepository interface {
	CreateMemory(ctx context.Context, input MemoryCreateInput) (Memory, error)
	ListMemories(ctx context.Context, project string, limit int) ([]Memory, error)
	GetMemory(ctx context.Context, id string) (Memory, error)
	UpdateMemory(ctx context.Context, input MemoryUpdateInput) (Memory, error)
	DeleteMemory(ctx context.Context, id string) error
	RecordRetrievals(ctx context.Context, ids []string) error
	IncrementFeedbackCount(ctx context.Context, id string) error
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

// BashHistoryRepository covers bash history persistence.
type BashHistoryRepository interface {
	InsertBashHistory(ctx context.Context, project string, command string, exitCode int, durationMs int, output string) error
	GetBashHistory(ctx context.Context, id string) (BashHistory, error)
	UpdateBashHistory(ctx context.Context, h BashHistory) error
	IncrementBashHistoryFeedbackCount(ctx context.Context, id string) error
}

// Repository encapsulates data access logic, usually persisting to a database.
// It composes the domain-scoped repository sub-interfaces.
type Repository interface {
	MemoryRepository
	SkillRepository
	NodeRepository
	BashHistoryRepository
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
	embedder          Embedder
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
func NewServiceWithSearch(repo Repository, backend SearchBackend, llmClient LLMClient, embedder Embedder, moderationEnabled bool) *AppService {
	return &AppService{repo: repo, searchBackend: backend, llmClient: llmClient, embedder: embedder, moderationEnabled: moderationEnabled}
}

// Health returns basic service health status.
func (s *AppService) Health() map[string]any {
	return map[string]any{"ok": true, "service": "contextsrv"}
}

// Search performs a search across memories, skills, and/or context nodes.
func (s *AppService) Search(opts SearchOptions) (map[string]any, error) {
	var out map[string]any
	var err error

	if s.searchBackend != nil {
		out, err = s.searchBackend.Search(opts)
		if err == nil {
			s.recordMemoryRetrievals(out)
			return out, nil
		}
		if s.repo == nil {
			return nil, err
		}
	}
	if s.repo == nil {
		return nil, errors.New("no search backend and no repository configured")
	}
	out, err = s.repo.SearchText(context.Background(), opts)
	if err == nil {
		s.recordMemoryRetrievals(out)
	}
	return out, err
}

// recordMemoryRetrievals fires a goroutine to record retrieval events for
// every memory ID in the search result set. Errors are silently dropped so
// that tracking never blocks or fails the caller.
func (s *AppService) recordMemoryRetrievals(results map[string]any) {
	if s.repo == nil {
		return
	}
	memories, _ := results["memories"].([]map[string]any)
	if len(memories) == 0 {
		return
	}
	ids := make([]string, 0, len(memories))
	for _, m := range memories {
		if id, ok := m["id"].(string); ok && id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return
	}
	go func() {
		_ = s.repo.RecordRetrievals(context.Background(), ids)
	}()
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
