package pipeline

import (
	"regexp"
	"sort"
)

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
}

func scanFormats(input string) (string, []Finding) {
	var ivs []interval

	for _, r := range formatRules {
		locs := r.re.FindAllStringIndex(input, -1)
		if len(locs) == 0 {
			continue
		}
		for _, loc := range locs {
			if overlapsInterval(ivs, loc[0], loc[1]) {
				continue
			}
			raw := input[loc[0]:loc[1]]
			ivs = append(ivs, interval{
				start: loc[0],
				end:   loc[1],
				kind:  r.kind,
				text:  r.re.ReplaceAllString(raw, r.replacement),
			})
		}
	}

	// Connection URIs with embedded basic-auth (hand-rolled; faster than regex
	// with backtracking). Layer-1's uri_credentials regex is an intentional
	// redundant safety net.
	for _, iv := range findConnURIs(input) {
		if overlapsInterval(ivs, iv.start, iv.end) {
			continue
		}
		ivs = append(ivs, iv)
	}

	sort.Slice(ivs, func(i, j int) bool { return ivs[i].start < ivs[j].start })
	out := applyIntervals(input, ivs)
	findings := make([]Finding, len(ivs))
	for i, iv := range ivs {
		findings[i] = Finding{
			Layer: LayerFormat,
			Kind:  iv.kind,
			Start: iv.start,
			End:   iv.end,
		}
	}
	return out, findings
}
