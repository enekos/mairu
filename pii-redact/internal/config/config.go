// Package config loads and merges pii-redact configuration from layered
// sources: embedded defaults, embedded profiles, --config-dir, --config files.
// Later sources override/extend earlier ones. Per-service overrides may add
// to safe_keys or redact_keys but may never remove from them.
package config

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Ruleset is the resolved redaction configuration passed to redactors.
// Unknown keys are redacted in strict mode; the caller toggles that.
type Ruleset struct {
	SafeKeys            map[string]struct{}
	RedactKeys          map[string]struct{}
	ContentPatterns     map[string]string // name -> regex source
	MaxSafeStringLength int
	ServiceField        string                  // JSON path used to pick service override
	ServiceOverrides    map[string]ServiceRules // service name -> additive overrides
}

// ServiceRules are additive: they can extend the global allow/deny lists but
// cannot remove keys from them.
type ServiceRules struct {
	SafeKeys   []string
	RedactKeys []string
}

// rawConfig mirrors the on-disk JSON shape so unmarshal is trivial.
type rawConfig struct {
	SafeKeys            []string          `json:"safe_keys"`
	RedactKeys          []string          `json:"redact_keys"`
	ContentPatterns     map[string]string `json:"content_patterns"`
	MaxSafeStringLength int               `json:"max_safe_string_length"`
	ServiceField        string            `json:"service_field"`
}

// LoadOptions describes the sources the caller wants merged, in priority
// order. Later sources override earlier ones for scalars (ServiceField,
// MaxSafeStringLength) and extend the lists/maps.
type LoadOptions struct {
	Profile    string   // name of a bundled profile, "" to skip
	ConfigDirs []string // directories containing global.json + services/*.json
	Configs    []string // individual config files
}

// Resolved is the merged output plus the service overrides discovered in
// --config-dir sources.
type Resolved struct {
	Ruleset Ruleset
}

//go:embed all:embedded
var embedded embed.FS

// Load resolves all sources into a single Ruleset. Returns an error if any
// source fails to parse — callers are expected to exit non-zero with no
// stdout written.
func Load(opts LoadOptions) (*Ruleset, error) {
	merged := &Ruleset{
		SafeKeys:         map[string]struct{}{},
		RedactKeys:       map[string]struct{}{},
		ContentPatterns:  map[string]string{},
		ServiceOverrides: map[string]ServiceRules{},
	}

	// 1. Always merge embedded defaults first (just content patterns).
	if err := mergeFromFS(merged, embedded, "embedded/defaults/patterns.json"); err != nil {
		return nil, fmt.Errorf("embedded defaults: %w", err)
	}

	// 2. Named profile.
	if opts.Profile != "" {
		path := "embedded/profiles/" + opts.Profile + ".json"
		if _, err := fs.Stat(embedded, path); err != nil {
			return nil, fmt.Errorf("profile %q not found", opts.Profile)
		}
		if err := mergeFromFS(merged, embedded, path); err != nil {
			return nil, fmt.Errorf("profile %s: %w", opts.Profile, err)
		}
	}

	// 3. Config dirs — global.json + services/*.json.
	for _, dir := range opts.ConfigDirs {
		if err := mergeConfigDir(merged, dir); err != nil {
			return nil, fmt.Errorf("config-dir %s: %w", dir, err)
		}
	}

	// 4. Individual config files.
	for _, path := range opts.Configs {
		if err := mergeFromDisk(merged, path); err != nil {
			return nil, fmt.Errorf("config %s: %w", path, err)
		}
	}

	return merged, nil
}

func mergeFromFS(dst *Ruleset, fsys fs.FS, path string) error {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return err
	}
	return applyRaw(dst, data, path)
}

func mergeFromDisk(dst *Ruleset, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return applyRaw(dst, data, path)
}

func applyRaw(dst *Ruleset, data []byte, source string) error {
	var raw rawConfig
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return fmt.Errorf("parse %s: %w", source, err)
	}
	for _, k := range raw.SafeKeys {
		dst.SafeKeys[k] = struct{}{}
	}
	for _, k := range raw.RedactKeys {
		dst.RedactKeys[k] = struct{}{}
	}
	for name, pat := range raw.ContentPatterns {
		dst.ContentPatterns[name] = pat
	}
	if raw.MaxSafeStringLength > 0 {
		dst.MaxSafeStringLength = raw.MaxSafeStringLength
	}
	if raw.ServiceField != "" {
		dst.ServiceField = raw.ServiceField
	}
	return nil
}

func mergeConfigDir(dst *Ruleset, dir string) error {
	globalPath := filepath.Join(dir, "global.json")
	if _, err := os.Stat(globalPath); err == nil {
		if err := mergeFromDisk(dst, globalPath); err != nil {
			return err
		}
	}
	svcDir := filepath.Join(dir, "services")
	entries, err := os.ReadDir(svcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		data, err := os.ReadFile(filepath.Join(svcDir, e.Name()))
		if err != nil {
			return err
		}
		var raw rawConfig
		dec := json.NewDecoder(strings.NewReader(string(data)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&raw); err != nil {
			return fmt.Errorf("parse services/%s: %w", e.Name(), err)
		}
		existing := dst.ServiceOverrides[name]
		existing.SafeKeys = append(existing.SafeKeys, raw.SafeKeys...)
		existing.RedactKeys = append(existing.RedactKeys, raw.RedactKeys...)
		dst.ServiceOverrides[name] = existing
	}
	return nil
}

// ResolveForService returns the merged Ruleset with a service override
// applied (additive only). If name is empty or unknown, the global ruleset
// is returned unchanged.
func (r *Ruleset) ResolveForService(name string) *Ruleset {
	if name == "" {
		return r
	}
	override, ok := r.ServiceOverrides[name]
	if !ok {
		return r
	}
	clone := &Ruleset{
		SafeKeys:            copySet(r.SafeKeys),
		RedactKeys:          copySet(r.RedactKeys),
		ContentPatterns:     r.ContentPatterns,
		MaxSafeStringLength: r.MaxSafeStringLength,
		ServiceField:        r.ServiceField,
		ServiceOverrides:    r.ServiceOverrides,
	}
	for _, k := range override.SafeKeys {
		clone.SafeKeys[k] = struct{}{}
	}
	for _, k := range override.RedactKeys {
		clone.RedactKeys[k] = struct{}{}
	}
	return clone
}

func copySet(s map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(s))
	for k := range s {
		out[k] = struct{}{}
	}
	return out
}

// SortedServiceNames is a helper for deterministic iteration in tests.
func (r *Ruleset) SortedServiceNames() []string {
	names := make([]string, 0, len(r.ServiceOverrides))
	for k := range r.ServiceOverrides {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
