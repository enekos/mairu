// Package patterns compiles content regex patterns and applies them to
// arbitrary strings. A Set is built once from config and reused for the
// lifetime of the process.
package patterns

import (
	"fmt"
	"regexp"
	"sort"

	"github.com/enekos/mairu/pii-redact/internal/mask"
)

// Set is a compiled, ordered collection of named patterns. Order matters
// for deterministic output and for letting long/specific patterns (jwt)
// take precedence over short/generic ones (ipv4) when ranges overlap.
type Set struct {
	entries    []entry
	validators map[string]mask.Validator
	masker     *mask.Masker
}

type entry struct {
	name string
	re   *regexp.Regexp
}

// Stats tracks how many matches each pattern produced.
type Stats = map[string]int

// Compile builds a Set from a name->regex source map. Ordering is stable
// alphabetical so behavior is reproducible across runs. The default
// masker is opaque (full redaction); use WithMasker to opt into
// partial-reveal.
func Compile(src map[string]string) (*Set, error) {
	if len(src) == 0 {
		return &Set{masker: mask.NewMasker(false), validators: mask.Validators}, nil
	}
	names := make([]string, 0, len(src))
	for n := range src {
		names = append(names, n)
	}
	sort.Strings(names)

	set := &Set{
		entries:    make([]entry, 0, len(names)),
		validators: mask.Validators,
		masker:     mask.NewMasker(false),
	}
	for _, name := range names {
		re, err := regexp.Compile(src[name])
		if err != nil {
			return nil, fmt.Errorf("compile pattern %q: %w", name, err)
		}
		set.entries = append(set.entries, entry{name: name, re: re})
	}
	return set, nil
}

// WithMasker swaps the masker used by Redact. nil is allowed and means
// "opaque redaction".
func (s *Set) WithMasker(m *mask.Masker) *Set {
	if m == nil {
		s.masker = mask.NewMasker(false)
	} else {
		s.masker = m
	}
	return s
}

// Redact replaces every validated match with the masker's output.
// Rejected matches (validator returned false) pass through untouched.
func (s *Set) Redact(in string) (string, Stats) {
	stats := Stats{}
	if s == nil || len(s.entries) == 0 {
		return in, stats
	}
	out := in
	for _, e := range s.entries {
		name := e.name
		out = e.re.ReplaceAllStringFunc(out, func(raw string) string {
			if v, ok := s.validators[name]; ok && !v(raw) {
				return raw
			}
			stats[name]++
			return s.masker.Apply(name, raw)
		})
	}
	return out, stats
}

// Names returns the pattern names in compilation order. Useful for tests
// and for stats iteration.
func (s *Set) Names() []string {
	out := make([]string, len(s.entries))
	for i, e := range s.entries {
		out[i] = e.name
	}
	return out
}
