package pipeline

import (
	"math"
	"regexp"
	"strings"
)

var tokenRe = regexp.MustCompile(`[A-Za-z0-9+/=_\-]{8,}`)
var gitSHARe = regexp.MustCompile(`^[0-9a-f]{40}$`)
var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// shannonEntropyASCII computes Shannon entropy over the raw bytes of s.
// Non-ASCII runes are counted per-byte; callers should only pass inputs
// that tokenRe has already restricted to the ASCII alphanumeric set.
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

	matches := tokenRe.FindAllStringIndex(input, -1)
	var ivs []interval
	for _, loc := range matches {
		tok := input[loc[0]:loc[1]]
		if len(tok) < minLen {
			continue
		}
		if gitSHARe.MatchString(tok) || uuidRe.MatchString(tok) {
			continue
		}
		if shannonEntropyASCII(tok) < threshold {
			continue
		}
		if strings.Contains(tok, "REDACTED") {
			continue
		}
		if overlapsInterval(ivs, loc[0], loc[1]) {
			continue
		}
		ivs = append(ivs, interval{
			start: loc[0],
			end:   loc[1],
			kind:  "high_entropy",
			text:  "[REDACTED:high_entropy]",
		})
	}
	out := applyIntervals(input, ivs)
	findings := make([]Finding, len(ivs))
	for i, iv := range ivs {
		findings[i] = Finding{
			Layer: LayerEntropy,
			Kind:  iv.kind,
			Start: iv.start,
			End:   iv.end,
		}
	}
	return out, findings
}
