package walkers

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/enekos/mairu/pii-redact/internal/config"
	"github.com/enekos/mairu/pii-redact/internal/mask"
	"github.com/enekos/mairu/pii-redact/internal/patterns"
)

// Options controls the structured JSON redactor's policy.
type Options struct {
	Rules     *config.Ruleset
	Set       *patterns.Set
	Masker    *mask.Masker // optional; controls key-based masking style
	Strict    bool         // true: unknown keys are redacted; false: unknown keys pass through
	ServiceOf func(entry any) string
}

// Redacted sentinels used when no Masker is configured (opaque mode).
const (
	tokenRedactKey     = "[REDACTED:KEY]"
	tokenRedactUnknown = "[REDACTED:UNKNOWN_KEY]"
)

// JSON reads a single JSON value (object OR array) from `in`, redacts it,
// and writes the indented JSON result to `out`. Top-level arrays have each
// element treated as a log entry for service-override purposes.
func JSON(in io.Reader, out io.Writer, opts Options) (patterns.Stats, error) {
	totals := patterns.Stats{}
	dec := json.NewDecoder(in)
	dec.UseNumber()

	var root any
	if err := dec.Decode(&root); err != nil {
		return totals, fmt.Errorf("json parse: %w", err)
	}

	var redacted any
	switch v := root.(type) {
	case []any:
		arr := make([]any, len(v))
		for i, elem := range v {
			rules := rulesForEntry(elem, opts)
			arr[i] = walk(elem, rules, opts, totals, "")
		}
		redacted = arr
	default:
		rules := rulesForEntry(v, opts)
		redacted = walk(v, rules, opts, totals, "")
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(redacted); err != nil {
		return totals, err
	}
	return totals, nil
}

// NDJSON reads one JSON object per line, redacts each, and writes one
// object per line. Preserves the NDJSON framing.
func NDJSON(in io.Reader, out io.Writer, opts Options) (patterns.Stats, error) {
	totals := patterns.Stats{}
	dec := json.NewDecoder(in)
	dec.UseNumber()
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)

	for {
		var entry any
		if err := dec.Decode(&entry); err != nil {
			if err == io.EOF {
				return totals, nil
			}
			return totals, fmt.Errorf("json parse: %w", err)
		}
		rules := rulesForEntry(entry, opts)
		redacted := walk(entry, rules, opts, totals, "")
		if err := enc.Encode(redacted); err != nil {
			return totals, err
		}
	}
}

func rulesForEntry(entry any, opts Options) *config.Ruleset {
	if opts.ServiceOf == nil {
		return opts.Rules
	}
	svc := opts.ServiceOf(entry)
	return opts.Rules.ResolveForService(svc)
}

// walk recursively redacts a decoded JSON value.
func walk(v any, rules *config.Ruleset, opts Options, stats patterns.Stats, parentKey string) any {
	switch node := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(node))
		for k, child := range node {
			out[k] = applyPolicy(k, child, rules, opts, stats)
		}
		return out
	case []any:
		arr := make([]any, len(node))
		for i, child := range node {
			arr[i] = walk(child, rules, opts, stats, parentKey)
		}
		return arr
	case string:
		return redactString(node, rules, opts, stats)
	default:
		return node
	}
}

func applyPolicy(key string, value any, rules *config.Ruleset, opts Options, stats patterns.Stats) any {
	if _, ok := rules.RedactKeys[key]; ok {
		stats["[KEY]"]++
		return maskKeyedValue(key, value, opts)
	}
	_, isSafe := rules.SafeKeys[key]

	switch v := value.(type) {
	case map[string]any, []any:
		return walk(v, rules, opts, stats, key)
	case string:
		if isSafe || !opts.Strict {
			return finishString(v, rules, opts, stats)
		}
		stats["[UNKNOWN_KEY]"]++
		return unknownKeyValue(v, opts)
	default:
		if isSafe || !opts.Strict {
			return v
		}
		stats["[UNKNOWN_KEY]"]++
		if opts.Masker != nil && opts.Masker.Reveal {
			if n, ok := v.(json.Number); ok {
				s := n.String()
				if len(s) >= 3 {
					return keepEnds(s)
				}
			}
		}
		return tokenRedactUnknown
	}
}

// maskKeyedValue handles a value under a redact_keys entry. If partial
// reveal is on and the value is a string, we run content-regex first (so
// an email value still renders as "j***@a***.io") and otherwise fall
// back to a length-preserving opaque form.
func maskKeyedValue(key string, value any, opts Options) any {
	if opts.Masker == nil || !opts.Masker.Reveal {
		return tokenRedactKey
	}
	switch v := value.(type) {
	case string:
		if out, _ := opts.Set.Redact(v); out != v {
			return out
		}
		if len(v) >= 3 {
			return keepEnds(v)
		}
		return strings.Repeat("*", len(v))
	case json.Number:
		s := v.String()
		return keepEnds(s)
	case nil:
		return nil
	default:
		// container under a redact key: opaque — we cannot guess a sane
		// per-field mask without leaking more structure than a marker.
		return tokenRedactKey
	}
}

func unknownKeyValue(v string, opts Options) any {
	if opts.Masker != nil && opts.Masker.Reveal {
		// keep pattern-matched hints (IPs, emails) but nothing else.
		if out, _ := opts.Set.Redact(v); out != v {
			return out
		}
		if len(v) >= 3 {
			return keepEnds(v)
		}
	}
	return tokenRedactUnknown
}

func keepEnds(s string) string {
	if len(s) <= 4 {
		return s[:1] + strings.Repeat("*", len(s)-1)
	}
	return s[:1] + strings.Repeat("*", len(s)-2) + s[len(s)-1:]
}

func finishString(s string, rules *config.Ruleset, opts Options, stats patterns.Stats) string {
	s = redactString(s, rules, opts, stats)
	if rules.MaxSafeStringLength > 0 && len(s) > rules.MaxSafeStringLength {
		truncated := s[:rules.MaxSafeStringLength]
		s = fmt.Sprintf("%s…[+%d chars]", truncated, len(s)-rules.MaxSafeStringLength)
	}
	return s
}

func redactString(s string, _ *config.Ruleset, opts Options, stats patterns.Stats) string {
	if opts.Set == nil {
		return s
	}
	out, match := opts.Set.Redact(s)
	for k, v := range match {
		stats[k] += v
	}
	return out
}

// ExtractByPath pulls a string value from a decoded log entry using a
// dotted path like "resource.labels.container_name". Missing intermediates
// return "".
func ExtractByPath(entry any, path string) string {
	if path == "" {
		return ""
	}
	parts := strings.Split(path, ".")
	cur := entry
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[p]
	}
	if s, ok := cur.(string); ok {
		return s
	}
	return ""
}
