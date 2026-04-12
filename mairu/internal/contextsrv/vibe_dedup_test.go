package contextsrv

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type mockRepo struct {
	updated MemoryUpdateInput
}

func (m *mockRepo) CreateMemory(ctx context.Context, input MemoryCreateInput) (Memory, error) {
	return Memory{ID: "mem_new", Content: input.Content, CreatedAt: time.Now().UTC()}, nil
}
func (m *mockRepo) ListMemories(ctx context.Context, project string, limit int) ([]Memory, error) {
	return []Memory{}, nil
}
func (m *mockRepo) UpdateMemory(ctx context.Context, input MemoryUpdateInput) (Memory, error) {
	m.updated = input
	return Memory{ID: input.ID, Content: input.Content}, nil
}
func (m *mockRepo) DeleteMemory(ctx context.Context, id string) error { return nil }

func (m *mockRepo) CreateSkill(ctx context.Context, input SkillCreateInput) (Skill, error) {
	return Skill{ID: "skill_1"}, nil
}
func (m *mockRepo) ListSkills(ctx context.Context, project string, limit int) ([]Skill, error) {
	return []Skill{}, nil
}
func (m *mockRepo) UpdateSkill(ctx context.Context, input SkillUpdateInput) (Skill, error) {
	return Skill{ID: input.ID}, nil
}
func (m *mockRepo) DeleteSkill(ctx context.Context, id string) error { return nil }

func (m *mockRepo) CreateContextNode(ctx context.Context, input ContextCreateInput) (ContextNode, error) {
	return ContextNode{URI: input.URI}, nil
}
func (m *mockRepo) ListContextNodes(ctx context.Context, project string, parentURI *string, limit int) ([]ContextNode, error) {
	return []ContextNode{}, nil
}
func (m *mockRepo) UpdateContextNode(ctx context.Context, input ContextUpdateInput) (ContextNode, error) {
	return ContextNode{URI: input.URI}, nil
}
func (m *mockRepo) DeleteContextNode(ctx context.Context, uri string) error { return nil }

func (m *mockRepo) SearchText(ctx context.Context, opts SearchOptions) (map[string]any, error) {
	return map[string]any{
		"memories": []map[string]any{
			{
				"id":      "mem_existing",
				"content": "remember migration uses Postgres and Meilisearch",
			},
		},
	}, nil
}
func (m *mockRepo) ListModerationQueue(ctx context.Context, limit int) ([]ModerationEvent, error) {
	return []ModerationEvent{}, nil
}
func (m *mockRepo) ReviewModeration(ctx context.Context, input ModerationReviewInput) error {
	return nil
}
func (m *mockRepo) GetMemory(ctx context.Context, id string) (Memory, error) {
	return Memory{}, nil
}
func (m *mockRepo) RecordRetrievals(ctx context.Context, ids []string) error    { return nil }
func (m *mockRepo) IncrementFeedbackCount(ctx context.Context, id string) error { return nil }
func (m *mockRepo) InsertBashHistory(ctx context.Context, project string, command string, exitCode int, durationMs int, output string) error {
	return nil
}
func (m *mockRepo) GetBashHistory(ctx context.Context, id string) (BashHistory, error) {
	return BashHistory{}, nil
}
func (m *mockRepo) UpdateBashHistory(ctx context.Context, h BashHistory) error { return nil }
func (m *mockRepo) IncrementBashHistoryFeedbackCount(ctx context.Context, id string) error {
	return nil
}

func (m *mockRepo) EnqueueOutbox(ctx context.Context, entityType, entityID, opType string, payload any) error {
	return nil
}

func TestPlanVibeMutation_DedupRoutesToUpdate(t *testing.T) {
	repo := &mockRepo{}
	svc := NewService(repo)

	plan, err := svc.PlanVibeMutation("remember migration uses Postgres and Meilisearch", "demo", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Operations) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(plan.Operations))
	}
	if plan.Operations[0].Op != "update_memory" {
		t.Fatalf("expected update_memory op, got %q", plan.Operations[0].Op)
	}
}

func TestExecuteVibeMutation_UpdateMemoryOp(t *testing.T) {
	repo := &mockRepo{}
	svc := NewService(repo)

	results, err := svc.ExecuteVibeMutation([]VibeMutationOp{
		{
			Op:     "update_memory",
			Target: "mem_existing",
			Data: map[string]any{
				"content":    "remember migration uses Postgres and Meilisearch v2",
				"importance": 7,
			},
		},
	}, "demo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if repo.updated.ID != "mem_existing" {
		t.Fatalf("expected updated id mem_existing, got %q", repo.updated.ID)
	}
	raw, _ := json.Marshal(results[0])
	if !json.Valid(raw) {
		t.Fatalf("expected valid json result")
	}
}
