// SQL walker — parses psql's aligned-table output and redacts each cell
// using the JSON walker's key+content policy. Targets the
// `cloud-sql-proxy` + `psql -c "..."` workflow common at JOIN.
//
// Recognised block shape (psql's default `aligned` format):
//
//	 col_a | col_b      | col_c
//	-------+------------+---------
//	 v1    | foo@bar.io | 42
//	 v2    | baz@qux.io | 43
//	(2 rows)            <- optional footer
//
// Limits (v1):
//   - Aligned ASCII only. Extended (\x), --csv, -A unaligned, and Unicode
//     box-drawing pass through unredacted; use --mode json / --mode line
//     for those.
//   - Single-line cells only. psql wraps very long strings across lines
//     when the terminal is narrow; those rows are misparsed.
//   - ID-shaped column names (e.g. `userId`, `candidate_id`) are not yet
//     auto-recognised; either add them to `safe_keys` or use --permissive
//     until the suffix-glob IDKeySet lands on Ruleset.
package walkers

import (
	"bufio"
	"io"
	"strings"

	"github.com/enekos/mairu/pii-redact/internal/patterns"
)

// SQL streams psql aligned-table output through the redactor.
//
// Behaviour: scan line by line. When a line looks like a psql separator
// (`---+---+---`) and the previous line has the matching column widths,
// treat the previous line as a header and the following lines (until a
// blank line or `(N rows)` footer) as data. Re-emit the block with
// redacted cells and recomputed column widths. Lines outside any block
// pass through verbatim — so a query echo, NOTICE/WARNING, or psql prompt
// mixed into the stream survives intact.
func SQL(in io.Reader, out io.Writer, opts Options) (patterns.Stats, error) {
	totals := patterns.Stats{}
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
	bw := bufio.NewWriter(out)
	defer bw.Flush()

	var pending string
	havePending := false

	flushPending := func() error {
		if !havePending {
			return nil
		}
		if _, err := bw.WriteString(pending); err != nil {
			return err
		}
		if err := bw.WriteByte('\n'); err != nil {
			return err
		}
		havePending = false
		return nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		if !havePending {
			pending = line
			havePending = true
			continue
		}
		boundaries, isSep := parseSQLSeparator(line)
		if isSep {
			headers := splitSQLByBoundaries(pending, boundaries)
			headerNames := make([]string, len(headers))
			for i, h := range headers {
				headerNames[i] = strings.TrimSpace(h)
			}
			rows, footer, err := readSQLRows(scanner, boundaries)
			if err != nil {
				return totals, err
			}
			redacted := make([][]string, len(rows))
			for i, row := range rows {
				redacted[i] = make([]string, len(row))
				for j, cell := range row {
					var key string
					if j < len(headerNames) {
						key = headerNames[j]
					}
					redacted[i][j] = redactSQLCell(key, cell, opts, totals)
				}
			}
			if err := writeSQLBlock(bw, headerNames, redacted, footer); err != nil {
				return totals, err
			}
			havePending = false
			continue
		}
		if err := flushPending(); err != nil {
			return totals, err
		}
		pending = line
		havePending = true
	}
	if err := flushPending(); err != nil {
		return totals, err
	}
	return totals, scanner.Err()
}

// parseSQLSeparator inspects `line`. If it's a psql separator (only `-`,
// `+`, and optionally trailing whitespace), it returns the byte indexes of
// each `+` (column boundary). A single-column separator has no `+` and
// returns an empty boundaries slice but isSep=true.
func parseSQLSeparator(line string) (boundaries []int, isSep bool) {
	if len(line) == 0 {
		return nil, false
	}
	sawDash := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch c {
		case '-':
			sawDash = true
		case '+':
			boundaries = append(boundaries, i)
		case ' ', '\t':
			// Only tolerate trailing whitespace after we've seen content.
			if !sawDash {
				return nil, false
			}
		default:
			return nil, false
		}
	}
	if !sawDash {
		return nil, false
	}
	return boundaries, true
}

// splitSQLByBoundaries cuts `line` at each boundary index, returning the
// substrings between (and outside) boundaries. Lines shorter than the
// final boundary still split cleanly — missing tail cells come back empty.
func splitSQLByBoundaries(line string, boundaries []int) []string {
	if len(boundaries) == 0 {
		return []string{line}
	}
	parts := make([]string, 0, len(boundaries)+1)
	start := 0
	for _, b := range boundaries {
		switch {
		case b <= start:
			parts = append(parts, "")
		case b > len(line):
			parts = append(parts, line[start:])
			start = b + 1
			continue
		default:
			parts = append(parts, line[start:b])
		}
		start = b + 1
	}
	if start <= len(line) {
		parts = append(parts, line[start:])
	} else {
		parts = append(parts, "")
	}
	return parts
}

// readSQLRows pulls subsequent rows from the scanner until a blank line,
// a `(N rows)` footer, or EOF. Returns the rows with their raw, untrimmed
// cell strings and the footer line if seen (otherwise "").
func readSQLRows(scanner *bufio.Scanner, boundaries []int) ([][]string, string, error) {
	rows := make([][]string, 0, 16)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			return rows, "", nil
		}
		if isSQLFooter(line) {
			return rows, line, nil
		}
		rows = append(rows, splitSQLByBoundaries(line, boundaries))
	}
	return rows, "", scanner.Err()
}

// isSQLFooter matches psql's row-count footer: "(N row)" or "(N rows)",
// possibly indented.
func isSQLFooter(line string) bool {
	t := strings.TrimSpace(line)
	if len(t) < 7 || t[0] != '(' || t[len(t)-1] != ')' {
		return false
	}
	inner := t[1 : len(t)-1]
	if !strings.HasSuffix(inner, " row") && !strings.HasSuffix(inner, " rows") {
		return false
	}
	// The prefix before " row(s)" must be digits.
	cut := strings.LastIndex(inner, " ")
	if cut <= 0 {
		return false
	}
	for _, r := range inner[:cut] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// redactSQLCell applies the JSON walker's per-key policy to a single
// SQL cell, treating the column header as the key.
func redactSQLCell(key, raw string, opts Options, stats patterns.Stats) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if opts.Rules != nil {
		if _, ok := opts.Rules.RedactKeys[key]; ok {
			stats["[KEY]"]++
			if v := maskKeyedValue(key, trimmed, opts); v != nil {
				if s, ok := v.(string); ok {
					return s
				}
			}
			return tokenRedactKey
		}
		if _, isSafe := opts.Rules.SafeKeys[key]; !isSafe && opts.Strict {
			stats["[UNKNOWN_KEY]"]++
			if v := unknownKeyValue(trimmed, opts); v != nil {
				if s, ok := v.(string); ok {
					return s
				}
			}
			return tokenRedactUnknown
		}
	}
	return contentOnly(trimmed, opts, stats)
}

// writeSQLBlock re-emits a header + separator + rows + optional footer,
// recomputing column widths from the redacted cells so opaque markers
// like "[REDACTED:UNKNOWN_KEY]" don't overrun their columns.
func writeSQLBlock(w *bufio.Writer, headers []string, rows [][]string, footer string) error {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for j := 0; j < len(headers) && j < len(row); j++ {
			cell := strings.TrimSpace(row[j])
			if len(cell) > widths[j] {
				widths[j] = len(cell)
			}
		}
	}
	if err := writeSQLRow(w, headers, widths); err != nil {
		return err
	}
	if err := writeSQLSeparator(w, widths); err != nil {
		return err
	}
	for _, row := range rows {
		// Normalize row to header column count.
		normalized := make([]string, len(headers))
		for j := range headers {
			if j < len(row) {
				normalized[j] = strings.TrimSpace(row[j])
			}
		}
		if err := writeSQLRow(w, normalized, widths); err != nil {
			return err
		}
	}
	if footer != "" {
		if _, err := w.WriteString(footer); err != nil {
			return err
		}
		if err := w.WriteByte('\n'); err != nil {
			return err
		}
	}
	return nil
}

func writeSQLRow(w *bufio.Writer, cells []string, widths []int) error {
	for i, width := range widths {
		var val string
		if i < len(cells) {
			val = cells[i]
		}
		if i > 0 {
			if err := w.WriteByte('|'); err != nil {
				return err
			}
		}
		if err := w.WriteByte(' '); err != nil {
			return err
		}
		if _, err := w.WriteString(val); err != nil {
			return err
		}
		if pad := width - len(val); pad > 0 {
			for k := 0; k < pad; k++ {
				if err := w.WriteByte(' '); err != nil {
					return err
				}
			}
		}
		if err := w.WriteByte(' '); err != nil {
			return err
		}
	}
	return w.WriteByte('\n')
}

func writeSQLSeparator(w *bufio.Writer, widths []int) error {
	for i, width := range widths {
		if i > 0 {
			if err := w.WriteByte('+'); err != nil {
				return err
			}
		}
		for k := 0; k < width+2; k++ {
			if err := w.WriteByte('-'); err != nil {
				return err
			}
		}
	}
	return w.WriteByte('\n')
}
