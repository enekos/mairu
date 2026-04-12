package llm

import (
	"context"
	"fmt"
	"strings"

	"mairu/internal/prompts"
)

type RouterAction struct {
	Action        string `json:"action"`
	TargetID      string `json:"targetId"`
	MergedContent string `json:"mergedContent"`
	Reason        string `json:"reason"`
}

type RouterCandidate struct {
	ID      string
	Content string
	Score   float64
}

// RouterLLMClient is the minimal interface needed for router decisions
type RouterLLMClient interface {
	GenerateJSON(ctx context.Context, system, user string, schema *JSONSchema, out any) error
}

const similarityGate = 0.75

func DecideMemoryAction(ctx context.Context, client RouterLLMClient, newContent string, candidates []RouterCandidate) (RouterAction, error) {
	if client == nil {
		return RouterAction{Action: "create"}, nil
	}

	var topCandidates []RouterCandidate
	for _, c := range candidates {
		if c.Score >= similarityGate {
			topCandidates = append(topCandidates, c)
		}
		if len(topCandidates) >= 4 {
			break
		}
	}

	if len(topCandidates) == 0 {
		return RouterAction{Action: "create"}, nil
	}

	var list []string
	for _, c := range topCandidates {
		list = append(list, fmt.Sprintf("ID: %s\nSIMILARITY: %.3f\nCONTENT: %s", c.ID, c.Score, c.Content))
	}
	candidateList := strings.Join(list, "\n---\n")

	prompt, err := prompts.Render("router_memory_action", struct {
		NewContent    string
		CandidateList string
	}{
		NewContent:    newContent,
		CandidateList: candidateList,
	})
	if err != nil {
		return RouterAction{Action: "create"}, fmt.Errorf("failed to render prompt: %w", err)
	}

	var decision RouterAction
	schema := &JSONSchema{
		Type: TypeObject,
		Properties: map[string]*JSONSchema{
			"action": {
				Type:        TypeString,
				Description: "One of: create, update, skip",
			},
			"targetId": {
				Type:        TypeString,
				Description: "ID of the target memory if updating",
			},
			"mergedContent": {
				Type:        TypeString,
				Description: "The new merged content if updating",
			},
			"reason": {
				Type:        TypeString,
				Description: "Reasoning for the action",
			},
		},
		Required: []string{"action", "reason"},
	}
	err = client.GenerateJSON(ctx, "system", prompt, schema, &decision)
	if err != nil {
		return RouterAction{Action: "create"}, err
	}

	if decision.Action == "update" {
		if decision.TargetID != "" && decision.MergedContent != "" {
			return decision, nil
		}
	} else if decision.Action == "skip" {
		return decision, nil
	}
	return RouterAction{Action: "create"}, nil
}

func DecideContextAction(ctx context.Context, client RouterLLMClient, uri, name, abstract string, candidates []RouterCandidate) (RouterAction, error) {
	if client == nil {
		return RouterAction{Action: "create"}, nil
	}

	var topCandidates []RouterCandidate
	for _, c := range candidates {
		if c.Score >= similarityGate {
			topCandidates = append(topCandidates, c)
		}
		if len(topCandidates) >= 4 {
			break
		}
	}

	if len(topCandidates) == 0 {
		return RouterAction{Action: "create"}, nil
	}

	var list []string
	for _, c := range topCandidates {
		list = append(list, fmt.Sprintf("ID: %s\nSIMILARITY: %.3f\nABSTRACT: %s", c.ID, c.Score, c.Content))
	}
	candidateList := strings.Join(list, "\n---\n")

	prompt, err := prompts.Render("router_context_action", struct {
		URI           string
		Name          string
		Abstract      string
		CandidateList string
	}{
		URI:           uri,
		Name:          name,
		Abstract:      abstract,
		CandidateList: candidateList,
	})
	if err != nil {
		return RouterAction{Action: "create"}, fmt.Errorf("failed to render prompt: %w", err)
	}

	var decision RouterAction
	schema := &JSONSchema{
		Type: TypeObject,
		Properties: map[string]*JSONSchema{
			"action": {
				Type:        TypeString,
				Description: "One of: create, update, skip",
			},
			"targetId": {
				Type:        TypeString,
				Description: "ID of the target memory if updating",
			},
			"mergedContent": {
				Type:        TypeString,
				Description: "The new merged content if updating",
			},
			"reason": {
				Type:        TypeString,
				Description: "Reasoning for the action",
			},
		},
		Required: []string{"action", "reason"},
	}
	err = client.GenerateJSON(ctx, "system", prompt, schema, &decision)
	if err != nil {
		return RouterAction{Action: "create"}, err
	}

	if decision.Action == "update" {
		if decision.TargetID != "" && decision.MergedContent != "" {
			return decision, nil
		}
	} else if decision.Action == "skip" {
		return decision, nil
	}
	return RouterAction{Action: "create"}, nil
}
