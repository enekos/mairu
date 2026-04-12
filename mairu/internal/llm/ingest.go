package llm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"mairu/internal/core"
	"mairu/internal/prompts"
)

const (
	MaxInputChars = 100_000
	maxRetries    = 3
	retryDelayMs  = 1000
)

type ProposedContextNode struct {
	URI       string  `json:"uri"`
	Name      string  `json:"name"`
	Abstract  string  `json:"abstract"`
	Overview  *string `json:"overview,omitempty"`
	Content   *string `json:"content,omitempty"`
	ParentURI *string `json:"parent_uri"`
}

type ContentGenerator interface {
	GenerateContent(ctx context.Context, model, prompt string) (string, error)
}

type StatusError interface {
	error
	StatusCode() int
}

var sleepFn = time.Sleep

func shouldRetry(err error) bool {
	if err == nil {
		return false
	}
	var se StatusError
	if errors.As(err, &se) {
		code := se.StatusCode()
		return code == 429 || code >= 500
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "fetch failed")
}

func generateWithRetry(ctx context.Context, gen ContentGenerator, model, prompt string, attempt int) (string, error) {
	out, err := gen.GenerateContent(ctx, model, prompt)
	if err == nil {
		return out, nil
	}
	if attempt >= maxRetries || !shouldRetry(err) {
		return "", err
	}
	delay := time.Duration(retryDelayMs*(1<<(attempt-1))) * time.Millisecond
	sleepFn(delay)
	return generateWithRetry(ctx, gen, model, prompt, attempt+1)
}

func ParseTextIntoContextNodes(ctx context.Context, gen ContentGenerator, model, text, baseURI string) ([]ProposedContextNode, error) {
	if os.Getenv("GEMINI_API_KEY") == "" {
		return nil, errors.New("GEMINI_API_KEY is not set — cannot parse text into context nodes")
	}
	if len(text) > MaxInputChars {
		return nil, fmt.Errorf("input text is too large (%d chars). maximum is %d", len(text), MaxInputChars)
	}
	if baseURI == "" {
		baseURI = "contextfs://ingested"
	}

	prompt, err := prompts.Render("ingest_context_nodes", struct {
		BaseURI string
		Text    string
	}{
		BaseURI: baseURI,
		Text:    text,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to render prompt: %w", err)
	}
	raw, err := generateWithRetry(ctx, gen, model, prompt, 1)
	if err != nil {
		return nil, err
	}

	parsed := core.ExtractJSONArray(strings.TrimSpace(raw))
	if parsed == nil {
		return nil, fmt.Errorf("LLM returned unparseable output: %s", truncate(raw, 500))
	}

	nodes := make([]ProposedContextNode, 0, len(parsed))
	for _, item := range parsed {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		uri, okU := m["uri"].(string)
		name, okN := m["name"].(string)
		abst, okA := m["abstract"].(string)
		if !okU || !okN || !okA {
			continue
		}
		n := ProposedContextNode{
			URI:      uri,
			Name:     name,
			Abstract: abst,
		}
		if ov, ok := m["overview"].(string); ok && strings.TrimSpace(ov) != "" {
			n.Overview = &ov
		}
		if c, ok := m["content"].(string); ok && strings.TrimSpace(c) != "" {
			n.Content = &c
		}
		if p, ok := m["parent_uri"].(string); ok {
			n.ParentURI = &p
		}
		nodes = append(nodes, n)
	}
	if len(nodes) == 0 {
		return nil, errors.New("LLM returned no valid context nodes")
	}
	return nodes, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
