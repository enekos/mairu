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

	"github.com/google/generative-ai-go/genai"
	"mairu/internal/llm"
	"mairu/internal/prompts"
)

type SavedPart struct {
	Type     string         `json:"type"`                // "text", "function_call", "function_response"
	Text     string         `json:"text,omitempty"`      // For "text" type
	FuncName string         `json:"func_name,omitempty"` // For function call/response
	FuncArgs map[string]any `json:"func_args,omitempty"` // For function call
	FuncResp map[string]any `json:"func_resp,omitempty"` // For function response
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
	return SessionFilePath(a.db.Root(), sessionName)
}

func SessionFilePath(projectRoot, sessionName string) (string, error) {
	if err := ValidateSessionName(sessionName); err != nil {
		return "", err
	}
	return filepath.Join(projectRoot, ".mairu", "sessions", strings.TrimSpace(sessionName)+".json"), nil
}

func (a *Agent) ListSessions() ([]string, error) {
	return ListSessions(a.db.Root())
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
	if _, err := ensureSessionsDir(a.db.Root()); err != nil {
		return err
	}

	history := a.llm.GetHistory()
	var saved []SavedMessage

	for _, c := range history {
		if c.Role == "user" || c.Role == "model" {
			var savedParts []SavedPart
			for _, p := range c.Parts {
				switch v := p.(type) {
				case genai.Text:
					savedParts = append(savedParts, SavedPart{
						Type: "text",
						Text: string(v),
					})
				case genai.FunctionCall:
					savedParts = append(savedParts, SavedPart{
						Type:     "function_call",
						FuncName: v.Name,
						FuncArgs: v.Args,
					})
				case genai.FunctionResponse:
					savedParts = append(savedParts, SavedPart{
						Type:     "function_response",
						FuncName: v.Name,
						FuncResp: v.Response,
					})
				}
			}
			if len(savedParts) > 0 {
				saved = append(saved, SavedMessage{
					Role:  c.Role,
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

	var history []*genai.Content
	for _, m := range saved {
		var genaiParts []genai.Part
		for _, sp := range m.Parts {
			switch sp.Type {
			case "text":
				genaiParts = append(genaiParts, genai.Text(sp.Text))
			case "function_call":
				genaiParts = append(genaiParts, genai.FunctionCall{
					Name: sp.FuncName,
					Args: sp.FuncArgs,
				})
			case "function_response":
				genaiParts = append(genaiParts, genai.FunctionResponse{
					Name:     sp.FuncName,
					Response: sp.FuncResp,
				})
			}
		}
		if len(genaiParts) > 0 {
			history = append(history, &genai.Content{
				Role:  m.Role,
				Parts: genaiParts,
			})
		}
	}

	a.llm.SetHistory(history)
	return nil
}

func (a *Agent) ResetSession() {
	a.llm.SetHistory(nil)
}

func (a *Agent) LoadSavedSessionMessages(sessionName string) ([]SavedMessage, error) {
	return LoadSavedSessionMessages(a.db.Root(), sessionName)
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
		if c.Role == "user" || c.Role == "model" {
			var textContent string
			for _, p := range c.Parts {
				if t, ok := p.(genai.Text); ok {
					textContent += string(t)
				}
			}
			if textContent != "" {
				prefix := "You: "
				if c.Role == "model" {
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
		if c.Role == "user" || c.Role == "model" {
			var textContent string
			for _, p := range c.Parts {
				if t, ok := p.(genai.Text); ok {
					textContent += string(t)
				}
			}
			conversation += fmt.Sprintf("[%s]: %s\n\n", c.Role, textContent)
		}
	}

	prompt := prompts.Render("session_summarize", struct {
		Conversation string
	}{
		Conversation: conversation,
	})

	// We use a fresh LLM instance to summarize it to avoid messing up current history
	// Need to import context and llm if they aren't already imported
	tempLLM, err := llm.NewGeminiProvider(context.Background(), a.apiKey)
	if err != nil {
		return err
	}
	defer tempLLM.Close()

	resp, err := tempLLM.ChatStream(context.Background(), prompt).Next()
	if err != nil || len(resp.Candidates) == 0 {
		return err
	}

	var summary string
	for _, p := range resp.Candidates[0].Content.Parts {
		if t, ok := p.(genai.Text); ok {
			summary += string(t)
		}
	}

	// Now replace the history with the summary
	compactedHistory := []*genai.Content{
		{
			Role:  "user",
			Parts: []genai.Part{genai.Text("Here is the compacted history of our session so far:\n" + summary)},
		},
		{
			Role:  "model",
			Parts: []genai.Part{genai.Text("Understood. I have the context. What's next?")},
		},
	}

	a.llm.SetHistory(compactedHistory)
	return nil
}
