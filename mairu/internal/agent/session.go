package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"mairu/internal/llm"
	"mairu/internal/prompts"
)

type SavedPart struct {
	Type     string         `json:"type"`                // "text", "function_call", "function_response", "executable_code", "code_execution_result"
	Text     string         `json:"text,omitempty"`      // For "text" type
	FuncName string         `json:"func_name,omitempty"` // For function call/response
	FuncArgs map[string]any `json:"func_args,omitempty"` // For function call
	FuncResp map[string]any `json:"func_resp,omitempty"` // For function response
	Language int32          `json:"language,omitempty"`  // For executable code
	Code     string         `json:"code,omitempty"`      // For executable code
	Outcome  int32          `json:"outcome,omitempty"`   // For code execution result
	Output   string         `json:"output,omitempty"`    // For code execution result
}

type SavedMessage struct {
	Role  string      `json:"role"`
	Parts []SavedPart `json:"parts"`
}

var sessionNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func ValidateSessionName(sessionName string) error {
	name := strings.TrimSpace(sessionName)
	if name == "" {
		return fmt.Errorf("session name cannot be empty")
	}
	if !sessionNamePattern.MatchString(name) {
		return fmt.Errorf("invalid session name %q: only letters, numbers, dot, underscore, and hyphen are allowed", sessionName)
	}
	return nil
}

func (a *Agent) sessionFilePath(sessionName string) (string, error) {
	return SessionFilePath(a.root, sessionName)
}

func SessionFilePath(projectRoot, sessionName string) (string, error) {
	if err := ValidateSessionName(sessionName); err != nil {
		return "", err
	}
	return filepath.Join(projectRoot, ".mairu", "sessions", strings.TrimSpace(sessionName)+".json"), nil
}

func (a *Agent) ListSessions() ([]string, error) {
	return ListSessions(a.root)
}

func ListSessions(projectRoot string) ([]string, error) {
	sessionsDir, err := ensureSessionsDir(projectRoot)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil, err
	}
	var sessions []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			sessions = append(sessions, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	slices.Sort(sessions)
	return sessions, nil
}

func ensureSessionsDir(projectRoot string) (string, error) {
	sessionsDir := filepath.Join(projectRoot, ".mairu", "sessions")
	info, err := os.Stat(sessionsDir)
	if err == nil {
		if info.IsDir() {
			return sessionsDir, nil
		}
		if err := migrateLegacySessionsFile(sessionsDir); err != nil {
			return "", err
		}
		return sessionsDir, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return "", err
	}
	return sessionsDir, nil
}

func migrateLegacySessionsFile(sessionsDir string) error {
	legacyData, readErr := os.ReadFile(sessionsDir)
	if readErr != nil {
		return readErr
	}

	backupPath := sessionsDir + ".legacy"
	if _, err := os.Stat(backupPath); err == nil {
		backupPath = fmt.Sprintf("%s.legacy.%d", sessionsDir, time.Now().UnixNano())
	}
	if err := os.Rename(sessionsDir, backupPath); err != nil {
		return err
	}
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return err
	}

	type LegacySavedMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	var legacyMessages []LegacySavedMessage
	if err := json.Unmarshal(legacyData, &legacyMessages); err == nil {
		var migrated []SavedMessage
		for _, lm := range legacyMessages {
			migrated = append(migrated, SavedMessage{
				Role: lm.Role,
				Parts: []SavedPart{
					{
						Type: "text",
						Text: lm.Content,
					},
				},
			})
		}
		migratedData, err := json.MarshalIndent(migrated, "", "  ")
		if err == nil {
			defaultPath := filepath.Join(sessionsDir, "default.json")
			if writeErr := os.WriteFile(defaultPath, migratedData, 0644); writeErr != nil {
				return writeErr
			}
		}
	}

	return nil
}

func CreateEmptySession(projectRoot, sessionName string) error {
	if strings.TrimSpace(sessionName) == "" {
		sessionName = "default"
	}

	if _, err := ensureSessionsDir(projectRoot); err != nil {
		return err
	}
	sessionPath, err := SessionFilePath(projectRoot, sessionName)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(sessionPath, []byte("[]"), 0644)
}

func (a *Agent) SaveSession(sessionName string) error {
	if strings.TrimSpace(sessionName) == "" {
		sessionName = "default"
	}
	if _, err := ensureSessionsDir(a.root); err != nil {
		return err
	}

	history := a.llm.GetHistory()
	var saved []SavedMessage

	for _, c := range history {
		if c.Role == "user" || c.Role == "model" || c.Role == "assistant" {
			var savedParts []SavedPart

			// Save text content
			if c.Content != "" {
				savedParts = append(savedParts, SavedPart{
					Type: "text",
					Text: c.Content,
				})
			}

			// Save tool calls
			for _, tc := range c.ToolCalls {
				savedParts = append(savedParts, SavedPart{
					Type:     "function_call",
					FuncName: tc.Name,
					FuncArgs: tc.Arguments,
				})
			}

			// Save tool response indicator
			if c.ToolCallID != "" {
				savedParts = append(savedParts, SavedPart{
					Type:     "function_response",
					FuncName: c.ToolCallID,
				})
			}

			if len(savedParts) > 0 {
				role := c.Role
				if role == "assistant" {
					role = "model"
				}
				saved = append(saved, SavedMessage{
					Role:  role,
					Parts: savedParts,
				})
			}
		}
	}

	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}

	sessionPath, err := a.sessionFilePath(sessionName)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(sessionPath, data, 0644)
}

func (a *Agent) LoadSession(sessionName string) error {
	if strings.TrimSpace(sessionName) == "" {
		sessionName = "default"
	}

	saved, err := a.LoadSavedSessionMessages(sessionName)
	if err != nil {
		return err
	}

	var history []llm.Message
	for _, m := range saved {
		msg := llm.Message{
			Role: m.Role,
		}
		if m.Role == "model" {
			msg.Role = "assistant"
		}

		for _, sp := range m.Parts {
			switch sp.Type {
			case "text":
				msg.Content += sp.Text
			case "function_call":
				msg.ToolCalls = append(msg.ToolCalls, llm.ToolCall{
					Name:      sp.FuncName,
					Arguments: sp.FuncArgs,
				})
			case "function_response":
				msg.ToolCallID = sp.FuncName
				// Note: Function response content is stored in Content field
			}
		}
		if msg.Content != "" || len(msg.ToolCalls) > 0 || msg.ToolCallID != "" {
			history = append(history, msg)
		}
	}

	a.llm.SetHistory(history)
	return nil
}

func (a *Agent) ResetSession() {
	a.llm.SetHistory(nil)
}

func (a *Agent) LoadSavedSessionMessages(sessionName string) ([]SavedMessage, error) {
	return LoadSavedSessionMessages(a.root, sessionName)
}

func LoadSavedSessionMessages(projectRoot, sessionName string) ([]SavedMessage, error) {
	if strings.TrimSpace(sessionName) == "" {
		sessionName = "default"
	}

	if _, err := ensureSessionsDir(projectRoot); err != nil {
		return nil, err
	}
	sessionPath, err := SessionFilePath(projectRoot, sessionName)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(sessionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read session: %w", err)
	}

	var saved []SavedMessage
	if err := json.Unmarshal(data, &saved); err != nil {
		return nil, fmt.Errorf("failed to parse session: %w", err)
	}

	return saved, nil
}

func (a *Agent) GetHistoryText() []string {
	history := a.llm.GetHistory()
	var lines []string

	for _, c := range history {
		if c.Role == "user" || c.Role == "model" || c.Role == "assistant" {
			textContent := c.Content

			// Add tool calls info
			for _, tc := range c.ToolCalls {
				argsStr, _ := json.Marshal(tc.Arguments)
				textContent += fmt.Sprintf("\n[Tool Call]: %s(%s)\n", tc.Name, string(argsStr))
			}

			if textContent != "" {
				prefix := "You: "
				if c.Role == "model" || c.Role == "assistant" {
					prefix = "Mairu: "
				}
				lines = append(lines, prefix+textContent)
			}
		}
	}
	return lines
}

func (a *Agent) CompactContext() error {
	history := a.llm.GetHistory()

	// If history is small, don't bother
	if len(history) < 20 {
		return nil
	}

	var conversation string
	for _, c := range history {
		if c.Role == "user" || c.Role == "model" || c.Role == "assistant" {
			textContent := c.Content

			// Add tool calls info
			for _, tc := range c.ToolCalls {
				argsStr, _ := json.Marshal(tc.Arguments)
				textContent += fmt.Sprintf("\n[Tool Call]: %s(%s)\n", tc.Name, string(argsStr))
			}

			// Add tool response indicator
			if c.ToolCallID != "" {
				textContent += fmt.Sprintf("\n[Tool Response for %s]\n", c.ToolCallID)
			}

			role := c.Role
			if role == "assistant" {
				role = "model"
			}
			conversation += fmt.Sprintf("[%s]: %s\n\n", role, strings.TrimSpace(textContent))
		}
	}

	prompt := prompts.Render("session_summarize", struct {
		Conversation string
	}{
		Conversation: conversation,
	})

	// We use a fresh LLM instance to summarize it to avoid messing up current history
	// Use the agent's stored provider config
	tempLLM, err := llm.NewProvider(a.providerCfg)
	if err != nil {
		return err
	}
	defer tempLLM.Close()

	resp, err := tempLLM.Chat(context.Background(), prompt)
	if err != nil {
		return err
	}

	summary := resp.Content

	// Now replace the history with the summary
	compactedHistory := []llm.Message{
		{
			Role:    "user",
			Content: "Here is the compacted history of our session so far:\n" + summary,
		},
		{
			Role:    "assistant",
			Content: "Understood. I have the context. What's next?",
		},
	}

	a.llm.SetHistory(compactedHistory)
	return nil
}
