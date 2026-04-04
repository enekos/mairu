package tui

type sessionStats struct {
	Model                string
	StreamState          string
	UserMessages         int
	AssistantMessages    int
	SystemMessages       int
	ErrorMessages        int
	DiffMessages         int
	ToolEvents           int
	ToolCalls            int
	ToolResults          int
	EstimatedUserTokens  int
	EstimatedAgentTokens int
	EstimatedTotalTokens int
}

func computeSessionStats(
	messages []ChatMessage,
	currentResponse string,
	toolEvents []toolEvent,
	thinking bool,
	modelName string,
) sessionStats {
	stats := sessionStats{
		Model:       modelName,
		StreamState: "idle",
		ToolEvents:  len(toolEvents),
	}

	if thinking {
		stats.StreamState = "streaming"
	}

	var userChars int
	var agentChars int

	for _, msg := range messages {
		switch msg.Role {
		case "You":
			stats.UserMessages++
			userChars += len(msg.Content)
		case "Mairu":
			stats.AssistantMessages++
			agentChars += len(msg.Content)
		case "System":
			stats.SystemMessages++
		case "Error":
			stats.ErrorMessages++
		case "Diff":
			stats.DiffMessages++
		}
	}

	for _, e := range toolEvents {
		if e.Kind == "call" {
			stats.ToolCalls++
		}
		if e.Kind == "result" {
			stats.ToolResults++
		}
	}

	agentChars += len(currentResponse)
	stats.EstimatedUserTokens = estimateTokenCount(userChars)
	stats.EstimatedAgentTokens = estimateTokenCount(agentChars)
	stats.EstimatedTotalTokens = stats.EstimatedUserTokens + stats.EstimatedAgentTokens

	return stats
}

func estimateTokenCount(charCount int) int {
	if charCount <= 0 {
		return 0
	}
	// Rule-of-thumb estimate for mixed code/text content.
	return (charCount + 3) / 4
}
