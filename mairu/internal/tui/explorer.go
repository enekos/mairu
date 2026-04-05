package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

type explorerTab int

const (
	tabContextNodes explorerTab = iota
	tabMemories
	tabSkills
)

type deleteItemMsg struct {
	tab explorerTab
	id  string
}

type dataExplorerModel struct {
	tabs       []string
	activeTab  explorerTab
	lists      []list.Model
	viewport   viewport.Model
	mdRenderer *glamour.TermRenderer
	width      int
	height     int
}

func newDataExplorerModel(contextItems, memoryItems, skillItems []list.Item) dataExplorerModel {
	tabs := []string{"Context Nodes", "Memories", "Skills"}
	lists := make([]list.Model, 3)

	l0 := list.New(contextItems, list.NewDefaultDelegate(), 0, 0)
	l0.Title = "Context Graph Explorer"
	l0.SetShowStatusBar(true)
	l0.SetFilteringEnabled(true)
	l0.DisableQuitKeybindings()
	lists[0] = l0

	l1 := list.New(memoryItems, list.NewDefaultDelegate(), 0, 0)
	l1.Title = "Memories"
	l1.SetShowStatusBar(true)
	l1.SetFilteringEnabled(true)
	l1.DisableQuitKeybindings()
	lists[1] = l1

	l2 := list.New(skillItems, list.NewDefaultDelegate(), 0, 0)
	l2.Title = "Skills"
	l2.SetShowStatusBar(true)
	l2.SetFilteringEnabled(true)
	l2.DisableQuitKeybindings()
	lists[2] = l2

	vp := viewport.New(0, 0)
	r, _ := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(80),
	)

	m := dataExplorerModel{
		tabs:       tabs,
		activeTab:  tabContextNodes,
		lists:      lists,
		viewport:   vp,
		mdRenderer: r,
	}
	m.updateViewportContent()
	return m
}

func (m *dataExplorerModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Tab header height = 3 (padding/border)
	listHeight := height - 3
	leftWidth := width / 2
	if leftWidth > 60 {
		leftWidth = 60
	}
	rightWidth := width - leftWidth - 2

	for i := range m.lists {
		m.lists[i].SetSize(leftWidth, listHeight)
	}

	m.viewport.Width = rightWidth
	m.viewport.Height = listHeight

	r, _ := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(rightWidth-2),
	)
	m.mdRenderer = r
	m.updateViewportContent()
}

func (m *dataExplorerModel) updateViewportContent() {
	selected := m.lists[m.activeTab].SelectedItem()
	if selected == nil {
		m.viewport.SetContent("No item selected.")
		return
	}

	var sb strings.Builder

	switch item := selected.(type) {
	case graphListItem:
		sb.WriteString(fmt.Sprintf("# [%s] %s\n\n", item.node.Project, item.node.Name))
		sb.WriteString(fmt.Sprintf("**URI:** `%s`\n\n", item.uri))
		if item.node.Parent != nil {
			sb.WriteString(fmt.Sprintf("**Parent:** `%s`\n\n", *item.node.Parent))
		}
		sb.WriteString(fmt.Sprintf("## Abstract\n%s\n\n", item.desc))
		if item.content != "" {
			sb.WriteString(fmt.Sprintf("## Content\n%s\n\n", item.content))
		}
		if len(item.node.Children) > 0 {
			sb.WriteString("## Children\n")
			for _, child := range item.node.Children {
				sb.WriteString(fmt.Sprintf("- `%s` (%s)\n", child.URI, child.Name))
			}
		}

	case memoryListItem:
		sb.WriteString(fmt.Sprintf("# [%s] Memory\n\n", item.project))
		sb.WriteString(fmt.Sprintf("**ID:** `%s`\n\n", item.id))
		sb.WriteString(fmt.Sprintf("**Category:** `%s` | **Owner:** `%s` | **Importance:** `%d`\n\n", item.category, item.owner, item.importance))
		sb.WriteString(fmt.Sprintf("## Content\n%s\n\n", item.content))

	case skillListItem:
		sb.WriteString(fmt.Sprintf("# [%s] Skill: %s\n\n", item.project, item.name))
		sb.WriteString(fmt.Sprintf("**ID:** `%s`\n\n", item.id))
		sb.WriteString(fmt.Sprintf("## Description\n%s\n\n", item.desc))
	}

	rendered, err := m.mdRenderer.Render(sb.String())
	if err != nil {
		m.viewport.SetContent(sb.String())
	} else {
		m.viewport.SetContent(rendered)
	}
}

func (m *dataExplorerModel) Update(msg tea.Msg) (dataExplorerModel, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.lists[m.activeTab].FilterState() == list.Filtering {
			m.lists[m.activeTab], cmd = m.lists[m.activeTab].Update(msg)
			cmds = append(cmds, cmd)
			m.updateViewportContent()
			return *m, tea.Batch(cmds...)
		}

		switch msg.Type {
		case tea.KeyTab:
			m.activeTab = (m.activeTab + 1) % explorerTab(len(m.tabs))
			m.updateViewportContent()
			return *m, nil
		case tea.KeyShiftTab:
			m.activeTab--
			if m.activeTab < 0 {
				m.activeTab = explorerTab(len(m.tabs) - 1)
			}
			m.updateViewportContent()
			return *m, nil
		case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown:
			m.lists[m.activeTab], cmd = m.lists[m.activeTab].Update(msg)
			cmds = append(cmds, cmd)
			m.updateViewportContent()
			return *m, tea.Batch(cmds...)
		case tea.KeyRight:
			m.viewport.LineDown(1)
		case tea.KeyLeft:
			m.viewport.LineUp(1)
		case tea.KeyRunes:
			if string(msg.Runes) == "/" {
				m.lists[m.activeTab], cmd = m.lists[m.activeTab].Update(msg)
				cmds = append(cmds, cmd)
				return *m, tea.Batch(cmds...)
			} else if string(msg.Runes) == "d" || string(msg.Runes) == "x" {
				selected := m.lists[m.activeTab].SelectedItem()
				if selected != nil {
					var id string
					switch item := selected.(type) {
					case graphListItem:
						id = item.uri
					case memoryListItem:
						id = item.id
					case skillListItem:
						id = item.id
					}
					if id != "" {
						return *m, func() tea.Msg {
							return deleteItemMsg{tab: m.activeTab, id: id}
						}
					}
				}
			}
		}
	}

	m.lists[m.activeTab], cmd = m.lists[m.activeTab].Update(msg)
	cmds = append(cmds, cmd)

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	m.updateViewportContent()
	return *m, tea.Batch(cmds...)
}

func (m *dataExplorerModel) View() string {
	// Tabs
	var tabs []string
	activeTabStyle := lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true, true, false, true).Padding(0, 1)
	inactiveTabStyle := lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, true, false).Padding(0, 1)
	
	for i, t := range m.tabs {
		if explorerTab(i) == m.activeTab {
			tabs = append(tabs, activeTabStyle.Render(t))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(t))
		}
	}
	tabRow := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)

	left := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(lipgloss.Color("238")).
		Width(m.lists[m.activeTab].Width()).
		Height(m.viewport.Height).
		Render(m.lists[m.activeTab].View())

	right := lipgloss.NewStyle().
		PaddingLeft(1).
		Width(m.viewport.Width).
		Height(m.viewport.Height).
		Render(m.viewport.View())

	content := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	
	return lipgloss.JoinVertical(lipgloss.Left, tabRow, content)
}
