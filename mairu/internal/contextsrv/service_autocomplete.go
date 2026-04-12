package contextsrv

import (
	"context"
	"fmt"
	"strings"

	"mairu/internal/prompts"
)

type AutocompleteRequest struct {
	Prefix   string `json:"prefix"`
	Suffix   string `json:"suffix"`
	Filename string `json:"filename"`
	Project  string `json:"project"`
}

type AutocompleteResponse struct {
	Completion string `json:"completion"`
}

func (s *AppService) Autocomplete(req AutocompleteRequest) (AutocompleteResponse, error) {
	if s.llmClient == nil {
		return AutocompleteResponse{}, fmt.Errorf("LLM client not configured")
	}

	// Optionally fetch ambient context for this file
	ambientQuery := fmt.Sprintf("contextfs://%s/%s", req.Project, req.Filename)
	ambientResult, _ := s.Search(SearchOptions{
		Query:   ambientQuery,
		Project: req.Project,
		Store:   StoreNode,
		TopK:    3,
	})

	contextStr := ""
	if ambientResult != nil && ambientResult["contextNodes"] != nil {
		nodes := toAnyMapSlice(ambientResult["contextNodes"])
		if len(nodes) > 0 {
			if abstract, ok := nodes[0]["abstract"].(string); ok {
				contextStr = "File Context:\n" + abstract + "\n\n"
			}
		}
	}

	// We use gemini-pro or whatever is configured, wait, we don't have model injected easily,
	// let's just pass "" and let the provider pick the default model.
	prompt, err := prompts.Render("autocomplete", map[string]any{
		"ContextStr": contextStr,
		"Filename":   req.Filename,
		"Prefix":     req.Prefix,
		"Suffix":     req.Suffix,
	})
	if err != nil {
		return AutocompleteResponse{}, fmt.Errorf("failed to render prompt: %w", err)
	}

	completion, err := s.llmClient.GenerateContent(context.Background(), "gemini-2.5-flash", prompt)
	if err != nil {
		return AutocompleteResponse{}, err
	}

	// Clean up potential markdown code block if the LLM ignores instructions
	completion = strings.TrimPrefix(completion, "```go\n")
	completion = strings.TrimPrefix(completion, "```lua\n")
	completion = strings.TrimPrefix(completion, "```javascript\n")
	completion = strings.TrimPrefix(completion, "```typescript\n")
	completion = strings.TrimPrefix(completion, "```python\n")
	completion = strings.TrimPrefix(completion, "```\n")
	completion = strings.TrimSuffix(completion, "\n```")

	return AutocompleteResponse{Completion: completion}, nil
}
