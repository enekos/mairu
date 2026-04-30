package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"mairu/internal/agent"
)

func (m model) handleEnter(msg tea.KeyMsg) (model, tea.Cmd, bool) {
	var cmd tea.Cmd

	if msg.Alt {
		m.textarea.InsertString("\n")
		return m, nil, true
	}
	if m.sidebarMode == "explore" {
		if m.selectedMessage >= 0 && m.selectedMessage < len(m.messages) {
			m.messages[m.selectedMessage].Expanded = !m.messages[m.selectedMessage].Expanded
			m.renderMessages()
			m.jumpToSelectedMessage()
		}
		return m, nil, true
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
			return m, nil, true
		}
	}

	m.filteredCommands = nil
	v := strings.TrimSpace(m.textarea.Value())
	m.textarea.Reset()

	if v == "" {
		return m, nil, true
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
		return m, nil, true
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
		return m.handleSlashCommand(v)
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
	m.thinkingStartedAt = time.Now()
	m.refreshThinkingIndicator(time.Now(), true)
	m.currentResponse = ""
	m.toolEvents = nil

	m.activeStream = make(chan agent.AgentEvent, 100)
	go m.agent.RunStream(v, m.activeStream)

	cmd = tea.Batch(waitForStream(m.activeStream), tickAnimSlow())
	return m, cmd, false
}
