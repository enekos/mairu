package pipeline

import (
	"regexp"
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
var freeTextPatterns = []freeTextPattern{
	{
		kind: "email",
		re:   regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`),
		validator: func(s string) bool {
			at := strings.LastIndexByte(s, '@')
			if at <= 0 || at >= len(s)-3 {
				return false
			}
			domain := s[at+1:]
			return strings.ContainsRune(domain, '.')
		},
		mask: maskEmail,
	},
	{
		kind: "iban",
		// 2-letter country + 2 check digits + up to 30 alnum.
		re:        regexp.MustCompile(`\b[A-Z]{2}[0-9]{2}[A-Z0-9]{10,30}\b`),
		validator: mask.ValidIBAN,
		mask: func(s string) string {
			if len(s) < 10 {
				return "[REDACTED:iban]"
			}
			return s[:4] + strings.Repeat("*", len(s)-8) + s[len(s)-4:]
		},
	},
	{
		kind: "credit_card",
		// 13-19 digits with optional space/dash separators.
		re:        regexp.MustCompile(`\b(?:\d[ \-]?){12,18}\d\b`),
		validator: mask.ValidLuhn,
		mask: func(s string) string {
			digits := stripSep(s)
			if len(digits) < 4 {
				return "[REDACTED:credit_card]"
			}
			return strings.Repeat("*", len(digits)-4) + digits[len(digits)-4:]
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
			return s[:3] + strings.Repeat("*", len(s)-5) + s[len(s)-2:]
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
	out := input
	var findings []Finding
	for _, p := range freeTextPatterns {
		locs := p.re.FindAllStringIndex(out, -1)
		if len(locs) == 0 {
			continue
		}
		for i := len(locs) - 1; i >= 0; i-- {
			loc := locs[i]
			raw := out[loc[0]:loc[1]]
			if strings.Contains(raw, "REDACTED") {
				continue
			}
			if p.validator != nil && !p.validator(raw) {
				continue
			}
			masked := p.mask(raw)
			findings = append(findings, Finding{
				Layer: LayerFreeText,
				Kind:  p.kind,
				Start: loc[0],
				End:   loc[1],
			})
			out = out[:loc[0]] + masked + out[loc[1]:]
		}
	}
	return out, findings
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
		return strings.Repeat("*", len(s))
	case len(s) <= 4:
		return s[:1] + strings.Repeat("*", len(s)-1)
	default:
		return s[:1] + strings.Repeat("*", len(s)-2) + s[len(s)-1:]
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
