package contextsrv

import (
	"bytes"
	"encoding/json"
	"mairu/internal/llm"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type stubService struct {
	lastMemoryCreate MemoryCreateInput
	memories         []Memory
	queue            []ModerationEvent
}

func (s *stubService) Health() map[string]any {
	return map[string]any{"ok": true, "service": "contextsrv"}
}

func (s *stubService) CreateMemory(input MemoryCreateInput) (Memory, error) {
	s.lastMemoryCreate = input
	now := time.Now().UTC()
	return Memory{
		ID:                "mem_1",
		Project:           input.Project,
		Content:           input.Content,
		Category:          input.Category,
		Owner:             input.Owner,
		Importance:        input.Importance,
		ModerationStatus:  input.ModerationStatus,
		ModerationReasons: input.ModerationReasons,
		ReviewRequired:    input.ReviewRequired,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

func (s *stubService) ListMemories(project string, limit int) ([]Memory, error) {
	return s.memories, nil
}

func (s *stubService) UpdateMemory(input MemoryUpdateInput) (Memory, error) {
	return Memory{ID: input.ID}, nil
}

func (s *stubService) DeleteMemory(id string) error { return nil }

func (s *stubService) CreateSkill(input SkillCreateInput) (Skill, error) {
	now := time.Now().UTC()
	return Skill{
		ID:          "skill_1",
		Project:     input.Project,
		Name:        input.Name,
		Description: input.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (s *stubService) ListSkills(project string, limit int) ([]Skill, error) {
	return nil, nil
}

func (s *stubService) UpdateSkill(input SkillUpdateInput) (Skill, error) {
	return Skill{ID: input.ID}, nil
}

func (s *stubService) DeleteSkill(id string) error { return nil }

func (s *stubService) CreateContextNode(input ContextCreateInput) (ContextNode, error) {
	now := time.Now().UTC()
	return ContextNode{
		URI:       input.URI,
		Project:   input.Project,
		Name:      input.Name,
		Abstract:  input.Abstract,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (s *stubService) ListContextNodes(project string, parentURI *string, limit int) ([]ContextNode, error) {
	return nil, nil
}

func (s *stubService) UpdateContextNode(input ContextUpdateInput) (ContextNode, error) {
	return ContextNode{URI: input.URI}, nil
}

func (s *stubService) DeleteContextNode(uri string) error { return nil }

func (s *stubService) Search(opts SearchOptions) (map[string]any, error) {
	return map[string]any{
		"query": opts.Query,
		"store": opts.Store,
		"items": []map[string]any{},
	}, nil
}

func (s *stubService) Dashboard(limit int, project string) (map[string]any, error) {
	return map[string]any{"skills": []any{}, "memories": []any{}, "contextNodes": []any{}}, nil
}

func (s *stubService) ClusterStats() map[string]any {
	return map[string]any{"ok": true}
}

func (s *stubService) VibeQuery(prompt, project string, topK int) (VibeQueryResult, error) {
	return VibeQueryResult{}, nil
}

func (s *stubService) PlanVibeMutation(prompt, project string, topK int) (VibeMutationPlan, error) {
	return VibeMutationPlan{}, nil
}

func (s *stubService) ExecuteVibeMutation(ops []VibeMutationOp, project string) ([]map[string]any, error) {
	return []map[string]any{}, nil
}

func (s *stubService) ListModerationQueue(limit int) ([]ModerationEvent, error) {
	return s.queue, nil
}

func (s *stubService) ReviewModeration(input ModerationReviewInput) error {
	return nil
}

func TestCreateMemoryAPIIncludesModerationFields(t *testing.T) {
	svc := &stubService{}
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

	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if _, ok := out["moderation_status"]; !ok {
		t.Fatalf("expected moderation_status in response")
	}
	if _, ok := out["review_required"]; !ok {
		t.Fatalf("expected review_required in response")
	}
}

func TestModerationQueueAPIContract(t *testing.T) {
	svc := &stubService{
		queue: []ModerationEvent{
			{
				ID:               1,
				EntityType:       "memory",
				EntityID:         "mem_1",
				Project:          "demo",
				Decision:         ModerationStatusFlaggedSoft,
				Reasons:          []string{"contains credential-like token"},
				ReviewStatus:     "pending",
				PolicyVersion:    "v1",
				CreatedAt:        time.Now().UTC(),
				ReviewRequired:   true,
				ReviewerDecision: "",
			},
		},
	}
	h := NewHandler(svc, "")
	req := httptest.NewRequest(http.MethodGet, "/api/moderation/queue?limit=20", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if _, ok := out["items"]; !ok {
		t.Fatalf("expected items key in response")
	}
}

func (s *stubService) Ingest(text, baseURI string) ([]llm.ProposedContextNode, error) {
	return nil, nil
}

func (s *stubService) Autocomplete(req AutocompleteRequest) (AutocompleteResponse, error) {
	return AutocompleteResponse{Completion: "mock completion"}, nil
}

func (s *stubService) ApplyMemoryFeedback(id string, reward int) (Memory, error) {
	return Memory{ID: id, Importance: reward}, nil
}

func (s *stubService) ApplyBashHistoryFeedback(id string, reward int) (BashHistory, error) {
	return BashHistory{ID: id, Importance: reward}, nil
}

func (s *stubService) GetMemory(id string) (Memory, error) {
	return Memory{ID: id}, nil
}
