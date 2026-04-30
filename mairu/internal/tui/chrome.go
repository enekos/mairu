package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	chromeBg          = lipgloss.Color("#2b303b")
	chromeFg          = lipgloss.Color("#d8dee9")
	chromeDim         = lipgloss.Color("#6b7385")
	chromeAccent      = lipgloss.Color("#88c0d0")
	chromeOk          = lipgloss.Color("#a3be8c")
	chromeWarn        = lipgloss.Color("#ebcb8b")
	chromeBad         = lipgloss.Color("#bf616a")
	chromeBrand       = lipgloss.Color("#b48ead")
	chromeSeparatorFg = lipgloss.Color("#3b4252")

	statusBarStyle = lipgloss.NewStyle().
			Background(chromeBg).
			Foreground(chromeFg)

	brandPillStyle = lipgloss.NewStyle().
			Background(chromeBrand).
			Foreground(lipgloss.Color("#1a1d23")).
			Bold(true).
			Padding(0, 1)

	chromeKeyStyle = lipgloss.NewStyle().Foreground(chromeAccent).Bold(true)
	chromeValStyle = lipgloss.NewStyle().Foreground(chromeFg)
	chromeSepStyle = lipgloss.NewStyle().Foreground(chromeSeparatorFg)
	chromeDimStyle = lipgloss.NewStyle().Foreground(chromeDim)

	keyCapStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#3b4252")).
			Foreground(lipgloss.Color("#eceff4")).
			Padding(0, 1).
			Bold(true)
	keyCapHintStyle = lipgloss.NewStyle().
			Foreground(chromeDim)
)

func renderKeyCap(key, hint string) string {
	return keyCapStyle.Render(key) + " " + keyCapHintStyle.Render(hint)
}

// renderStatusBar paints the top chrome line: brand, project, model, stream, tokens, ctx bar.
func (m model) renderStatusBar() string {
	if m.width <= 0 {
		return ""
	}

	stats := computeSessionStats(m.messages, m.currentResponse, m.toolEvents, m.thinking, m.agent.GetModelName())

	root := ""
	if m.agent != nil {
		root = filepath.Base(m.agent.GetRoot())
	}
	if root == "" {
		root = "—"
	}

	streamLabel := "idle"
	streamColor := chromeOk
	if m.thinking {
		streamLabel = "streaming"
		streamColor = chromeWarn
	}
	streamStyle := lipgloss.NewStyle().Foreground(streamColor).Bold(true)

	elapsed := ""
	if m.thinking && !m.thinkingStartedAt.IsZero() {
		d := time.Since(m.thinkingStartedAt).Truncate(time.Millisecond * 100)
		elapsed = fmt.Sprintf(" %s", d)
	}

	pct := 0.0
	if stats.ContextLimit > 0 {
		pct = float64(stats.EstimatedTotalTokens) / float64(stats.ContextLimit)
	}
	if pct > 1 {
		pct = 1
	}

	tokenColor := chromeOk
	switch {
	case pct >= 0.85:
		tokenColor = chromeBad
	case pct >= 0.6:
		tokenColor = chromeWarn
	}

	tokensStr := lipgloss.NewStyle().Foreground(tokenColor).Bold(true).
		Render(fmt.Sprintf("%s tok", humanCount(stats.EstimatedTotalTokens)))

	sep := chromeSepStyle.Render(" │ ")

	brand := brandPillStyle.Render(" mairu ")
	if m.thinking {
		// Animate the brand pill background while streaming.
		brand = lipgloss.NewStyle().
			Background(pulseColor(m.animFrame)).
			Foreground(lipgloss.Color("#19181a")).
			Bold(true).
			Padding(0, 1).
			Render(" mairu ")
	}

	left := strings.Join([]string{
		brand,
		chromeKeyStyle.Render("⌂") + " " + chromeValStyle.Render(root),
		chromeKeyStyle.Render("✦") + " " + chromeValStyle.Render(stats.Model),
	}, sep)

	mid := strings.Join([]string{
		chromeKeyStyle.Render("◧") + " " + chromeValStyle.Render(m.sessionName),
		streamStyle.Render("● "+streamLabel) + chromeDimStyle.Render(elapsed),
	}, sep)

	right := strings.Join([]string{
		tokensStr + chromeDimStyle.Render(fmt.Sprintf("/%s", humanCount(stats.ContextLimit))),
		renderContextBar(pct, 14, tokenColor),
	}, sep)

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	midW := lipgloss.Width(mid)
	avail := m.width - leftW - rightW - midW
	if avail < 2 {
		// Drop the middle section if too narrow.
		gap := m.width - leftW - rightW
		if gap < 1 {
			gap = 1
		}
		return statusBarStyle.Width(m.width).Render(left + strings.Repeat(" ", gap) + right)
	}

	pad := avail / 2
	line := left + strings.Repeat(" ", pad) + mid + strings.Repeat(" ", avail-pad) + right
	return statusBarStyle.Width(m.width).Render(line)
}

// renderContextBar produces a compact unicode progress bar.
func renderContextBar(pct float64, width int, color lipgloss.Color) string {
	if width < 4 {
		width = 4
	}
	filled := int(pct * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	on := lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("█", filled))
	off := chromeDimStyle.Render(strings.Repeat("░", width-filled))
	return on + off
}

// streamingBorderColor smoothly cycles through the rainbow palette while
// the agent is streaming, so the chat pane border "breathes" with color.
func streamingBorderColor(frame int) lipgloss.Color {
	return pulseColor(frame)
}

// sineLite is a small sine helper that avoids importing math twice.
func sineLite(x float64) float64 {
	// Reduce x to [-pi, pi] to keep precision reasonable.
	const twoPi = 6.283185307179586
	for x > twoPi {
		x -= twoPi
	}
	for x < -twoPi {
		x += twoPi
	}
	// Bhaskara I approximation for speed; precision is plenty for color blending.
	if x < 0 {
		return -sineLite(-x)
	}
	if x > 3.141592653589793 {
		return -sineLite(x - 3.141592653589793)
	}
	return (16 * x * (3.141592653589793 - x)) / (49.348022005446796 - 4*x*(3.141592653589793-x))
}

func humanCount(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// renderRichFooter draws the bottom footer with key caps.
func (m model) renderRichFooter() string {
	if m.width <= 0 {
		return ""
	}

	keys := []struct {
		k, h string
	}{
		{"⏎", "send"},
		{"⇥", "complete"},
		{"^F", "follow"},
		{"^E", "sidebar"},
		{"^O", "tab"},
		{"^N", "nvim"},
		{"^G", "lazygit"},
		{"/", "cmds"},
	}
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, renderKeyCap(k.k, k.h))
	}
	gap := chromeSepStyle.Render("  ")
	line := strings.Join(parts, gap)

	scroll := m.scrollIndicator()
	if scroll != "" {
		line = lipgloss.JoinHorizontal(lipgloss.Top, line,
			lipgloss.NewStyle().Width(m.width-lipgloss.Width(line)).Align(lipgloss.Right).Render(scroll))
	}
	return statusBarStyle.Width(m.width).Render(line)
}

// scrollIndicator shows a compact "12%↓ follow" hint about viewport position.
func (m model) scrollIndicator() string {
	pct := m.viewport.ScrollPercent()
	follow := ""
	if m.followMode {
		follow = lipgloss.NewStyle().Foreground(chromeOk).Bold(true).Render("● follow")
	} else {
		follow = chromeDimStyle.Render("○ paused")
	}
	pctStr := lipgloss.NewStyle().Foreground(chromeAccent).Render(fmt.Sprintf("%3.0f%%", pct*100))
	return follow + chromeSepStyle.Render(" │ ") + pctStr
}
