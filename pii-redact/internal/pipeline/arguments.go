package pipeline

import "regexp"

type argHeuristic struct {
	kind        string
	re          *regexp.Regexp
	replacement string
}

// argHeuristics is the Layer-2 rule set. Each pattern captures the
// flag/header/env prefix in group 1 and retains it; the secret value is
// replaced with a [REDACTED:<kind>] placeholder. Layer 2 never flips
// EmbeddingSafe — only Layer 1 does.
var argHeuristics = []argHeuristic{
	// curl -H "Authorization: Bearer ..." / -H "X-Api-Key: ..." etc.
	{
		kind:        "auth_header",
		re:          regexp.MustCompile(`(?i)(-H\s+["']?(?:Authorization|Proxy-Authorization|Cookie|Set-Cookie|X-[A-Za-z-]*(?:Auth|Key|Token|Secret)[A-Za-z-]*):\s*)([^"'\n]+?)(["']|$)`),
		replacement: `${1}[REDACTED:auth_header]${3}`,
	},
	// curl -u user:pass
	{
		kind:        "basic_auth",
		re:          regexp.MustCompile(`(-u\s+)([^\s:]+:[^\s]+)`),
		replacement: `${1}[REDACTED:basic_auth]`,
	},
	// --token=VALUE / --bearer=VALUE / --service-account-key-file=... (with =)
	{
		kind: "sensitive_flag_eq",
		re: regexp.MustCompile(`(?i)(--(?:token|secret|key|password|passwd|pass|auth|credential|api[-_]?key|access[-_]?token|client[-_]?secret|` +
			`bearer|private[-_]?key|service[-_]?account[-_]?key(?:[-_]?file)?|github[-_]?token|gh[-_]?token|npm[-_]?token)=)([^\s]+)`),
		replacement: `${1}[REDACTED:sensitive_flag]`,
	},
	// --password VALUE (space-separated)
	{
		kind: "sensitive_flag_sp",
		re: regexp.MustCompile(`(?i)(--(?:token|secret|key|password|passwd|pass|auth|credential|api[-_]?key|access[-_]?token|client[-_]?secret|` +
			`bearer|private[-_]?key|service[-_]?account[-_]?key(?:[-_]?file)?|github[-_]?token|gh[-_]?token|npm[-_]?token)\s+)([^\s]+)`),
		replacement: `${1}[REDACTED:sensitive_flag]`,
	},
	// Short flags for password (-p VALUE). Scoped to known-risky tools
	// (mysql/psql/redis-cli/mongo) to avoid false-positives on -p meaning "port".
	{
		kind:        "short_password_flag",
		re:          regexp.MustCompile(`(?i)\b(mysql|psql|redis-cli|mongosh|mongo|mariadb)\b([^\n]*?)(-p\s+)([^\s]+)`),
		replacement: `${1}${2}${3}[REDACTED:short_password_flag]`,
	},
	// Docker -e KEY=SECRET / --env KEY=SECRET
	{
		kind:        "docker_env",
		re:          regexp.MustCompile(`(?i)(-e\s+|--env\s+)([A-Z][A-Z0-9_]*(?:TOKEN|SECRET|KEY|PASSWORD|PASSWD|PASS|AUTH|CREDENTIAL|APIKEY|PRIVATE)[A-Z0-9_]*=)([^\s]+)`),
		replacement: `${1}${2}[REDACTED:docker_env]`,
	},
	// Inline env prefix: FOO_TOKEN=bar cmd ... (at start or after ;/&/|)
	{
		kind:        "env_prefix",
		re:          regexp.MustCompile(`(?i)((?:^|[;&|]\s*)[A-Z][A-Z0-9_]*(?:TOKEN|SECRET|KEY|PASSWORD|PASSWD|PASS|AUTH|CREDENTIAL|APIKEY|ACCESS_TOKEN|PRIVATE)[A-Z0-9_]*=)([^\s]+)`),
		replacement: `${1}[REDACTED:env_prefix]`,
	},
	// ssh -i /path/to/key (identity file — path is not a secret itself, but
	// the filename often leaks who/what; mask filename while keeping dir).
	{
		kind:        "ssh_identity",
		re:          regexp.MustCompile(`\b(ssh|scp|rsync)(\b[^\n]*?\s+-i\s+)([^\s]+)`),
		replacement: `${1}${2}[REDACTED:ssh_identity]`,
	},
}

func scanArguments(input string) (string, []Finding) {
	out := input
	var findings []Finding
	for _, h := range argHeuristics {
		for _, loc := range h.re.FindAllStringIndex(out, -1) {
			findings = append(findings, Finding{
				Layer: LayerArgFlag,
				Kind:  h.kind,
				Start: loc[0],
				End:   loc[1],
			})
		}
		out = h.re.ReplaceAllString(out, h.replacement)
	}
	return out, findings
}
