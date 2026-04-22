package pipeline

import "regexp"

type formatRule struct {
	kind        string
	re          *regexp.Regexp
	replacement string
}

// formatRules is the Layer-0 rule set. These patterns target whole
// well-known serializations where a secret-looking KEY pairs with a VALUE
// that should be scrubbed — e.g. `.env` / `export` lines, YAML
// `password: ...`, connection URIs with embedded credentials, bare HTTP
// header lines. They run first so later layers see the already-normalized
// residual and don't re-match.
//
// Each rule preserves the structural prefix (flag name, key, scheme) and
// replaces only the sensitive tail. Flipping EmbeddingSafe is reserved for
// Layer 1.
var formatRules = []formatRule{
	// .env / shell export — only when the LHS name carries a secret-ish
	// marker. Multiline-anchored so one line per match.
	{
		kind: "dotenv_pair",
		re: regexp.MustCompile(`(?mi)^((?:export\s+)?[A-Z][A-Z0-9_]*` +
			`(?:TOKEN|SECRET|KEY|PASSWORD|PASSWD|PASS|AUTH|CREDENTIAL|APIKEY|ACCESS_TOKEN|PRIVATE|DSN|CONN(?:ECTION)?)` +
			`[A-Z0-9_]*\s*=\s*)(.+)$`),
		replacement: `${1}[REDACTED:dotenv_pair]`,
	},
	// YAML-style `password: value` / `api_key: value` (single-line values only).
	{
		kind: "yaml_pair",
		re: regexp.MustCompile(`(?mi)^(\s*[a-z_][\w-]*` +
			`(?:token|secret|key|password|passwd|pass|auth|credential|api[_-]?key|access[_-]?token|private|dsn)` +
			`[\w-]*\s*:\s*)(?:"([^"\n]+)"|'([^'\n]+)'|([^\s#\n][^\n]*?))(\s*(?:#.*)?)$`),
		replacement: `${1}[REDACTED:yaml_pair]${5}`,
	},
	// HTTP header on its own line — "Authorization: ..." / "Cookie: ..." /
	// "Proxy-Authorization: ...". Captures the header name to preserve it.
	{
		kind: "http_header",
		re: regexp.MustCompile(`(?mi)^((?:Authorization|Proxy-Authorization|Cookie|Set-Cookie|` +
			`X-[A-Za-z-]*(?:Auth|Key|Token|Secret)[A-Za-z-]*)\s*:\s*)(.+)$`),
		replacement: `${1}[REDACTED:http_header]`,
	},
	// Connection URI with embedded basic-auth:
	//   postgres://user:pass@host / redis://:pw@host / amqp://u:p@host / etc.
	// Caught here for the `user:pass@` half; Layer-1's uri_credentials also
	// covers the token edge-case but the two are intentionally redundant.
	{
		kind:        "conn_uri",
		re:          regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.\-]*://)(?:[^\s:@/]*):([^\s@/]+)@`),
		replacement: `${1}[REDACTED:conn_uri]@`,
	},
}

func scanFormats(input string) (string, []Finding) {
	out := input
	var findings []Finding
	for _, r := range formatRules {
		for _, loc := range r.re.FindAllStringIndex(out, -1) {
			findings = append(findings, Finding{
				Layer: LayerFormat,
				Kind:  r.kind,
				Start: loc[0],
				End:   loc[1],
			})
		}
		out = r.re.ReplaceAllString(out, r.replacement)
	}
	return out, findings
}
