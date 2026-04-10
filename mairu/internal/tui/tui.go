package tui

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

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
	Role       string
	Content    string
	ToolEvents []toolEvent
	Expanded   bool
}

type internalLogEntry struct {
	Timestamp time.Time
	Kind      string
	Summary   string
	Detail    string
}

type listItem struct {
	title, desc string
}

func (i listItem) Title() string       { return i.title }
func (i listItem) Description() string { return i.desc }
func (i listItem) FilterValue() string { return i.title }

type model struct {
	showAnim  bool
	animFrame int
	viewport  viewport.Model
	messages  []ChatMessage
	textarea  textarea.Model
	err       error
	agent     *agent.Agent
	thinking  bool
	spinner   spinner.Model
	rng       *rand.Rand

	thinkingGlyph      string
	thinkingPhrase     string
	nextPhraseSwitchAt time.Time

	currentResponse   string
	currentBashOutput string
	toolEvents        []toolEvent
	mdRenderer        *glamour.TermRenderer
	activeStream      chan agent.AgentEvent

	sessionName    string
	sessions       []string
	queuedMessages []string

	showList  bool
	listModel list.Model
	listType  string // "session" or "model"

	showGraph    bool
	dataExplorer dataExplorerModel

	width  int
	height int

	chatPaneWidth int
	sidebarWidth  int
	panesHeight   int

	followMode       bool
	sidebarMode      string // "session", "explore", or "logs"
	selectedMessage  int
	selectedEvent    int
	messageLineStart []int
	toolLog          []string
	internalLogs     []internalLogEntry
	selectedLog      int

	autocompleteIndex int
	filteredCommands  []SlashCommand

	activePane workspacePane
}

type animTickMsg time.Time

type agentStreamMsg agent.AgentEvent

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

	if sessionName == "" {
		sessionName = "default"
	}

	sessions, _ := a.ListSessions()
	found := false
	for _, s := range sessions {
		if s == sessionName {
			found = true
			break
		}
	}
	if !found {
		sessions = append(sessions, sessionName)
	}

	m := model{
		showAnim:        true,
		animFrame:       0,
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
		selectedEvent:   -1,
		rng:             rand.New(rand.NewSource(time.Now().UnixNano())),
		activePane:      paneAgent,
		sessionName:     sessionName,
		sessions:        sessions,
	}
	m.refreshThinkingIndicator(time.Now(), true)
	m.renderMessages()
	return m
}

func Start(a *agent.Agent, sessionName string) error {
	p := tea.NewProgram(initialModel(a, sessionName), tea.WithAltScreen(), tea.WithMouseAllMotion())
	_, err := p.Run()
	return err
}

func tickAnim() tea.Cmd {
	return tea.Tick(time.Millisecond*20, func(t time.Time) tea.Msg {
		return animTickMsg(t)
	})
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick, tickAnim())
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
	sessionTabsHeight := 0
	if len(m.sessions) > 0 {
		sessionTabsHeight = 2
	}
	panesHeight := m.height - taHeight - 3 - sessionTabsHeight
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
	viewportHeight := panesHeight - 4 // Reserve rows for pane tabs and breathing room.
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

func (m *model) pushInternalLog(kind, summary, detail string) {
	entry := internalLogEntry{
		Timestamp: time.Now(),
		Kind:      strings.TrimSpace(kind),
		Summary:   strings.TrimSpace(summary),
		Detail:    strings.TrimSpace(detail),
	}
	m.internalLogs = append(m.internalLogs, entry)
	if len(m.internalLogs) > 1000 {
		m.internalLogs = m.internalLogs[len(m.internalLogs)-1000:]
	}
	m.selectedLog = len(m.internalLogs) - 1
}
