package tui

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// rainbowPalette is a vivid spectrum used for animated gradients across the TUI.
var rainbowPalette = []lipgloss.Color{
	lipgloss.Color("#ff6188"), // pink
	lipgloss.Color("#fc9867"), // orange
	lipgloss.Color("#ffd866"), // yellow
	lipgloss.Color("#a9dc76"), // green
	lipgloss.Color("#78dce8"), // cyan
	lipgloss.Color("#ab9df2"), // purple
}

// agentGradient is the pink→purple→cyan spectrum used to color agent role labels.
var agentGradient = []string{"#ff6188", "#ab9df2", "#78dce8"}

// gradientText paints a string with a smooth gradient between hex colors,
// optionally shifted by phase (in radians) so it can animate over time.
func gradientText(s string, hexStops []string, phase float64, bold bool) string {
	if s == "" || len(hexStops) == 0 {
		return s
	}
	runes := []rune(s)
	n := len(runes)
	if n == 0 {
		return s
	}
	var sb strings.Builder
	for i, r := range runes {
		if r == ' ' {
			sb.WriteRune(r)
			continue
		}
		// Position 0..1 along the string, animated by phase.
		pos := math.Mod(float64(i)/float64(n)+phase, 1.0)
		if pos < 0 {
			pos += 1
		}
		c := sampleHexGradient(hexStops, pos)
		st := lipgloss.NewStyle().Foreground(c)
		if bold {
			st = st.Bold(true)
		}
		sb.WriteString(st.Render(string(r)))
	}
	return sb.String()
}

// sampleHexGradient picks a color along an evenly-spaced hex stop list.
func sampleHexGradient(stops []string, pos float64) lipgloss.Color {
	if len(stops) == 1 {
		return lipgloss.Color(stops[0])
	}
	if pos < 0 {
		pos = 0
	}
	if pos > 1 {
		pos = 1
	}
	scaled := pos * float64(len(stops)-1)
	idx := int(math.Floor(scaled))
	if idx >= len(stops)-1 {
		return lipgloss.Color(stops[len(stops)-1])
	}
	t := scaled - float64(idx)
	return lipgloss.Color(blendHex(stops[idx], stops[idx+1], t))
}

// blendHex linearly interpolates between two #rrggbb colors.
func blendHex(a, b string, t float64) string {
	ar, ag, ab := parseHex(a)
	br, bg, bb := parseHex(b)
	r := int(float64(ar)*(1-t) + float64(br)*t)
	g := int(float64(ag)*(1-t) + float64(bg)*t)
	bl := int(float64(ab)*(1-t) + float64(bb)*t)
	return rgbHex(r, g, bl)
}

func parseHex(h string) (int, int, int) {
	if len(h) < 7 {
		return 0, 0, 0
	}
	r := hexNibble(h[1])<<4 | hexNibble(h[2])
	g := hexNibble(h[3])<<4 | hexNibble(h[4])
	b := hexNibble(h[5])<<4 | hexNibble(h[6])
	return r, g, b
}

func hexNibble(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return 0
}

// pulseColor cycles through the rainbow palette with phase.
func pulseColor(frame int) lipgloss.Color {
	t := math.Mod(float64(frame)*0.04, float64(len(rainbowPalette)))
	idx := int(math.Floor(t))
	next := (idx + 1) % len(rainbowPalette)
	mix := t - float64(idx)
	a := string(rainbowPalette[idx])
	b := string(rainbowPalette[next])
	return lipgloss.Color(blendHex(a, b, mix))
}
