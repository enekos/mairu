package pipeline

import (
	"math"
	"strings"
)

// shannonEntropyASCII computes Shannon entropy over the raw bytes of s.
// Non-ASCII runes are counted per-byte; callers should only pass inputs
// that have already been restricted to the ASCII alphanumeric set.
func shannonEntropyASCII(s string) float64 {
	if s == "" {
		return 0
	}
	var counts [256]int
	for i := 0; i < len(s); i++ {
		counts[s[i]]++
	}
	n := float64(len(s))
	var h float64
	for _, c := range counts[:] {
		if c == 0 {
			continue
		}
		p := float64(c) / n
		h -= p * math.Log2(p)
	}
	return h
}

// isTokenChar reports whether c is in the character class used by the
// old tokenRe: [A-Za-z0-9+/=_\-].
func isTokenChar(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
		c == '+' || c == '/' || c == '=' || c == '_' || c == '-'
}

// isGitSHA reports whether s looks like a full 40-char hexadecimal SHA-1.
func isGitSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// isUUID reports whether s matches the standard UUID layout.
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}

// scanEntropy redacts high-entropy substrings. UUIDs and full git SHAs are
// allowlisted because they are structurally indistinguishable from random
// blobs but operationally non-sensitive.
func scanEntropy(input string, threshold float64, minLen int) (string, []Finding) {
	if threshold <= 0 {
		threshold = 4.5
	}
	if minLen <= 0 {
		minLen = 20
	}

	var ivs []interval
	i := 0
	for i < len(input) {
		// Skip non-token chars.
		for i < len(input) && !isTokenChar(input[i]) {
			i++
		}
		if i >= len(input) {
			break
		}
		start := i
		for i < len(input) && isTokenChar(input[i]) {
			i++
		}
		runLen := i - start
		if runLen < minLen {
			continue
		}
		tok := input[start:i]
		if isGitSHA(tok) || isUUID(tok) {
			continue
		}
		if shannonEntropyASCII(tok) < threshold {
			continue
		}
		if strings.Contains(tok, "REDACTED") {
			continue
		}
		ivs = append(ivs, interval{
			start: start,
			end:   i,
			kind:  "high_entropy",
			text:  "[REDACTED:high_entropy]",
		})
	}

	out := applyIntervals(input, ivs)
	findings := make([]Finding, len(ivs))
	for j, iv := range ivs {
		findings[j] = Finding{
			Layer: LayerEntropy,
			Kind:  iv.kind,
			Start: iv.start,
			End:   iv.end,
		}
	}
	return out, findings
}
