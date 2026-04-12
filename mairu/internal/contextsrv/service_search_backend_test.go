package contextsrv

import (
	"context"
	"testing"
)

type repoSearchStub struct {
	searchCalls int
	searchOut   map[string]any
	searchErr   error
}

func (r *repoSearchStub) CreateMemory(ctx context.Context, input MemoryCreateInput) (Memory, error) {
	return Memory{}, nil
}
func (r *repoSearchStub) ListMemories(ctx context.Context, project string, limit int) ([]Memory, error) {
	return []Memory{}, nil
}
func (r *repoSearchStub) UpdateMemory(ctx context.Context, input MemoryUpdateInput) (Memory, error) {
	return Memory{}, nil
}
func (r *repoSearchStub) DeleteMemory(ctx context.Context, id string) error { return nil }
func (r *repoSearchStub) CreateSkill(ctx context.Context, input SkillCreateInput) (Skill, error) {
	return Skill{}, nil
}
func (r *repoSearchStub) ListSkills(ctx context.Context, project string, limit int) ([]Skill, error) {
	return []Skill{}, nil
}
func (r *repoSearchStub) UpdateSkill(ctx context.Context, input SkillUpdateInput) (Skill, error) {
	return Skill{}, nil
}
func (r *repoSearchStub) DeleteSkill(ctx context.Context, id string) error { return nil }
func (r *repoSearchStub) CreateContextNode(ctx context.Context, input ContextCreateInput) (ContextNode, error) {
	return ContextNode{}, nil
}
func (r *repoSearchStub) ListContextNodes(ctx context.Context, project string, parentURI *string, limit int) ([]ContextNode, error) {
	return []ContextNode{}, nil
}
func (r *repoSearchStub) UpdateContextNode(ctx context.Context, input ContextUpdateInput) (ContextNode, error) {
	return ContextNode{}, nil
}
func (r *repoSearchStub) DeleteContextNode(ctx context.Context, uri string) error { return nil }
func (r *repoSearchStub) SearchText(ctx context.Context, opts SearchOptions) (map[string]any, error) {
	r.searchCalls++
	return r.searchOut, r.searchErr
}
func (r *repoSearchStub) ListModerationQueue(ctx context.Context, limit int) ([]ModerationEvent, error) {
	return []ModerationEvent{}, nil
}
func (r *repoSearchStub) ReviewModeration(ctx context.Context, input ModerationReviewInput) error {
	return nil
}
func (r *repoSearchStub) EnqueueOutbox(ctx context.Context, entityType, entityID, opType string, payload any) error {
	return nil
}

func (r *repoSearchStub) GetMemory(ctx context.Context, id string) (Memory, error) {
	return Memory{}, nil
}
func (r *repoSearchStub) RecordRetrievals(ctx context.Context, ids []string) error {
	return nil
}
func (r *repoSearchStub) IncrementFeedbackCount(ctx context.Context, id string) error {
	return nil
}
func (r *repoSearchStub) InsertBashHistory(ctx context.Context, project string, command string, exitCode int, durationMs int, output string) error {
	return nil
}
func (r *repoSearchStub) GetBashHistory(ctx context.Context, id string) (BashHistory, error) {
	return BashHistory{}, nil
}
func (r *repoSearchStub) UpdateBashHistory(ctx context.Context, h BashHistory) error {
	return nil
}
func (r *repoSearchStub) IncrementBashHistoryFeedbackCount(ctx context.Context, id string) error {
	return nil
}

type backendStub struct {
	searchCalls int
	searchOut   map[string]any
	searchErr   error
	cluster     map[string]any
}

func (b *backendStub) Search(opts SearchOptions) (map[string]any, error) {
	b.searchCalls++
	return b.searchOut, b.searchErr
}
func (b *backendStub) ClusterStats() map[string]any { return b.cluster }

func TestSearchPrefersBackendWhenAvailable(t *testing.T) {
	repo := &repoSearchStub{searchOut: map[string]any{"memories": []map[string]any{{"id": "repo"}}}}
	backend := &backendStub{searchOut: map[string]any{"memories": []map[string]any{{"id": "meili"}}}}
	svc := NewServiceWithSearch(repo, backend, nil, nil, true)

	out, err := svc.Search(SearchOptions{Query: "auth"})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	memories, _ := out["memories"].([]map[string]any)
	if len(memories) == 0 || memories[0]["id"] != "meili" {
		t.Fatalf("expected backend result, got %#v", out)
	}
	if repo.searchCalls != 0 {
		t.Fatalf("expected repo fallback to be skipped, got %d calls", repo.searchCalls)
	}
}

func TestSearchFallsBackToRepoOnBackendError(t *testing.T) {
	repo := &repoSearchStub{searchOut: map[string]any{"memories": []map[string]any{{"id": "repo"}}}}
	backend := &backendStub{searchErr: context.DeadlineExceeded}
	svc := NewServiceWithSearch(repo, backend, nil, nil, true)

	out, err := svc.Search(SearchOptions{Query: "auth"})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	memories, _ := out["memories"].([]map[string]any)
	if len(memories) == 0 || memories[0]["id"] != "repo" {
		t.Fatalf("expected repo fallback result, got %#v", out)
	}
	if repo.searchCalls != 1 {
		t.Fatalf("expected one fallback repo call, got %d", repo.searchCalls)
	}
}

func TestClusterStatsUsesBackendWhenAvailable(t *testing.T) {
	repo := &repoSearchStub{}
	backend := &backendStub{cluster: map[string]any{"ok": true, "service": "meili"}}
	svc := NewServiceWithSearch(repo, backend, nil, nil, true)
	out := svc.ClusterStats()
	if out["service"] != "meili" {
		t.Fatalf("expected backend cluster stats, got %#v", out)
	}
}
