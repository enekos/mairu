package agent

import (
	"fmt"
)

func (a *Agent) GetRecentContext() string {
	history := a.llm.GetHistory()
	var conversation string

	// Get up to the last 10 messages
	start := len(history) - 10
	if start < 0 {
		start = 0
	}

	for i := start; i < len(history); i++ {
		c := history[i]
		if c.Role == "user" || c.Role == "model" || c.Role == "assistant" {
			textContent := c.Content
			if textContent != "" {
				role := c.Role
				if role == "assistant" {
					role = "model"
				}
				conversation += fmt.Sprintf("[%s]: %s\n\n", role, textContent)
			}
		}
	}
	return conversation
}
