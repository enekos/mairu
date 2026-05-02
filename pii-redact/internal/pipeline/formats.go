package pipeline

import (
	"sort"
	"strings"
)

// secretUpperKeywords is the set of keywords used by the dotenv scanner.
var secretUpperKeywords = []string{
	"TOKEN", "SECRET", "KEY", "PASSWORD", "PASSWD", "PASS", "AUTH",
	"CREDENTIAL", "APIKEY", "ACCESS_TOKEN", "PRIVATE", "DSN", "CONNECTION", "CONN",
}

// secretLowerKeywords is the set of keywords used by the yaml scanner.
var secretLowerKeywords = []string{
	"token", "secret", "key", "password", "passwd", "pass", "auth",
	"credential", "apikey", "api_key", "access_token", "access-token", "private", "dsn",
}

// httpHeaderNames is the exact list of header names matched by the old regex.
var httpHeaderNames = []string{
	"Authorization", "Proxy-Authorization", "Cookie", "Set-Cookie",
}

// hasSecretUpperKeyword reports whether s (assumed to be an UPPERCASE
// identifier) contains any of the secret keywords as a substring.
func hasSecretUpperKeyword(s string) bool {
	for _, kw := range secretUpperKeywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

// hasSecretLowerKeyword reports whether s (assumed to be a lowercase
// identifier) contains any of the secret keywords as a substring.
func hasSecretLowerKeyword(s string) bool {
	for _, kw := range secretLowerKeywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

// isHTTPHeaderName reports whether s equals one of the known header names
// or matches the X-Custom-(Auth|Key|Token|Secret) pattern.
func isHTTPHeaderName(s string) bool {
	for _, h := range httpHeaderNames {
		if s == h {
			return true
		}
	}
	if !strings.HasPrefix(s, "X-") {
		return false
	}
	lower := strings.ToLower(s)
	return strings.Contains(lower, "auth") || strings.Contains(lower, "key") ||
		strings.Contains(lower, "token") || strings.Contains(lower, "secret")
}

func scanFormats(input string) (string, []Finding) {
	ivs := make([]interval, 0, 4)

	// .env / export KEY=VALUE
	scanDotenvPairs(input, &ivs)

	// YAML key: value
	scanYamlPairs(input, &ivs)

	// HTTP headers
	scanHTTPHeaders(input, &ivs)

	// Connection URIs with embedded basic-auth (hand-rolled; faster than regex
	// with backtracking).
	for _, iv := range findConnURIs(input) {
		if overlapsInterval(ivs, iv.start, iv.end) {
			continue
		}
		ivs = append(ivs, iv)
	}

	if len(ivs) > 1 {
		sort.Slice(ivs, func(i, j int) bool { return ivs[i].start < ivs[j].start })
	}
	out := applyIntervals(input, ivs)
	findings := make([]Finding, len(ivs))
	for i, iv := range ivs {
		findings[i] = Finding{
			Layer: LayerFormat,
			Kind:  iv.kind,
			Start: iv.start,
			End:   iv.end,
		}
	}
	return out, findings
}

// scanDotenvPairs walks input line-by-line looking for KEY=VALUE where KEY
// contains a secret keyword.  It preserves the exact byte positions so that
// intervals can be applied correctly.
func scanDotenvPairs(input string, ivs *[]interval) {
	start := 0
	for start <= len(input) {
		end := start
		for end < len(input) && input[end] != '\n' {
			end++
		}
		line := input[start:end]

		rest := line
		prefix := ""
		if strings.HasPrefix(rest, "export ") {
			prefix = "export "
			rest = rest[len(prefix):]
		}
		eq := strings.IndexByte(rest, '=')
		if eq > 0 {
			key := strings.TrimSpace(rest[:eq])
			if len(key) > 0 && key[0] >= 'A' && key[0] <= 'Z' && hasSecretUpperKeyword(key) {
				valStart := start + len(prefix) + eq + 1
				// Trim leading spaces after '='
				for valStart < end && input[valStart] == ' ' {
					valStart++
				}
				if valStart < end {
					*ivs = append(*ivs, interval{
						start: valStart,
						end:   end,
						kind:  "dotenv_pair",
						text:  "[REDACTED:dotenv_pair]",
					})
				}
			}
		}

		start = end + 1
	}
}

// scanYamlPairs walks input line-by-line looking for key: value where key
// contains a secret keyword.  Handles double-quoted, single-quoted and bare
// values (stops at end of line or # comment).
func scanYamlPairs(input string, ivs *[]interval) {
	start := 0
	for start <= len(input) {
		end := start
		for end < len(input) && input[end] != '\n' {
			end++
		}
		line := input[start:end]

		// Skip leading whitespace and record indent length.
		pos := 0
		for pos < len(line) && (line[pos] == ' ' || line[pos] == '\t') {
			pos++
		}
		if pos >= len(line) {
			start = end + 1
			continue
		}

		// Find ':' separating key from value.
		colon := strings.IndexByte(line[pos:], ':')
		if colon < 0 {
			start = end + 1
			continue
		}
		colon += pos

		key := strings.TrimSpace(line[pos:colon])
		if len(key) == 0 || key[0] < 'a' || key[0] > 'z' || !hasSecretLowerKeyword(key) {
			start = end + 1
			continue
		}

		// Position just after ':'.
		valPos := colon + 1
		for valPos < len(line) && (line[valPos] == ' ' || line[valPos] == '\t') {
			valPos++
		}
		if valPos >= len(line) {
			start = end + 1
			continue
		}

		var valStart, valEnd int
		switch line[valPos] {
		case '"':
			// Double-quoted value.
			q := strings.IndexByte(line[valPos+1:], '"')
			if q < 0 {
				start = end + 1
				continue
			}
			valStart = start + valPos + 1
			valEnd = valStart + q
		case '\'':
			// Single-quoted value.
			q := strings.IndexByte(line[valPos+1:], '\'')
			if q < 0 {
				start = end + 1
				continue
			}
			valStart = start + valPos + 1
			valEnd = valStart + q
		default:
			// Bare value — read until comment or end of line.
			valStart = start + valPos
			valEnd = end
			for valEnd > valStart && (input[valEnd-1] == ' ' || input[valEnd-1] == '\t') {
				valEnd--
			}
			// Truncate at inline comment.
			if hash := strings.IndexByte(line[valPos:], '#'); hash >= 0 {
				candidate := start + valPos + hash
				if candidate > valStart {
					valEnd = candidate
				}
			}
			for valEnd > valStart && (input[valEnd-1] == ' ' || input[valEnd-1] == '\t') {
				valEnd--
			}
		}

		if valStart < valEnd {
			*ivs = append(*ivs, interval{
				start: valStart,
				end:   valEnd,
				kind:  "yaml_pair",
				text:  "[REDACTED:yaml_pair]",
			})
		}

		start = end + 1
	}
}

// scanHTTPHeaders walks input line-by-line looking for known HTTP header
// names followed by ':'.
func scanHTTPHeaders(input string, ivs *[]interval) {
	start := 0
	for start <= len(input) {
		end := start
		for end < len(input) && input[end] != '\n' {
			end++
		}
		line := input[start:end]

		colon := strings.IndexByte(line, ':')
		if colon > 0 {
			name := strings.TrimSpace(line[:colon])
			if isHTTPHeaderName(name) {
				valStart := start + colon + 1
				for valStart < end && (input[valStart] == ' ' || input[valStart] == '\t') {
					valStart++
				}
				if valStart < end {
					*ivs = append(*ivs, interval{
						start: valStart,
						end:   end,
						kind:  "http_header",
						text:  "[REDACTED:http_header]",
					})
				}
			}
		}

		start = end + 1
	}
}
