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
	{"github_oauth", regexp.MustCompile(`gh[osu]_[A-Za-z0-9]{36,}`), ""},
	{"gitlab_pat", regexp.MustCompile(`glpat-[A-Za-z0-9_\-]{20,}`), "glpat"},
	{"bitbucket_app_pw", regexp.MustCompile(`ATBB[A-Za-z0-9_\-]{32,}`), "ATBB"},

	// -- payments / commerce --
	{"stripe_key", regexp.MustCompile(`sk_(?:live|test)_[A-Za-z0-9]{24,}`), "sk_"},
	{"stripe_pub", regexp.MustCompile(`pk_(?:live|test)_[A-Za-z0-9]{24,}`), "pk_"},
	{"stripe_restricted", regexp.MustCompile(`rk_(?:live|test)_[A-Za-z0-9]{24,}`), "rk_"},
	{"shopify_token", regexp.MustCompile(`shp(?:at|ca|pa|ss)_[a-fA-F0-9]{32,}`), "shp"},
	{"square_access_token", regexp.MustCompile(`EAAA[A-Za-z0-9_\-]{60,}`), "EAAA"},

	// -- cloud --
	{"aws_access_key", regexp.MustCompile(`\b(?:AKIA|ASIA|AGPA|AIDA|AROA|ANPA|ANVA|ACCA)[0-9A-Z]{16}\b`), "AKIA"},
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
	{"slack_token", regexp.MustCompile(`xox[abpors]-[A-Za-z0-9-]{10,}`), "xox"},
	{"slack_webhook", regexp.MustCompile(`https://hooks\.slack\.com/services/T[A-Za-z0-9_\-]+/B[A-Za-z0-9_\-]+/[A-Za-z0-9_\-]{24,}`), "hooks.slack.com"},
	{"discord_webhook", regexp.MustCompile(`https://(?:canary\.|ptb\.)?discord(?:app)?\.com/api/webhooks/\d+/[A-Za-z0-9_\-]{60,}`), "discord"},
	{"discord_bot_token", regexp.MustCompile(`[MN][A-Za-z0-9_\-]{23}\.[A-Za-z0-9_\-]{6}\.[A-Za-z0-9_\-]{27,}`), ""},
	{"datadog_api_key", regexp.MustCompile(`\bdd[ap]i_[a-f0-9]{32}\b`), ""},
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
	{"supabase_service", regexp.MustCompile(`eyJhbGciOi[A-Za-z0-9_\-]+\.eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+`), "eyJhbGciOi"},

	// -- generic / structural --
	{"jwt", regexp.MustCompile(`eyJ[A-Za-z0-9_\-]+\.eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+`), "eyJ"},
	{"uri_credentials", regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.\-]*://)[^\s:@/]+:([^\s@/]+)@`), ""},
	{"pem_private_key", regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`), "-----BEGIN"},
	{"pem_openssh", regexp.MustCompile(`(?s)-----BEGIN OPENSSH PRIVATE KEY-----.*?-----END OPENSSH PRIVATE KEY-----`), "-----BEGIN OPENSSH"},
	{"pkcs12_b64", regexp.MustCompile(`MIIK[A-Za-z0-9+/]{400,}={0,2}`), "MIIK"},
}

// scanKnownTokens replaces every match with "[REDACTED:<kind>]" and flips
// the EmbeddingSafe bit if any pattern hit. The scan walks patterns in
// declaration order so longer/more-specific rules (jwt, supabase_service)
// win against shorter generic ones (uri_credentials) when ranges overlap.
func scanKnownTokens(input string) (string, []Finding, bool) {
	var ivs []interval
	hit := false
	hasColonSlashSlash := strings.Contains(input, "://")
	hasAt := strings.Contains(input, "@")
	for _, p := range knownTokenPatterns {
		if p.probe != "" && !strings.Contains(input, p.probe) {
			continue
		}
		if p.kind == "uri_credentials" && (!hasColonSlashSlash || !hasAt) {
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

	sort.Slice(ivs, func(i, j int) bool { return ivs[i].start < ivs[j].start })
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
