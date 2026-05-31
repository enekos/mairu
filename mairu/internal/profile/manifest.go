// Package profile implements mairu's profile distribution system:
// packaged, git-distributable bundles that override the bundled system
// prompt and config for a named profile. Modelled on hermes-agent's
// profile distributions; v0 scope is prompt + config only.
package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ManifestFilename is the well-known name of a distribution manifest at
// the root of a profile directory or distribution repo.
const ManifestFilename = "distribution.yaml"

// EnvExampleFilename is the file emitted at install time so users can fill
// in their own values without having a real .env clobbered by updates.
const EnvExampleFilename = ".env.EXAMPLE"

// DefaultDistOwned is the set of paths a distribution claims ownership of
// when the manifest does not list `distribution_owned` explicitly.
// On update these are replaced from the new source; user-owned paths
// (see UserOwnedExclude) are never touched.
var DefaultDistOwned = []string{
	"agent_system.md",
	"prompts",
	"config.toml",
	ManifestFilename,
}

// UserOwnedExclude lists profile-root entries that are NEVER part of a
// distribution. They are skipped on copy and protected on update.
var UserOwnedExclude = map[string]struct{}{
	".env":         {},
	"mairu.db":     {},
	"mairu.db-shm": {},
	"mairu.db-wal": {},
	"logs":         {},
	"sessions":     {},
	"local":        {},
	"active":       {}, // sticky-active marker (parallel to hermes active_profile)
}

// EnvRequirement describes an environment variable a distribution needs.
type EnvRequirement struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	Required    bool   `yaml:"required"`
	Default     string `yaml:"default,omitempty"`
}

// Manifest is the parsed distribution.yaml.
type Manifest struct {
	Name              string           `yaml:"name"`
	Version           string           `yaml:"version,omitempty"`
	Description       string           `yaml:"description,omitempty"`
	MairuRequires     string           `yaml:"mairu_requires,omitempty"`
	Author            string           `yaml:"author,omitempty"`
	License           string           `yaml:"license,omitempty"`
	EnvRequires       []EnvRequirement `yaml:"env_requires,omitempty"`
	DistributionOwned []string         `yaml:"distribution_owned,omitempty"`

	// Source records where the distribution was pulled from so that
	// `mairu profile update` can re-pull without the user passing the URL
	// again. Authors do not populate this — it's stamped on install.
	Source string `yaml:"source,omitempty"`
	// InstalledAt is an ISO-8601 timestamp stamped on install/update.
	InstalledAt string `yaml:"installed_at,omitempty"`
}

// OwnedPaths returns DistributionOwned or DefaultDistOwned if unset.
func (m *Manifest) OwnedPaths() []string {
	if len(m.DistributionOwned) > 0 {
		out := make([]string, 0, len(m.DistributionOwned))
		for _, p := range m.DistributionOwned {
			p = strings.Trim(strings.TrimSpace(p), "/")
			if p != "" {
				out = append(out, p)
			}
		}
		return out
	}
	return append([]string(nil), DefaultDistOwned...)
}

// ReadManifest parses distribution.yaml from a profile directory. Returns
// (nil, nil) if the file does not exist — the caller decides whether that
// is an error (install) or just means "not a distribution" (info).
func ReadManifest(dir string) (*Manifest, error) {
	path := filepath.Join(dir, ManifestFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var m Manifest
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(false)
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	m.Name = strings.TrimSpace(m.Name)
	if m.Name == "" {
		return nil, fmt.Errorf("%s: missing 'name'", path)
	}
	return &m, nil
}

// WriteManifest writes m to <dir>/distribution.yaml.
func WriteManifest(dir string, m *Manifest) error {
	path := filepath.Join(dir, ManifestFilename)
	out, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// versionOpRE matches a leading comparator in a `mairu_requires` spec.
var versionOpRE = regexp.MustCompile(`^\s*(>=|<=|==|!=|>|<)\s*(.+?)\s*$`)

type semver struct{ major, minor, patch int }

func parseSemver(v string) (semver, error) {
	s := strings.TrimSpace(v)
	s = strings.TrimPrefix(s, "v")
	// Strip pre-release / build metadata (e.g. "0.12.0-rc1+abc").
	for _, sep := range []string{"-", "+"} {
		if i := strings.Index(s, sep); i >= 0 {
			s = s[:i]
		}
	}
	parts := strings.Split(s, ".")
	for len(parts) < 3 {
		parts = append(parts, "0")
	}
	var sv semver
	var err error
	if sv.major, err = strconv.Atoi(parts[0]); err != nil {
		return sv, fmt.Errorf("unparseable version %q", v)
	}
	if sv.minor, err = strconv.Atoi(parts[1]); err != nil {
		return sv, fmt.Errorf("unparseable version %q", v)
	}
	if sv.patch, err = strconv.Atoi(parts[2]); err != nil {
		return sv, fmt.Errorf("unparseable version %q", v)
	}
	return sv, nil
}

func (a semver) cmp(b semver) int {
	switch {
	case a.major != b.major:
		return a.major - b.major
	case a.minor != b.minor:
		return a.minor - b.minor
	default:
		return a.patch - b.patch
	}
}

// CheckMairuRequires returns an error if currentVersion does not satisfy
// spec. Empty spec is a no-op. Empty currentVersion is also a no-op — dev
// builds (`go run`, ad-hoc binaries) have no meaningful version and would
// otherwise fail every `>=anything` check. A bare version ("0.5.0") is
// treated as ">=".
func CheckMairuRequires(spec, currentVersion string) error {
	spec = strings.TrimSpace(spec)
	if spec == "" || strings.TrimSpace(currentVersion) == "" {
		return nil
	}
	op, target := ">=", spec
	if m := versionOpRE.FindStringSubmatch(spec); m != nil {
		op, target = m[1], m[2]
	}
	cur, err := parseSemver(currentVersion)
	if err != nil {
		return err
	}
	tgt, err := parseSemver(target)
	if err != nil {
		return err
	}
	c := cur.cmp(tgt)
	ok := false
	switch op {
	case ">=":
		ok = c >= 0
	case "<=":
		ok = c <= 0
	case "==":
		ok = c == 0
	case "!=":
		ok = c != 0
	case ">":
		ok = c > 0
	case "<":
		ok = c < 0
	}
	if !ok {
		return fmt.Errorf("distribution requires mairu %s%s, but this is %s", op, target, currentVersion)
	}
	return nil
}

// RenderEnvExample produces the body of a .env.EXAMPLE file from a
// manifest's env_requires.
func RenderEnvExample(m *Manifest) string {
	var b strings.Builder
	b.WriteString("# Environment variables required by this mairu distribution.\n")
	b.WriteString("# Copy to `.env` and fill in your own values before running.\n\n")
	for _, r := range m.EnvRequires {
		if r.Description != "" {
			fmt.Fprintf(&b, "# %s\n", r.Description)
		}
		status := "required"
		prefix := ""
		if !r.Required {
			status = "optional"
			prefix = "# "
		}
		fmt.Fprintf(&b, "# (%s)\n", status)
		fmt.Fprintf(&b, "%s%s=%s\n\n", prefix, r.Name, r.Default)
	}
	return b.String()
}
