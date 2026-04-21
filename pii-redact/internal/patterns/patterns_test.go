package patterns

import (
	"strings"
	"testing"
)

var defaultPatterns = map[string]string{
	"email":           `[\w.+-]+@[\w-]+\.[\w.-]+`,
	"phone_e164":      `\+\d{7,15}`,
	"iban":            `[A-Z]{2}\d{2}[A-Z0-9]{11,30}`,
	"credit_card":     `\b(?:\d[ -]*?){13,19}\b`,
	"jwt":             `eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`,
	"ipv4":            `\b(?:\d{1,3}\.){3}\d{1,3}\b`,
	"ipv6":            `\b(?:[0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}\b`,
	"bearer":          `Bearer\s+[A-Za-z0-9_.\-~+/=]+`,
	"ssn_us":          `\b\d{3}-\d{2}-\d{4}\b`,
	"nino_uk":         `\b[A-Z]{2}\d{6}[A-D]\b`,
	"vat_eu":          `\b(?:AT|BE|BG|CY|CZ|DE|DK|EE|EL|ES|FI|FR|HR|HU|IE|IT|LT|LU|LV|MT|NL|PL|PT|RO|SE|SI|SK|XI)[0-9A-Z]{8,12}\b`,
	"mac_address":     `\b(?:[0-9a-fA-F]{2}[:-]){5}[0-9a-fA-F]{2}\b`,
	"latlong":         `-?\d{1,3}\.\d{3,},\s*-?\d{1,3}\.\d{3,}`,
	"aws_access_key":  `\bAKIA[0-9A-Z]{16}\b`,
	"gcp_api_key":     `\bAIza[0-9A-Za-z_-]{35}\b`,
	"github_token":    `\bgh[pousr]_[A-Za-z0-9]{36,}\b`,
	"slack_token":     `xox[baprs]-[A-Za-z0-9-]{10,}`,
	"stripe_key":      `\b(?:sk|pk|rk)_(?:live|test)_[0-9a-zA-Z]{20,}\b`,
	"private_key_pem": `-----BEGIN[ A-Z]*PRIVATE KEY-----`,
	"basic_auth_url":  `https?://[^\s:/@]+:[^\s@/]+@`,
}

func TestRedact_Defaults(t *testing.T) {
	set, err := Compile(defaultPatterns)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name      string
		in        string
		mustMatch []string // substrings that must be redacted from output
		mustKeep  []string // substrings that must remain unredacted
	}{
		{
			name:      "email in free text",
			in:        "User john.doe@acme.io failed login",
			mustMatch: []string{"john.doe@acme.io"},
			mustKeep:  []string{"User", "failed login"},
		},
		{
			name:      "phone E.164",
			in:        "Called +14155551234 at 10:00",
			mustMatch: []string{"+14155551234"},
		},
		{
			name:      "IPv4 address",
			in:        "from 10.0.0.5 to 192.168.1.1",
			mustMatch: []string{"10.0.0.5", "192.168.1.1"},
		},
		{
			name:      "JWT token",
			in:        "Authorization: eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.signature_here",
			mustMatch: []string{"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.signature_here"},
		},
		{
			name:      "Bearer token",
			in:        "header Bearer abc123.def-456/xyz=",
			mustMatch: []string{"Bearer abc123.def-456/xyz="},
		},
		{
			name:      "IBAN",
			in:        "send to DE89370400440532013000 today",
			mustMatch: []string{"DE89370400440532013000"},
		},
		{
			name:     "nothing to redact",
			in:       "ordinary log message without secrets",
			mustKeep: []string{"ordinary log message without secrets"},
		},
		{
			name:      "US SSN",
			in:        "SSN 123-45-6789 on record",
			mustMatch: []string{"123-45-6789"},
			mustKeep:  []string{"on record"},
		},
		{
			name:      "UK NI number",
			in:        "ni AB123456C registered",
			mustMatch: []string{"AB123456C"},
		},
		{
			name:      "EU VAT number",
			in:        "vat DE123456789 charged",
			mustMatch: []string{"DE123456789"},
		},
		{
			name:      "MAC address colon",
			in:        "device aa:bb:cc:dd:ee:ff online",
			mustMatch: []string{"aa:bb:cc:dd:ee:ff"},
		},
		{
			name:      "MAC address hyphen",
			in:        "device aa-bb-cc-dd-ee-ff online",
			mustMatch: []string{"aa-bb-cc-dd-ee-ff"},
		},
		{
			name:      "IPv6 full form",
			in:        "origin 2001:db8:85a3:8d3:1319:8a2e:370:7348",
			mustMatch: []string{"2001:db8:85a3:8d3:1319:8a2e:370:7348"},
		},
		{
			name:      "geo lat/long",
			in:        "at 52.5200, 13.4050 today",
			mustMatch: []string{"52.5200, 13.4050"},
		},
		{
			name:      "AWS access key",
			in:        "key AKIAIOSFODNN7EXAMPLE in env",
			mustMatch: []string{"AKIAIOSFODNN7EXAMPLE"},
		},
		{
			name:      "GCP API key",
			in:        "key=AIzaSyABCDEFGHIJKLMNOPQRSTUVWXYZ1234567 end",
			mustMatch: []string{"AIzaSyABCDEFGHIJKLMNOPQRSTUVWXYZ1234567"},
		},
		{
			name:      "GitHub token",
			in:        "auth ghp_abcdefghijklmnopqrstuvwxyz0123456789 set",
			mustMatch: []string{"ghp_abcdefghijklmnopqrstuvwxyz0123456789"},
		},
		{
			name:      "Slack token",
			in:        "slack xoxb-1234567890-abcdefghij here",
			mustMatch: []string{"xoxb-1234567890-abcdefghij"},
		},
		{
			name:      "Stripe live key",
			in:        "stripe sk_live_abcdefghijklmnopqrstuv used",
			mustMatch: []string{"sk_live_abcdefghijklmnopqrstuv"},
		},
		{
			name:      "PEM private key header",
			in:        "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA",
			mustMatch: []string{"-----BEGIN RSA PRIVATE KEY-----"},
		},
		{
			name:      "basic auth in URL",
			in:        "fetch https://admin:hunter2@hooks.internal.corp/path",
			mustMatch: []string{"admin:hunter2@"},
		},
	}

	// negative cases: things that must NOT match
	negCases := []struct {
		name string
		in   string
	}{
		{"version string not ipv6", "version 2001:db8"},        // fewer than 8 groups
		{"nine digits dashed not SSN", "numeric 1234-56-7890"}, // wrong grouping
		{"arbitrary 8-char prefix not VAT", "code ZZ12345678"}, // ZZ not a country
		{"short aws prefix", "AKIA123"},
		{"short stripe key", "sk_test_short"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, stats := set.Redact(tc.in)
			for _, m := range tc.mustMatch {
				if strings.Contains(out, m) {
					t.Errorf("expected %q to be redacted; output: %q", m, out)
				}
			}
			for _, k := range tc.mustKeep {
				if !strings.Contains(out, k) {
					t.Errorf("expected %q to remain; output: %q", k, out)
				}
			}
			if len(tc.mustMatch) > 0 && len(stats) == 0 {
				t.Error("expected stats to record at least one match")
			}
		})
	}

	for _, tc := range negCases {
		t.Run("neg/"+tc.name, func(t *testing.T) {
			out, _ := set.Redact(tc.in)
			if out != tc.in {
				t.Errorf("expected pass-through for %q, got %q", tc.in, out)
			}
		})
	}
}

func TestRedact_EmptySet(t *testing.T) {
	set, _ := Compile(nil)
	out, stats := set.Redact("john@acme.io")
	if out != "john@acme.io" {
		t.Errorf("empty set must pass through, got %q", out)
	}
	if len(stats) != 0 {
		t.Error("empty set must produce empty stats")
	}
}

func TestRedact_MultipleMatchesCounted(t *testing.T) {
	set, _ := Compile(map[string]string{"email": defaultPatterns["email"]})
	out, stats := set.Redact("a@b.io and c@d.io and e@f.io")
	if strings.Contains(out, "@") {
		t.Errorf("expected all emails redacted, got %q", out)
	}
	if stats["email"] != 3 {
		t.Errorf("want 3 email matches, got %d", stats["email"])
	}
}

func TestCompile_BadRegexReturnsError(t *testing.T) {
	_, err := Compile(map[string]string{"bad": `[`})
	if err == nil {
		t.Fatal("expected compile error for invalid regex")
	}
}
