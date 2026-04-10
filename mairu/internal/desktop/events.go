package desktop

import (
	"context"
	"log/slog"
	"os"

	"mairu/internal/agent"
	"mairu/internal/contextsrv"
	"mairu/internal/llm"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ListSessions returns available chat session names.
func (a *App) ListSessions() ([]string, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return agent.ListSessions(root)
}

// CreateSession creates a new empty chat session.
func (a *App) CreateSession(name string) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := agent.ValidateSessionName(name); err != nil {
		return err
	}
	return agent.CreateEmptySession(root, name)
}

// LoadSessionHistory returns saved messages for a session.
func (a *App) LoadSessionHistory(session string) ([]agent.SavedMessage, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return agent.LoadSavedSessionMessages(root, session)
}

// SendMessage sends a user message and streams agent responses as Wails events.
func (a *App) SendMessage(session, text string) {
	go a.runChat(session, text)
}

func (a *App) getOrCreateAgent(session string) (*agent.Agent, error) {
	a.agentsMu.Lock()
	defer a.agentsMu.Unlock()

	if ag, ok := a.agents[session]; ok {
		return ag, nil
	}

	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	llmProvider, err := llm.NewGeminiProvider(context.Background(), a.cfg.API.GeminiAPIKey)
	if err != nil {
		return nil, err
	}

	indexer := contextsrv.NewMeiliIndexer(a.meili.URL(), a.meili.APIKey(), llmProvider)

	ag, err := agent.New(root, a.cfg.API.GeminiAPIKey, agent.Config{
		SymbolLocator: indexer,
	})
	if err != nil {
		return nil, err
	}

	if err := ag.LoadSession(session); err != nil {
		ag.Close()
		return nil, err
	}

	a.agents[session] = ag
	return ag, nil
}

func (a *App) runChat(session, text string) {
	ag, err := a.getOrCreateAgent(session)
	if err != nil {
		wailsRuntime.EventsEmit(a.ctx, "chat:error", "Failed to init agent: "+err.Error())
		return
	}

	outChan := make(chan agent.AgentEvent)
	go ag.RunStream(text, outChan)

	for ev := range outChan {
		wailsRuntime.EventsEmit(a.ctx, "chat:message", ev)
	}

	if err := ag.SaveSession(session); err != nil {
		slog.Error("failed to save session", "session", session, "error", err)
	}

	wailsRuntime.EventsEmit(a.ctx, "chat:done", session)
}

func (a *App) ApproveAction(session string, approved bool) {
	a.agentsMu.Lock()
	ag, ok := a.agents[session]
	a.agentsMu.Unlock()
	if ok {
		ag.ApproveAction(approved)
	}
}
