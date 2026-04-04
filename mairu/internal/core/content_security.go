package core

import "regexp"

type ScanResult struct {
	Safe     bool
	Warnings []string
}

type Pattern struct {
	Regex *regexp.Regexp
	Name  string
}

var (
	InvisibleUnicode = regexp.MustCompile(`[\x{200B}\x{200C}\x{200D}\x{200E}\x{200F}\x{202A}-\x{202E}\x{2060}\x{2066}-\x{2069}\x{FEFF}\x{FE00}-\x{FE0F}]`)

	InjectionPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)ignore\s+(?:all\s+)?previous\s+instructions`),
		regexp.MustCompile(`(?i)disregard\s+(?:all\s+)?(?:previous|prior|above)`),
		regexp.MustCompile(`(?i)you\s+are\s+now\s+a`),
		regexp.MustCompile(`(?i)override\s+your\s+(?:instructions|rules|guidelines)`),
		regexp.MustCompile(`(?i)forget\s+everything\s+(?:you|and)`),
		regexp.MustCompile(`(?i)new\s+instructions\s*:`),
		regexp.MustCompile(`(?i)system\s+prompt`),
		regexp.MustCompile(`(?i)print\s+your\s+instructions`),
		regexp.MustCompile(`(?i)reveal\s+(?:your\s+)?(?:secret\s+)?prompt`),
		regexp.MustCompile(`(?i)do\s+anything\s+now`),
		regexp.MustCompile(`(?i)simulate\s+a\s+(?:developer|admin)`),
		regexp.MustCompile(`(?i)developer\s+mode`),
		regexp.MustCompile(`(?i)bypass\s+(?:filters|security)`),
		regexp.MustCompile(`(?i)ignore\s+(?:the\s+)?above\s+and\s+instead`),
	}

	CredentialPatterns = []Pattern{
		{Regex: regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), Name: "AWS Access Key ID"},
		{Regex: regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs|ghr)_[a-zA-Z0-9]{36}\b`), Name: "GitHub Token"},
		{Regex: regexp.MustCompile(`\bxox[baprs]-[0-9]{10,13}-[0-9]{10,13}[a-zA-Z0-9-]*\b`), Name: "Slack Token"},
		{Regex: regexp.MustCompile(`\b(?:sk|rk)_live_[0-9a-zA-Z]{24}\b`), Name: "Stripe Live Key"},
		{Regex: regexp.MustCompile(`-----BEGIN (?:RSA|OPENSSH|DSA|EC|PGP) PRIVATE KEY-----`), Name: "Private Key"},
	}

	PIIPatterns = []Pattern{
		{Regex: regexp.MustCompile(`\b[0-9]{3}-[0-9]{2}-[0-9]{4}\b`), Name: "Social Security Number"},
		{Regex: regexp.MustCompile(`\b(?:4[0-9]{12}(?:[0-9]{3})?|5[1-5][0-9]{14}|3[47][0-9]{13}|6(?:011|5[0-9][0-9])[0-9]{12})\b`), Name: "Credit Card Number"},
	}

	SystemInjection = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:;|\||&&|` + "`" + `|\$\()\s*(?:rm\s+-|wget|curl|bash|sh|nc|netcat|nmap|python|perl|ruby|php|node)\b`),
		regexp.MustCompile(`(?i)/etc/(?:passwd|shadow|hosts|sudoers)`),
	}

	WebInjection = []*regexp.Regexp{
		regexp.MustCompile(`(?i)<script\b[^>]*>[\s\S]*?</script>`),
		regexp.MustCompile(`(?i)javascript\s*:`),
		regexp.MustCompile(`(?i)\bon(?:error|load|click|mouseover|keydown)\s*=`),
		regexp.MustCompile(`(?i)UNION\s+(?:ALL\s+)?SELECT`),
		regexp.MustCompile(`(?i)\bOR\s+['"]?1['"]?\s*=\s*['"]?1['"]?`),
		regexp.MustCompile(`(?i);\s*(?:DROP|ALTER|TRUNCATE)\s+TABLE`),
	}

	ExfiltrationTool   = regexp.MustCompile(`(?i)\b(?:curl|wget)\b|fetch\s*\(`)
	ExfiltrationSecret = regexp.MustCompile(`(?i)\$[A-Z_]*(?:SECRET|KEY|TOKEN|PASSWORD)|process\.env\b|\.env\b`)

	LongBase64 = regexp.MustCompile(`[A-Za-z0-9+/=]{100,}`)
)

func ScanContent(content string) ScanResult {
	var warnings []string

	if InvisibleUnicode.MatchString(content) {
		warnings = append(warnings, "Invisible unicode characters detected (zero-width, directional override, or variation selector)")
	}

	for _, p := range InjectionPatterns {
		if p.MatchString(content) {
			warnings = append(warnings, "Possible prompt injection pattern: "+p.String())
			break
		}
	}

	for _, p := range CredentialPatterns {
		if p.Regex.MatchString(content) {
			warnings = append(warnings, "Possible exposed credential: "+p.Name)
		}
	}

	for _, p := range PIIPatterns {
		if p.Regex.MatchString(content) {
			warnings = append(warnings, "Possible Personally Identifiable Information (PII) detected: "+p.Name)
		}
	}

	for _, p := range SystemInjection {
		if p.MatchString(content) {
			warnings = append(warnings, "Possible system command injection or sensitive file access: "+p.String())
		}
	}

	for _, p := range WebInjection {
		if p.MatchString(content) {
			warnings = append(warnings, "Possible web/SQL injection detected: "+p.String())
		}
	}

	if ExfiltrationTool.MatchString(content) && ExfiltrationSecret.MatchString(content) {
		warnings = append(warnings, "Possible exfiltration attempt: HTTP tool combined with secret/env variable reference")
	}

	if LongBase64.MatchString(content) {
		warnings = append(warnings, "Suspicious encoded payload: long base64-like string (100+ chars)")
	}

	return ScanResult{Safe: len(warnings) == 0, Warnings: warnings}
}
