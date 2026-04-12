package dreamer

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBuildPrompt(t *testing.T) {
	p := BuildPrompt("ship migration", []string{"module A", "module B"})
	if !strings.Contains(p, "ship migration") || !strings.Contains(p, "module A") {
		t.Fatalf("unexpected prompt: %s", p)
	}
}

type stubStore struct {
	listPages     [][]Memory
	searchResults [][]Memory
	updated       []struct {
		id    string
		patch map[string]any
	}
	deleted []string
	added   []Memory

	listBashPages     [][]BashHistory
	searchBashResults [][]BashHistory
}

func (s *stubStore) ListMemories(project string, limit, offset int) ([]Memory, error) {
	if len(s.listPages) == 0 {
		return nil, nil
	}
	page := s.listPages[0]
	s.listPages = s.listPages[1:]
	return page, nil
}

func (s *stubStore) SearchMemoriesByVector(embedding []float32, topK int, project string) ([]Memory, error) {
	if len(s.searchResults) == 0 {
		return nil, nil
	}
	res := s.searchResults[0]
	s.searchResults = s.searchResults[1:]
	return res, nil
}

func (s *stubStore) UpdateMemory(id string, patch map[string]any, embedding []float32) error {
	s.updated = append(s.updated, struct {
		id    string
		patch map[string]any
	}{id: id, patch: patch})
	return nil
}

func (s *stubStore) DeleteMemory(id string) error {
	s.deleted = append(s.deleted, id)
	return nil
}

func (s *stubStore) AddMemory(memory Memory, embedding []float32) error {
	s.added = append(s.added, memory)
	return nil
}

func (s *stubStore) ListBashHistory(project string, limit, offset int) ([]BashHistory, error) {
	if len(s.listBashPages) == 0 {
		return nil, nil
	}
	page := s.listBashPages[0]
	s.listBashPages = s.listBashPages[1:]
	return page, nil
}

func (s *stubStore) SearchBashHistoryByVector(embedding []float32, topK int, project string) ([]BashHistory, error) {
	if len(s.searchBashResults) == 0 {
		return nil, nil
	}
	res := s.searchBashResults[0]
	s.searchBashResults = s.searchBashResults[1:]
	return res, nil
}

type stubEmbedder struct{}

func (s stubEmbedder) GetEmbedding(text string) ([]float32, error) {
	return make([]float32, 768), nil
}

type stubLLM struct {
	outputs []string
}

func (s *stubLLM) Generate(prompt string) (string, error) {
	if len(s.outputs) == 0 {
		return "{}", nil
	}
	out := s.outputs[0]
	s.outputs = s.outputs[1:]
	return out, nil
}

func TestDeductionPassMerge(t *testing.T) {
	mem1 := Memory{
		ID: "mem_1", Content: "We use Vitest for testing", Project: "proj",
		CreatedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
	}
	mem2 := Memory{
		ID: "mem_2", Content: "Testing framework is Vitest", Project: "proj", Score: 0.92,
		CreatedAt: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
	}
	store := &stubStore{
		listPages:     [][]Memory{{mem1, mem2}},
		searchResults: [][]Memory{{mem2}},
	}
	llm := &stubLLM{outputs: []string{
		`{"decisions":[{"candidateIndex":0,"action":"MERGE","mergedContent":"We use Vitest as our testing framework"}]}`,
	}}
	e := NewEngine(store, stubEmbedder{}, llm)
	if err := e.DeductionPass("proj"); err != nil {
		t.Fatalf("deduction failed: %v", err)
	}
	if len(store.updated) != 1 || store.updated[0].id != "mem_2" {
		t.Fatalf("expected update on newer memory, got %#v", store.updated)
	}
	if got := store.updated[0].patch["content"]; got != "We use Vitest as our testing framework" {
		t.Fatalf("unexpected merged content: %#v", got)
	}
	if !reflect.DeepEqual(store.deleted, []string{"mem_1"}) {
		t.Fatalf("expected older memory deleted, got %#v", store.deleted)
	}
}

func TestDeductionPassContradictionKeepsNewer(t *testing.T) {
	mem1 := Memory{
		ID: "mem_1", Content: "We use Jest for testing", Project: "proj",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	mem2 := Memory{
		ID: "mem_2", Content: "We migrated from Jest to Vitest", Project: "proj", Score: 0.88,
		CreatedAt: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
	}
	store := &stubStore{
		listPages:     [][]Memory{{mem1, mem2}},
		searchResults: [][]Memory{{mem2}},
	}
	llm := &stubLLM{outputs: []string{
		`{"decisions":[{"candidateIndex":0,"action":"CONTRADICTION"}]}`,
	}}
	e := NewEngine(store, stubEmbedder{}, llm)
	if err := e.DeductionPass("proj"); err != nil {
		t.Fatalf("deduction failed: %v", err)
	}
	if len(store.updated) != 0 {
		t.Fatalf("did not expect updates, got %#v", store.updated)
	}
	if !reflect.DeepEqual(store.deleted, []string{"mem_1"}) {
		t.Fatalf("expected older memory deleted, got %#v", store.deleted)
	}
}

func TestDeductionPassSkipsNoDuplicates(t *testing.T) {
	mem1 := Memory{
		ID: "mem_1", Content: "Auth uses JWT tokens", Project: "proj",
		CreatedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
	}
	store := &stubStore{
		listPages:     [][]Memory{{mem1}},
		searchResults: [][]Memory{{}},
	}
	llm := &stubLLM{}
	e := NewEngine(store, stubEmbedder{}, llm)
	if err := e.DeductionPass("proj"); err != nil {
		t.Fatalf("deduction failed: %v", err)
	}
	if len(store.updated) != 0 || len(store.deleted) != 0 {
		t.Fatalf("expected no writes, got updates=%d deletes=%d", len(store.updated), len(store.deleted))
	}
}

func TestInductionPassCreatesDerivedPattern(t *testing.T) {
	memories := []Memory{
		{ID: "mem_1", Content: "Use early returns in auth handlers", Category: "decision", Importance: 5, Project: "proj", CreatedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "mem_2", Content: "Prefer early returns in validation logic", Category: "decision", Importance: 6, Project: "proj", CreatedAt: time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)},
		{ID: "mem_3", Content: "Always use early returns for error handling", Category: "decision", Importance: 5, Project: "proj", CreatedAt: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)},
	}
	store := &stubStore{
		listPages: [][]Memory{memories},
		searchResults: [][]Memory{
			{
				{ID: "mem_2", Content: memories[1].Content, Category: "decision", Score: 0.82, CreatedAt: memories[1].CreatedAt},
				{ID: "mem_3", Content: memories[2].Content, Category: "decision", Score: 0.80, CreatedAt: memories[2].CreatedAt},
			},
			{},
		},
	}
	llm := &stubLLM{outputs: []string{
		`{"pattern":"Team consistently prefers early returns for error/validation paths across all handlers","confidence":"high","evidence":"Observed in auth handlers, validation logic, and error handling"}`,
	}}
	e := NewEngine(store, stubEmbedder{}, llm)
	e.GenerateID = func() string { return "mem_generated" }
	e.Now = func() time.Time { return time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC) }
	if err := e.InductionPass("proj"); err != nil {
		t.Fatalf("induction failed: %v", err)
	}
	if len(store.added) != 1 {
		t.Fatalf("expected one derived pattern, got %d", len(store.added))
	}
	added := store.added[0]
	if added.Category != "derived_pattern" || added.Owner != "system" || added.Project != "proj" {
		t.Fatalf("unexpected added memory: %#v", added)
	}
}

func TestInductionPassSkipsSmallClusters(t *testing.T) {
	memories := []Memory{
		{ID: "mem_1", Content: "Use TypeScript strict mode", Category: "decision", Project: "proj", CreatedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "mem_2", Content: "Enable strict TypeScript", Category: "decision", Project: "proj", CreatedAt: time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)},
	}
	store := &stubStore{
		listPages: [][]Memory{memories},
		searchResults: [][]Memory{
			{
				{ID: "mem_2", Content: memories[1].Content, Category: "decision", Score: 0.82, CreatedAt: memories[1].CreatedAt},
			},
		},
	}
	e := NewEngine(store, stubEmbedder{}, &stubLLM{})
	if err := e.InductionPass("proj"); err != nil {
		t.Fatalf("induction failed: %v", err)
	}
	if len(store.added) != 0 {
		t.Fatalf("expected no patterns, got %#v", store.added)
	}
}

func TestInductionPassSkipsNullPattern(t *testing.T) {
	memories := []Memory{
		{ID: "m1", Content: "A", Category: "observation", Project: "proj", CreatedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "m2", Content: "B", Category: "observation", Project: "proj", CreatedAt: time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)},
		{ID: "m3", Content: "C", Category: "observation", Project: "proj", CreatedAt: time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)},
	}
	store := &stubStore{
		listPages: [][]Memory{memories},
		searchResults: [][]Memory{
			{
				{ID: "m2", Content: "B", Category: "observation", Score: 0.80, CreatedAt: memories[1].CreatedAt},
				{ID: "m3", Content: "C", Category: "observation", Score: 0.78, CreatedAt: memories[2].CreatedAt},
			},
		},
	}
	llm := &stubLLM{outputs: []string{`{"pattern":null,"confidence":null,"evidence":null}`}}
	e := NewEngine(store, stubEmbedder{}, llm)
	if err := e.InductionPass("proj"); err != nil {
		t.Fatalf("induction failed: %v", err)
	}
	if len(store.added) != 0 {
		t.Fatalf("expected no pattern write, got %#v", store.added)
	}
}

func TestEnginePaginationFetchesAllMemories(t *testing.T) {
	first := make([]Memory, 0, ListPageSize)
	for i := 0; i < ListPageSize; i++ {
		first = append(first, Memory{ID: fmt.Sprintf("m_%d", i), Content: "x", Project: "proj", CreatedAt: time.Now()})
	}
	second := []Memory{{ID: "m_last", Content: "y", Project: "proj", CreatedAt: time.Now()}}
	store := &stubStore{
		listPages:     [][]Memory{first, second},
		searchResults: [][]Memory{make([]Memory, 0, len(first)+len(second))},
	}
	e := NewEngine(store, stubEmbedder{}, &stubLLM{})
	if err := e.DeductionPass("proj"); err != nil {
		t.Fatalf("deduction failed: %v", err)
	}
}

func TestBashInductionPass(t *testing.T) {
	history := []BashHistory{
		{ID: "bh_1", Command: "npm run dev", Project: "proj", CreatedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "bh_2", Command: "npm run start:dev", Project: "proj", CreatedAt: time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)},
		{ID: "bh_3", Command: "npm run dev", Project: "proj", CreatedAt: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)},
	}
	store := &stubStore{
		listBashPages: [][]BashHistory{history},
		searchBashResults: [][]BashHistory{
			{
				{ID: "bh_2", Command: history[1].Command, Score: 0.82, CreatedAt: history[1].CreatedAt},
				{ID: "bh_3", Command: history[2].Command, Score: 0.80, CreatedAt: history[2].CreatedAt},
			},
			{},
		},
		searchResults: [][]Memory{
			{}, // No existing derived patterns
		},
	}
	llm := &stubLLM{outputs: []string{
		`{"pattern":"User frequently starts the local development server using npm run dev or start:dev","confidence":"high","evidence":"Observed multiple executions"}`,
	}}
	e := NewEngine(store, stubEmbedder{}, llm)
	e.GenerateID = func() string { return "mem_bash_generated" }
	e.Now = func() time.Time { return time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC) }

	if err := e.BashInductionPass("proj"); err != nil {
		t.Fatalf("bash induction failed: %v", err)
	}
	if len(store.added) != 1 {
		t.Fatalf("expected one derived pattern, got %d", len(store.added))
	}
	added := store.added[0]
	if added.Category != "derived_pattern" || added.Owner != "system" || added.Project != "proj" {
		t.Fatalf("unexpected added memory: %#v", added)
	}
	if added.ID != "mem_bash_generated" {
		t.Fatalf("unexpected generated ID: %s", added.ID)
	}
}
