package tui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

type toolEvent struct {
	Kind  string
	Title string
	Lines []string
	Raw   map[string]any
}

func buildToolCallEvent(toolName string, args map[string]any) toolEvent {
	lines := formatToolFields(args, 120)
	if len(lines) == 0 {
		lines = []string{"(no args)"}
	}
	return toolEvent{
		Kind:  "call",
		Title: fmt.Sprintf("Tool call: %s", toolName),
		Lines: lines,
		Raw:   args,
	}
}

func buildToolResultEvent(toolName string, result map[string]any) toolEvent {
	lines := formatToolFields(result, 120)
	if len(lines) == 0 {
		lines = []string{"(no result payload)"}
	}
	return toolEvent{
		Kind:  "result",
		Title: fmt.Sprintf("Tool result: %s", toolName),
		Lines: lines,
		Raw:   result,
	}
}

func buildToolStatusEvent(raw string) toolEvent {
	return toolEvent{
		Kind:  "status",
		Title: sanitizeStatusText(raw),
	}
}

func sanitizeStatusText(raw string) string {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimLeftFunc(cleaned, func(r rune) bool {
		return unicode.IsSymbol(r) || unicode.IsPunct(r) || unicode.IsSpace(r) || unicode.IsMark(r)
	})
	return strings.TrimSpace(cleaned)
}

func formatToolFields(data map[string]any, maxValueLen int) []string {
	if len(data) == 0 {
		return nil
	}
	const maxFields = 8
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for idx, k := range keys {
		if idx >= maxFields {
			remaining := len(keys) - maxFields
			fieldWord := "fields"
			if remaining == 1 {
				fieldWord = "field"
			}
			lines = append(lines, fmt.Sprintf("... and %d more %s", remaining, fieldWord))
			break
		}
		lines = append(lines, fmt.Sprintf("%s: %s", k, previewToolValue(data[k], maxValueLen)))
	}
	return lines
}

func previewToolValue(v any, maxLen int) string {
	s := strings.TrimSpace(strings.ReplaceAll(fmt.Sprintf("%v", v), "\n", " "))
	if maxLen > 3 && len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

func renderToolEventBox(ev toolEvent) string {
	var body strings.Builder
	icon := toolKindIcon(ev.Kind)
	switch ev.Kind {
	case "call":
		body.WriteString(toolCallTitleStyle.Render(icon + "  " + ev.Title))
	case "result":
		body.WriteString(toolResultTitleStyle.Render(icon + "  " + ev.Title))
	default:
		body.WriteString(toolStatusTitleStyle.Render(icon + "  " + ev.Title))
	}
	if len(ev.Lines) > 0 {
		for _, line := range ev.Lines {
			body.WriteString("\n")
			body.WriteString("  ")
			colon := strings.Index(line, ": ")
			if colon > 0 && !strings.HasPrefix(line, "... and ") {
				key := line[:colon]
				value := line[colon+2:]
				body.WriteString(toolFieldKeyStyle.Render(key + ": "))
				body.WriteString(toolFieldValueStyle.Render(value))
			} else {
				body.WriteString(toolFieldValueStyle.Render(line))
			}
		}
	}

	switch ev.Kind {
	case "call":
		return toolCallBoxStyle.Render(body.String())
	case "result":
		return toolResultBoxStyle.Render(body.String())
	default:
		return toolStatusBoxStyle.Render(body.String())
	}
}

func renderExpandedToolEventBox(ev toolEvent) string {
	var body strings.Builder
	icon := toolKindIcon(ev.Kind)
	switch ev.Kind {
	case "call":
		body.WriteString(toolCallTitleStyle.Render(icon + "  " + ev.Title))
	case "result":
		body.WriteString(toolResultTitleStyle.Render(icon + "  " + ev.Title))
	default:
		body.WriteString(toolStatusTitleStyle.Render(icon + "  " + ev.Title))
	}

	if ev.Raw != nil {
		body.WriteString("\n")
		// Format without limits
		keys := make([]string, 0, len(ev.Raw))
		for k := range ev.Raw {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			v := ev.Raw[k]
			body.WriteString("\n  ")
			body.WriteString(toolFieldKeyStyle.Render(k + ":\n    "))
			var strVal string
			if s, ok := v.(string); ok {
				strVal = s
			} else {
				b, err := json.MarshalIndent(v, "", "  ")
				if err == nil {
					strVal = string(b)
				} else {
					strVal = fmt.Sprintf("%v", v)
				}
			}
			strVal = strings.TrimSpace(strVal)
			strVal = strings.ReplaceAll(strVal, "\n", "\n    ")
			body.WriteString(toolFieldValueStyle.Render(strVal))
		}
	} else if len(ev.Lines) > 0 {
		for _, line := range ev.Lines {
			body.WriteString("\n  ")
			body.WriteString(toolFieldValueStyle.Render(line))
		}
	}

	switch ev.Kind {
	case "call":
		return toolCallBoxStyle.Render(body.String())
	case "result":
		return toolResultBoxStyle.Render(body.String())
	default:
		return toolStatusBoxStyle.Render(body.String())
	}
}
