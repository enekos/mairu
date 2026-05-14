package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) handleSlashCommand(v string) (model, tea.Cmd, bool) {
	parts := strings.Fields(v)
	if len(parts) == 0 {
		return m, nil, true
	}
	cmd := parts[0]
	arg := ""
	if len(parts) > 1 {
		arg = strings.Join(parts[1:], " ")
	}

	switch cmd {
	case "/clear":
		m.messages = []ChatMessage{{Role: "System", Content: "Terminal cleared."}}
		m.renderMessages()
		m.autoScroll()
		return m, nil, true

	case "/help":
		var b strings.Builder
		b.WriteString("Available commands:\n")
		for _, sc := range allSlashCommands {
			b.WriteString(fmt.Sprintf("  %s – %s\n", sc.Name, sc.Description))
		}
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: b.String()})
		m.renderMessages()
		m.autoScroll()
		return m, nil, true

	case "/save":
		if m.sessionName != "" {
			_ = m.agent.SaveSession(m.sessionName)
			m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Session saved: " + m.sessionName})
		} else {
			m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "No active session to save."})
		}
		m.renderMessages()
		m.autoScroll()
		return m, nil, true

	case "/new", "/reset":
		if m.sessionName != "" {
			_ = m.agent.SaveSession(m.sessionName)
		}
		m.messages = []ChatMessage{{Role: "System", Content: "Started a fresh session."}}
		m.sessionName = ""
		m.renderMessages()
		m.autoScroll()
		return m, nil, true

	case "/fork":
		if arg == "" {
			m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Usage: /fork <new-session-name>"})
			m.renderMessages()
			m.autoScroll()
			return m, nil, true
		}
		if m.sessionName != "" {
			_ = m.agent.SaveSession(m.sessionName)
		}
		m.sessionName = arg
		m.sessions = append(m.sessions, arg)
		_ = m.agent.SaveSession(arg)
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Forked session to: " + arg})
		m.renderMessages()
		m.autoScroll()
		return m, nil, true

	case "/session":
		if arg == "" {
			m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Usage: /session <name>"})
			m.renderMessages()
			m.autoScroll()
			return m, nil, true
		}
		if m.sessionName != "" {
			_ = m.agent.SaveSession(m.sessionName)
		}
		err := m.agent.LoadSession(arg)
		if err != nil {
			m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Failed to load session: " + err.Error()})
		} else {
			m.sessionName = arg
			m.messages = []ChatMessage{{Role: "System", Content: "Loaded session: " + arg}}
			found := false
			for _, s := range m.sessions {
				if s == arg {
					found = true
					break
				}
			}
			if !found {
				m.sessions = append(m.sessions, arg)
			}
			for _, text := range m.agent.GetHistoryText() {
				if strings.HasPrefix(text, "You: ") {
					m.messages = append(m.messages, ChatMessage{Role: "You", Content: strings.TrimPrefix(text, "You: ")})
				} else if strings.HasPrefix(text, "Mairu: ") {
					m.messages = append(m.messages, ChatMessage{Role: "Mairu", Content: strings.TrimPrefix(text, "Mairu: ")})
				}
			}
		}
		m.renderMessages()
		m.autoScroll()
		return m, nil, true

	case "/sessions":
		if len(m.sessions) == 0 {
			m.messages = append(m.messages, ChatMessage{Role: "System", Content: "No sessions available."})
		} else {
			m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Sessions: " + strings.Join(m.sessions, ", ")})
		}
		m.renderMessages()
		m.autoScroll()
		return m, nil, true

	case "/model":
		if arg == "" {
			m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Usage: /model <name>"})
			m.renderMessages()
			m.autoScroll()
			return m, nil, true
		}
		m.agent.SetModel(arg)
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Switched model to: " + arg})
		m.renderMessages()
		m.autoScroll()
		return m, nil, true

	case "/models":
		models := []string{
			"gemini-1.5-flash-latest",
			"gemini-1.5-pro-latest",
			"gemini-2.0-flash-exp",
			"gemini-2.0-pro-exp",
			"gemini-2.5-flash",
			"gemini-2.5-pro",
		}
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Available models: " + strings.Join(models, ", ")})
		m.renderMessages()
		m.autoScroll()
		return m, nil, true

	case "/approve":
		m.agent.ApproveAction(true)
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Action approved."})
		m.renderMessages()
		m.autoScroll()
		return m, nil, true

	case "/deny":
		m.agent.ApproveAction(false)
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Action denied."})
		m.renderMessages()
		m.autoScroll()
		return m, nil, true

	case "/exit", "/quit":
		if m.sessionName != "" {
			_ = m.agent.SaveSession(m.sessionName)
		}
		return m, tea.Quit, true

	case "/explore":
		if m.sidebarMode == "explore" {
			m.sidebarMode = "session"
		} else {
			m.sidebarMode = "explore"
			m.selectedMessage = clampMessageIndex(len(m.messages)-1, 0, len(m.messages))
			m.selectedEvent = -1
		}
		return m, nil, true

	case "/logs":
		if m.sidebarMode == "logs" {
			m.sidebarMode = "session"
		} else {
			m.sidebarMode = "logs"
			if len(m.internalLogs) > 0 {
				m.selectedLog = len(m.internalLogs) - 1
			}
		}
		return m, nil, true

	case "/memory":
		msgs := m.handleMemoryCommand(arg)
		m.messages = append(m.messages, msgs...)
		m.renderMessages()
		m.autoScroll()
		return m, nil, true

	case "/agent", "/nvim", "/lazygit":
		// No-op in text mode; these open panes in the GUI build.
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Pane switching is only available in the GUI build."})
		m.renderMessages()
		m.autoScroll()
		return m, nil, true

	default:
		m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Unknown command: " + cmd + " (try /help)"})
		m.renderMessages()
		m.autoScroll()
		return m, nil, true
	}
}
