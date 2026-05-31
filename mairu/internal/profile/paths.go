package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DefaultProfileName is the special name that resolves to the global
// (pre-profile) mairu config root: ~/.config/mairu/. All named profiles
// live as siblings under ~/.config/mairu/profiles/<name>/.
const DefaultProfileName = "default"

// nameRE accepts lowercase ASCII identifiers; matches hermes profile
// naming so distributions move cleanly between agent ecosystems.
var nameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// Reserved names cannot be profiles — they collide with the installation
// root or with mairu subcommands.
var reservedNames = map[string]struct{}{
	"mairu":   {},
	"default": {}, // valid, but handled specially in resolution
	"test":    {},
	"tmp":     {},
	"root":    {},
	"sudo":    {},
}

// NormalizeName canonicalises a name for on-disk + CLI use. Returns
// "default" case-insensitively; otherwise lowercases.
func NormalizeName(name string) (string, error) {
	s := strings.TrimSpace(name)
	if s == "" {
		return "", fmt.Errorf("profile name cannot be empty")
	}
	if strings.EqualFold(s, "default") {
		return DefaultProfileName, nil
	}
	return strings.ToLower(s), nil
}

// ValidateName checks the already-normalized name against the allowed
// pattern and the reserved-name list. Pass NormalizeName output here.
func ValidateName(name string) error {
	if name == DefaultProfileName {
		return nil
	}
	if !nameRE.MatchString(name) {
		return fmt.Errorf("invalid profile name %q: must match [a-z0-9][a-z0-9_-]{0,63}", name)
	}
	if _, ok := reservedNames[name]; ok {
		return fmt.Errorf("profile name %q is reserved", name)
	}
	return nil
}

// Root returns the directory under which named profiles live.
// Honours MAIRU_HOME if set; otherwise ~/.config/mairu.
func Root() (string, error) {
	if h := os.Getenv("MAIRU_HOME"); h != "" {
		return h, nil
	}
	if h, err := os.UserConfigDir(); err == nil {
		return filepath.Join(h, "mairu"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "mairu"), nil
}

// ProfilesRoot is where named profiles live (Root/profiles).
func ProfilesRoot() (string, error) {
	r, err := Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(r, "profiles"), nil
}

// Dir resolves a profile name to its on-disk directory.
// The special "default" resolves to Root() itself.
func Dir(name string) (string, error) {
	canon, err := NormalizeName(name)
	if err != nil {
		return "", err
	}
	if err := ValidateName(canon); err != nil {
		return "", err
	}
	if canon == DefaultProfileName {
		return Root()
	}
	pr, err := ProfilesRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(pr, canon), nil
}

// Exists reports whether a profile directory exists. "default" always exists.
func Exists(name string) (bool, error) {
	canon, err := NormalizeName(name)
	if err != nil {
		return false, err
	}
	if canon == DefaultProfileName {
		return true, nil
	}
	dir, err := Dir(canon)
	if err != nil {
		return false, err
	}
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.IsDir(), nil
}

// List returns the canonical names of all installed profiles, alphabetically.
// Does not include "default".
func List() ([]string, error) {
	pr, err := ProfilesRoot()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(pr)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}
