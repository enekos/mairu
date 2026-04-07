package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"mairu/internal/prompts"
)

type LLMClient interface {
	GenerateJSON(ctx context.Context, system, user string, schema *genai.Schema, out any) error
}

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

const similarityGate = 0.75

func DecideMemoryAction(ctx context.Context, client LLMClient, newContent string, candidates []RouterCandidate) (RouterAction, error) {
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

	prompt := prompts.Render("router_memory_action", struct {
		NewContent    string
		CandidateList string
	}{
		NewContent:    newContent,
		CandidateList: candidateList,
	})

	var decision RouterAction
	schema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"action": {
				Type:        genai.TypeString,
				Description: "One of: create, update, skip",
			},
			"targetId": {
				Type:        genai.TypeString,
				Description: "ID of the target memory if updating",
			},
			"mergedContent": {
				Type:        genai.TypeString,
				Description: "The new merged content if updating",
			},
			"reason": {
				Type:        genai.TypeString,
				Description: "Reasoning for the action",
			},
		},
		Required: []string{"action", "reason"},
	}
	err := client.GenerateJSON(ctx, "system", prompt, schema, &decision)
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

func DecideContextAction(ctx context.Context, client LLMClient, uri, name, abstract string, candidates []RouterCandidate) (RouterAction, error) {
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

	prompt := prompts.Render("router_context_action", struct {
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

	var decision RouterAction
	schema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"action": {
				Type:        genai.TypeString,
				Description: "One of: create, update, skip",
			},
			"targetId": {
				Type:        genai.TypeString,
				Description: "ID of the target memory if updating",
			},
			"mergedContent": {
				Type:        genai.TypeString,
				Description: "The new merged content if updating",
			},
			"reason": {
				Type:        genai.TypeString,
				Description: "Reasoning for the action",
			},
		},
		Required: []string{"action", "reason"},
	}
	err := client.GenerateJSON(ctx, "system", prompt, schema, &decision)
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
