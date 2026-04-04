package llm

import (
	"context"
	"fmt"
	"strings"

	"mairu/internal/prompts"
)

type LLMClient interface {
	GenerateJSON(ctx context.Context, system, user string) (map[string]any, error)
}

type RouterAction struct {
	Action        string
	TargetID      string
	MergedContent string
	Reason        string
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

	decision, err := client.GenerateJSON(ctx, "system", prompt)
	if err != nil {
		return RouterAction{Action: "create"}, nil
	}

	act, _ := decision["action"].(string)
	if act == "update" {
		targetID, _ := decision["targetId"].(string)
		merged, _ := decision["mergedContent"].(string)
		if targetID != "" && merged != "" {
			return RouterAction{Action: "update", TargetID: targetID, MergedContent: merged}, nil
		}
	} else if act == "skip" {
		reason, _ := decision["reason"].(string)
		return RouterAction{Action: "skip", Reason: reason}, nil
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

	decision, err := client.GenerateJSON(ctx, "system", prompt)
	if err != nil {
		return RouterAction{Action: "create"}, nil
	}

	act, _ := decision["action"].(string)
	if act == "update" {
		targetID, _ := decision["targetId"].(string)
		merged, _ := decision["mergedContent"].(string)
		if targetID != "" && merged != "" {
			return RouterAction{Action: "update", TargetID: targetID, MergedContent: merged}, nil
		}
	} else if act == "skip" {
		reason, _ := decision["reason"].(string)
		return RouterAction{Action: "skip", Reason: reason}, nil
	}
	return RouterAction{Action: "create"}, nil
}
