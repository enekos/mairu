package mask

import (
	"encoding/base64"
	"strconv"
	"strings"
)

// Validator returns true if `raw` is a plausible instance of the named
// pattern. Validators exist to cut false positives from greedy regex
// (e.g. version strings matching the ipv4 pattern).
type Validator func(raw string) bool

// Validators is the default validator table. Patterns without an entry
// are always accepted.
var Validators = map[string]Validator{
	"ipv4":           validIPv4,
	"credit_card":    validLuhn,
	"jwt":            validJWT,
	"iban":           validIBAN,
	"eth_address":    validEth,
	"basic_auth_url": validBasicAuth,
}

// Exported aliases so packages outside mask (e.g. pipeline) can reuse the
// same validator logic without duplicating it.
var (
	ValidIPv4      = validIPv4
	ValidLuhn      = validLuhn
	ValidJWT       = validJWT
	ValidIBAN      = validIBAN
	ValidEth       = validEth
	ValidBasicAuth = validBasicAuth
)

func validIPv4(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	allZero := true
	for _, p := range parts {
		if len(p) == 0 || len(p) > 3 {
			return false
		}
		// reject octets with leading zero unless single digit; avoids
		// semver "01.02.03.04" false positives.
		if len(p) > 1 && p[0] == '0' {
			return false
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || n > 255 {
			return false
		}
		if n != 0 {
			allZero = false
		}
	}
	return !allZero
}

func validLuhn(s string) bool {
	var digits []int
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits = append(digits, int(r-'0'))
		} else if r == ' ' || r == '-' {
			continue
		} else {
			return false
		}
	}
	if len(digits) < 13 || len(digits) > 19 {
		return false
	}
	sum := 0
	for i, d := range digits {
		if (len(digits)-1-i)%2 == 1 {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	return sum%10 == 0
}

func validJWT(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return false
	}
	// header must be valid base64url and start with '{'
	hdr, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		// try std
		hdr, err = base64.StdEncoding.DecodeString(parts[0])
		if err != nil {
			return false
		}
	}
	if len(hdr) == 0 || hdr[0] != '{' {
		return false
	}
	return len(parts[2]) >= 1
}

func validIBAN(s string) bool {
	if len(s) < 15 || len(s) > 34 {
		return false
	}
	// mod-97 check: move first 4 chars to end, letters->digits (A=10..Z=35)
	rearranged := s[4:] + s[:4]
	var n strings.Builder
	for _, r := range rearranged {
		switch {
		case r >= '0' && r <= '9':
			n.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			n.WriteString(strconv.Itoa(int(r-'A') + 10))
		default:
			return false
		}
	}
	// compute mod 97 piecewise (number too big for int64 in general)
	rem := 0
	for _, c := range n.String() {
		rem = (rem*10 + int(c-'0')) % 97
	}
	return rem == 1
}

func validEth(s string) bool {
	if len(s) != 42 || s[:2] != "0x" {
		return false
	}
	for _, r := range s[2:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

func validBasicAuth(s string) bool {
	// cheap sanity: must contain "://" then ":" then "@"
	i := strings.Index(s, "://")
	if i < 0 {
		return false
	}
	rest := s[i+3:]
	colon := strings.IndexByte(rest, ':')
	at := strings.IndexByte(rest, '@')
	return colon > 0 && at > colon
}
