package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorUser   = lipgloss.Color("#78dce8") // cyan
	colorAgent  = lipgloss.Color("#ab9df2") // violet
	colorSystem = lipgloss.Color("#727072") // muted gray
	colorTool   = lipgloss.Color("#a9dc76") // green
	colorError  = lipgloss.Color("#ff6188") // pink
	colorPrompt = lipgloss.Color("#ffd866") // yellow
	colorInfo   = lipgloss.Color("#fc9867") // orange

	appStyle    = lipgloss.NewStyle().Padding(0, 1)
	userStyle   = lipgloss.NewStyle().Foreground(colorUser).Bold(true)
	agentStyle  = lipgloss.NewStyle().Foreground(colorAgent).Bold(true)
	systemStyle = lipgloss.NewStyle().Foreground(colorSystem).Italic(true)
	errorStyle  = lipgloss.NewStyle().Foreground(colorError).Bold(true)

	// Left accent bars on chat messages — narrow vertical bar on the left edge.
	agentAccentBar = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder(), false, false, false, true).
			BorderForeground(colorAgent).
			PaddingLeft(1)
	userAccentBar = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder(), false, false, false, true).
			BorderForeground(colorUser).
			PaddingLeft(1)
	systemAccentBar = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder(), false, false, false, true).
			BorderForeground(colorSystem).
			PaddingLeft(1)
	errorAccentBar = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder(), false, false, false, true).
			BorderForeground(colorError).
			PaddingLeft(1)

	// Pill / badge styles for role headers.
	rolePillUser = lipgloss.NewStyle().
			Background(colorUser).
			Foreground(lipgloss.Color("#1a1d23")).
			Bold(true).
			Padding(0, 1)
	rolePillAgent = lipgloss.NewStyle().
			Background(colorAgent).
			Foreground(lipgloss.Color("#1a1d23")).
			Bold(true).
			Padding(0, 1)
	rolePillSystem = lipgloss.NewStyle().
			Background(lipgloss.Color("#3b3a3c")).
			Foreground(lipgloss.Color("#fcfcfa")).
			Italic(true).
			Padding(0, 1)
	rolePillError = lipgloss.NewStyle().
			Background(colorError).
			Foreground(lipgloss.Color("#1a1d23")).
			Bold(true).
			Padding(0, 1)

	toolCallBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorInfo).
				Background(lipgloss.Color("#221f1a")).
				Padding(0, 1).
				MarginTop(0).
				MarginBottom(1)
	toolResultBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorTool).
				Background(lipgloss.Color("#1c2418")).
				Padding(0, 1).
				MarginBottom(1)
	toolStatusBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPrompt).
				Background(lipgloss.Color("#22201a")).
				Padding(0, 1).
				MarginBottom(1)
	toolCallTitleStyle   = lipgloss.NewStyle().Foreground(colorInfo).Bold(true)
	toolResultTitleStyle = lipgloss.NewStyle().Foreground(colorTool).Bold(true)
	toolStatusTitleStyle = lipgloss.NewStyle().Foreground(colorPrompt).Bold(true)
	toolFieldKeyStyle    = lipgloss.NewStyle().Foreground(colorPrompt).Bold(true)
	toolFieldValueStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#fcfcfa"))

	chatPaneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorUser).
			Padding(0, 1)

	sidebarPaneStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorAgent).
				Padding(0, 1)

	sidebarHeaderStyle  = lipgloss.NewStyle().Foreground(colorPrompt).Bold(true)
	sidebarLabelStyle   = lipgloss.NewStyle().Foreground(colorSystem)
	footerStyle         = lipgloss.NewStyle().Foreground(colorSystem).Italic(true)
	thinkingGlyphStyle  = lipgloss.NewStyle().Foreground(colorAgent).Bold(true)
	thinkingPhraseStyle = lipgloss.NewStyle().
				Foreground(colorTool).
				Italic(true)
	paneTabActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#19181a")).
				Background(colorPrompt).
				Bold(true).
				Padding(0, 1)
	paneTabStyle = lipgloss.NewStyle().
			Foreground(colorSystem).
			Background(lipgloss.Color("#2d2a2e")).
			Padding(0, 1)
	sessionTabActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#19181a")).
				Background(colorAgent).
				Bold(true).
				Padding(0, 1)
	sessionTabStyle = lipgloss.NewStyle().
			Foreground(colorSystem).
			Background(lipgloss.Color("#2d2a2e")).
			Padding(0, 1)
)

// toolKindIcon returns a per-kind glyph used in tool event headers.
func toolKindIcon(kind string) string {
	switch kind {
	case "call":
		return "⚙"
	case "result":
		return "✓"
	default:
		return "◆"
	}
}
