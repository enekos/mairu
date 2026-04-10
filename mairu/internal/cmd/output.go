package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

// Formatter renders structured CLI output in different formats.
type Formatter struct {
	format string
	w      io.Writer
}

// NewFormatter creates a Formatter for the given format ("table", "json", "plain").
func NewFormatter(format string) *Formatter {
	if format == "" {
		format = "table"
	}
	return &Formatter{format: format, w: os.Stdout}
}

// PrintTable renders rows as a table with the given headers.
// Each row is a map from header name to value.
func (f *Formatter) PrintTable(headers []string, rows []map[string]string) {
	switch f.format {
	case "json":
		f.PrintJSON(rows)
	case "plain":
		f.printPlain(headers, rows)
	default:
		f.printTabular(headers, rows)
	}
}

// PrintItems renders a slice of any JSON-serializable items.
// headers + extractFn are used for table/plain; json mode serializes directly.
func (f *Formatter) PrintItems(headers []string, items []map[string]any, extractFn func(map[string]any) map[string]string) {
	switch f.format {
	case "json":
		f.PrintJSON(items)
	default:
		rows := make([]map[string]string, len(items))
		for i, item := range items {
			rows[i] = extractFn(item)
		}
		if f.format == "plain" {
			f.printPlain(headers, rows)
		} else {
			f.printTabular(headers, rows)
		}
	}
}

// PrintRaw prints a single value (used by config get, etc.).
func (f *Formatter) PrintRaw(v any) {
	switch f.format {
	case "json":
		f.PrintJSON(v)
	default:
		fmt.Fprintln(f.w, v)
	}
}

func (f *Formatter) PrintJSON(v any) {
	enc := json.NewEncoder(f.w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func (f *Formatter) printPlain(headers []string, rows []map[string]string) {
	for _, row := range rows {
		vals := make([]string, len(headers))
		for i, h := range headers {
			vals[i] = row[h]
		}
		fmt.Fprintln(f.w, strings.Join(vals, "\t"))
	}
}

func (f *Formatter) printTabular(headers []string, rows []map[string]string) {
	tw := tabwriter.NewWriter(f.w, 0, 0, 2, ' ', 0)
	// Header
	upperHeaders := make([]string, len(headers))
	for i, h := range headers {
		upperHeaders[i] = strings.ToUpper(h)
	}
	fmt.Fprintln(tw, strings.Join(upperHeaders, "\t"))
	// Rows
	for _, row := range rows {
		vals := make([]string, len(headers))
		for i, h := range headers {
			vals[i] = row[h]
		}
		fmt.Fprintln(tw, strings.Join(vals, "\t"))
	}
	tw.Flush()
}
