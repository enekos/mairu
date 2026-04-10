package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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
			showEvents := msg.Expanded || (m.sidebarMode == "explore" && m.selectedMessage == idx)
			if showEvents && len(msg.ToolEvents) > 0 {
				var toolSb strings.Builder
				toolSb.WriteString("\n")
				for eIdx, e := range msg.ToolEvents {
					isFocused := m.sidebarMode == "explore" && m.selectedMessage == idx && m.selectedEvent == eIdx
					if msg.Expanded || isFocused {
						evStr := renderExpandedToolEventBox(e)
						if isFocused {
							evStr = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, false, true).BorderForeground(lipgloss.Color("#88c0d0")).PaddingLeft(1).Render(evStr)
						}
						toolSb.WriteString(evStr + "\n")
					} else {
						toolSb.WriteString(renderToolEventBox(e) + "\n")
					}
				}
				chunk += toolSb.String()
			}
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
		if len(m.toolEvents) > 0 {
			for _, e := range m.toolEvents {
				eventChunk := renderToolEventBox(e) + "\n"
				sb.WriteString(eventChunk)
			}
			sb.WriteString("\n")
		}
		if m.currentBashOutput != "" {
			bashChunk := toolStatusBoxStyle.Render(toolStatusTitleStyle.Render("🖥️ Running Bash...")+"\n\n"+m.currentBashOutput) + "\n"
			sb.WriteString(bashChunk)
		}
	}

	m.messageLineStart = starts
	if m.selectedMessage < 0 && len(m.messages) > 0 {
		m.selectedMessage = len(m.messages) - 1
	}
	m.viewport.SetContent(sb.String())
}

func (m model) renderSessionTabs() string {
	if len(m.sessions) == 0 {
		return ""
	}
	var parts []string
	for _, s := range m.sessions {
		if s == m.sessionName {
			parts = append(parts, sessionTabActiveStyle.Render(" "+s+" "))
		} else {
			parts = append(parts, sessionTabStyle.Render(" "+s+" "))
		}
	}
	// Add a little padding to the right of the tabs so it doesn't look cut off
	return lipgloss.NewStyle().MarginBottom(1).Render(lipgloss.JoinHorizontal(lipgloss.Left, parts...)) + "\n"
}

func (m model) View() string {
	if m.showAnim {
		return m.renderAnimation()
	}
	if m.showList {
		return m.listModel.View()
	}
	if m.showGraph {
		return m.dataExplorer.View()
	}

	statusStr := ""
	if m.thinking {
		glyph := m.thinkingGlyph
		if glyph == "" {
			glyph = m.spinner.View()
		}
		statusStr = thinkingGlyphStyle.Render(glyph) + " " + thinkingPhraseStyle.Render(m.thinkingPhrase) + " "
	}

	inputView := statusStr + m.textarea.View()

	if m.chatPaneWidth == 0 || m.sidebarWidth == 0 || m.panesHeight == 0 {
		m.recomputeLayout()
	}

	chatPane := chatPaneStyle.
		Width(m.chatPaneWidth).
		Height(m.panesHeight).
		Render(m.renderPaneTabs() + "\n" + m.viewport.View())
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
		"%s\n%s\n%s%s\n%s",
		appStyle.Render(m.renderSessionTabs()),
		appStyle.Render(mainRow),
		acView,
		appStyle.Render(inputView),
		appStyle.Render(m.renderFooter()),
	)
	return viewStr
}

func (m model) renderFooter() string {
	footer := "PgUp/PgDn scroll  ·  Home/End top-bottom  ·  Ctrl+F follow  ·  Ctrl+E sidebars  ·  Ctrl+O next tab  ·  Ctrl+N nvim  ·  Ctrl+G lazygit  ·  /help"
	return footerStyle.Render(footer)
}

func (m model) renderPaneTabs() string {
	panes := []workspacePane{paneAgent, paneNvim, paneLazygit}
	parts := make([]string, 0, len(panes))
	for _, pane := range panes {
		label := paneLabel(pane)
		if pane == m.activePane {
			parts = append(parts, paneTabActiveStyle.Render(label))
		} else {
			parts = append(parts, paneTabStyle.Render(label))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, parts...)
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

func (m model) renderAnimation() string {
	width, height := m.width, m.height
	if width <= 0 || height <= 0 {
		return ""
	}
	chars := []rune(" 010101¦|/:=+")
	var sb strings.Builder

	// t goes from 0.0 to 1.0 over the ~400ms
	t := float64(m.animFrame) / 20.0

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Crazy noise that changes significantly with x, y, and t
			crazyNoise := math.Sin(float64(x)*0.5+t*10.0) * math.Cos(float64(y)*0.8-t*5.0)

			// t goes 0 to 1. Wave propagates downwards but with crazy edges
			wave := t*1.5 - float64(y)/float64(height) + crazyNoise*0.3

			idx := 0
			// wave > 0 means the wave has reached this point
			// wave < 0.8 means it's the "tail" of the wave
			if wave > 0 && wave < 0.8 {
				// Characters change constantly
				randVal := math.Sin(float64(x*17 + y*31 + m.animFrame*43))
				charIdx := int((randVal+1.0)*0.5*float64(len(chars)-1)) + 1

				// Fade out the tail
				if wave > 0.6 {
					charIdx = charIdx / 3
				} else if wave > 0.4 {
					charIdx = charIdx / 2
				}

				idx = charIdx
			}

			if idx < 0 {
				idx = 0
			}
			if idx >= len(chars) {
				idx = len(chars) - 1
			}
			sb.WriteRune(chars[idx])
		}
		if y < height-1 {
			sb.WriteString("\n")
		}
	}
	return lipgloss.NewStyle().Foreground(colorTool).Render(sb.String())
}
