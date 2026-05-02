package pipeline

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/enekos/mairu/pii-redact/internal/mask"
)

type freeTextPattern struct {
	kind      string
	re        *regexp.Regexp
	validator func(string) bool
	mask      func(string) string
}

// Regexes deliberately overshoot and the validator filters false-positives.
// Masking reuses the partial-reveal shapes defined in internal/mask so the
// pipeline output stays consistent with JSON-walker output.
//
// Emails are handled separately by findEmailsFast (hand-rolled scanner).
var freeTextPatterns = []freeTextPattern{
	{
		kind: "iban",
		// 2-letter country + 2 check digits + up to 30 alnum.
		re:        regexp.MustCompile(`\b[A-Z]{2}[0-9]{2}[A-Z0-9]{10,30}\b`),
		validator: mask.ValidIBAN,
		mask: func(s string) string {
			if len(s) < 10 {
				return "[REDACTED:iban]"
			}
			return s[:4] + stars(len(s)-8) + s[len(s)-4:]
		},
	},
	{
		kind: "ssn_us",
		re:   regexp.MustCompile(`\b(\d{3})-(\d{2})-(\d{4})\b`),
		validator: func(s string) bool {
			// reject obvious sentinels / invalid area/group/serial.
			parts := strings.Split(s, "-")
			if len(parts) != 3 {
				return false
			}
			area, _ := strconv.Atoi(parts[0])
			group, _ := strconv.Atoi(parts[1])
			serial, _ := strconv.Atoi(parts[2])
			if area == 0 || area == 666 || area >= 900 {
				return false
			}
			if group == 0 || serial == 0 {
				return false
			}
			return true
		},
		mask: func(s string) string { return "***-**-" + s[len(s)-4:] },
	},
	{
		kind: "phone_e164",
		// +<country><number>, 7-15 digits per E.164.
		re: regexp.MustCompile(`\+[1-9]\d{6,14}\b`),
		validator: func(s string) bool {
			// Length-gated to reject short false-positives; keep all E.164.
			return len(s) >= 8
		},
		mask: func(s string) string {
			if len(s) < 6 {
				return "[REDACTED:phone]"
			}
			return s[:3] + stars(len(s)-5) + s[len(s)-2:]
		},
	},
	{
		kind: "ipv4_public",
		re:   regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
		validator: func(s string) bool {
			octets := strings.Split(s, ".")
			if len(octets) != 4 {
				return false
			}
			var v [4]int
			for i, o := range octets {
				// No leading-zero octets (0.0.0.0 allowed, 01.x rejected).
				if len(o) > 1 && o[0] == '0' {
					return false
				}
				n, err := strconv.Atoi(o)
				if err != nil || n < 0 || n > 255 {
					return false
				}
				v[i] = n
			}
			// Reject private / loopback / link-local / CGNAT / multicast / reserved.
			switch {
			case v[0] == 10:
				return false
			case v[0] == 127:
				return false
			case v[0] == 0:
				return false
			case v[0] == 169 && v[1] == 254:
				return false
			case v[0] == 172 && v[1] >= 16 && v[1] <= 31:
				return false
			case v[0] == 192 && v[1] == 168:
				return false
			case v[0] == 100 && v[1] >= 64 && v[1] <= 127:
				return false
			case v[0] >= 224:
				return false
			}
			return true
		},
		mask: func(s string) string {
			parts := strings.Split(s, ".")
			if len(parts) != 4 {
				return "[REDACTED:ipv4]"
			}
			return parts[0] + "." + parts[1] + ".*.*"
		},
	},
}

func scanFreeText(input string) (string, []Finding) {
	ivs := make([]interval, 0, 4)

	for _, iv := range findEmailsFast(input) {
		raw := input[iv.start:iv.end]
		if strings.Contains(raw, "REDACTED") {
			continue
		}
		ivs = append(ivs, iv)
	}

	// Hand-rolled credit-card scanner avoids expensive regex backtracking.
	if hasCCProbe(input) {
		for _, iv := range findCreditCardsFast(input) {
			if overlapsInterval(ivs, iv.start, iv.end) {
				continue
			}
			ivs = append(ivs, iv)
		}
	}

	// Quick probes to skip regexes when no candidate substrings exist.
	hasPlus := strings.Contains(input, "+")
	hasDash := strings.Contains(input, "-")
	dotCount := strings.Count(input, ".")
	hasIBAN := hasIBANProbe(input)

	for _, p := range freeTextPatterns {
		switch p.kind {
		case "iban":
			if !hasIBAN {
				continue
			}
		case "ssn_us":
			if !hasDash {
				continue
			}
		case "phone_e164":
			if !hasPlus {
				continue
			}
		case "ipv4_public":
			if dotCount < 3 {
				continue
			}
		}
		locs := p.re.FindAllStringIndex(input, -1)
		if len(locs) == 0 {
			continue
		}
		for _, loc := range locs {
			raw := input[loc[0]:loc[1]]
			if strings.Contains(raw, "REDACTED") {
				continue
			}
			if p.validator != nil && !p.validator(raw) {
				continue
			}
			if overlapsInterval(ivs, loc[0], loc[1]) {
				continue
			}
			ivs = append(ivs, interval{
				start: loc[0],
				end:   loc[1],
				kind:  p.kind,
				text:  p.mask(raw),
			})
		}
	}
	if len(ivs) > 1 {
		sort.Slice(ivs, func(i, j int) bool { return ivs[i].start < ivs[j].start })
	}
	out := applyIntervals(input, ivs)
	findings := make([]Finding, len(ivs))
	for i, iv := range ivs {
		findings[i] = Finding{
			Layer: LayerFreeText,
			Kind:  iv.kind,
			Start: iv.start,
			End:   iv.end,
		}
	}
	return out, findings
}

// hasIBANProbe reports whether s contains 2 uppercase letters followed by 2 digits.
func hasIBANProbe(s string) bool {
	for i := 0; i+3 < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' && s[i+1] >= 'A' && s[i+1] <= 'Z' &&
			s[i+2] >= '0' && s[i+2] <= '9' && s[i+3] >= '0' && s[i+3] <= '9' {
			return true
		}
	}
	return false
}

// hasCCProbe reports whether s contains at least 12 digits total.
func hasCCProbe(s string) bool {
	count := 0
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			count++
			if count >= 12 {
				return true
			}
		}
	}
	return false
}

func maskEmail(raw string) string {
	at := strings.LastIndex(raw, "@")
	if at <= 0 || at >= len(raw)-1 {
		return "[REDACTED:email]"
	}
	local := keepEnds(raw[:at])
	domain := raw[at+1:]
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return local + "@" + keepEnds(domain)
	}
	head := strings.Join(parts[:len(parts)-1], ".")
	tld := parts[len(parts)-1]
	return local + "@" + keepEnds(head) + "." + tld
}

func keepEnds(s string) string {
	switch {
	case len(s) == 0:
		return ""
	case len(s) <= 2:
		return stars(len(s))
	case len(s) <= 4:
		return s[:1] + stars(len(s)-1)
	default:
		return s[:1] + stars(len(s)-2) + s[len(s)-1:]
	}
}

func stripSep(s string) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			b = append(b, c)
		}
	}
	return string(b)
}

func maskCreditCard(s string) string {
	digits := stripSep(s)
	if len(digits) < 4 {
		return "[REDACTED:credit_card]"
	}
	return stars(len(digits)-4) + digits[len(digits)-4:]
}

// findCreditCardsFast scans for 13-19 digit sequences with optional
// space/dash separators, validates with Luhn, and returns intervals.
func findCreditCardsFast(input string) []interval {
	var ivs []interval
	i := 0
	for i < len(input) {
		// Look for a digit to start a candidate.
		for i < len(input) && (input[i] < '0' || input[i] > '9') {
			i++
		}
		if i >= len(input) {
			break
		}
		start := i
		digitCount := 0
		for i < len(input) {
			c := input[i]
			if c >= '0' && c <= '9' {
				digitCount++
				i++
			} else if c == ' ' || c == '-' {
				i++
			} else {
				break
			}
		}
		if digitCount < 13 || digitCount > 19 {
			continue
		}
		// Trim trailing spaces/dashes so the match ends on a digit.
		end := i
		for end > start && (input[end-1] == ' ' || input[end-1] == '-') {
			end--
		}
		candidate := input[start:end]
		if !mask.ValidLuhn(candidate) {
			continue
		}
		// Word boundary: previous char must not be a word char.
		if start > 0 {
			prev := input[start-1]
			if (prev >= '0' && prev <= '9') || (prev >= 'A' && prev <= 'Z') || (prev >= 'a' && prev <= 'z') || prev == '_' {
				continue
			}
		}
		// Word boundary: next char must not be a word char.
		if end < len(input) {
			next := input[end]
			if (next >= '0' && next <= '9') || (next >= 'A' && next <= 'Z') || (next >= 'a' && next <= 'z') || next == '_' {
				continue
			}
		}
		ivs = append(ivs, interval{
			start: start,
			end:   end,
			kind:  "credit_card",
			text:  maskCreditCard(candidate),
		})
	}
	return ivs
}
