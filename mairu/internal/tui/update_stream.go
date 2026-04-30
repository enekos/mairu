package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"mairu/internal/agent"
)

func (m *model) handleAgentStream(msg agentStreamMsg) tea.Cmd {
	var cmds []tea.Cmd

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
			m.thinkingStartedAt = time.Now()
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
	if len(cmds) > 0 {
		return tea.Batch(cmds...)
	}
	return nil
}
