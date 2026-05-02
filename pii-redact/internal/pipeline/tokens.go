package pipeline

import (
	"regexp"
	"sort"
	"strings"
)

type tokenPattern struct {
	kind  string
	re    *regexp.Regexp
	probe string // distinctive substring; if absent the regex is skipped
}

// knownTokenPatterns is the Layer-1 rule set. Patterns are high-precision —
// prefer false negatives over false positives here; Layer 4 (entropy)
// catches the long tail. Any hit here flips EmbeddingSafe to false.
//
// Sources: derived from Apache-2.0 licensed rule packs
// (github.com/gitleaks/gitleaks, github.com/trufflesecurity/trufflehog).
var knownTokenPatterns = []tokenPattern{
	// -- git hosts --
	{"github_pat_classic", regexp.MustCompile(`ghp_[A-Za-z0-9]{36,}`), "ghp_"},
	{"github_pat_fine", regexp.MustCompile(`github_pat_[A-Za-z0-9_]{22,}`), "github_pat_"},
	{"github_oauth", regexp.MustCompile(`gh[osu]_[A-Za-z0-9]{36,}`), "gho_"},
	{"gitlab_pat", regexp.MustCompile(`glpat-[A-Za-z0-9_\-]{20,}`), "glpat"},
	{"bitbucket_app_pw", regexp.MustCompile(`ATBB[A-Za-z0-9_\-]{32,}`), "ATBB"},

	// -- payments / commerce --
	{"stripe_pub", regexp.MustCompile(`pk_(?:live|test)_[A-Za-z0-9]{24,}`), "pk_"},
	{"stripe_restricted", regexp.MustCompile(`rk_(?:live|test)_[A-Za-z0-9]{24,}`), "rk_"},
	{"shopify_token", regexp.MustCompile(`shp(?:at|ca|pa|ss)_[a-fA-F0-9]{32,}`), "shp"},
	{"square_access_token", regexp.MustCompile(`EAAA[A-Za-z0-9_\-]{60,}`), "EAAA"},

	// -- cloud --
	// AWS access keys split by prefix so the probe system skips the
	// expensive alternation when only one prefix is present.
	{"aws_access_key", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), "AKIA"},
	{"aws_access_key", regexp.MustCompile(`\bASIA[0-9A-Z]{16}\b`), "ASIA"},
	{"aws_access_key", regexp.MustCompile(`\bAGPA[0-9A-Z]{16}\b`), "AGPA"},
	{"aws_access_key", regexp.MustCompile(`\bAIDA[0-9A-Z]{16}\b`), "AIDA"},
	{"aws_access_key", regexp.MustCompile(`\bAROA[0-9A-Z]{16}\b`), "AROA"},
	{"aws_access_key", regexp.MustCompile(`\bANPA[0-9A-Z]{16}\b`), "ANPA"},
	{"aws_access_key", regexp.MustCompile(`\bANVA[0-9A-Z]{16}\b`), "ANVA"},
	{"aws_access_key", regexp.MustCompile(`\bACCA[0-9A-Z]{16}\b`), "ACCA"},
	{"gcp_service_account", regexp.MustCompile(`"type":\s*"service_account"`), "service_account"},
	{"gcp_api_key", regexp.MustCompile(`AIza[0-9A-Za-z_\-]{35}`), "AIza"},
	{"gcp_oauth_refresh", regexp.MustCompile(`1//0[A-Za-z0-9_\-]{40,}`), "1//0"},
	{"azure_client_secret", regexp.MustCompile(`[a-zA-Z0-9_~.\-]{3}8Q~[A-Za-z0-9_~.\-]{34}`), "8Q~"},
	{"azure_storage_key", regexp.MustCompile(`DefaultEndpointsProtocol=https?;AccountName=[A-Za-z0-9]+;AccountKey=[A-Za-z0-9+/=]{64,}`), "DefaultEndpointsProtocol"},
	{"digitalocean_pat", regexp.MustCompile(`dop_v1_[a-f0-9]{64}`), "dop_v1"},
	{"digitalocean_oauth", regexp.MustCompile(`do[ort]_v1_[a-f0-9]{64}`), "do_"},
	{"cloudflare_api_token", regexp.MustCompile(`\b[A-Za-z0-9_\-]{40}\b_Cf`), "_Cf"},
	{"heroku_api_key", regexp.MustCompile(`HRKU-[A-Za-z0-9_\-]{32,}`), "HRKU"},

	// -- saas / observability --

	{"slack_webhook", regexp.MustCompile(`https://hooks\.slack\.com/services/T[A-Za-z0-9_\-]+/B[A-Za-z0-9_\-]+/[A-Za-z0-9_\-]{24,}`), "hooks.slack.com"},
	{"discord_webhook", regexp.MustCompile(`https://(?:canary\.|ptb\.)?discord(?:app)?\.com/api/webhooks/\d+/[A-Za-z0-9_\-]{60,}`), "discord"},
	{"discord_bot_token", regexp.MustCompile(`[MN][A-Za-z0-9_\-]{23}\.[A-Za-z0-9_\-]{6}\.[A-Za-z0-9_\-]{27,}`), ""},
	{"datadog_api_key", regexp.MustCompile(`\bdd[ap]i_[a-f0-9]{32}\b`), "ddapi_"},
	{"pagerduty_token", regexp.MustCompile(`\bpdu[a-zA-Z0-9_\-]{20,}\b`), "pdu"},
	{"newrelic_user_key", regexp.MustCompile(`NRAK-[A-Z0-9]{27}`), "NRAK"},
	{"sentry_dsn", regexp.MustCompile(`https://[a-f0-9]{32}@o?\d+\.ingest\.sentry\.io/\d+`), "ingest.sentry.io"},
	{"linear_key", regexp.MustCompile(`lin_api_[A-Za-z0-9]{32,}`), "lin_api"},
	{"asana_pat", regexp.MustCompile(`\b[0-9]/[0-9]{16,}:[a-f0-9]{32}\b`), ""},
	{"notion_key", regexp.MustCompile(`secret_[A-Za-z0-9]{43}`), "secret_"},

	// -- registries / ci --
	{"npm_token", regexp.MustCompile(`\bnpm_[A-Za-z0-9]{36}\b`), "npm_"},
	{"pypi_token", regexp.MustCompile(`pypi-AgEIc[A-Za-z0-9_\-]{50,}`), "pypi-AgEIc"},
	{"crates_token", regexp.MustCompile(`\bcioyDWFzqVjAxisxn[A-Za-z0-9_]{10,}\b`), "cioyDWFzqVjAxisxn"},
	{"dockerhub_pat", regexp.MustCompile(`dckr_pat_[A-Za-z0-9_\-]{27,}`), "dckr_pat"},

	// -- messaging / email --
	{"twilio_api_key", regexp.MustCompile(`\bSK[a-f0-9]{32}\b`), "SK"},
	{"twilio_account_sid", regexp.MustCompile(`\bAC[a-f0-9]{32}\b`), "AC"},
	{"sendgrid_api_key", regexp.MustCompile(`SG\.[A-Za-z0-9_\-]{22}\.[A-Za-z0-9_\-]{43}`), "SG."},
	{"mailgun_key", regexp.MustCompile(`key-[0-9a-zA-Z]{32}`), "key-"},
	{"postmark_token", regexp.MustCompile(`\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}-POSTMARK\b`), "POSTMARK"},

	// -- ai / llm --
	{"openai_key_project", regexp.MustCompile(`sk-proj-[A-Za-z0-9_\-]{40,}`), "sk-proj"},
	{"openai_key_classic", regexp.MustCompile(`\bsk-[A-Za-z0-9]{48,}\b`), "sk-"},
	{"anthropic_key", regexp.MustCompile(`sk-ant-[a-zA-Z0-9_\-]{90,}`), "sk-ant"},
	{"huggingface_token", regexp.MustCompile(`hf_[A-Za-z0-9]{34,}`), "hf_"},
	{"replicate_token", regexp.MustCompile(`r8_[A-Za-z0-9]{37,}`), "r8_"},
	{"cohere_api_key", regexp.MustCompile(`co-[A-Za-z0-9]{40,}`), "co-"},

	// -- dev platforms --
	{"vercel_token", regexp.MustCompile(`\b[A-Za-z0-9]{24}_vercel\b`), "_vercel"},
	{"netlify_pat", regexp.MustCompile(`\bnfp_[A-Za-z0-9_\-]{40,}\b`), "nfp_"},
	{"figma_pat", regexp.MustCompile(`figd_[A-Za-z0-9_\-]{40,}`), "figd_"},
	// -- generic / structural --
	// jwt is handled by findJWTsFast (hand-rolled scanner) instead of regex
	// to avoid backtracking overhead on the three quantified segments.
	// uri_credentials is intentionally removed: findConnURIs in Layer 0 already
	// covers the same URI-with-embedded-credentials shape with a faster hand-
	// rolled scanner, and the regex here was only a redundant safety net that
	// rarely matched (overlapsRedacted skipped it on redacted text).
	{"pem_private_key", regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`), "-----BEGIN"},
	{"pem_openssh", regexp.MustCompile(`(?s)-----BEGIN OPENSSH PRIVATE KEY-----.*?-----END OPENSSH PRIVATE KEY-----`), "-----BEGIN OPENSSH"},
	{"pkcs12_b64", regexp.MustCompile(`MIIK[A-Za-z0-9+/]{400,}={0,2}`), "MIIK"},
}

// scanKnownTokens replaces every match with "[REDACTED:<kind>]" and flips
// the EmbeddingSafe bit if any pattern hit. The scan walks patterns in
// declaration order so longer/more-specific rules (jwt, supabase_service)
// win against shorter generic ones (uri_credentials) when ranges overlap.
func scanKnownTokens(input string) (string, []Finding, bool) {
	ivs := make([]interval, 0, 4)
	hit := false
	hasDiscordBot := hasDiscordBotTokenProbe(input)
	hasAsana := hasAsanaPatProbe(input)

	// Hand-rolled scanners for the most common anchored-prefix patterns.
	// These avoid regex backtracking and allocations entirely.
	for _, iv := range findJWTsFast(input) {
		if overlapsRedacted(input, iv.start, iv.end, input[iv.start:iv.end]) {
			continue
		}
		if overlapsInterval(ivs, iv.start, iv.end) {
			continue
		}
		hit = true
		ivs = append(ivs, iv)
	}
	for _, iv := range findGitHubPATClassic(input) {
		if checkAndAppend(&ivs, input, iv) {
			hit = true
		}
	}
	for _, iv := range findGitHubPATFine(input) {
		if checkAndAppend(&ivs, input, iv) {
			hit = true
		}
	}
	for _, iv := range findAWSAccessKeys(input) {
		if checkAndAppend(&ivs, input, iv) {
			hit = true
		}
	}
	for _, iv := range findGCPAPIKey(input) {
		if checkAndAppend(&ivs, input, iv) {
			hit = true
		}
	}
	for _, iv := range findSlackToken(input) {
		if checkAndAppend(&ivs, input, iv) {
			hit = true
		}
	}
	for _, iv := range findStripeKey(input) {
		if checkAndAppend(&ivs, input, iv) {
			hit = true
		}
	}

	for _, p := range knownTokenPatterns {
		if p.probe != "" && !strings.Contains(input, p.probe) {
			continue
		}
		if p.kind == "discord_bot_token" && !hasDiscordBot {
			continue
		}
		if p.kind == "asana_pat" && !hasAsana {
			continue
		}
		locs := p.re.FindAllStringIndex(input, -1)
		if len(locs) == 0 {
			continue
		}
		replacement := "[REDACTED:" + p.kind + "]"
		for _, loc := range locs {
			// Skip matches that overlap an existing placeholder from Layer 0
			// or an earlier pattern in this layer.
			raw := input[loc[0]:loc[1]]
			if overlapsRedacted(input, loc[0], loc[1], raw) {
				continue
			}
			if overlapsInterval(ivs, loc[0], loc[1]) {
				continue
			}
			hit = true
			ivs = append(ivs, interval{
				start: loc[0],
				end:   loc[1],
				kind:  p.kind,
				text:  replacement,
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
			Layer: LayerKnownToken,
			Kind:  iv.kind,
			Start: iv.start,
			End:   iv.end,
		}
	}
	return out, findings, hit
}

// overlapsRedacted reports whether the substring [start:end] overlaps (or
// lies entirely inside) an existing "[REDACTED:...]" span in s.
func overlapsRedacted(s string, start, end int, raw string) bool {
	if !containsRedacted(raw) {
		// Cheap path: no REDACTED token inside the match itself means we
		// could only overlap if the match extends into a neighboring one.
		// Check a narrow window on each side.
		return boundaryHitsRedacted(s, start, end)
	}
	return true
}

func containsRedacted(s string) bool {
	for i := 0; i+10 <= len(s); i++ {
		if s[i] == '[' && s[i+1] == 'R' && s[i+2] == 'E' && s[i+3] == 'D' &&
			s[i+4] == 'A' && s[i+5] == 'C' && s[i+6] == 'T' && s[i+7] == 'E' &&
			s[i+8] == 'D' && s[i+9] == ':' {
			return true
		}
	}
	return false
}

func boundaryHitsRedacted(s string, start, end int) bool {
	// Look back for the nearest '[' or ']' — if '[' comes first and it's
	// the start of "[REDACTED:", the match is inside a placeholder.
	for i := start - 1; i >= 0 && start-i < 32; i-- {
		if s[i] == ']' {
			return false
		}
		if s[i] == '[' && i+10 <= len(s) {
			if s[i:i+10] == "[REDACTED:" {
				return true
			}
			return false
		}
	}
	return false
}

// findJWTsFast scans for JSON Web Tokens using a hand-rolled parser.
// It anchors on the "eyJ" prefix (base64url of '{"') and validates the
// three dot-separated base64url segments without regex backtracking.
func findJWTsFast(input string) []interval {
	ivs := make([]interval, 0, 4)
	start := 0
	for {
		i := strings.Index(input[start:], "eyJ")
		if i < 0 {
			break
		}
		seg0 := start + i
		// Walk over base64url chars until first '.'
		dot1 := -1
		for j := seg0 + 3; j < len(input); j++ {
			c := input[j]
			if c == '.' {
				dot1 = j
				break
			}
			if !isBase64URL(c) {
				break
			}
		}
		if dot1 < 0 || dot1-seg0 < 10 {
			start = seg0 + 3
			continue
		}
		// Walk over base64url chars until second '.'
		dot2 := -1
		for j := dot1 + 1; j < len(input); j++ {
			c := input[j]
			if c == '.' {
				dot2 = j
				break
			}
			if !isBase64URL(c) {
				break
			}
		}
		if dot2 < 0 || dot2-dot1 < 10 {
			start = seg0 + 3
			continue
		}
		// Walk over base64url chars until end of segment
		seg2End := dot2 + 1
		for seg2End < len(input) && isBase64URL(input[seg2End]) {
			seg2End++
		}
		if seg2End-dot2 < 2 {
			start = seg0 + 3
			continue
		}
		ivs = append(ivs, interval{
			start: seg0,
			end:   seg2End,
			kind:  "jwt",
			text:  "[REDACTED:jwt]",
		})
		start = seg2End
	}
	return ivs
}

func isBase64URL(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
		c == '_' || c == '-'
}

// hasDiscordBotTokenProbe reports whether s contains at least two dots and
// at least one 'M' or 'N', the minimum requirements for a Discord bot token.
func hasDiscordBotTokenProbe(s string) bool {
	if strings.Count(s, ".") < 2 {
		return false
	}
	return strings.ContainsAny(s, "MN")
}

// checkAndAppend is a helper that runs the overlap checks for hand-rolled
// intervals and appends them if clean.  It returns true when an interval
// was accepted.
func checkAndAppend(ivs *[]interval, input string, iv interval) bool {
	if overlapsRedacted(input, iv.start, iv.end, input[iv.start:iv.end]) {
		return false
	}
	if overlapsInterval(*ivs, iv.start, iv.end) {
		return false
	}
	*ivs = append(*ivs, iv)
	return true
}

// findGitHubPATClassic finds ghp_<36+ alnum> tokens.
func findGitHubPATClassic(input string) []interval {
	return findPrefixMatches(input, "ghp_", "github_pat_classic", 40,
		func(c byte) bool { return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') })
}

// findGitHubPATFine finds github_pat_<22+ alnum/_> tokens.
func findGitHubPATFine(input string) []interval {
	return findPrefixMatches(input, "github_pat_", "github_pat_fine", 33,
		func(c byte) bool {
			return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_' || c == '-'
		})
}

// findAWSAccessKeys finds AKIA/ASIA/AGPA/AIDA/AROA/ANPA/ANVA/ACCA prefixes
// followed by 16 uppercase/digit characters.
func findAWSAccessKeys(input string) []interval {
	var ivs []interval
	prefixes := []string{"AKIA", "ASIA", "AGPA", "AIDA", "AROA", "ANPA", "ANVA", "ACCA"}
	for _, pre := range prefixes {
		ivs = append(ivs, findPrefixMatches(input, pre, "aws_access_key", 20,
			func(c byte) bool { return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') })...)
	}
	return ivs
}

// findGCPAPIKey finds AIza<35+ alnum/_> tokens.
func findGCPAPIKey(input string) []interval {
	return findPrefixMatches(input, "AIza", "gcp_api_key", 39,
		func(c byte) bool {
			return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_' || c == '-'
		})
}

// findSlackToken finds xox[abpors]-<10+ alnum/-> tokens.
func findSlackToken(input string) []interval {
	var ivs []interval
	start := 0
	for {
		i := strings.Index(input[start:], "xox")
		if i < 0 {
			break
		}
		pos := start + i
		if pos+3 >= len(input) {
			break
		}
		c := input[pos+3]
		if c != 'a' && c != 'b' && c != 'p' && c != 'o' && c != 'r' && c != 's' {
			start = pos + 3
			continue
		}
		if pos+4 >= len(input) || input[pos+4] != '-' {
			start = pos + 3
			continue
		}
		end := pos + 5
		for end < len(input) {
			ch := input[end]
			if (ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '-' {
				end++
			} else {
				break
			}
		}
		if end-pos >= 15 {
			ivs = append(ivs, interval{start: pos, end: end, kind: "slack_token", text: "[REDACTED:slack_token]"})
		}
		start = pos + 3
	}
	return ivs
}

// findStripeKey finds sk_(live|test)_<24+ alnum> tokens.
func findStripeKey(input string) []interval {
	var ivs []interval
	start := 0
	for {
		i := strings.Index(input[start:], "sk_")
		if i < 0 {
			break
		}
		pos := start + i
		if pos+3 >= len(input) {
			break
		}
		rest := input[pos+3:]
		var envLen int
		if strings.HasPrefix(rest, "live_") {
			envLen = 5
		} else if strings.HasPrefix(rest, "test_") {
			envLen = 5
		} else {
			start = pos + 3
			continue
		}
		end := pos + 3 + envLen
		for end < len(input) {
			ch := input[end]
			if (ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') {
				end++
			} else {
				break
			}
		}
		if end-pos >= 32 {
			ivs = append(ivs, interval{start: pos, end: end, kind: "stripe_key", text: "[REDACTED:stripe_key]"})
		}
		start = pos + 3
	}
	return ivs
}

// findPrefixMatches is a generic helper for anchored-prefix token scanners.
// It finds all occurrences of prefix, checks word boundaries, expands over
// validChars, and returns intervals of at least minLen bytes.
func findPrefixMatches(input, prefix, kind string, minLen int, validChar func(byte) bool) []interval {
	var ivs []interval
	start := 0
	for {
		i := strings.Index(input[start:], prefix)
		if i < 0 {
			break
		}
		pos := start + i
		// Word boundary before prefix.
		if pos > 0 {
			prev := input[pos-1]
			if (prev >= '0' && prev <= '9') || (prev >= 'A' && prev <= 'Z') || (prev >= 'a' && prev <= 'z') || prev == '_' {
				start = pos + len(prefix)
				continue
			}
		}
		end := pos + len(prefix)
		for end < len(input) && validChar(input[end]) {
			end++
		}
		// Word boundary after match.
		if end < len(input) {
			next := input[end]
			if (next >= '0' && next <= '9') || (next >= 'A' && next <= 'Z') || (next >= 'a' && next <= 'z') || next == '_' {
				start = pos + len(prefix)
				continue
			}
		}
		if end-pos >= minLen {
			ivs = append(ivs, interval{start: pos, end: end, kind: kind, text: "[REDACTED:" + kind + "]"})
		}
		start = pos + len(prefix)
	}
	return ivs
}

// hasAsanaPatProbe reports whether s contains a slash followed by many
// digits followed by a colon, which is the core shape of an Asana PAT.
func hasAsanaPatProbe(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' && i+18 < len(s) {
			// look for 16+ digits then ':'
			j := i + 1
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			if j-i >= 17 && j < len(s) && s[j] == ':' {
				return true
			}
		}
	}
	return false
}
