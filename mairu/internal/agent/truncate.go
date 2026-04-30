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
	// Fast path: count lines without splitting.
	totalLines := strings.Count(content, "\n") + 1
	if totalLines <= maxLines && totalBytes <= maxBytes {
		return TruncateResult{
			Content: content, Truncated: false,
			TotalLines: totalLines, TotalBytes: totalBytes,
			OutputLines: totalLines, OutputBytes: totalBytes,
			MaxLines: maxLines, MaxBytes: maxBytes,
		}
	}

	// Scan forward without allocating a lines slice.
	bytesUsed := 0
	linesKept := 0
	by := "lines"
	var i int
	for i = 0; i < len(content); {
		j := i
		for j < len(content) && content[j] != '\n' {
			j++
		}
		add := j - i
		if i > 0 {
			add++ // newline
		}
		if bytesUsed+add > maxBytes {
			by = "bytes"
			break
		}
		bytesUsed += add
		linesKept++
		if linesKept >= maxLines {
			by = "lines"
			// Move j past the newline so the cut includes it.
			if j < len(content) && content[j] == '\n' {
				j++
			}
			i = j
			break
		}
		if j < len(content) && content[j] == '\n' {
			j++
		}
		i = j
	}
	outStr := content[:i]
	if i > 0 && content[i-1] == '\n' {
		outStr = content[:i-1]
	}
	return TruncateResult{
		Content: outStr, Truncated: true, TruncatedBy: by,
		TotalLines: totalLines, TotalBytes: totalBytes,
		OutputLines: linesKept, OutputBytes: len(outStr),
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
	// Fast path: count lines without splitting.
	totalLines := strings.Count(content, "\n") + 1
	if totalLines <= maxLines && totalBytes <= maxBytes {
		return TruncateResult{
			Content: content, Truncated: false,
			TotalLines: totalLines, TotalBytes: totalBytes,
			OutputLines: totalLines, OutputBytes: totalBytes,
			MaxLines: maxLines, MaxBytes: maxBytes,
		}
	}

	// Scan backward without allocating a lines slice.
	bytesUsed := 0
	linesKept := 0
	by := "lines"
	var i int = len(content)
	for i > 0 && linesKept < maxLines {
		j := i - 1
		for j > 0 && content[j-1] != '\n' {
			j--
		}
		add := i - j
		if linesKept > 0 {
			add++ // newline
		}
		if bytesUsed+add > maxBytes {
			by = "bytes"
			// Edge case: single line bigger than maxBytes — keep its tail.
			if linesKept == 0 {
				start := j + (i - j) - maxBytes
				if start < j {
					start = j
				}
				// Walk forward to a valid UTF-8 boundary.
				for start < i && !utf8.RuneStart(content[start]) {
					start++
				}
				return TruncateResult{
					Content:     content[start:],
					Truncated:   true,
					TruncatedBy: "bytes",
					TotalLines:  totalLines,
					TotalBytes:  totalBytes,
					OutputLines: 1,
					OutputBytes: i - start,
					MaxLines:    maxLines,
					MaxBytes:    maxBytes,
				}
			}
			break
		}
		bytesUsed += add
		linesKept++
		i = j
	}
	if linesKept >= maxLines && bytesUsed <= maxBytes {
		by = "lines"
	}
	// i now points to the start of the first kept line.
	return TruncateResult{
		Content: content[i:], Truncated: true, TruncatedBy: by,
		TotalLines: totalLines, TotalBytes: totalBytes,
		OutputLines: linesKept, OutputBytes: len(content) - i,
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
