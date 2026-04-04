package llm

import (
	"context"
	"fmt"
	"strings"
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

	prompt := fmt.Sprintf(`You are managing an AI agent's memory database. Decide what to do with new incoming information.

NEW INFORMATION:
%s

EXISTING SIMILAR MEMORIES:
%s

Rules:
- "create": the new information is genuinely new, adds detail not captured by any existing memory
- "update": an existing memory should be enriched/corrected. Provide merged content that combines both into one complete sentence/fact. Use the ID of the single best matching memory as targetId.
- "skip": the new information is already fully captured by an existing memory

Respond with ONLY a JSON object:
- {"action":"create"}
- {"action":"update","targetId":"<exact id>","mergedContent":"<merged text>"}
- {"action":"skip","reason":"<brief reason>"}`, newContent, candidateList)

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

	prompt := fmt.Sprintf(`You are managing a hierarchical context database for a software project. Decide what to do with a new context node.

NEW NODE:
URI: %s
NAME: %s
ABSTRACT: %s

EXISTING SIMILAR NODES:
%s

Rules:
- "create": the new node covers genuinely new territory
- "update": an existing node should have its abstract enriched. Provide merged abstract as mergedContent. Use the existing node's URI as targetId.
- "skip": the new node is already fully covered by an existing node

Respond with ONLY a JSON object:
- {"action":"create"}
- {"action":"update","targetId":"<exact uri>","mergedContent":"<merged abstract>"}
- {"action":"skip","reason":"<brief reason>"}`, uri, name, abstract, candidateList)

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
