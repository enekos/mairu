package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"mairu/internal/agent"
	"mairu/internal/contextsrv"
)

type externalToolDoneMsg struct {
	pane workspacePane
	err  error
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		spCmd tea.Cmd
		lsCmd tea.Cmd
		cmds  []tea.Cmd
	)

	// === Autocomplete interception ===
	if keyMsg, ok := msg.(tea.KeyMsg); ok && len(m.filteredCommands) > 0 {
		switch keyMsg.Type {
		case tea.KeyUp:
			m.autocompleteIndex--
			if m.autocompleteIndex < 0 {
				m.autocompleteIndex = len(m.filteredCommands) - 1
			}
			return m, nil
		case tea.KeyDown:
			m.autocompleteIndex++
			if m.autocompleteIndex >= len(m.filteredCommands) {
				m.autocompleteIndex = 0
			}
			return m, nil
		case tea.KeyTab:
			m.textarea.SetValue(m.filteredCommands[m.autocompleteIndex].Name + " ")
			m.textarea.SetCursor(len(m.textarea.Value()))
			m.filteredCommands = nil
			return m, nil
		case tea.KeyEsc:
			m.filteredCommands = nil
			return m, nil
		}
	}

	if m.showList {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.width = msg.Width
			m.height = msg.Height
			m.listModel.SetSize(msg.Width, msg.Height)
			return m, nil
		case tea.KeyMsg:
			switch msg.Type {
			case tea.KeyEsc, tea.KeyCtrlC:
				m.showList = false
				return m, nil
			case tea.KeyEnter:
				m.showList = false
				selectedItem := m.listModel.SelectedItem()
				if selectedItem != nil {
					if m.listType == "session" {
						sessionName := selectedItem.(listItem).title
						if m.sessionName != "" {
							_ = m.agent.SaveSession(m.sessionName)
						} else {
							_ = m.agent.SaveSession("current")
						}
						err := m.agent.LoadSession(sessionName)
						if err != nil {
							m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Failed to load session: " + err.Error()})
						} else {
							m.sessionName = sessionName
							m.messages = []ChatMessage{{Role: "System", Content: "Loaded session: " + sessionName}}
							found := false
							for _, s := range m.sessions {
								if s == sessionName {
									found = true
									break
								}
							}
							if !found {
								m.sessions = append(m.sessions, sessionName)
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
						m.viewport.GotoBottom()
					} else if m.listType == "model" {
						modelName := selectedItem.(listItem).title
						m.agent.SetModel(modelName)
						m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Switched model to: " + modelName})
						m.renderMessages()
						m.viewport.GotoBottom()
					}
				}
				return m, nil
			}
		}
		m.listModel, lsCmd = m.listModel.Update(msg)
		return m, lsCmd
	}

	if m.showGraph {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.width = msg.Width
			m.height = msg.Height
			m.dataExplorer.SetSize(msg.Width, msg.Height)
			return m, nil
		case tea.KeyMsg:
			switch msg.Type {
			case tea.KeyEsc, tea.KeyCtrlC:
				m.showGraph = false
				return m, nil
			case tea.KeyEnter:
				selected := m.dataExplorer.lists[m.dataExplorer.activeTab].SelectedItem()
				if selected != nil {
					if graphItem, ok := selected.(graphListItem); ok {
						m.showGraph = false
						m.messages = append(m.messages, ChatMessage{Role: "System", Content: fmt.Sprintf("**Graph Node: %s**\n\n%s\n\n```\n%s\n```", graphItem.uri, graphItem.desc, graphItem.content)})
						m.renderMessages()
						m.viewport.GotoBottom()
					}
				}
				return m, nil
			}
		}
		m.dataExplorer, lsCmd = m.dataExplorer.Update(msg)
		return m, lsCmd
	}

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)
	m.spinner, spCmd = m.spinner.Update(msg)
	cmds = append(cmds, tiCmd, vpCmd, spCmd)

	if _, ok := msg.(tea.KeyMsg); ok {
		val := m.textarea.Value()
		if strings.HasPrefix(val, "/") {
			m.filteredCommands = nil
			for _, cmd := range allSlashCommands {
				if strings.HasPrefix(cmd.Name, val) {
					m.filteredCommands = append(m.filteredCommands, cmd)
				}
			}
			if m.autocompleteIndex >= len(m.filteredCommands) {
				m.autocompleteIndex = 0
			}
		} else {
			m.filteredCommands = nil
		}
	}

	switch msg := msg.(type) {
	case deleteItemMsg:
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

			// Refresh list by triggering /data command internally
			m.messages = append(m.messages, ChatMessage{Role: "System", Content: fmt.Sprintf("Deleted item %s", msg.id)})
			m.renderMessages()
			m.autoScroll()

			// Auto-refresh the view
			return m, func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/data\n")}
			}
		}
		return m, nil
	case animTickMsg:
		if m.showAnim {
			m.animFrame++
			if m.animFrame > 20 {
				m.showAnim = false
				m.recomputeLayout()
				m.renderMessages()
				return m, nil
			}
			return m, tickAnim()
		}
	case spinner.TickMsg:
		if m.thinking {
			m.refreshThinkingIndicator(time.Now(), false)
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.listModel.SetSize(msg.Width, msg.Height)
		m.recomputeLayout()
		m.renderMessages()
		m.autoScroll()

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress {
			if msg.Button == tea.MouseButtonWheelUp {
				m.followMode = false
			} else if msg.Button == tea.MouseButtonWheelDown {
				if m.viewport.AtBottom() {
					m.followMode = true
				}
			}
		}

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			if m.thinking {
				m.agent.Interrupt()
				m.messages = append(m.messages, ChatMessage{Role: "System", Content: "🛑 Interruption requested..."})
				m.renderMessages()
				m.autoScroll()
			} else {
				m.textarea.Reset()
			}
			return m, nil
		case tea.KeyCtrlC, tea.KeyCtrlD:
			if m.sessionName != "" {
				_ = m.agent.SaveSession(m.sessionName)
			}
			return m, tea.Quit
		case tea.KeyPgUp:
			m.viewport.HalfPageUp()
			m.followMode = false
			return m, nil
		case tea.KeyPgDown:
			m.viewport.HalfPageDown()
			m.followMode = false
			return m, nil
		case tea.KeyHome:
			m.viewport.GotoTop()
			m.followMode = false
			return m, nil
		case tea.KeyEnd:
			m.followMode = true
			m.viewport.GotoBottom()
			return m, nil
		case tea.KeyCtrlF:
			m.followMode = !m.followMode
			if m.followMode {
				m.viewport.GotoBottom()
				m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Follow mode enabled."})
			} else {
				m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Follow mode paused. Use End or Ctrl+F to resume."})
			}
			m.renderMessages()
			m.autoScroll()
			return m, nil
		case tea.KeyCtrlE:
			switch m.sidebarMode {
			case "session":
				m.sidebarMode = "explore"
				m.selectedMessage = clampMessageIndex(len(m.messages)-1, 0, len(m.messages))
				m.selectedEvent = -1
			case "explore":
				m.sidebarMode = "logs"
				if len(m.internalLogs) > 0 {
					m.selectedLog = len(m.internalLogs) - 1
				}
			default:
				m.sidebarMode = "session"
			}
			return m, nil
		case tea.KeyCtrlJ:
			if m.sidebarMode == "explore" {
				if m.selectedMessage >= 0 && m.selectedMessage < len(m.messages) {
					msg := m.messages[m.selectedMessage]
					if m.selectedEvent < len(msg.ToolEvents)-1 {
						m.selectedEvent++
					} else if m.selectedMessage < len(m.messages)-1 {
						m.selectedMessage++
						m.selectedEvent = -1
					}
				} else {
					m.selectedMessage = clampMessageIndex(m.selectedMessage, 1, len(m.messages))
					m.selectedEvent = -1
				}
				m.jumpToSelectedMessage()
				m.followMode = false
				m.renderMessages()
				return m, nil
			}
			if m.sidebarMode == "logs" {
				if len(m.internalLogs) > 0 {
					m.selectedLog = clampMessageIndex(m.selectedLog, 1, len(m.internalLogs))
				}
				return m, nil
			}
		case tea.KeyCtrlK:
			if m.sidebarMode == "explore" {
				if m.selectedEvent >= 0 {
					m.selectedEvent--
				} else if m.selectedMessage > 0 {
					m.selectedMessage--
					m.selectedEvent = len(m.messages[m.selectedMessage].ToolEvents) - 1
				} else {
					m.selectedMessage = clampMessageIndex(m.selectedMessage, -1, len(m.messages))
					m.selectedEvent = -1
				}
				m.jumpToSelectedMessage()
				m.followMode = false
				m.renderMessages()
				return m, nil
			}
			if m.sidebarMode == "logs" {
				if len(m.internalLogs) > 0 {
					m.selectedLog = clampMessageIndex(m.selectedLog, -1, len(m.internalLogs))
				}
				return m, nil
			}
		case tea.KeyCtrlP:
			models := []string{
				"gemini-3.1-flash-lite-preview",
				"gemini-3.1-pro-preview",
				"gemini-1.5-pro-latest",
				"gemini-1.5-flash-latest",
				"gemini-2.0-flash-exp",
				"gemini-2.0-pro-exp",
			}
			current := m.agent.GetModelName()
			idx := 0
			for i, mod := range models {
				if mod == current {
					idx = i
					break
				}
			}
			idx = (idx + 1) % len(models)
			newMod := models[idx]
			m.agent.SetModel(newMod)
			m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Switched model to: " + newMod})
			m.renderMessages()
			m.autoScroll()
			return m, nil
		case tea.KeyCtrlL:
			m.messages = []ChatMessage{{Role: "System", Content: "Terminal cleared."}}
			m.renderMessages()
			m.autoScroll()
			return m, nil
		case tea.KeyCtrlO:
			if len(m.sessions) > 1 {
				// find current
				idx := -1
				for i, s := range m.sessions {
					if s == m.sessionName {
						idx = i
						break
					}
				}
				idx = (idx + 1) % len(m.sessions)
				newSession := m.sessions[idx]

				if m.sessionName != "" {
					_ = m.agent.SaveSession(m.sessionName)
				}
				err := m.agent.LoadSession(newSession)
				if err != nil {
					m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Failed to load session: " + err.Error()})
				} else {
					m.sessionName = newSession
					m.messages = []ChatMessage{{Role: "System", Content: "Loaded session: " + newSession}}
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
			}
			return m, nil
		case tea.KeyCtrlN:
			return m, m.openWorkspacePane(paneNvim)
		case tea.KeyCtrlG:
			return m, m.openWorkspacePane(paneLazygit)
		case tea.KeyEnter:
			if msg.Alt {
				m.textarea.InsertString("\n")
				return m, nil
			}
			if m.sidebarMode == "explore" {
				if m.selectedMessage >= 0 && m.selectedMessage < len(m.messages) {
					m.messages[m.selectedMessage].Expanded = !m.messages[m.selectedMessage].Expanded
					m.renderMessages()
					m.jumpToSelectedMessage()
				}
				return m, nil
			}
			if m.thinking {
				v := strings.TrimSpace(m.textarea.Value())
				if v == "/approve" || v == "/deny" {
					// Proceed to process the command
				} else {
					if v != "" {
						m.queuedMessages = append(m.queuedMessages, v)
						m.textarea.Reset()
						m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Message queued... (will be sent after current task)"})
						m.renderMessages()
						m.viewport.GotoBottom()
					}
					return m, nil
				}
			}

			m.filteredCommands = nil
			v := strings.TrimSpace(m.textarea.Value())
			m.textarea.Reset()

			if v == "" {
				return m, nil
			}

			if strings.HasPrefix(v, "!!") {
				cmdStr := strings.TrimSpace(strings.TrimPrefix(v, "!!"))
				m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Running local command: " + cmdStr})
				m.renderMessages()
				m.viewport.GotoBottom()

				out, err := m.agent.RunBash(context.Background(), cmdStr, 60000, nil)
				if err != nil {
					m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Command failed: " + err.Error() + "\n" + out})
				} else {
					m.messages = append(m.messages, ChatMessage{Role: "System", Content: "```\n" + out + "\n```"})
				}
				m.renderMessages()
				m.viewport.GotoBottom()
				return m, nil
			} else if strings.HasPrefix(v, "!") {
				cmdStr := strings.TrimSpace(strings.TrimPrefix(v, "!"))
				m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Running command and appending to prompt: " + cmdStr})
				m.renderMessages()
				m.viewport.GotoBottom()

				out, err := m.agent.RunBash(context.Background(), cmdStr, 60000, nil)
				if err != nil {
					v += fmt.Sprintf("\n\nCommand `!%s` failed: %v\nOutput: %s", cmdStr, err, out)
				} else {
					v += fmt.Sprintf("\n\nOutput of `!%s`:\n```\n%s\n```", cmdStr, out)
				}
			}

			fileRegex := regexp.MustCompile(`@([a-zA-Z0-9_./-]+)`)
			matches := fileRegex.FindAllStringSubmatch(v, -1)
			if len(matches) > 0 {
				for _, match := range matches {
					filePath := match[1]
					content, err := os.ReadFile(filepath.Join(m.agent.GetRoot(), filePath))
					if err != nil {
						content, err = os.ReadFile(filePath)
					}
					if err == nil {
						v += fmt.Sprintf("\n\nFile: %s\n```\n%s\n```", filePath, string(content))
					} else {
						m.messages = append(m.messages, ChatMessage{Role: "Error", Content: fmt.Sprintf("Could not read file @%s: %v", filePath, err)})
					}
				}
			}

			if strings.HasPrefix(v, "/") {
				cmdParts := strings.Fields(v)
				command := cmdParts[0]

				switch command {
				case "/copy":
					var lastMairu string
					for i := len(m.messages) - 1; i >= 0; i-- {
						if m.messages[i].Role == "Mairu" {
							lastMairu = m.messages[i].Content
							break
						}
					}
					if lastMairu != "" {
						err := clipboard.WriteAll(lastMairu)
						if err != nil {
							m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Failed to copy to clipboard: " + err.Error()})
						} else {
							m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Copied last response to clipboard."})
						}
					} else {
						m.messages = append(m.messages, ChatMessage{Role: "System", Content: "No response to copy."})
					}
					m.renderMessages()
					m.viewport.GotoBottom()
					return m, nil
				case "/exit", "/quit":
					return m, tea.Quit
				case "/approve":
					m.agent.ApproveAction(true)
					m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Action approved."})
					m.renderMessages()
					m.viewport.GotoBottom()
					return m, nil
				case "/deny":
					m.agent.ApproveAction(false)
					m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Action denied."})
					m.renderMessages()
					m.viewport.GotoBottom()
					return m, nil
				case "/clear":
					m.messages = []ChatMessage{{Role: "System", Content: "Terminal cleared."}}
					m.renderMessages()
					m.viewport.GotoBottom()
					return m, nil
				case "/help":
					helpText := `**Available Commands:**
- /help: Show this help message
- /clear: Clear the terminal screen (or Ctrl+L)
- /copy: Copy last response to clipboard
- /models: Open model selector
- /model <name>: Switch to a specific model
- /sessions: Open session selector
- /session <name>: Load a specific session
- /memory <search|read|store|write> <text>: Interact with contextfs memory
- /node <search|read|ls|store|write> <text|uri|args>: Interact with contextfs nodes
- /vibe <query>: Run contextfs vibe-query
- /remember <text>: Run contextfs vibe-mutation
- /save <name>: Save the current session
- /fork <name>: Fork the current session to a new name
- /reset or /new: Start a fresh session and clear context
- /compact: Summarize history to save tokens
- /export <file>: Export conversation to a file
- /approve: Approve pending agent action
- /deny: Deny pending agent action
- /graph: Interactive Context Graph Explorer
- /data: Interactive Workspace Data Explorer
- /explore: Toggle explore sidebar (message navigator + tool drilldown)
- /logs: Toggle dedicated internal logs sidebar
- /agent: Focus agent chat pane
- /nvim: Open Neovim in workspace pane
- /lazygit: Open LazyGit in workspace pane
- /pane <agent|nvim|lazygit>: Switch workspace pane
- /jump <n>: Jump to message number n
- /exit or /quit: Exit Mairu

**Navigation**
- PgUp/PgDown: Scroll chat by half-page
- Home/End: Jump top/bottom
- Ctrl+F: Toggle follow mode during streaming
- Ctrl+E: Cycle session/explore/logs sidebar
- Ctrl+J / Ctrl+K: Navigate explore/logs selection
- Ctrl+N / Ctrl+G: Jump to Neovim/LazyGit panes`
					m.messages = append(m.messages, ChatMessage{Role: "System", Content: helpText})
					m.renderMessages()
					m.autoScroll()
					return m, nil
				case "/fork":
					if len(cmdParts) > 1 {
						newName := cmdParts[1]
						err := m.agent.SaveSession(newName)
						if err != nil {
							m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Failed to fork session: " + err.Error()})
						} else {
							m.sessionName = newName
							m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Forked session to: " + newName})
							found := false
							for _, s := range m.sessions {
								if s == newName {
									found = true
									break
								}
							}
							if !found {
								m.sessions = append(m.sessions, newName)
							}
						}
					} else {
						m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Usage: /fork <name>"})
					}
					m.renderMessages()
					m.viewport.GotoBottom()
					return m, nil
				case "/save":
					if len(cmdParts) > 1 {
						sessionName := cmdParts[1]
						err := m.agent.SaveSession(sessionName)
						if err != nil {
							m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Failed to save session: " + err.Error()})
						} else {
							m.sessionName = sessionName
							m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Session saved as: " + sessionName})
							found := false
							for _, s := range m.sessions {
								if s == sessionName {
									found = true
									break
								}
							}
							if !found {
								m.sessions = append(m.sessions, sessionName)
							}
						}
					} else {
						m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Usage: /save <name>"})
					}
					m.renderMessages()
					m.viewport.GotoBottom()
					return m, nil
				case "/reset", "/new":
					m.agent.ResetSession()
					m.sessionName = "default"
					m.messages = []ChatMessage{{Role: "System", Content: "Session reset. Starting fresh context."}}
					found := false
					for _, s := range m.sessions {
						if s == "default" {
							found = true
							break
						}
					}
					if !found {
						m.sessions = append(m.sessions, "default")
					}
					m.renderMessages()
					m.viewport.GotoBottom()
					return m, nil
				case "/compact", "/squash":
					m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Compacting context... please wait."})
					m.renderMessages()
					m.viewport.GotoBottom()

					// Simple spinner update while it happens synchronously
					err := m.agent.CompactContext()
					if err != nil {
						m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Failed to compact: " + err.Error()})
					} else {
						m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Context compacted successfully!"})
					}
					m.renderMessages()
					m.viewport.GotoBottom()
					return m, nil
				case "/export":
					if len(cmdParts) > 1 {
						fileName := cmdParts[1]
						history := m.agent.GetHistoryText()
						content := strings.Join(history, "\n\n")
						err := os.WriteFile(fileName, []byte(content), 0644)
						if err != nil {
							m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Failed to export: " + err.Error()})
						} else {
							m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Conversation exported to: " + fileName})
						}
					} else {
						m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Usage: /export <filename>"})
					}
					m.renderMessages()
					m.viewport.GotoBottom()
					return m, nil
				case "/memory":
					if len(cmdParts) >= 3 {
						subCmd := cmdParts[1]
						args := strings.Join(cmdParts[2:], " ")
						projectName := filepath.Base(m.agent.GetRoot())

						var out []byte
						var err error
						if subCmd == "search" || subCmd == "read" {
							m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Searching memory: " + args})
							out, err = m.contextGet("/api/search", map[string]string{
								"q":       args,
								"type":    "memory",
								"topK":    "5",
								"project": projectName,
							})
						} else if subCmd == "store" || subCmd == "write" {
							m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Storing memory: " + args})
							out, err = m.contextPost("/api/memories", map[string]any{
								"project":    projectName,
								"content":    args,
								"category":   "observation",
								"owner":      "user",
								"importance": 5,
							})
						} else {
							m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Usage: /memory <search|read|store|write> <text>"})
							m.renderMessages()
							m.viewport.GotoBottom()
							return m, nil
						}

						m.renderMessages()
						m.viewport.GotoBottom()

						if err != nil {
							m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Memory operation failed: " + err.Error()})
						} else {
							m.messages = append(m.messages, ChatMessage{Role: "System", Content: "```json\n" + prettyJSON(out) + "\n```"})
						}
					} else {
						m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Usage: /memory <search|read|store|write> <text>"})
					}
					m.renderMessages()
					m.viewport.GotoBottom()
					return m, nil

				case "/node":
					if len(cmdParts) >= 3 {
						subCmd := cmdParts[1]
						args := strings.Join(cmdParts[2:], " ")
						projectName := filepath.Base(m.agent.GetRoot())

						var out []byte
						var err error
						if subCmd == "search" || subCmd == "read" {
							m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Searching nodes: " + args})
							out, err = m.contextGet("/api/search", map[string]string{
								"q":       args,
								"type":    "context",
								"topK":    "5",
								"project": projectName,
							})
						} else if subCmd == "ls" {
							m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Listing node: " + args})
							out, err = m.contextGet("/api/context", map[string]string{
								"project":   projectName,
								"parentUri": args,
							})
						} else {
							m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Usage: /node <search|read|ls> <text|uri>"})
							m.renderMessages()
							m.viewport.GotoBottom()
							return m, nil
						}

						m.renderMessages()
						m.viewport.GotoBottom()

						if err != nil {
							m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Node operation failed: " + err.Error()})
						} else {
							m.messages = append(m.messages, ChatMessage{Role: "System", Content: "```json\n" + prettyJSON(out) + "\n```"})
						}
					} else {
						m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Usage: /node <search|read|ls> <text|uri>"})
					}
					m.renderMessages()
					m.viewport.GotoBottom()
					return m, nil

				case "/vibe":
					if len(cmdParts) > 1 {
						args := strings.Join(cmdParts[1:], " ")
						projectName := filepath.Base(m.agent.GetRoot())

						m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Vibe querying: " + args})
						m.renderMessages()
						m.viewport.GotoBottom()

						out, err := m.contextPost("/api/vibe/query", map[string]any{
							"prompt":  args,
							"project": projectName,
							"topK":    5,
						})
						if err != nil {
							m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Vibe query failed: " + err.Error()})
						} else {
							m.messages = append(m.messages, ChatMessage{Role: "System", Content: "```json\n" + prettyJSON(out) + "\n```"})
						}
					} else {
						m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Usage: /vibe <query>"})
					}
					m.renderMessages()
					m.viewport.GotoBottom()
					return m, nil

				case "/remember":
					if len(cmdParts) > 1 {
						args := strings.Join(cmdParts[1:], " ")
						projectName := filepath.Base(m.agent.GetRoot())

						m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Remembering: " + args})
						m.renderMessages()
						m.viewport.GotoBottom()

						planOut, err := m.contextPost("/api/vibe/mutation/plan", map[string]any{
							"prompt":  args,
							"project": projectName,
							"topK":    5,
						})
						if err != nil {
							m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Vibe planning failed: " + err.Error()})
						} else {
							var plan struct {
								Operations []map[string]any `json:"operations"`
							}
							if err := json.Unmarshal(planOut, &plan); err != nil {
								m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Failed to parse mutation plan: " + err.Error()})
							} else {
								execOut, execErr := m.contextPost("/api/vibe/mutation/execute", map[string]any{
									"project":    projectName,
									"operations": plan.Operations,
								})
								if execErr != nil {
									m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Vibe mutation failed: " + execErr.Error()})
								} else {
									m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Knowledge stored successfully:\n```json\n" + prettyJSON(execOut) + "\n```"})
								}
							}
						}
					} else {
						m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Usage: /remember <fact or rule>"})
					}
					m.renderMessages()
					m.viewport.GotoBottom()
					return m, nil
				case "/models":
					m.listType = "model"
					m.showList = true
					var items []list.Item
					models := []string{
						"gemini-3.1-flash-lite-preview",
						"gemini-3.1-pro-preview",
						"gemini-1.5-pro-latest",
						"gemini-1.5-flash-latest",
						"gemini-2.0-flash-exp",
						"gemini-2.0-pro-exp",
					}
					for _, mod := range models {
						items = append(items, listItem{title: mod, desc: "Gemini Model"})
					}
					m.listModel.SetItems(items)
					m.listModel.Title = "Select Model (Current: " + m.agent.GetModelName() + ")"
					return m, nil
				case "/model":
					if len(cmdParts) > 1 {
						modelName := cmdParts[1]
						m.agent.SetModel(modelName)
						m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Switched model to: " + modelName})
					} else {
						m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Usage: /model <name>"})
					}
					m.renderMessages()
					m.autoScroll()
					return m, nil
				case "/graph", "/data":
					m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Fetching workspace data..."})
					m.renderMessages()
					m.viewport.GotoBottom()

					var cItems, mItems, sItems []list.Item

					// Context Nodes
					out, err := m.contextGet("/api/context", map[string]string{"limit": "1000"})
					if err == nil {
						var nodes []NodeItem
						if json.Unmarshal(out, &nodes) == nil {
							graphItems := buildGraphItems(nodes)
							for _, g := range graphItems {
								cItems = append(cItems, g)
							}
						}
					}

					// Memories
					out, err = m.contextGet("/api/memories", map[string]string{"limit": "1000"})
					if err == nil {
						var mems []contextsrv.Memory
						if json.Unmarshal(out, &mems) == nil {
							for _, mem := range mems {
								mItems = append(mItems, memoryListItem{
									id: mem.ID, project: mem.Project, content: mem.Content,
									category: mem.Category, owner: mem.Owner, importance: mem.Importance,
								})
							}
						}
					}

					// Skills
					out, err = m.contextGet("/api/skills", map[string]string{"limit": "1000"})
					if err == nil {
						var skills []contextsrv.Skill
						if json.Unmarshal(out, &skills) == nil {
							for _, s := range skills {
								sItems = append(sItems, skillListItem{
									id: s.ID, project: s.Project, name: s.Name, desc: s.Description,
								})
							}
						}
					}

					m.dataExplorer = newDataExplorerModel(cItems, mItems, sItems)
					m.dataExplorer.SetSize(m.width, m.height)
					m.showGraph = true

					m.renderMessages()
					m.autoScroll()
					return m, nil
				case "/explore":
					if m.sidebarMode != "explore" {
						m.sidebarMode = "explore"
						m.selectedMessage = clampMessageIndex(len(m.messages)-1, 0, len(m.messages))
						m.selectedEvent = -1
						m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Explore sidebar enabled. Use Ctrl+J / Ctrl+K to jump between messages."})
					} else {
						m.sidebarMode = "session"
						m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Explore sidebar hidden."})
					}
					m.renderMessages()
					m.autoScroll()
					return m, nil
				case "/logs":
					if m.sidebarMode == "logs" {
						m.sidebarMode = "session"
						m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Log sidebar hidden."})
					} else {
						m.sidebarMode = "logs"
						if len(m.internalLogs) > 0 {
							m.selectedLog = len(m.internalLogs) - 1
						}
						m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Log sidebar enabled. Use Ctrl+J / Ctrl+K to inspect entries."})
					}
					m.renderMessages()
					m.autoScroll()
					return m, nil
				case "/agent":
					return m, m.openWorkspacePane(paneAgent)
				case "/nvim":
					return m, m.openWorkspacePane(paneNvim)
				case "/lazygit":
					return m, m.openWorkspacePane(paneLazygit)
				case "/pane":
					if len(cmdParts) < 2 {
						m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Usage: /pane <agent|nvim|lazygit>"})
						m.renderMessages()
						m.autoScroll()
						return m, nil
					}
					pane, ok := parseWorkspacePane(cmdParts[1])
					if !ok {
						m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Unknown pane. Use: agent, nvim, lazygit"})
						m.renderMessages()
						m.autoScroll()
						return m, nil
					}
					return m, m.openWorkspacePane(pane)
				case "/jump":
					if len(cmdParts) > 1 {
						idx, err := strconv.Atoi(cmdParts[1])
						if err != nil {
							m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Usage: /jump <message-number>"})
						} else {
							target := idx - 1
							if target < 0 || target >= len(m.messages) {
								m.messages = append(m.messages, ChatMessage{Role: "Error", Content: fmt.Sprintf("Message %d is out of range.", idx)})
							} else {
								m.selectedMessage = target
								m.selectedEvent = -1
								m.jumpToSelectedMessage()
								m.followMode = false
							}
						}
					} else {
						m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Usage: /jump <message-number>"})
					}
					m.renderMessages()
					return m, nil
				case "/sessions":
					m.listType = "session"
					m.showList = true
					sessions, _ := m.agent.ListSessions()
					var items []list.Item
					for _, s := range sessions {
						items = append(items, listItem{title: s, desc: "Saved session"})
					}
					m.listModel.SetItems(items)
					m.listModel.Title = "Select Session"
					return m, nil
				case "/session":
					if len(cmdParts) > 1 {
						sessionName := cmdParts[1]
						if m.sessionName != "" {
							_ = m.agent.SaveSession(m.sessionName)
						} else {
							_ = m.agent.SaveSession("current")
						}
						err := m.agent.LoadSession(sessionName)
						if err != nil {
							m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Failed to load session: " + err.Error()})
						} else {
							m.sessionName = sessionName
							m.messages = []ChatMessage{{Role: "System", Content: "Loaded session: " + sessionName}}
							found := false
							for _, s := range m.sessions {
								if s == sessionName {
									found = true
									break
								}
							}
							if !found {
								m.sessions = append(m.sessions, sessionName)
							}
							for _, text := range m.agent.GetHistoryText() {
								if strings.HasPrefix(text, "You: ") {
									m.messages = append(m.messages, ChatMessage{Role: "You", Content: strings.TrimPrefix(text, "You: ")})
								} else if strings.HasPrefix(text, "Mairu: ") {
									m.messages = append(m.messages, ChatMessage{Role: "Mairu", Content: strings.TrimPrefix(text, "Mairu: ")})
								}
							}
						}
					} else {
						m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Usage: /session <name>"})
					}
					m.renderMessages()
					m.autoScroll()
					return m, nil
				default:
					m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Unknown command: " + command + "\nType /help for a list of commands."})
					m.renderMessages()
					m.autoScroll()
					return m, nil
				}
			}

			// Adjust textarea height dynamically if text was multiple lines
			taHeight := m.textarea.Height()
			newHeight := m.height - taHeight - 2
			if newHeight < 5 {
				newHeight = 5
			}
			m.viewport.Height = newHeight

			m.followMode = true
			m.messages = append(m.messages, ChatMessage{Role: "You", Content: v})
			m.pushInternalLog("user", "User prompt submitted", v)
			m.renderMessages()
			m.autoScroll()

			m.thinking = true
			m.refreshThinkingIndicator(time.Now(), true)
			m.currentResponse = ""
			m.toolEvents = nil

			m.activeStream = make(chan agent.AgentEvent, 100)
			go m.agent.RunStream(v, m.activeStream)

			cmds = append(cmds, waitForStream(m.activeStream))
		}

	case agentStreamMsg:
		if msg.Type == "done" {
			m.thinking = false
			m.clearThinkingIndicator()

			// If there's no text response but there are tool events,
			// expand them by default so the user can see what the agent did.
			expanded := false
			if strings.TrimSpace(m.currentResponse) == "" && len(m.toolEvents) > 0 {
				expanded = true
			}

			m.messages = append(m.messages, ChatMessage{
				Role:       "Mairu",
				Content:    m.currentResponse,
				ToolEvents: m.toolEvents,
				Expanded:   expanded,
			})
			m.currentResponse = ""
			m.currentBashOutput = ""
			m.toolEvents = nil
			m.activeStream = nil
			m.pushInternalLog("done", "Stream completed", "")
			m.renderMessages()
			m.autoScroll()

			if m.sessionName != "" {
				_ = m.agent.SaveSession(m.sessionName)
			}

			if len(m.queuedMessages) > 0 {
				nextMsg := m.queuedMessages[0]
				m.queuedMessages = m.queuedMessages[1:]

				m.messages = append(m.messages, ChatMessage{Role: "You", Content: nextMsg})
				m.pushInternalLog("user", "Dequeued user prompt", nextMsg)
				m.renderMessages()
				m.autoScroll()

				m.thinking = true
				m.refreshThinkingIndicator(time.Now(), true)
				m.currentResponse = ""
				m.toolEvents = nil

				m.activeStream = make(chan agent.AgentEvent, 100)
				go m.agent.RunStream(nextMsg, m.activeStream)

				cmds = append(cmds, waitForStream(m.activeStream))
			}
		} else if msg.Type == "text" {
			m.currentBashOutput = ""
			m.currentResponse += msg.Content
			m.pushInternalLog("text", "Assistant text chunk", msg.Content)
			m.renderMessages()
			m.autoScroll()
			cmds = append(cmds, waitForStream(m.activeStream))
		} else if msg.Type == "diff" {
			m.messages = append(m.messages, ChatMessage{Role: "Diff", Content: msg.Content})
			m.renderMessages()
			m.autoScroll()
			cmds = append(cmds, waitForStream(m.activeStream))
		} else if msg.Type == "log" {
			// Print to toolLog so it shows up in "Tool Drilldown" inside explore sidebar
			m.pushToolLog("log", msg.Content)
			m.pushInternalLog("log", msg.Content, msg.Content)
			// Don't show log events in main chat, just background sidebar
			cmds = append(cmds, waitForStream(m.activeStream))
		} else if msg.Type == "bash_output" {
			m.pushToolLog("bash", msg.Content)
			m.pushInternalLog("bash", "Bash output chunk", msg.Content)
			m.currentBashOutput += msg.Content
			if len(m.currentBashOutput) > 2000 {
				m.currentBashOutput = m.currentBashOutput[len(m.currentBashOutput)-2000:]
			}
			m.renderMessages()
			m.autoScroll()
			cmds = append(cmds, waitForStream(m.activeStream))
		} else if msg.Type == "status" {
			ev := buildToolStatusEvent(msg.Content)
			m.toolEvents = append(m.toolEvents, ev)
			m.pushToolLog("status", ev.Title)
			m.pushInternalLog("status", ev.Title, msg.Content)
			m.currentBashOutput = ""
			m.renderMessages()
			m.autoScroll()
			cmds = append(cmds, waitForStream(m.activeStream))
		} else if msg.Type == "tool_call" {
			m.currentBashOutput = ""
			ev := buildToolCallEvent(msg.ToolName, msg.ToolArgs)
			m.toolEvents = append(m.toolEvents, ev)
			m.pushToolLog("call", ev.Title)
			m.pushInternalLog("call", ev.Title, fmt.Sprintf("%v", msg.ToolArgs))
			m.renderMessages()
			m.autoScroll()
			cmds = append(cmds, waitForStream(m.activeStream))
		} else if msg.Type == "tool_result" {
			m.currentBashOutput = ""
			ev := buildToolResultEvent(msg.ToolName, msg.ToolResult)
			m.toolEvents = append(m.toolEvents, ev)
			m.pushToolLog("result", ev.Title)
			m.pushInternalLog("result", ev.Title, fmt.Sprintf("%v", msg.ToolResult))
			m.renderMessages()
			m.autoScroll()
			cmds = append(cmds, waitForStream(m.activeStream))
		} else if msg.Type == "approval_request" {
			m.messages = append(m.messages, ChatMessage{Role: "System", Content: msg.Content})
			m.renderMessages()
			m.autoScroll()
			cmds = append(cmds, waitForStream(m.activeStream))
		} else if msg.Type == "error" {
			m.thinking = false
			m.clearThinkingIndicator()

			// Append the partial response before the error so it's not lost
			if strings.TrimSpace(m.currentResponse) != "" || len(m.toolEvents) > 0 {
				expanded := false
				if strings.TrimSpace(m.currentResponse) == "" && len(m.toolEvents) > 0 {
					expanded = true
				}
				m.messages = append(m.messages, ChatMessage{
					Role:       "Mairu",
					Content:    m.currentResponse,
					ToolEvents: m.toolEvents,
					Expanded:   expanded,
				})
			}
			m.currentResponse = ""
			m.currentBashOutput = ""
			m.toolEvents = nil

			m.messages = append(m.messages, ChatMessage{Role: "Error", Content: msg.Content})
			m.activeStream = nil
			m.pushInternalLog("error", "Agent stream error", msg.Content)
			m.renderMessages()
			m.autoScroll()
		}

	case errMsg:
		m.err = msg
		return m, nil
	case externalToolDoneMsg:
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

	// Dynamic resizing of textarea based on content
	lines := strings.Count(m.textarea.Value(), "\n") + 1
	if lines > 5 {
		lines = 5
	}
	if m.textarea.Height() != lines {
		m.textarea.SetHeight(lines)
		m.recomputeLayout()
		m.autoScroll()
	}

	return m, tea.Batch(cmds...)
}

func (m *model) openWorkspacePane(p workspacePane) tea.Cmd {
	m.activePane = p
	if p == paneAgent {
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Agent pane focused."})
		m.renderMessages()
		m.autoScroll()
		return nil
	}
	if m.thinking {
		m.activePane = paneAgent
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Wait for the current stream to finish before opening a workspace pane."})
		m.renderMessages()
		m.autoScroll()
		return nil
	}

	bin, args, ok := paneCommandSpec(p)
	if !ok {
		m.activePane = paneAgent
		return nil
	}
	if _, err := exec.LookPath(bin); err != nil {
		m.activePane = paneAgent
		m.messages = append(m.messages, ChatMessage{Role: "Error", Content: fmt.Sprintf("`%s` is not installed or not on PATH.", bin)})
		m.renderMessages()
		m.autoScroll()
		return nil
	}

	label := paneLabel(p)
	m.messages = append(m.messages, ChatMessage{
		Role:    "System",
		Content: fmt.Sprintf("Opening %s pane. Exit %s to return to Mairu.", label, label),
	})
	m.renderMessages()
	m.autoScroll()

	cmd := exec.Command(bin, args...)
	if m.agent != nil && m.agent.GetRoot() != "" {
		cmd.Dir = m.agent.GetRoot()
	}

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return externalToolDoneMsg{pane: p, err: err}
	})
}
