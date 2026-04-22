package pipeline

import "strings"

// findEmailsFast scans for email addresses by anchoring on '@' and expanding
// over email-safe bytes; it requires a final dot with a 2+ char TLD.
// Hand-rolled to avoid regex backtracking on adversarial inputs.
func findEmailsFast(input string) []interval {
	var ivs []interval
	start := 0
	for {
		i := strings.IndexByte(input[start:], '@')
		if i < 0 {
			break
		}
		at := start + i

		// Expand left over email-safe bytes.
		left := at - 1
		for left >= 0 && isEmailByte(input[left]) {
			left--
		}
		left++

		// Expand right over email-safe bytes, but never cross another '@'.
		right := at + 1
		for right < len(input) && isEmailByte(input[right]) && input[right] != '@' {
			right++
		}

		if at-left <= 0 || right-at <= 3 {
			start = at + 1
			continue
		}

		// Require a dot in the domain with at least 2 chars after the last dot.
		domain := input[at+1 : right]
		lastDot := strings.LastIndexByte(domain, '.')
		if lastDot < 0 || len(domain)-lastDot-1 < 2 {
			start = at + 1
			continue
		}

		ivs = append(ivs, interval{
			start: left,
			end:   right,
			kind:  "email",
			text:  maskEmail(input[left:right]),
		})
		start = right
	}
	return ivs
}

func isEmailByte(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
		c == '.' || c == '_' || c == '%' || c == '+' || c == '-'
}
