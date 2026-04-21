// Package mask produces partial-reveal tokens for matched PII. The goal
// is "readable enough to distinguish entries" without letting the raw
// value survive: e.g. email "john.doe@acme.io" -> "j***e@a***e.io",
// ipv4 "10.0.0.5" -> "10.0.*.*", credit card "4111 1111 1111 1111" ->
// "****-****-****-1111". If a strategy rejects the input (too short,
// malformed), we fall back to the opaque "[REDACTED:<name>]" token so
// we never leak more than the caller intended.
package mask

import (
	"fmt"
	"regexp"
	"strings"
)

// Strategy is the partial-reveal function for one named pattern. It
// receives the raw matched value and returns the display string. If it
// returns ok=false the caller uses the opaque fallback.
type Strategy func(raw string) (masked string, ok bool)

// Masker holds per-pattern strategies plus a mode flag.
type Masker struct {
	// Reveal: when false, every match renders as "[REDACTED:<name>]".
	// When true we try the strategy and fall back to the opaque token.
	Reveal     bool
	strategies map[string]Strategy
}

// NewMasker returns a masker with the default reveal strategies
// registered. Callers can override any strategy before use.
func NewMasker(reveal bool) *Masker {
	m := &Masker{Reveal: reveal, strategies: map[string]Strategy{}}
	for k, v := range defaults {
		m.strategies[k] = v
	}
	return m
}

// Register overrides or adds a strategy.
func (m *Masker) Register(name string, s Strategy) {
	m.strategies[name] = s
}

// Apply returns the display token for a pattern match.
func (m *Masker) Apply(name, raw string) string {
	if !m.Reveal {
		return opaque(name)
	}
	if s, ok := m.strategies[name]; ok {
		if out, ok2 := s(raw); ok2 {
			return out
		}
	}
	if out, ok := genericMask(raw); ok {
		return fmt.Sprintf("[REDACTED:%s:%s]", name, out)
	}
	return opaque(name)
}

func opaque(name string) string { return "[REDACTED:" + name + "]" }

// -- building blocks --------------------------------------------------

// keepEnds returns first+last char of s bracketing stars of length
// between them, collapsing to "***" if too short to reveal safely.
func keepEnds(s string) string {
	if len(s) <= 2 {
		return strings.Repeat("*", len(s))
	}
	if len(s) <= 4 {
		return s[:1] + strings.Repeat("*", len(s)-1)
	}
	return s[:1] + strings.Repeat("*", len(s)-2) + s[len(s)-1:]
}

// tailN returns "****<last n>". Refuses if s is shorter than n+2.
func tailN(s string, n int) (string, bool) {
	if len(s) < n+2 {
		return "", false
	}
	return strings.Repeat("*", len(s)-n) + s[len(s)-n:], true
}

func genericMask(s string) (string, bool) {
	if len(s) < 3 {
		return "", false
	}
	return keepEnds(s), true
}

// -- default strategies ----------------------------------------------

var defaults = map[string]Strategy{
	"email": func(raw string) (string, bool) {
		at := strings.LastIndex(raw, "@")
		if at <= 0 || at >= len(raw)-1 {
			return "", false
		}
		local := keepEnds(raw[:at])
		domain := raw[at+1:]
		// keep the TLD (last label) readable; mask everything before it.
		parts := strings.Split(domain, ".")
		if len(parts) < 2 {
			return local + "@" + keepEnds(domain), true
		}
		head := strings.Join(parts[:len(parts)-1], ".")
		tld := parts[len(parts)-1]
		return local + "@" + keepEnds(head) + "." + tld, true
	},

	"ipv4": func(raw string) (string, bool) {
		parts := strings.Split(raw, ".")
		if len(parts) != 4 {
			return "", false
		}
		return parts[0] + "." + parts[1] + ".*.*", true
	},

	"ipv6": func(raw string) (string, bool) {
		groups := strings.Split(raw, ":")
		if len(groups) < 3 {
			return "", false
		}
		return groups[0] + ":***:" + groups[len(groups)-1], true
	},

	"mac_address": func(raw string) (string, bool) {
		sep := ":"
		if strings.Contains(raw, "-") {
			sep = "-"
		}
		g := strings.Split(raw, sep)
		if len(g) != 6 {
			return "", false
		}
		return g[0] + sep + g[1] + sep + "**" + sep + "**" + sep + "**" + sep + g[5], true
	},

	"credit_card": func(raw string) (string, bool) {
		digits := onlyDigits(raw)
		if len(digits) < 13 || len(digits) > 19 {
			return "", false
		}
		return strings.Repeat("*", len(digits)-4) + digits[len(digits)-4:], true
	},

	"iban": func(raw string) (string, bool) {
		if len(raw) < 10 {
			return "", false
		}
		return raw[:4] + strings.Repeat("*", len(raw)-8) + raw[len(raw)-4:], true
	},

	"phone_e164": func(raw string) (string, bool) {
		if len(raw) < 6 {
			return "", false
		}
		// keep "+cc" (2-4 chars) and last 2 digits.
		return raw[:3] + strings.Repeat("*", len(raw)-5) + raw[len(raw)-2:], true
	},

	"phone_us": func(raw string) (string, bool) {
		d := onlyDigits(raw)
		if len(d) != 10 {
			return "", false
		}
		return "***-***-" + d[6:], true
	},

	"ssn_us": func(raw string) (string, bool) {
		if len(raw) != 11 {
			return "", false
		}
		return "***-**-" + raw[7:], true
	},

	"nino_uk": func(raw string) (string, bool) {
		if len(raw) != 9 {
			return "", false
		}
		return raw[:2] + "******" + raw[8:], true
	},

	"vat_eu": func(raw string) (string, bool) {
		if len(raw) < 6 {
			return "", false
		}
		return raw[:2] + strings.Repeat("*", len(raw)-4) + raw[len(raw)-2:], true
	},

	"jwt": func(raw string) (string, bool) {
		parts := strings.Split(raw, ".")
		if len(parts) != 3 {
			return "", false
		}
		hdr := parts[0]
		sig := parts[2]
		if len(hdr) < 6 {
			return "", false
		}
		tail := sig
		if len(sig) > 4 {
			tail = "***" + sig[len(sig)-4:]
		}
		return hdr[:6] + "…." + "***." + tail, true
	},

	"bearer": func(raw string) (string, bool) {
		// "Bearer <token>"
		idx := strings.IndexByte(raw, ' ')
		if idx < 0 || idx == len(raw)-1 {
			return "", false
		}
		tok := raw[idx+1:]
		if len(tok) < 4 {
			return "Bearer ****", true
		}
		return "Bearer ****" + tok[len(tok)-4:], true
	},

	"aws_access_key": func(raw string) (string, bool) {
		if len(raw) < 8 {
			return "", false
		}
		return raw[:4] + strings.Repeat("*", len(raw)-8) + raw[len(raw)-4:], true
	},

	"gcp_api_key": func(raw string) (string, bool) {
		if len(raw) < 8 {
			return "", false
		}
		return raw[:4] + strings.Repeat("*", len(raw)-8) + raw[len(raw)-4:], true
	},

	"google_oauth": func(raw string) (string, bool) {
		if len(raw) < 10 {
			return "", false
		}
		return "ya29." + strings.Repeat("*", len(raw)-9) + raw[len(raw)-4:], true
	},

	"github_token": func(raw string) (string, bool) {
		if len(raw) < 8 {
			return "", false
		}
		return raw[:4] + strings.Repeat("*", len(raw)-8) + raw[len(raw)-4:], true
	},

	"slack_token": func(raw string) (string, bool) {
		// keep "xoxb-" style prefix, mask the rest, keep last 4.
		dash := strings.IndexByte(raw, '-')
		if dash < 0 || len(raw)-dash < 5 {
			return "", false
		}
		return raw[:dash+1] + "****" + raw[len(raw)-4:], true
	},

	"stripe_key": func(raw string) (string, bool) {
		// sk_live_xxx / pk_test_xxx — keep 8-char prefix, last 4.
		if len(raw) < 14 {
			return "", false
		}
		return raw[:8] + strings.Repeat("*", len(raw)-12) + raw[len(raw)-4:], true
	},

	"private_key_pem": func(_ string) (string, bool) {
		return "[REDACTED:private_key_pem]", true
	},

	"basic_auth_url": func(raw string) (string, bool) {
		// https://user:pass@ -> https://***:***@
		i := strings.Index(raw, "://")
		at := strings.LastIndex(raw, "@")
		if i < 0 || at < 0 || at < i {
			return "", false
		}
		return raw[:i+3] + "***:***@", true
	},

	"latlong": func(raw string) (string, bool) {
		parts := strings.Split(raw, ",")
		if len(parts) != 2 {
			return "", false
		}
		lat := strings.TrimSpace(parts[0])
		lng := strings.TrimSpace(parts[1])
		return truncDecimal(lat, 0) + ", " + truncDecimal(lng, 0), true
	},

	"uuid": func(raw string) (string, bool) {
		if len(raw) != 36 {
			return "", false
		}
		return "********-****-****-****-" + raw[len(raw)-4:], true
	},

	"eth_address": func(raw string) (string, bool) {
		if len(raw) != 42 {
			return "", false
		}
		return raw[:6] + "…" + raw[len(raw)-4:], true
	},

	"azure_conn_str": func(_ string) (string, bool) {
		return "[REDACTED:azure_conn_str]", true
	},
}

var digitsRE = regexp.MustCompile(`\D+`)

func onlyDigits(s string) string { return digitsRE.ReplaceAllString(s, "") }

// truncDecimal returns a value with at most `frac` decimal digits. Used
// to coarsen lat/long so the city remains but the street does not.
func truncDecimal(s string, frac int) string {
	dot := strings.IndexByte(s, '.')
	if dot < 0 {
		return s
	}
	if frac <= 0 {
		return s[:dot]
	}
	end := dot + 1 + frac
	if end > len(s) {
		end = len(s)
	}
	return s[:end]
}
