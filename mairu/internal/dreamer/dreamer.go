package dreamer

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"mairu/internal/core"
	"mairu/internal/prompts"
)

func BuildPrompt(goal string, context []string) string {
	contextBlock := ""
	if len(context) > 0 {
		contextBlock = "- " + strings.Join(context, "\n- ")
	}
	return prompts.Render("dreamer_build_prompt", struct {
		Goal         string
		ContextBlock string
	}{
		Goal:         goal,
		ContextBlock: contextBlock,
	})
}

const (
	DedupSimilarityThreshold   = 0.85
	PatternSimilarityThreshold = 0.75
	ListPageSize               = 100
	MinClusterSize             = 3
)

type Memory struct {
	ID         string
	Project    string
	Content    string
	Category   string
	Owner      string
	Importance int
	CreatedAt  time.Time
	Score      float64
	Metadata   map[string]any
}

type BashHistory struct {
	ID        string
	Project   string
	Command   string
	Output    string
	ExitCode  int
	CreatedAt time.Time
	Score     float64
}

type Store interface {
	ListMemories(project string, limit, offset int) ([]Memory, error)
	SearchMemoriesByVector(embedding []float32, topK int, project string) ([]Memory, error)
	UpdateMemory(id string, patch map[string]any, embedding []float32) error
	DeleteMemory(id string) error
	AddMemory(memory Memory, embedding []float32) error

	ListBashHistory(project string, limit, offset int) ([]BashHistory, error)
	SearchBashHistoryByVector(embedding []float32, topK int, project string) ([]BashHistory, error)
}

type Embedder interface {
	GetEmbedding(text string) ([]float32, error)
}

type LLM interface {
	Generate(prompt string) (string, error)
}

type IDGenerator func() string

type Engine struct {
	Store      Store
	Embedder   Embedder
	LLM        LLM
	GenerateID IDGenerator
	Now        func() time.Time
}

func NewEngine(store Store, embedder Embedder, llm LLM) *Engine {
	return &Engine{
		Store:    store,
		Embedder: embedder,
		LLM:      llm,
		GenerateID: func() string {
			return fmt.Sprintf("mem_%d", time.Now().UnixNano())
		},
		Now: time.Now,
	}
}

func (e *Engine) fetchAllMemories(project string) ([]Memory, error) {
	var all []Memory
	offset := 0
	for {
		page, err := e.Store.ListMemories(project, ListPageSize, offset)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		all = append(all, page...)
		offset += len(page)
		if len(page) < ListPageSize {
			break
		}
	}
	return all, nil
}

type deductionDecision struct {
	CandidateIndex int
	Action         string
	MergedContent  string
}

func (e *Engine) DeductionPass(project string) error {
	memories, err := e.fetchAllMemories(project)
	if err != nil {
		return err
	}
	processed := map[string]bool{}

	for _, memory := range memories {
		if processed[memory.ID] {
			continue
		}
		processed[memory.ID] = true

		embedding, err := e.Embedder.GetEmbedding(memory.Content)
		if err != nil {
			continue
		}
		candidates, err := e.Store.SearchMemoriesByVector(embedding, 10, project)
		if err != nil {
			continue
		}

		var similar []Memory
		for _, c := range candidates {
			if c.ID == memory.ID || processed[c.ID] || c.Score < DedupSimilarityThreshold {
				continue
			}
			similar = append(similar, c)
		}
		if len(similar) == 0 {
			continue
		}

		candidateList := []string{}
		for i, c := range similar {
			candidateList = append(candidateList, fmt.Sprintf("[%d] ID: %s | Created: %s | Content: %s", i, c.ID, c.CreatedAt.Format(time.RFC3339), c.Content))
		}
		prompt := prompts.Render("dreamer_deduction", struct {
			Source     string
			Candidates string
		}{
			Source:     memory.Content,
			Candidates: strings.Join(candidateList, "\n"),
		})
		text, err := e.LLM.Generate(prompt)
		if err != nil {
			continue
		}
		decisions := parseDeductionDecisions(text)
		for _, decision := range decisions {
			if decision.CandidateIndex < 0 || decision.CandidateIndex >= len(similar) {
				continue
			}
			candidate := similar[decision.CandidateIndex]
			older, newer := memory, candidate
			if candidate.CreatedAt.Before(memory.CreatedAt) {
				older, newer = candidate, memory
			}

			switch decision.Action {
			case "MERGE":
				if strings.TrimSpace(decision.MergedContent) == "" {
					break
				}
				mergedEmbedding, err := e.Embedder.GetEmbedding(decision.MergedContent)
				if err == nil {
					_ = e.Store.UpdateMemory(newer.ID, map[string]any{"content": decision.MergedContent}, mergedEmbedding)
					_ = e.Store.DeleteMemory(older.ID)
				}
			case "CONTRADICTION":
				_ = e.Store.DeleteMemory(older.ID)
			default:
			}
			processed[candidate.ID] = true
		}
	}

	return nil
}

func (e *Engine) InductionPass(project string) error {
	all, err := e.fetchAllMemories(project)
	if err != nil {
		return err
	}
	memories := []Memory{}
	for _, m := range all {
		if m.Category != "derived_pattern" {
			memories = append(memories, m)
		}
	}

	assigned := map[string]bool{}
	var clusters [][]Memory
	for _, memory := range memories {
		if assigned[memory.ID] {
			continue
		}
		assigned[memory.ID] = true

		embedding, err := e.Embedder.GetEmbedding(memory.Content)
		if err != nil {
			continue
		}
		similar, err := e.Store.SearchMemoriesByVector(embedding, 20, project)
		if err != nil {
			continue
		}

		cluster := []Memory{memory}
		for _, c := range similar {
			if c.ID == memory.ID || assigned[c.ID] || c.Category == "derived_pattern" {
				continue
			}
			if c.Score >= PatternSimilarityThreshold {
				cluster = append(cluster, c)
				assigned[c.ID] = true
			}
		}
		if len(cluster) >= MinClusterSize {
			clusters = append(clusters, cluster)
		}
	}

	for _, cluster := range clusters {
		memberLines := make([]string, 0, len(cluster))
		for i, m := range cluster {
			memberLines = append(memberLines, fmt.Sprintf("[%d] (%s, importance: %d): %s", i, m.Category, m.Importance, m.Content))
		}
		prompt := prompts.Render("dreamer_induction", struct {
			Memories string
		}{
			Memories: strings.Join(memberLines, "\n"),
		})
		text, err := e.LLM.Generate(prompt)
		if err != nil {
			continue
		}
		pattern, confidence, evidence := parsePatternObject(text)
		if strings.TrimSpace(pattern) == "" {
			continue
		}

		patternEmbedding, err := e.Embedder.GetEmbedding(pattern)
		if err != nil {
			continue
		}
		existing, err := e.Store.SearchMemoriesByVector(patternEmbedding, 3, project)
		if err != nil {
			continue
		}
		isDup := false
		for _, p := range existing {
			if p.Category == "derived_pattern" && p.Score >= DedupSimilarityThreshold {
				isDup = true
				break
			}
		}
		if isDup {
			continue
		}

		importance := len(cluster) + 3
		if importance > 10 {
			importance = 10
		}
		now := e.Now()
		mem := Memory{
			ID:         e.GenerateID(),
			Project:    project,
			Content:    pattern,
			Category:   "derived_pattern",
			Owner:      "system",
			Importance: importance,
			CreatedAt:  now,
			Metadata: map[string]any{
				"dream_source": "induction",
				"cluster_size": len(cluster),
				"confidence":   confidence,
				"evidence":     evidence,
			},
		}
		_ = e.Store.AddMemory(mem, patternEmbedding)
	}
	return nil
}

func (e *Engine) fetchAllBashHistory(project string) ([]BashHistory, error) {
	var all []BashHistory
	offset := 0
	for {
		page, err := e.Store.ListBashHistory(project, ListPageSize, offset)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		all = append(all, page...)
		offset += len(page)
		if len(page) < ListPageSize {
			break
		}
	}
	return all, nil
}

func truncate(s string, l int) string {
	if len(s) > l {
		return s[:l] + "..."
	}
	return s
}

func (e *Engine) BashInductionPass(project string) error {
	history, err := e.fetchAllBashHistory(project)
	if err != nil {
		return err
	}

	assigned := map[string]bool{}
	var clusters [][]BashHistory
	for _, h := range history {
		if assigned[h.ID] {
			continue
		}
		assigned[h.ID] = true

		embedding, err := e.Embedder.GetEmbedding(h.Command)
		if err != nil {
			continue
		}
		similar, err := e.Store.SearchBashHistoryByVector(embedding, 20, project)
		if err != nil {
			continue
		}

		cluster := []BashHistory{h}
		for _, c := range similar {
			if c.ID == h.ID || assigned[c.ID] {
				continue
			}
			if c.Score >= PatternSimilarityThreshold {
				cluster = append(cluster, c)
				assigned[c.ID] = true
			}
		}
		if len(cluster) >= MinClusterSize {
			clusters = append(clusters, cluster)
		}
	}

	for _, cluster := range clusters {
		memberLines := make([]string, 0, len(cluster))
		for i, c := range cluster {
			memberLines = append(memberLines, fmt.Sprintf("[%d] Command: %s (Exit: %d) OutputSnippet: %s", i, c.Command, c.ExitCode, truncate(c.Output, 100)))
		}
		prompt := prompts.Render("dreamer_bash_induction", struct {
			Commands string
		}{
			Commands: strings.Join(memberLines, "\n"),
		})
		text, err := e.LLM.Generate(prompt)
		if err != nil {
			continue
		}
		pattern, confidence, evidence := parsePatternObject(text)
		if strings.TrimSpace(pattern) == "" {
			continue
		}

		patternEmbedding, err := e.Embedder.GetEmbedding(pattern)
		if err != nil {
			continue
		}
		existing, err := e.Store.SearchMemoriesByVector(patternEmbedding, 3, project)
		if err != nil {
			continue
		}
		isDup := false
		for _, p := range existing {
			if p.Category == "derived_pattern" && p.Score >= DedupSimilarityThreshold {
				isDup = true
				break
			}
		}
		if isDup {
			continue
		}

		importance := len(cluster) + 3
		if importance > 10 {
			importance = 10
		}
		now := e.Now()
		mem := Memory{
			ID:         e.GenerateID(),
			Project:    project,
			Content:    pattern,
			Category:   "derived_pattern",
			Owner:      "system",
			Importance: importance,
			CreatedAt:  now,
			Metadata: map[string]any{
				"dream_source": "bash_induction",
				"cluster_size": len(cluster),
				"confidence":   confidence,
				"evidence":     evidence,
			},
		}
		_ = e.Store.AddMemory(mem, patternEmbedding)
	}
	return nil
}

func parseDeductionDecisions(text string) []deductionDecision {
	obj := core.ExtractJSONObject(text)
	if obj == nil {
		return nil
	}
	anyDecisions, ok := obj["decisions"].([]any)
	if !ok {
		return nil
	}
	out := make([]deductionDecision, 0, len(anyDecisions))
	for _, d := range anyDecisions {
		m, ok := d.(map[string]any)
		if !ok {
			continue
		}
		cand, _ := m["candidateIndex"].(float64)
		action, _ := m["action"].(string)
		merged, _ := m["mergedContent"].(string)
		out = append(out, deductionDecision{
			CandidateIndex: int(cand),
			Action:         action,
			MergedContent:  merged,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CandidateIndex < out[j].CandidateIndex })
	return out
}

func parsePatternObject(text string) (pattern, confidence, evidence string) {
	obj := core.ExtractJSONObject(text)
	if obj == nil {
		return "", "", ""
	}
	pattern, _ = obj["pattern"].(string)
	confidence, _ = obj["confidence"].(string)
	evidence, _ = obj["evidence"].(string)
	return pattern, confidence, evidence
}
