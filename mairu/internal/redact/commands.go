package redact

import "strings"

// riskyArgs maps a denylisted program name to tokens that, when present,
// signal the invocation is secret-adjacent (and the whole arg list should
// be collapsed). A bare program invocation without any risky token passes
// through untouched — we don't want `aws s3 ls` to get flattened.
var riskyArgs = map[string][]string{
	"vault":  {"kv", "read", "write", "login", "token"},
	"op":     {"read", "item", "signin", "get"},
	"pass":   {"show", "insert", "edit", "generate"},
	"gpg":    {"--decrypt", "-d", "--export-secret-keys"},
	"aws":    {"configure", "sso"},
	"gh":     {"auth"},
	"doctl":  {"auth"},
	"gcloud": {"auth"},
}

// scanCommandDenylist applies Layer 4 only to KindCommand inputs. The first
// whitespace-separated token is the program name; if it's on the configured
// denylist AND the command includes one of the program's risky-arg tokens,
// the whole argument list is collapsed to "<program> [REDACTED:denylisted_command]".
func (r *Redactor) scanCommandDenylist(input string, kind Kind) (string, []Finding) {
	if kind != KindCommand {
		return input, nil
	}
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return input, nil
	}
	program := fields[0]

	for _, d := range r.denylistCommands {
		if d != program {
			continue
		}
		risky := riskyArgs[program]
		if len(risky) == 0 {
			continue
		}
		if !containsAny(input, risky) {
			continue
		}
		return program + " [REDACTED:denylisted_command]", []Finding{{
			Layer: LayerDenylist,
			Kind:  "denylisted_command",
			Start: 0,
			End:   len(input),
		}}
	}
	return input, nil
}

func containsAny(s string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}
