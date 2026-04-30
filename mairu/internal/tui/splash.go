package tui

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// splashTotalFrames controls splash duration (frames at ~20ms tick = ~1.6s).
const splashTotalFrames = 80

var splashLogo = []string{
	"  __  __    _    ___ ____  _   _ ",
	" |  \\/  |  / \\  |_ _|  _ \\| | | |",
	" | |\\/| | / _ \\  | || |_) | | | |",
	" | |  | |/ ___ \\ | ||  _ <| |_| |",
	" |_|  |_/_/   \\_\\___|_| \\_\\\\___/ ",
}

var splashTagline = "context · memory · for coding agents"
var splashSparkles = []rune{'✦', '✧', '✺', '✸', '✶', '·', '∗'}

// renderSplash draws a colorful logo reveal with drifting sparkles and a
// rainbow shimmer that sweeps across the wordmark.
func (m model) renderSplash() string {
	width, height := m.width, m.height
	if width <= 0 || height <= 0 {
		return ""
	}

	t := float64(m.animFrame) / float64(splashTotalFrames)
	if t > 1 {
		t = 1
	}

	logoH := len(splashLogo)
	logoW := 0
	for _, l := range splashLogo {
		if w := len([]rune(l)); w > logoW {
			logoW = w
		}
	}

	totalH := logoH + 2 + 1
	topPad := (height - totalH) / 2
	if topPad < 0 {
		topPad = 0
	}
	leftPad := (width - logoW) / 2
	if leftPad < 0 {
		leftPad = 0
	}

	var sb strings.Builder

	// Top sparkle field — full-width rows, no overlap with the logo.
	for row := 0; row < topPad; row++ {
		sb.WriteString(sparkleRow(width, row, m.animFrame))
		sb.WriteString("\n")
	}

	// Logo rows: simple left padding + shimmer-revealed glyphs. No inline sparkles.
	for row, line := range splashLogo {
		var rowSb strings.Builder
		rowSb.WriteString(strings.Repeat(" ", leftPad))
		runes := []rune(line)
		for col, r := range runes {
			localT := t*1.6 - float64(col)/float64(logoW)
			if localT < 0 {
				rowSb.WriteRune(' ')
				continue
			}
			pos := math.Mod(float64(col)/float64(logoW)+t*1.5+float64(row)*0.05, 1.0)
			c := sampleHexGradient([]string{
				"#ff6188", "#fc9867", "#ffd866", "#a9dc76", "#78dce8", "#ab9df2", "#ff6188",
			}, pos)
			alpha := math.Min(1.0, localT*2.5)
			if alpha < 0.35 {
				rowSb.WriteString(lipgloss.NewStyle().Foreground(chromeDim).Render(string(r)))
			} else {
				rowSb.WriteString(lipgloss.NewStyle().Foreground(c).Bold(true).Render(string(r)))
			}
		}
		sb.WriteString(rowSb.String())
		sb.WriteString("\n")
	}

	// Gap row (plain).
	sb.WriteString("\n")

	// Tagline fades in late with rainbow gradient.
	if t > 0.45 {
		tlPad := (width - len(splashTagline)) / 2
		if tlPad < 0 {
			tlPad = 0
		}
		alpha := (t - 0.45) / 0.55
		if alpha > 1 {
			alpha = 1
		}
		var tagline string
		if alpha < 0.4 {
			tagline = lipgloss.NewStyle().Foreground(chromeDim).Italic(true).Render(splashTagline)
		} else {
			tagline = gradientText(splashTagline, []string{"#ff6188", "#ffd866", "#a9dc76", "#78dce8", "#ab9df2"}, t*0.6, false)
			tagline = lipgloss.NewStyle().Italic(true).Render(tagline)
		}
		sb.WriteString(strings.Repeat(" ", tlPad))
		sb.WriteString(tagline)
		sb.WriteString("\n")
	} else {
		sb.WriteString("\n")
	}

	// Sparkle row above the progress dots for visual texture.
	sb.WriteString(sparkleRow(width, topPad+logoH+3, m.animFrame))
	sb.WriteString("\n")

	// Animated rainbow progress dots.
	if t < 1 {
		dots := 28
		filled := int(t * float64(dots))
		var bar strings.Builder
		for i := 0; i < dots; i++ {
			ch := "·"
			var st lipgloss.Style
			if i < filled {
				ch = "●"
				c := sampleHexGradient([]string{"#ff6188", "#fc9867", "#ffd866", "#a9dc76", "#78dce8", "#ab9df2"},
					math.Mod(float64(i)/float64(dots)+t, 1.0))
				st = lipgloss.NewStyle().Foreground(c).Bold(true)
			} else {
				st = lipgloss.NewStyle().Foreground(chromeDim)
			}
			bar.WriteString(st.Render(ch))
		}
		barPad := (width - dots) / 2
		if barPad < 0 {
			barPad = 0
		}
		sb.WriteString(strings.Repeat(" ", barPad))
		sb.WriteString(bar.String())
	}

	return sb.String()
}

// sparkleRow returns a single decorative row of width cells, with rare,
// colored sparkles drifting based on frame. Safe to compose freely (each
// cell is either a plain space or a fully-styled single-rune token).
func sparkleRow(width, row, frame int) string {
	if width <= 0 {
		return ""
	}
	var sb strings.Builder
	for col := 0; col < width; col++ {
		h := (col*131 + row*17 + frame*3) & 0xff
		if h%53 == 0 {
			h2 := (col*7 + row*5 + frame) & 0xff
			rch := splashSparkles[h2%len(splashSparkles)]
			c := rainbowPalette[(col+row+frame/4)%len(rainbowPalette)]
			sb.WriteString(lipgloss.NewStyle().Foreground(c).Render(string(rch)))
		} else {
			sb.WriteByte(' ')
		}
	}
	return sb.String()
}

func rgbHex(r, g, b int) string {
	hexChars := "0123456789abcdef"
	out := make([]byte, 7)
	out[0] = '#'
	out[1] = hexChars[(r>>4)&0xf]
	out[2] = hexChars[r&0xf]
	out[3] = hexChars[(g>>4)&0xf]
	out[4] = hexChars[g&0xf]
	out[5] = hexChars[(b>>4)&0xf]
	out[6] = hexChars[b&0xf]
	return string(out)
}
