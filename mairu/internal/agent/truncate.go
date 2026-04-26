package agent

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Dual-axis output truncation. Whichever limit hits first wins. Inspired by
// pi-mono's truncate.ts: tools that show file beginnings (read) truncate at
// the head; tools that show command tails (bash) truncate at the tail.

const (
	DefaultMaxLines   = 2000
	DefaultMaxBytes   = 50 * 1024
	GrepMaxLineLength = 500
)

type TruncateResult struct {
	Content     string
	Truncated   bool
	TruncatedBy string // "lines", "bytes", or ""
	TotalLines  int
	TotalBytes  int
	OutputLines int
	OutputBytes int
	MaxLines    int
	MaxBytes    int
}

// FormatSize renders a byte count as a short human string.
func FormatSize(b int) string {
	switch {
	case b < 1024:
		return fmt.Sprintf("%dB", b)
	case b < 1024*1024:
		return fmt.Sprintf("%.1fKB", float64(b)/1024)
	default:
		return fmt.Sprintf("%.1fMB", float64(b)/(1024*1024))
	}
}

// TruncateHead keeps the first N lines/bytes; never returns partial lines.
func TruncateHead(content string, maxLines, maxBytes int) TruncateResult {
	if maxLines <= 0 {
		maxLines = DefaultMaxLines
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}

	totalBytes := len(content)
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	if totalLines <= maxLines && totalBytes <= maxBytes {
		return TruncateResult{
			Content: content, Truncated: false,
			TotalLines: totalLines, TotalBytes: totalBytes,
			OutputLines: totalLines, OutputBytes: totalBytes,
			MaxLines: maxLines, MaxBytes: maxBytes,
		}
	}

	out := make([]string, 0, maxLines)
	bytesUsed := 0
	by := "lines"
	for i, line := range lines {
		if i >= maxLines {
			break
		}
		add := len(line)
		if i > 0 {
			add++ // newline
		}
		if bytesUsed+add > maxBytes {
			by = "bytes"
			break
		}
		out = append(out, line)
		bytesUsed += add
	}
	if len(out) >= maxLines && bytesUsed <= maxBytes {
		by = "lines"
	}
	outStr := strings.Join(out, "\n")
	return TruncateResult{
		Content: outStr, Truncated: true, TruncatedBy: by,
		TotalLines: totalLines, TotalBytes: totalBytes,
		OutputLines: len(out), OutputBytes: len(outStr),
		MaxLines: maxLines, MaxBytes: maxBytes,
	}
}

// TruncateTail keeps the last N lines/bytes. Useful for command output where
// the relevant info (errors, final state) is at the end.
func TruncateTail(content string, maxLines, maxBytes int) TruncateResult {
	if maxLines <= 0 {
		maxLines = DefaultMaxLines
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}

	totalBytes := len(content)
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	if totalLines <= maxLines && totalBytes <= maxBytes {
		return TruncateResult{
			Content: content, Truncated: false,
			TotalLines: totalLines, TotalBytes: totalBytes,
			OutputLines: totalLines, OutputBytes: totalBytes,
			MaxLines: maxLines, MaxBytes: maxBytes,
		}
	}

	out := make([]string, 0, maxLines)
	bytesUsed := 0
	by := "lines"
	for i := len(lines) - 1; i >= 0 && len(out) < maxLines; i-- {
		line := lines[i]
		add := len(line)
		if len(out) > 0 {
			add++
		}
		if bytesUsed+add > maxBytes {
			by = "bytes"
			// Edge case: single line bigger than maxBytes — keep its tail.
			if len(out) == 0 {
				start := len(line) - maxBytes
				if start < 0 {
					start = 0
				}
				// Walk forward to a valid UTF-8 boundary.
				for start < len(line) && !utf8.RuneStart(line[start]) {
					start++
				}
				out = append([]string{line[start:]}, out...)
				bytesUsed = len(line) - start
			}
			break
		}
		out = append([]string{line}, out...)
		bytesUsed += add
	}
	if len(out) >= maxLines && bytesUsed <= maxBytes {
		by = "lines"
	}
	outStr := strings.Join(out, "\n")
	return TruncateResult{
		Content: outStr, Truncated: true, TruncatedBy: by,
		TotalLines: totalLines, TotalBytes: totalBytes,
		OutputLines: len(out), OutputBytes: len(outStr),
		MaxLines: maxLines, MaxBytes: maxBytes,
	}
}

// TruncateLine caps a single line length, used for grep matches that may have
// huge minified-file lines.
func TruncateLine(line string, maxChars int) (string, bool) {
	if maxChars <= 0 {
		maxChars = GrepMaxLineLength
	}
	if utf8.RuneCountInString(line) <= maxChars {
		return line, false
	}
	// Cut on byte index — close enough for source code, avoids extra walk.
	if len(line) <= maxChars {
		return line, false
	}
	cut := maxChars
	for cut > 0 && !utf8.RuneStart(line[cut]) {
		cut--
	}
	return line[:cut] + "... [truncated]", true
}

// FormatTruncationNote renders a one-line footer the model can rely on.
func FormatTruncationNote(r TruncateResult, mode string) string {
	if !r.Truncated {
		return ""
	}
	hint := ""
	if mode == "head" {
		hint = " Use offset/limit or grep for the rest."
	} else if mode == "tail" {
		hint = " Earlier output was dropped."
	}
	return fmt.Sprintf("\n...[truncated by %s: kept %d/%d lines, %s/%s].%s",
		r.TruncatedBy, r.OutputLines, r.TotalLines,
		FormatSize(r.OutputBytes), FormatSize(r.TotalBytes), hint)
}
