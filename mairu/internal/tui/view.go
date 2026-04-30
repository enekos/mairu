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
			head := rolePillSystem.Render(" ◇ system ") + " " + sidebarLabelStyle.Render(header)
			body := systemAccentBar.Render(strings.TrimRight(rendered, "\n"))
			chunk = head + "\n" + body + "\n\n"
		case "You":
			head := rolePillUser.Render(" ○ you ") + " " + sidebarLabelStyle.Render(header)
			body := userAccentBar.Render(msg.Content)
			chunk = head + "\n" + body + "\n\n"
		case "Error":
			head := rolePillError.Render(" ✗ error ") + " " + sidebarLabelStyle.Render(header)
			body := errorAccentBar.Render(errorStyle.Render(strings.TrimRight(msg.Content, "\n")))
			chunk = head + "\n" + body + "\n\n"
		case "Diff":
			rendered, _ := m.mdRenderer.Render(msg.Content)
			head := lipgloss.NewStyle().Background(colorInfo).Foreground(lipgloss.Color("#19181a")).Bold(true).Padding(0, 1).Render(" ⎇ diff ") + " " + sidebarLabelStyle.Render(header)
			chunk = head + "\n" + rendered + "\n"
		default:
			rendered, _ := m.mdRenderer.Render(msg.Content)
			// Static gradient label for completed agent messages.
			roleLabel := gradientText(" ● mairu ", agentGradient, 0, true)
			head := rolePillAgent.Render(roleLabel) + " " + sidebarLabelStyle.Render(header)
			body := agentAccentBar.Render(strings.TrimRight(rendered, "\n"))
			chunk = head + "\n" + body + "\n\n"
			showEvents := msg.Expanded || (m.sidebarMode == "explore" && m.selectedMessage == idx)
			if showEvents && len(msg.ToolEvents) > 0 {
				var toolSb strings.Builder
				toolSb.WriteString("\n")
				for eIdx, e := range msg.ToolEvents {
					isFocused := m.sidebarMode == "explore" && m.selectedMessage == idx && m.selectedEvent == eIdx
					if msg.Expanded || isFocused {
						evStr := renderExpandedToolEventBox(e)
						if isFocused {
							evStr = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, false, true).BorderForeground(colorPrompt).PaddingLeft(1).Render(evStr)
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
		// Animated streaming label: rainbow gradient that sweeps with animFrame.
		phase := math.Mod(float64(m.animFrame)*0.02, 1.0)
		roleLabel := gradientText(" ● mairu ", []string{"#ff6188", "#fc9867", "#ffd866", "#a9dc76", "#78dce8", "#ab9df2"}, phase, true)
		streamingTag := lipgloss.NewStyle().Foreground(pulseColor(m.animFrame)).Italic(true).Render(" streaming…")
		var chunk string
		if m.currentResponse != "" {
			rendered, _ := m.mdRenderer.Render(m.currentResponse)
			head := rolePillAgent.Render(roleLabel) + streamingTag
			body := agentAccentBar.BorderForeground(pulseColor(m.animFrame)).Render(strings.TrimRight(rendered, "\n"))
			chunk = head + "\n" + body + "\n"
		} else {
			head := rolePillAgent.Render(roleLabel) + streamingTag
			chunk = head + "\n\n"
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
		return m.renderSplash()
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

	cps := chatPaneStyle.
		Width(m.chatPaneWidth).
		Height(m.panesHeight)
	if m.thinking {
		cps = cps.BorderForeground(streamingBorderColor(m.animFrame))
	}
	chatPane := cps.Render(m.renderPaneTabs() + "\n" + m.viewport.View())
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
		"%s\n%s\n%s\n%s%s\n%s",
		m.renderStatusBar(),
		appStyle.Render(m.renderSessionTabs()),
		appStyle.Render(mainRow),
		acView,
		appStyle.Render(inputView),
		m.renderRichFooter(),
	)
	return viewStr
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
