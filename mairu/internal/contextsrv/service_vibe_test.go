package contextsrv

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/generative-ai-go/genai"
)

type vibeRepo struct {
	deletedMemoryID string
	deletedSkillID  string
	deletedNodeURI  string
	updatedNodeURI  string
	updatedSkillID  string
	createdNodeURI  string
}

func (r *vibeRepo) CreateMemory(ctx context.Context, input MemoryCreateInput) (Memory, error) {
	return Memory{ID: "mem_created", Content: input.Content, CreatedAt: time.Now().UTC()}, nil
}
func (r *vibeRepo) ListMemories(ctx context.Context, project string, limit int) ([]Memory, error) {
	return []Memory{}, nil
}
func (r *vibeRepo) UpdateMemory(ctx context.Context, input MemoryUpdateInput) (Memory, error) {
	return Memory{ID: input.ID, Content: input.Content, UpdatedAt: time.Now().UTC()}, nil
}
func (r *vibeRepo) DeleteMemory(ctx context.Context, id string) error {
	r.deletedMemoryID = id
	return nil
}
func (r *vibeRepo) CreateSkill(ctx context.Context, input SkillCreateInput) (Skill, error) {
	return Skill{ID: "skill_created", Name: input.Name}, nil
}
func (r *vibeRepo) ListSkills(ctx context.Context, project string, limit int) ([]Skill, error) {
	return []Skill{}, nil
}
func (r *vibeRepo) UpdateSkill(ctx context.Context, input SkillUpdateInput) (Skill, error) {
	r.updatedSkillID = input.ID
	return Skill{ID: input.ID, Name: input.Name, Description: input.Description}, nil
}
func (r *vibeRepo) DeleteSkill(ctx context.Context, id string) error {
	r.deletedSkillID = id
	return nil
}
func (r *vibeRepo) CreateContextNode(ctx context.Context, input ContextCreateInput) (ContextNode, error) {
	r.createdNodeURI = input.URI
	return ContextNode{URI: input.URI, Name: input.Name, Abstract: input.Abstract}, nil
}
func (r *vibeRepo) ListContextNodes(ctx context.Context, project string, parentURI *string, limit int) ([]ContextNode, error) {
	return []ContextNode{}, nil
}
func (r *vibeRepo) UpdateContextNode(ctx context.Context, input ContextUpdateInput) (ContextNode, error) {
	r.updatedNodeURI = input.URI
	return ContextNode{URI: input.URI, Name: input.Name, Abstract: input.Abstract}, nil
}
func (r *vibeRepo) DeleteContextNode(ctx context.Context, uri string) error {
	r.deletedNodeURI = uri
	return nil
}
func (r *vibeRepo) SearchText(ctx context.Context, opts SearchOptions) (map[string]any, error) {
	return map[string]any{
		"memories": []map[string]any{
			{"id": "mem_1", "content": "existing memory for migration", "_score": 0.98},
		},
		"skills": []map[string]any{
			{"id": "skill_1", "name": "Migration", "description": "Run migrations safely"},
		},
		"contextNodes": []map[string]any{
			{"uri": "contextfs://demo/db/migrations", "name": "Migrations", "abstract": "Database migration runbook"},
		},
	}, nil
}
func (r *vibeRepo) ListModerationQueue(ctx context.Context, limit int) ([]ModerationEvent, error) {
	return []ModerationEvent{}, nil
}
func (r *vibeRepo) ReviewModeration(ctx context.Context, input ModerationReviewInput) error {
	return nil
}
func (r *vibeRepo) GetMemory(ctx context.Context, id string) (Memory, error) {
	return Memory{}, nil
}
func (r *vibeRepo) RecordRetrievals(ctx context.Context, ids []string) error    { return nil }
func (r *vibeRepo) IncrementFeedbackCount(ctx context.Context, id string) error { return nil }
func (r *vibeRepo) GetBashHistory(ctx context.Context, id string) (BashHistory, error) {
	return BashHistory{}, nil
}
func (r *vibeRepo) UpdateBashHistory(ctx context.Context, h BashHistory) error { return nil }
func (r *vibeRepo) IncrementBashHistoryFeedbackCount(ctx context.Context, id string) error {
	return nil
}

func (r *vibeRepo) EnqueueOutbox(ctx context.Context, entityType, entityID, opType string, payload any) error {
	return nil
}

type vibeLLM struct {
	lastSystem string
	lastUser   string
	payload    map[string]any
}

func (l *vibeLLM) GenerateJSON(ctx context.Context, system, user string, schema *genai.Schema, out any) error {
	l.lastSystem = system
	l.lastUser = user
	b, err := json.Marshal(l.payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func TestPlanVibeMutation_UsesBoundedExistingContext(t *testing.T) {
	repo := &vibeRepo{}
	llm := &vibeLLM{
		payload: map[string]any{
			"reasoning": "plan from llm",
			"operations": []any{
				map[string]any{
					"op":          "create_memory",
					"description": "store memory",
					"data": map[string]any{
						"content": "remember this",
					},
				},
			},
		},
	}
	svc := NewServiceWithSearch(repo, nil, llm, true)

	plan, err := svc.PlanVibeMutation("remember migration rules", "demo", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Operations) != 1 || plan.Operations[0].Op != "create_memory" {
		t.Fatalf("expected one create_memory operation, got %+v", plan.Operations)
	}
	if !strings.Contains(llm.lastSystem, "EXISTING ENTRIES") {
		t.Fatalf("expected system prompt to include existing entries section")
	}
	if !strings.Contains(llm.lastSystem, "\"id\": \"mem_1\"") {
		t.Fatalf("expected system prompt to include compacted search context")
	}
}

func TestPlanVibeMutation_FiltersInvalidLLMOps(t *testing.T) {
	repo := &vibeRepo{}
	llm := &vibeLLM{
		payload: map[string]any{
			"reasoning": "mixed plan",
			"operations": []any{
				map[string]any{
					"op":          "not_supported",
					"description": "invalid op",
					"data":        map[string]any{},
				},
				map[string]any{
					"op":   "create_skill",
					"data": map[string]any{"name": "x", "description": "y"},
				},
				map[string]any{
					"op":          "update_node",
					"target":      "contextfs://demo/a",
					"description": "valid op",
					"data":        map[string]any{"name": "A"},
				},
			},
		},
	}
	svc := NewServiceWithSearch(repo, nil, llm, true)

	plan, err := svc.PlanVibeMutation("update architecture node", "demo", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Operations) != 1 {
		t.Fatalf("expected exactly one valid operation, got %d", len(plan.Operations))
	}
	if plan.Operations[0].Op != "update_node" {
		t.Fatalf("expected valid op to be update_node, got %q", plan.Operations[0].Op)
	}
}

func TestExecuteVibeMutation_SupportsNodeAndDeleteOps(t *testing.T) {
	repo := &vibeRepo{}
	svc := NewService(repo)

	results, err := svc.ExecuteVibeMutation([]VibeMutationOp{
		{Op: "delete_memory", Target: "mem_1", Data: map[string]any{}},
		{Op: "delete_skill", Target: "skill_1", Data: map[string]any{}},
		{Op: "create_node", Data: map[string]any{"uri": "contextfs://demo/new", "name": "New", "abstract": "A"}},
		{Op: "update_node", Target: "contextfs://demo/new", Data: map[string]any{"name": "Renamed"}},
		{Op: "delete_node", Target: "contextfs://demo/new", Data: map[string]any{}},
	}, "demo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
	if repo.deletedMemoryID != "mem_1" {
		t.Fatalf("expected memory delete for mem_1, got %q", repo.deletedMemoryID)
	}
	if repo.deletedSkillID != "skill_1" {
		t.Fatalf("expected skill delete for skill_1, got %q", repo.deletedSkillID)
	}
	if repo.createdNodeURI != "contextfs://demo/new" {
		t.Fatalf("expected node create for contextfs://demo/new, got %q", repo.createdNodeURI)
	}
	if repo.updatedNodeURI != "contextfs://demo/new" {
		t.Fatalf("expected node update for contextfs://demo/new, got %q", repo.updatedNodeURI)
	}
	if repo.deletedNodeURI != "contextfs://demo/new" {
		t.Fatalf("expected node delete for contextfs://demo/new, got %q", repo.deletedNodeURI)
	}
}

func (l *vibeLLM) GenerateContent(ctx context.Context, model, prompt string) (string, error) {
	return "", nil
}
