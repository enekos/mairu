package redact

import (
	"math"
	"regexp"
	"strings"
)

// tokenRe matches a contiguous run of base64/hex/url-safe characters. We
// scan these candidates and measure Shannon entropy.
var tokenRe = regexp.MustCompile(`[A-Za-z0-9+/=_\-]{8,}`)

// gitSHARe matches full 40-char git SHAs (lowercase hex). Short SHAs don't
// pass the minEntropyLen gate, so we only allowlist the 40-char form.
var gitSHARe = regexp.MustCompile(`^[0-9a-f]{40}$`)

// uuidRe matches canonical UUIDs (8-4-4-4-12 hex groups).
var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func shannonEntropy(s string) float64 {
	if s == "" {
		return 0
	}
	counts := make(map[rune]int, len(s))
	for _, r := range s {
		counts[r]++
	}
	n := float64(len(s))
	var h float64
	for _, c := range counts {
		p := float64(c) / n
		h -= p * math.Log2(p)
	}
	return h
}

// scanEntropy redacts high-entropy substrings. UUIDs and full git SHAs are
// allowlisted because they are structurally indistinguishable from random
// blobs but operationally non-sensitive.
func (r *Redactor) scanEntropy(input string) (string, []Finding) {
	out := input
	var findings []Finding

	matches := tokenRe.FindAllStringIndex(out, -1)
	for i := len(matches) - 1; i >= 0; i-- {
		loc := matches[i]
		tok := out[loc[0]:loc[1]]
		if len(tok) < r.minEntropyLen {
			continue
		}
		if isAllowlisted(tok) {
			continue
		}
		if shannonEntropy(tok) < r.entropyThreshold {
			continue
		}
		if strings.Contains(tok, "REDACTED") {
			continue
		}
		findings = append(findings, Finding{
			Layer: LayerEntropy,
			Kind:  "high_entropy",
			Start: loc[0],
			End:   loc[1],
		})
		out = out[:loc[0]] + "[REDACTED:high_entropy]" + out[loc[1]:]
	}
	return out, findings
}

func isAllowlisted(tok string) bool {
	return gitSHARe.MatchString(tok) || uuidRe.MatchString(tok)
}
