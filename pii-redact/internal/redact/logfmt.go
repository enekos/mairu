package redact

import (
	"bufio"
	"io"
	"strings"

	"github.com/join-com/pii-redact/internal/patterns"
)

// Logfmt streams `in` to `out` parsing each line as key=value pairs
// (Heroku-style logfmt). Known redact_keys get masked; other values run
// through content regex. Unquoted or missing parts fall back to running
// content regex on the remainder so stray free text still gets scrubbed.
func Logfmt(in io.Reader, out io.Writer, opts Options) (patterns.Stats, error) {
	totals := patterns.Stats{}
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
	bw := bufio.NewWriter(out)
	defer bw.Flush()

	for scanner.Scan() {
		line := scanner.Text()
		redacted := redactLogfmtLine(line, opts, totals)
		if _, err := bw.WriteString(redacted); err != nil {
			return totals, err
		}
		if err := bw.WriteByte('\n'); err != nil {
			return totals, err
		}
	}
	return totals, scanner.Err()
}

func redactLogfmtLine(line string, opts Options, stats patterns.Stats) string {
	var sb strings.Builder
	i := 0
	n := len(line)
	for i < n {
		// skip whitespace
		for i < n && (line[i] == ' ' || line[i] == '\t') {
			sb.WriteByte(line[i])
			i++
		}
		if i >= n {
			break
		}
		// read key
		start := i
		for i < n && line[i] != '=' && line[i] != ' ' && line[i] != '\t' {
			i++
		}
		key := line[start:i]
		if i >= n || line[i] != '=' {
			// bare token — run content regex on it
			sb.WriteString(contentOnly(key, opts, stats))
			continue
		}
		sb.WriteString(key)
		sb.WriteByte('=')
		i++ // consume '='
		// read value: quoted or bare
		val, consumed, quoted := readLogfmtValue(line[i:])
		i += consumed
		masked := maskLogfmtValue(key, val, opts, stats)
		if quoted {
			sb.WriteByte('"')
			sb.WriteString(strings.ReplaceAll(masked, `"`, `\"`))
			sb.WriteByte('"')
		} else {
			sb.WriteString(masked)
		}
	}
	return sb.String()
}

func readLogfmtValue(s string) (val string, consumed int, quoted bool) {
	if len(s) == 0 {
		return "", 0, false
	}
	if s[0] == '"' {
		// quoted — honor backslash escapes
		var sb strings.Builder
		i := 1
		for i < len(s) {
			if s[i] == '\\' && i+1 < len(s) {
				sb.WriteByte(s[i+1])
				i += 2
				continue
			}
			if s[i] == '"' {
				return sb.String(), i + 1, true
			}
			sb.WriteByte(s[i])
			i++
		}
		return sb.String(), i, true
	}
	i := 0
	for i < len(s) && s[i] != ' ' && s[i] != '\t' {
		i++
	}
	return s[:i], i, false
}

func maskLogfmtValue(key, val string, opts Options, stats patterns.Stats) string {
	if val == "" {
		return val
	}
	if opts.Rules != nil {
		if _, ok := opts.Rules.RedactKeys[key]; ok {
			stats["[KEY]"]++
			m := maskKeyedValue(key, val, opts)
			if s, ok := m.(string); ok {
				return s
			}
			return tokenRedactKey
		}
	}
	return contentOnly(val, opts, stats)
}

func contentOnly(s string, opts Options, stats patterns.Stats) string {
	if opts.Set == nil {
		return s
	}
	out, match := opts.Set.Redact(s)
	for k, v := range match {
		stats[k] += v
	}
	return out
}
