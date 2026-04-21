package mask

import (
	"regexp"
	"strings"
	"testing"
)

func TestDefaults_PartialReveal(t *testing.T) {
	m := NewMasker(true)
	cases := []struct {
		name    string
		pattern string
		in      string
		wantHas []string // substrings expected in output
		wantNot []string // substrings that MUST NOT appear
	}{
		{"email", "email", "john.doe+hr@sub.example.co.uk",
			[]string{"@", ".uk"},
			[]string{"john.doe", "example.co"}},
		{"ipv4", "ipv4", "10.0.0.5",
			[]string{"10.0."},
			[]string{"0.5"}},
		{"ipv6", "ipv6", "2001:db8:85a3:8d3:1319:8a2e:370:7348",
			[]string{"2001:", "7348"},
			[]string{"db8:85a3"}},
		{"mac", "mac_address", "aa:bb:cc:dd:ee:ff",
			[]string{"aa:", ":ff"},
			[]string{"cc:dd", "ee:ff"}},
		{"credit_card", "credit_card", "4111 1111 1111 1111",
			[]string{"1111"},
			[]string{"4111 1111"}},
		{"iban", "iban", "DE89370400440532013000",
			[]string{"DE89", "3000"},
			[]string{"370400440532"}},
		{"phone_e164", "phone_e164", "+14155551234",
			[]string{"+14", "34"},
			[]string{"4155551"}},
		{"phone_us", "phone_us", "(415) 555-1234",
			[]string{"1234", "***"},
			[]string{"415"}},
		{"ssn", "ssn_us", "123-45-6789",
			[]string{"6789"},
			[]string{"123-45"}},
		{"jwt", "jwt", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.signatureXYZ",
			[]string{"eyJhbG", "eXYZ"},
			[]string{"signature"}},
		{"bearer", "bearer", "Bearer abcdefghij",
			[]string{"Bearer", "ghij"},
			[]string{"abcdef"}},
		{"aws", "aws_access_key", "AKIAIOSFODNN7EXAMPLE",
			[]string{"AKIA", "MPLE"},
			[]string{"IOSFODNN7EXA"}},
		{"stripe", "stripe_key", "sk_live_abcdefghijklmnopqrstuv",
			[]string{"sk_live_", "stuv"},
			[]string{"abcdefghijklmnop"}},
		{"uuid", "uuid", "550e8400-e29b-41d4-a716-446655440000",
			[]string{"0000"},
			[]string{"550e8400", "e29b"}},
		{"eth", "eth_address", "0xAbCdef0123456789abcdef0123456789abcdef01",
			[]string{"0xAbCd", "ef01"},
			[]string{"0123456789abcdef012345"}},
		{"basic_auth", "basic_auth_url", "https://admin:hunter2@hooks.corp/",
			[]string{"https://***:***@"},
			[]string{"admin", "hunter2"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := m.Apply(tc.pattern, tc.in)
			for _, h := range tc.wantHas {
				if !strings.Contains(got, h) {
					t.Errorf("want %q in %q", h, got)
				}
			}
			for _, n := range tc.wantNot {
				if strings.Contains(got, n) {
					t.Errorf("raw fragment leaked: %q appears in %q", n, got)
				}
			}
			if strings.Contains(got, tc.in) {
				t.Errorf("full raw leaked: %q", got)
			}
		})
	}
}

func TestOpaqueMode_AlwaysToken(t *testing.T) {
	m := NewMasker(false)
	out := m.Apply("email", "x@y.z")
	if out != "[REDACTED:email]" {
		t.Errorf("opaque mode changed: %q", out)
	}
}

func TestUnknownPattern_GenericMask(t *testing.T) {
	m := NewMasker(true)
	out := m.Apply("nonesuch", "somelongsecret")
	if out == "somelongsecret" {
		t.Fatal("nothing masked")
	}
	if matched, _ := regexp.MatchString(`^\[REDACTED:nonesuch:s\*+t\]$`, out); !matched {
		t.Errorf("unexpected generic mask shape: %q", out)
	}
}
