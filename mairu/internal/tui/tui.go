package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"mairu/internal/agent"
)

type errMsg error

type ChatMessage struct {
	Role    string
	Content string
}

type listItem struct {
	title, desc string
}

func (i listItem) Title() string       { return i.title }
func (i listItem) Description() string { return i.desc }
func (i listItem) FilterValue() string { return i.title }

type model struct {
	viewport viewport.Model
	messages []ChatMessage
	textarea textarea.Model
	err      error
	agent    *agent.Agent
	thinking bool
	spinner  spinner.Model

	currentResponse string
	toolEvents      []toolEvent
	mdRenderer      *glamour.TermRenderer
	activeStream    chan agent.AgentEvent

	showList  bool
	listModel list.Model
	listType  string // "session" or "model"

	width  int
	height int

	chatPaneWidth int
	sidebarWidth  int
	panesHeight   int

	followMode       bool
	sidebarMode      string // "session" or "explore"
	selectedMessage  int
	messageLineStart []int
	toolLog          []string

	autocompleteIndex int
	filteredCommands  []SlashCommand
}

type agentStreamMsg agent.AgentEvent

type SlashCommand struct {
	Name        string
	Description string
}

type toolEvent struct {
	Kind  string
	Title string
	Lines []string
}

var allSlashCommands = []SlashCommand{
	{"/help", "Show this help message"},
	{"/clear", "Clear the terminal screen"},
	{"/copy", "Copy last response to clipboard"},
	{"/models", "Open model selector"},
	{"/model", "Switch to a specific model"},
	{"/sessions", "Open session selector"},
	{"/session", "Load a specific session"},
	{"/memory search", "Search contextfs memory"},
	{"/memory read", "Read contextfs memory"},
	{"/memory write", "Write fact to contextfs memory"},
	{"/memory store", "Store fact in contextfs memory"},
	{"/node search", "Search contextfs nodes"},
	{"/node read", "Read contextfs nodes"},
	{"/node ls", "List contextfs node children"},
	{"/node store", "Store/update a contextfs node"},
	{"/node write", "Write/update a contextfs node"},
	{"/vibe", "Run contextfs vibe-query"},
	{"/remember", "Run contextfs vibe-mutation"},
	{"/save", "Save the current session"},
	{"/fork", "Fork the current session to a new name"},
	{"/reset", "Start a fresh session"},
	{"/new", "Start a fresh session"},
	{"/compact", "Summarize history to save tokens"},
	{"/squash", "Summarize history to save tokens"},
	{"/export", "Export conversation to a file"},
	{"/explore", "Toggle explore sidebar"},
	{"/jump", "Jump to message number n"},
	{"/exit", "Exit Mairu"},
	{"/quit", "Exit Mairu"},
}

var (
	colorUser   = lipgloss.Color("#5e81ac") // Nord blue
	colorAgent  = lipgloss.Color("#b48ead") // Nord purple
	colorSystem = lipgloss.Color("#4c566a") // Nord dark gray
	colorTool   = lipgloss.Color("#a3be8c") // Nord green
	colorError  = lipgloss.Color("#bf616a") // Nord red
	colorPrompt = lipgloss.Color("#88c0d0") // Nord light blue

	appStyle    = lipgloss.NewStyle().Padding(0, 1)
	userStyle   = lipgloss.NewStyle().Foreground(colorUser).Bold(true)
	agentStyle  = lipgloss.NewStyle().Foreground(colorAgent).Bold(true)
	systemStyle = lipgloss.NewStyle().Foreground(colorSystem).Italic(true)
	toolStyle   = lipgloss.NewStyle().Foreground(colorTool).Italic(true)
	errorStyle  = lipgloss.NewStyle().Foreground(colorError).Bold(true)

	toolCallBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorTool).
				Background(lipgloss.Color("#1f2a22")).
				Padding(0, 1).
				MarginTop(0).
				MarginBottom(1)
	toolResultBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPrompt).
				Background(lipgloss.Color("#1c2630")).
				Padding(0, 1).
				MarginBottom(1)
	toolStatusBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(colorSystem).
				Background(lipgloss.Color("#1f2125")).
				Padding(0, 1).
				MarginBottom(1)
	toolCallTitleStyle   = lipgloss.NewStyle().Foreground(colorTool).Bold(true)
	toolResultTitleStyle = lipgloss.NewStyle().Foreground(colorPrompt).Bold(true)
	toolStatusTitleStyle = lipgloss.NewStyle().Foreground(colorSystem).Bold(true)
	toolFieldKeyStyle    = lipgloss.NewStyle().Foreground(colorPrompt).Bold(true)
	toolFieldValueStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#d8dee9"))

	chatPaneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorUser).
			Padding(0, 1)

	sidebarPaneStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorSystem).
				Padding(0, 1)

	sidebarHeaderStyle = lipgloss.NewStyle().Foreground(colorPrompt).Bold(true)
	sidebarLabelStyle  = lipgloss.NewStyle().Foreground(colorSystem)
	footerStyle        = lipgloss.NewStyle().Foreground(colorSystem).Italic(true)
)

func initialModel(a *agent.Agent, sessionName string) model {
	ta := textarea.New()
	ta.Placeholder = "Type a message or /help for commands..."
	ta.Focus()
	ta.Prompt = "╰─○ "
	ta.CharLimit = 10000
	ta.SetWidth(100)
	ta.SetHeight(1)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)
	ta.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(colorPrompt).Bold(true)
	ta.FocusedStyle.Text = lipgloss.NewStyle()

	vp := viewport.New(100, 20)
	vp.SetContent("Mairu Agent initialized.\n")

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(colorAgent)

	r, _ := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(95),
	)

	var msgs []ChatMessage

	if sessionName != "" {
		msgs = append(msgs, ChatMessage{Role: "System", Content: fmt.Sprintf("Loaded session: %s", sessionName)})
	}
	for _, text := range a.GetHistoryText() {
		if strings.HasPrefix(text, "You: ") {
			msgs = append(msgs, ChatMessage{Role: "You", Content: strings.TrimPrefix(text, "You: ")})
		} else if strings.HasPrefix(text, "Mairu: ") {
			msgs = append(msgs, ChatMessage{Role: "Mairu", Content: strings.TrimPrefix(text, "Mairu: ")})
		}
	}

	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.SetShowTitle(true)

	m := model{
		textarea:        ta,
		messages:        msgs,
		viewport:        vp,
		err:             nil,
		agent:           a,
		spinner:         s,
		mdRenderer:      r,
		listModel:       l,
		followMode:      true,
		sidebarMode:     "session",
		selectedMessage: -1,
	}
	m.renderMessages()
	return m
}

func Start(a *agent.Agent, sessionName string) error {
	p := tea.NewProgram(initialModel(a, sessionName), tea.WithAltScreen(), tea.WithMouseAllMotion())
	_, err := p.Run()
	return err
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick)
}

func waitForStream(ch <-chan agent.AgentEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return agentStreamMsg{Type: "done"}
		}
		return agentStreamMsg(ev)
	}
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
						m.agent.SaveSession("current")
						err := m.agent.LoadSession(sessionName)
						if err != nil {
							m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Failed to load session: " + err.Error()})
						} else {
							m.messages = []ChatMessage{{Role: "System", Content: "Loaded session: " + sessionName}}
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
		case tea.KeyCtrlC, tea.KeyCtrlD:
			return m, tea.Quit
		case tea.KeyPgUp:
			m.viewport.HalfViewUp()
			m.followMode = false
			return m, nil
		case tea.KeyPgDown:
			m.viewport.HalfViewDown()
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
			if m.sidebarMode == "session" {
				m.sidebarMode = "explore"
				m.selectedMessage = clampMessageIndex(len(m.messages)-1, 0, len(m.messages))
			} else {
				m.sidebarMode = "session"
			}
			return m, nil
		case tea.KeyCtrlJ:
			if m.sidebarMode == "explore" {
				m.selectedMessage = clampMessageIndex(m.selectedMessage, 1, len(m.messages))
				m.jumpToSelectedMessage()
				m.followMode = false
				return m, nil
			}
		case tea.KeyCtrlK:
			if m.sidebarMode == "explore" {
				m.selectedMessage = clampMessageIndex(m.selectedMessage, -1, len(m.messages))
				m.jumpToSelectedMessage()
				m.followMode = false
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
		case tea.KeyEnter:
			if msg.Alt {
				m.textarea.InsertString("\n")
				return m, nil
			}
			if m.thinking {
				return m, nil
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

				out, err := m.agent.RunBash(cmdStr, 60000)
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

				out, err := m.agent.RunBash(cmdStr, 60000)
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
- /explore: Toggle explore sidebar (message navigator + tool drilldown)
- /jump <n>: Jump to message number n
- /exit or /quit: Exit Mairu

**Navigation**
- PgUp/PgDown: Scroll chat by half-page
- Home/End: Jump top/bottom
- Ctrl+F: Toggle follow mode during streaming
- Ctrl+E: Toggle session/explore sidebar
- Ctrl+J / Ctrl+K: Next/previous message (explore mode)`
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
							m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Forked session to: " + newName})
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
							m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Session saved as: " + sessionName})
						}
					} else {
						m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Usage: /save <name>"})
					}
					m.renderMessages()
					m.viewport.GotoBottom()
					return m, nil
				case "/reset", "/new":
					m.agent.ResetSession()
					m.messages = []ChatMessage{{Role: "System", Content: "Session reset. Starting fresh context."}}
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
				case "/explore":
					if m.sidebarMode == "session" {
						m.sidebarMode = "explore"
						m.selectedMessage = clampMessageIndex(len(m.messages)-1, 0, len(m.messages))
						m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Explore sidebar enabled. Use Ctrl+J / Ctrl+K to jump between messages."})
					} else {
						m.sidebarMode = "session"
						m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Explore sidebar hidden."})
					}
					m.renderMessages()
					m.autoScroll()
					return m, nil
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
						m.agent.SaveSession("current")
						err := m.agent.LoadSession(sessionName)
						if err != nil {
							m.messages = append(m.messages, ChatMessage{Role: "Error", Content: "Failed to load session: " + err.Error()})
						} else {
							m.messages = []ChatMessage{{Role: "System", Content: "Loaded session: " + sessionName}}
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
			m.renderMessages()
			m.autoScroll()

			m.thinking = true
			m.currentResponse = ""
			m.toolEvents = nil

			m.activeStream = make(chan agent.AgentEvent)
			go m.agent.RunStream(v, m.activeStream)

			cmds = append(cmds, waitForStream(m.activeStream))
		}

	case agentStreamMsg:
		if msg.Type == "done" {
			m.thinking = false
			m.messages = append(m.messages, ChatMessage{Role: "Mairu", Content: m.currentResponse})
			m.currentResponse = ""
			m.toolEvents = nil
			m.activeStream = nil
			m.renderMessages()
			m.autoScroll()
		} else if msg.Type == "text" {
			m.currentResponse += msg.Content
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
			// Don't show log events in main chat, just background sidebar
			cmds = append(cmds, waitForStream(m.activeStream))
		} else if msg.Type == "status" {
			ev := buildToolStatusEvent(msg.Content)
			m.toolEvents = append(m.toolEvents, ev)
			m.pushToolLog("status", ev.Title)
			m.renderMessages()
			m.autoScroll()
			cmds = append(cmds, waitForStream(m.activeStream))
		} else if msg.Type == "tool_call" {
			ev := buildToolCallEvent(msg.ToolName, msg.ToolArgs)
			m.toolEvents = append(m.toolEvents, ev)
			m.pushToolLog("call", ev.Title)
			m.renderMessages()
			m.autoScroll()
			cmds = append(cmds, waitForStream(m.activeStream))
		} else if msg.Type == "tool_result" {
			ev := buildToolResultEvent(msg.ToolName, msg.ToolResult)
			m.toolEvents = append(m.toolEvents, ev)
			m.pushToolLog("result", ev.Title)
			m.renderMessages()
			m.autoScroll()
			cmds = append(cmds, waitForStream(m.activeStream))
		} else if msg.Type == "error" {
			m.thinking = false
			m.messages = append(m.messages, ChatMessage{Role: "Error", Content: msg.Content})
			m.activeStream = nil
			m.renderMessages()
			m.autoScroll()
		}

	case errMsg:
		m.err = msg
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

func (m *model) renderMessages() {
	var sb strings.Builder
	starts := make([]int, len(m.messages))
	line := 0

	for idx, msg := range m.messages {
		starts[idx] = line
		header := fmt.Sprintf("#%d", idx+1)
		var chunk string

		switch msg.Role {
		case "System":
			rendered, _ := m.mdRenderer.Render(msg.Content)
			chunk = systemStyle.Render("◇ System "+header) + "\n" + systemStyle.Render(rendered) + "\n"
		case "You":
			chunk = userStyle.Render("○ You "+header) + "\n" + msg.Content + "\n\n"
		case "Error":
			chunk = errorStyle.Render("✗ Error "+header) + "\n" + msg.Content + "\n\n"
		case "Diff":
			rendered, _ := m.mdRenderer.Render(msg.Content)
			chunk = sidebarLabelStyle.Render("⎇ Diff "+header) + "\n" + rendered + "\n"
		default:
			rendered, _ := m.mdRenderer.Render(msg.Content)
			chunk = agentStyle.Render("● Mairu "+header) + "\n" + rendered + "\n"
		}

		sb.WriteString(chunk)
		line += strings.Count(chunk, "\n")
	}

	if m.thinking {
		var chunk string
		if m.currentResponse != "" {
			rendered, _ := m.mdRenderer.Render(m.currentResponse)
			chunk = agentStyle.Render("● Mairu (streaming)") + "\n" + rendered + "\n"
		} else {
			chunk = agentStyle.Render("● Mairu (streaming)") + "\n\n"
		}
		sb.WriteString(chunk)
		line += strings.Count(chunk, "\n")
		if len(m.toolEvents) > 0 {
			for _, e := range m.toolEvents {
				eventChunk := renderToolEventBox(e) + "\n"
				sb.WriteString(eventChunk)
				line += strings.Count(eventChunk, "\n")
			}
			sb.WriteString("\n")
			line++
		}
	}

	m.messageLineStart = starts
	if m.selectedMessage < 0 && len(m.messages) > 0 {
		m.selectedMessage = len(m.messages) - 1
	}
	m.viewport.SetContent(sb.String())
}

func (m model) View() string {
	if m.showList {
		return m.listModel.View()
	}

	statusStr := ""
	if m.thinking {
		statusStr = m.spinner.View() + " "
	}

	inputView := statusStr + m.textarea.View()

	if m.chatPaneWidth == 0 || m.sidebarWidth == 0 || m.panesHeight == 0 {
		m.recomputeLayout()
	}

	chatPane := chatPaneStyle.
		Width(m.chatPaneWidth).
		Height(m.panesHeight).
		Render(m.viewport.View())
	mainRow := chatPane
	if m.sidebarWidth > 0 {
		sidebarPane := sidebarPaneStyle.
			Width(m.sidebarWidth).
			Height(m.panesHeight).
			Render(m.renderSidebar())
		mainRow = lipgloss.JoinHorizontal(lipgloss.Top, chatPane, sidebarPane)
	}

	acView := m.renderAutocomplete()
	if acView != "" {
		acView = appStyle.Render(acView) + "\n"
	}

	viewStr := fmt.Sprintf(
		"%s\n%s%s\n%s",
		appStyle.Render(mainRow),
		acView,
		appStyle.Render(inputView),
		appStyle.Render(m.renderFooter()),
	)
	return viewStr
}

func (m *model) recomputeLayout() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	availableWidth := m.width - 2
	if availableWidth < 60 {
		availableWidth = m.width
	}

	sidebar := availableWidth / 3
	if sidebar < 30 {
		sidebar = 30
	}
	if sidebar > 46 {
		sidebar = 46
	}
	chatWidth := availableWidth - sidebar - 1
	if chatWidth < 24 {
		chatWidth = availableWidth
		sidebar = 0
	}

	taHeight := m.textarea.Height()
	panesHeight := m.height - taHeight - 3
	if panesHeight < 7 {
		panesHeight = 7
	}

	m.chatPaneWidth = chatWidth
	m.sidebarWidth = sidebar
	m.panesHeight = panesHeight

	viewportWidth := chatWidth - 4
	if viewportWidth < 20 {
		viewportWidth = 20
	}
	viewportHeight := panesHeight - 2
	if viewportHeight < 3 {
		viewportHeight = 3
	}

	m.viewport.Width = viewportWidth
	m.viewport.Height = viewportHeight
	m.textarea.SetWidth(availableWidth)

	wrap := viewportWidth - 2
	if wrap < 20 {
		wrap = 20
	}
	r, _ := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(wrap),
	)
	m.mdRenderer = r
}

func (m model) renderSidebar() string {
	stats := computeSessionStats(m.messages, m.currentResponse, m.toolEvents, m.thinking, m.agent.GetModelName())
	if m.sidebarMode == "explore" {
		return m.renderExploreSidebar(stats)
	}

	var sb strings.Builder
	sb.WriteString(sidebarHeaderStyle.Render("Session"))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%s %s\n", sidebarLabelStyle.Render("Model:"), stats.Model))
	sb.WriteString(fmt.Sprintf("%s %s\n", sidebarLabelStyle.Render("State:"), stats.StreamState))
	sb.WriteString(fmt.Sprintf("%s %d\n", sidebarLabelStyle.Render("Messages:"), len(m.messages)))
	sb.WriteString(fmt.Sprintf("%s U:%d A:%d S:%d E:%d D:%d\n",
		sidebarLabelStyle.Render("By role:"),
		stats.UserMessages,
		stats.AssistantMessages,
		stats.SystemMessages,
		stats.ErrorMessages,
		stats.DiffMessages,
	))
	sb.WriteString("\n")
	sb.WriteString(sidebarHeaderStyle.Render("Tools"))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%s %d\n", sidebarLabelStyle.Render("Events:"), stats.ToolEvents))
	sb.WriteString(fmt.Sprintf("%s %d\n", sidebarLabelStyle.Render("Calls:"), stats.ToolCalls))
	sb.WriteString(fmt.Sprintf("%s %d\n", sidebarLabelStyle.Render("Results:"), stats.ToolResults))
	sb.WriteString("\n")
	sb.WriteString(sidebarHeaderStyle.Render("Token Estimate"))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%s %d\n", sidebarLabelStyle.Render("User:"), stats.EstimatedUserTokens))
	sb.WriteString(fmt.Sprintf("%s %d\n", sidebarLabelStyle.Render("Mairu:"), stats.EstimatedAgentTokens))
	sb.WriteString(fmt.Sprintf("%s %d\n", sidebarLabelStyle.Render("Total:"), stats.EstimatedTotalTokens))

	if len(m.toolEvents) > 0 {
		sb.WriteString("\n\n")
		sb.WriteString(sidebarHeaderStyle.Render("Recent Tool Activity"))
		sb.WriteString("\n")
		start := len(m.toolEvents) - 5
		if start < 0 {
			start = 0
		}
		for _, e := range m.toolEvents[start:] {
			sb.WriteString("• " + previewText(e.Title, 44) + "\n")
		}
	}

	return sb.String()
}

func (m model) renderExploreSidebar(stats sessionStats) string {
	var sb strings.Builder
	sb.WriteString(sidebarHeaderStyle.Render("Explore"))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%s %d / %d\n", sidebarLabelStyle.Render("Selected:"), m.selectedMessage+1, len(m.messages)))
	sb.WriteString(fmt.Sprintf("%s %s\n\n", sidebarLabelStyle.Render("Follow mode:"), boolLabel(m.followMode)))

	sb.WriteString(sidebarHeaderStyle.Render("Messages"))
	sb.WriteString("\n")
	start := len(m.messages) - 12
	if start < 0 {
		start = 0
	}
	for i := start; i < len(m.messages); i++ {
		msg := m.messages[i]
		prefix := " "
		if i == m.selectedMessage {
			prefix = ">"
		}
		sb.WriteString(fmt.Sprintf("%s #%d %-6s %s\n",
			prefix,
			i+1,
			msg.Role,
			previewText(msg.Content, 28),
		))
	}

	sb.WriteString("\n")
	sb.WriteString(sidebarHeaderStyle.Render("Tool Drilldown"))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%s events:%d calls:%d results:%d\n",
		sidebarLabelStyle.Render("Stats:"),
		stats.ToolEvents,
		stats.ToolCalls,
		stats.ToolResults,
	))
	logStart := len(m.toolLog) - 6
	if logStart < 0 {
		logStart = 0
	}
	for _, entry := range m.toolLog[logStart:] {
		sb.WriteString("• " + previewText(entry, 60) + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString(sidebarLabelStyle.Render("Ctrl+J/Ctrl+K navigate  ·  /jump <n>"))
	return sb.String()
}

func (m model) renderFooter() string {
	footer := "PgUp/PgDn scroll  ·  Home/End top-bottom  ·  Ctrl+F follow  ·  Ctrl+E explore  ·  /help"
	return footerStyle.Render(footer)
}

func (m model) renderAutocomplete() string {
	if len(m.filteredCommands) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, cmd := range m.filteredCommands {
		if i > 5 { // Show max 6 items
			sb.WriteString(sidebarLabelStyle.Render(fmt.Sprintf("  ... %d more", len(m.filteredCommands)-6)) + "\n")
			break
		}
		prefix := "  "
		style := sidebarLabelStyle
		if i == m.autocompleteIndex {
			prefix = "> "
			style = sidebarHeaderStyle
		}
		sb.WriteString(style.Render(fmt.Sprintf("%s%-18s %s", prefix, cmd.Name, cmd.Description)) + "\n")
	}
	return sb.String()
}

func (m *model) autoScroll() {
	if m.followMode {
		m.viewport.GotoBottom()
	}
}

func (m *model) jumpToSelectedMessage() {
	if m.selectedMessage < 0 || m.selectedMessage >= len(m.messageLineStart) {
		return
	}
	m.viewport.SetYOffset(m.messageLineStart[m.selectedMessage])
}

func (m *model) pushToolLog(kind, content string) {
	entry := fmt.Sprintf("%s: %s", kind, strings.TrimSpace(content))
	m.toolLog = append(m.toolLog, entry)
	if len(m.toolLog) > 200 {
		m.toolLog = m.toolLog[len(m.toolLog)-200:]
	}
}

func buildToolCallEvent(toolName string, args map[string]any) toolEvent {
	lines := formatToolFields(args, 120)
	if len(lines) == 0 {
		lines = []string{"(no args)"}
	}
	return toolEvent{
		Kind:  "call",
		Title: fmt.Sprintf("Tool call: %s", toolName),
		Lines: lines,
	}
}

func buildToolResultEvent(toolName string, result map[string]any) toolEvent {
	lines := formatToolFields(result, 120)
	if len(lines) == 0 {
		lines = []string{"(no result payload)"}
	}
	return toolEvent{
		Kind:  "result",
		Title: fmt.Sprintf("Tool result: %s", toolName),
		Lines: lines,
	}
}

func buildToolStatusEvent(raw string) toolEvent {
	return toolEvent{
		Kind:  "status",
		Title: sanitizeStatusText(raw),
	}
}

func sanitizeStatusText(raw string) string {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimLeftFunc(cleaned, func(r rune) bool {
		return unicode.IsSymbol(r) || unicode.IsPunct(r) || unicode.IsSpace(r) || unicode.IsMark(r)
	})
	return strings.TrimSpace(cleaned)
}

func formatToolFields(data map[string]any, maxValueLen int) []string {
	if len(data) == 0 {
		return nil
	}
	const maxFields = 8
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for idx, k := range keys {
		if idx >= maxFields {
			remaining := len(keys) - maxFields
			fieldWord := "fields"
			if remaining == 1 {
				fieldWord = "field"
			}
			lines = append(lines, fmt.Sprintf("... and %d more %s", remaining, fieldWord))
			break
		}
		lines = append(lines, fmt.Sprintf("%s: %s", k, previewToolValue(data[k], maxValueLen)))
	}
	return lines
}

func previewToolValue(v any, maxLen int) string {
	s := strings.TrimSpace(strings.ReplaceAll(fmt.Sprintf("%v", v), "\n", " "))
	if maxLen > 3 && len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

func renderToolEventBox(ev toolEvent) string {
	var body strings.Builder
	switch ev.Kind {
	case "call":
		body.WriteString(toolCallTitleStyle.Render(ev.Title))
	case "result":
		body.WriteString(toolResultTitleStyle.Render(ev.Title))
	default:
		body.WriteString(toolStatusTitleStyle.Render(ev.Title))
	}
	if len(ev.Lines) > 0 {
		for _, line := range ev.Lines {
			body.WriteString("\n")
			body.WriteString("  ")
			colon := strings.Index(line, ": ")
			if colon > 0 && !strings.HasPrefix(line, "... and ") {
				key := line[:colon]
				value := line[colon+2:]
				body.WriteString(toolFieldKeyStyle.Render(key + ": "))
				body.WriteString(toolFieldValueStyle.Render(value))
			} else {
				body.WriteString(toolFieldValueStyle.Render(line))
			}
		}
	}

	switch ev.Kind {
	case "call":
		return toolCallBoxStyle.Render(body.String())
	case "result":
		return toolResultBoxStyle.Render(body.String())
	default:
		return toolStatusBoxStyle.Render(body.String())
	}
}

func boolLabel(v bool) string {
	if v {
		return "on"
	}
	return "off"
}
