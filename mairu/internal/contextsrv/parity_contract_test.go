package contextsrv

import (
	"bytes"
	"encoding/json"
	"mairu/internal/llm"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type parityStubService struct {
	lastSearch SearchOptions
}

func (s *parityStubService) Health() map[string]any {
	return map[string]any{"ok": true, "service": "contextsrv"}
}
func (s *parityStubService) ClusterStats() map[string]any {
	return map[string]any{"ok": true, "indexes": []string{IndexMemories, IndexSkills, IndexNodes}}
}
func (s *parityStubService) CreateMemory(input MemoryCreateInput) (Memory, error) {
	fixed := time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
	return Memory{
		ID:                "mem_contract_1",
		Project:           input.Project,
		Content:           input.Content,
		Category:          input.Category,
		Owner:             input.Owner,
		Importance:        input.Importance,
		ModerationStatus:  ModerationStatusFlaggedSoft,
		ModerationReasons: []string{"contains credential-like token"},
		ReviewRequired:    true,
		CreatedAt:         fixed,
		UpdatedAt:         fixed,
	}, nil
}
func (s *parityStubService) ListMemories(project string, limit int) ([]Memory, error) {
	return []Memory{}, nil
}
func (s *parityStubService) UpdateMemory(input MemoryUpdateInput) (Memory, error) {
	return Memory{ID: input.ID}, nil
}
func (s *parityStubService) DeleteMemory(id string) error { return nil }

func (s *parityStubService) CreateSkill(input SkillCreateInput) (Skill, error) {
	return Skill{ID: "skill_1"}, nil
}
func (s *parityStubService) ListSkills(project string, limit int) ([]Skill, error) {
	return []Skill{}, nil
}
func (s *parityStubService) UpdateSkill(input SkillUpdateInput) (Skill, error) {
	return Skill{ID: input.ID}, nil
}
func (s *parityStubService) DeleteSkill(id string) error { return nil }

func (s *parityStubService) CreateContextNode(input ContextCreateInput) (ContextNode, error) {
	return ContextNode{URI: input.URI}, nil
}
func (s *parityStubService) ListContextNodes(project string, parentURI *string, limit int) ([]ContextNode, error) {
	return []ContextNode{}, nil
}
func (s *parityStubService) UpdateContextNode(input ContextUpdateInput) (ContextNode, error) {
	return ContextNode{URI: input.URI}, nil
}
func (s *parityStubService) DeleteContextNode(uri string) error { return nil }

func (s *parityStubService) Search(opts SearchOptions) (map[string]any, error) {
	s.lastSearch = opts
	return map[string]any{
		StoreMemories:     []map[string]any{},
		StoreSkills:       []map[string]any{},
		StoreContextNodes: []map[string]any{},
	}, nil
}
func (s *parityStubService) Dashboard(limit int, project string) (map[string]any, error) {
	return map[string]any{StoreSkills: []any{}, StoreMemories: []any{}, StoreContextNodes: []any{}}, nil
}
func (s *parityStubService) VibeQuery(prompt, project string, topK int) (VibeQueryResult, error) {
	return VibeQueryResult{Reasoning: "ok", Results: []VibeSearchGroup{}}, nil
}
func (s *parityStubService) PlanVibeMutation(prompt, project string, topK int) (VibeMutationPlan, error) {
	return VibeMutationPlan{Reasoning: "ok", Operations: []VibeMutationOp{}}, nil
}
func (s *parityStubService) ExecuteVibeMutation(ops []VibeMutationOp, project string) ([]map[string]any, error) {
	return []map[string]any{}, nil
}
func (s *parityStubService) ListModerationQueue(limit int) ([]ModerationEvent, error) {
	return []ModerationEvent{}, nil
}
func (s *parityStubService) ReviewModeration(input ModerationReviewInput) error { return nil }

func TestParityContract_CreateMemoryResponseGolden(t *testing.T) {
	svc := &parityStubService{}
	h := NewHandler(svc, "")

	body := map[string]any{
		"project":    "demo",
		"content":    "password=plaintext",
		"category":   "observation",
		"owner":      "agent",
		"importance": 5,
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/memories", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	assertJSONGolden(t, "create_memory_response.json", rec.Body.Bytes())
}

func TestParityContract_SearchQueryMappingGolden(t *testing.T) {
	svc := &parityStubService{}
	h := NewHandler(svc, "")

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/search?q=auth+token&project=demo&type=all&topK=7&minScore=0.72&highlight=true&weightVector=0.6&weightKeyword=0.25&weightRecency=0.1&weightImportance=0.05&recencyScale=30d&recencyDecay=0.5",
		nil,
	)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	got := map[string]any{
		"query":            svc.lastSearch.Query,
		"project":          svc.lastSearch.Project,
		"store":            svc.lastSearch.Store,
		"topK":             svc.lastSearch.TopK,
		"minScore":         svc.lastSearch.MinScore,
		"highlight":        svc.lastSearch.Highlight,
		"weightVector":     svc.lastSearch.WeightVector,
		"weightKeyword":    svc.lastSearch.WeightKeyword,
		"weightRecency":    svc.lastSearch.WeightRecency,
		"weightImportance": svc.lastSearch.WeightImp,
		"recencyScale":     svc.lastSearch.RecencyScale,
		"recencyDecay":     svc.lastSearch.RecencyDecay,
	}
	raw, _ := json.Marshal(got)
	assertJSONGolden(t, "search_query_mapping.json", raw)
}

func assertJSONGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	goldenPath := filepath.Join("testdata", "parity", name)

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("failed to read golden file %s: %v", goldenPath, err)
	}

	var gotObj any
	if err := json.Unmarshal(got, &gotObj); err != nil {
		t.Fatalf("failed to decode got json: %v", err)
	}
	var wantObj any
	if err := json.Unmarshal(want, &wantObj); err != nil {
		t.Fatalf("failed to decode golden json: %v", err)
	}

	gotNorm, _ := json.MarshalIndent(gotObj, "", "  ")
	wantNorm, _ := json.MarshalIndent(wantObj, "", "  ")

	if !bytes.Equal(gotNorm, wantNorm) {
		t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, gotNorm, wantNorm)
	}
}

func (s *parityStubService) Ingest(text, baseURI string) ([]llm.ProposedContextNode, error) {
	return nil, nil
}
