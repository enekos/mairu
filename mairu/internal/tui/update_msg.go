package tui

import (
	"fmt"
	"net/http"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func (m model) handleDeleteItemMsg(msg deleteItemMsg) (tea.Model, tea.Cmd) {
	var apiPath string
	var qs map[string]string
	if msg.tab == tabContextNodes {
		apiPath = "/api/context"
		qs = map[string]string{"uri": msg.id}
	} else if msg.tab == tabMemories {
		apiPath = "/api/memories"
		qs = map[string]string{"id": msg.id}
	} else if msg.tab == tabSkills {
		apiPath = "/api/skills"
		qs = map[string]string{"id": msg.id}
	}

	if apiPath != "" {
		req, err := http.NewRequest(http.MethodDelete, m.contextAPIBase()+apiPath, nil)
		if err == nil {
			q := req.URL.Query()
			for k, v := range qs {
				q.Add(k, v)
			}
			req.URL.RawQuery = q.Encode()
			token := m.contextToken()
			if token != "" {
				req.Header.Set("X-Context-Token", token)
			}
			doContextRequest(req)
		}

		m.messages = append(m.messages, ChatMessage{Role: "System", Content: fmt.Sprintf("Deleted item %s", msg.id)})
		m.renderMessages()
		m.autoScroll()

		return m, func() tea.Msg {
			return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/data\n")}
		}
	}
	return m, nil
}

func (m model) handleAnimTickMsg(msg animTickMsg) (model, tea.Cmd, bool) {
	if m.showAnim {
		m.animFrame++
		if m.animFrame > splashTotalFrames {
			m.showAnim = false
			m.recomputeLayout()
			m.renderMessages()
			return m, nil, true
		}
		return m, tickAnim(), true
	}
	if m.thinking {
		// Advance animFrame so streaming border, brand pill, and glyph animate.
		m.animFrame++
		return m, tickAnim(), false
	}
	return m, nil, false
}

func (m model) handleSpinnerTickMsg(msg spinner.TickMsg) model {
	if m.thinking {
		m.refreshThinkingIndicator(time.Now(), false)
	}
	return m
}

func (m model) handleWindowSizeMsg(msg tea.WindowSizeMsg) model {
	m.width = msg.Width
	m.height = msg.Height
	m.listModel.SetSize(msg.Width, msg.Height)
	m.recomputeLayout()
	m.renderMessages()
	m.autoScroll()
	return m
}

func (m model) handleMouseMsg(msg tea.MouseMsg) model {
	if msg.Action == tea.MouseActionPress {
		if msg.Button == tea.MouseButtonWheelUp {
			m.followMode = false
		} else if msg.Button == tea.MouseButtonWheelDown {
			if m.viewport.AtBottom() {
				m.followMode = true
			}
		}
	}
	return m
}

func (m model) handleExternalToolDoneMsg(msg externalToolDoneMsg) (tea.Model, tea.Cmd) {
	m.activePane = paneAgent
	if msg.err != nil {
		m.messages = append(m.messages, ChatMessage{
			Role:    "Error",
			Content: fmt.Sprintf("%s exited with error: %v", paneLabel(msg.pane), msg.err),
		})
	} else {
		m.messages = append(m.messages, ChatMessage{
			Role:    "System",
			Content: fmt.Sprintf("Returned from %s pane.", paneLabel(msg.pane)),
		})
	}
	m.renderMessages()
	m.autoScroll()
	return m, nil
}
