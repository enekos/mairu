package pipeline

import "strings"

// findConnURIs finds connection URIs with embedded credentials
// (scheme://user:pass@ or scheme://:pass@) using a hand-rolled scanner.
// This avoids the backtracking overhead of the equivalent regex.
func findConnURIs(input string) []interval {
	var ivs []interval
	start := 0
	for {
		i := strings.Index(input[start:], "://")
		if i < 0 {
			break
		}
		colonSlashSlash := start + i

		// Walk backwards to find the scheme start.
		schemeStart := colonSlashSlash - 1
		for schemeStart >= 0 {
			c := input[schemeStart]
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
				c == '+' || c == '.' || c == '-' {
				schemeStart--
			} else {
				break
			}
		}
		schemeStart++
		if schemeStart >= colonSlashSlash {
			start = colonSlashSlash + 3
			continue
		}
		// Scheme must start with a letter.
		first := input[schemeStart]
		if !((first >= 'A' && first <= 'Z') || (first >= 'a' && first <= 'z')) {
			start = colonSlashSlash + 3
			continue
		}

		// Find the next '@' after ://
		at := strings.IndexByte(input[colonSlashSlash+3:], '@')
		if at < 0 {
			break
		}
		at += colonSlashSlash + 3

		// Find the ':' between :// and @.
		colon := -1
		for j := at - 1; j > colonSlashSlash+2; j-- {
			if input[j] == ':' {
				colon = j
				break
			}
		}
		if colon < 0 {
			start = colonSlashSlash + 3
			continue
		}

		// Validate user segment (between :// and :) has no ws/@/
		valid := true
		for j := colonSlashSlash + 3; j < colon; j++ {
			c := input[j]
			if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '@' || c == '/' {
				valid = false
				break
			}
		}
		if !valid {
			start = colonSlashSlash + 3
			continue
		}

		// Validate password segment (between : and @) has no ws/@/
		for j := colon + 1; j < at; j++ {
			c := input[j]
			if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '@' || c == '/' {
				valid = false
				break
			}
		}
		if !valid {
			start = colonSlashSlash + 3
			continue
		}

		scheme := input[schemeStart:colonSlashSlash]
		ivs = append(ivs, interval{
			start: schemeStart,
			end:   at + 1,
			kind:  "conn_uri",
			text:  scheme + "://[REDACTED:conn_uri]@",
		})
		start = at + 1
	}
	return ivs
}
