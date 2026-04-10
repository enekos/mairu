package tui

import "github.com/charmbracelet/lipgloss"

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

	sidebarHeaderStyle  = lipgloss.NewStyle().Foreground(colorPrompt).Bold(true)
	sidebarLabelStyle   = lipgloss.NewStyle().Foreground(colorSystem)
	footerStyle         = lipgloss.NewStyle().Foreground(colorSystem).Italic(true)
	thinkingGlyphStyle  = lipgloss.NewStyle().Foreground(colorAgent).Bold(true)
	thinkingPhraseStyle = lipgloss.NewStyle().
				Foreground(colorTool).
				Italic(true)
	paneTabActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#2e3440")).
				Background(colorPrompt).
				Bold(true).
				Padding(0, 1)
	paneTabStyle = lipgloss.NewStyle().
			Foreground(colorSystem).
			Background(lipgloss.Color("#2b303b")).
			Padding(0, 1)
	sessionTabActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#2e3440")).
				Background(colorAgent).
				Bold(true).
				Padding(0, 1)
	sessionTabStyle = lipgloss.NewStyle().
			Foreground(colorSystem).
			Background(lipgloss.Color("#2b303b")).
			Padding(0, 1)
)
