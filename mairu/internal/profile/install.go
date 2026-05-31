package profile

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Plan describes what an Install or Update will do, suitable for surfacing
// to the user before any files are written.
type Plan struct {
	Manifest        *Manifest
	StagedDir       string
	Source          string // provenance written into the installed manifest
	TargetDir       string
	Existing        bool
	PreservesConfig bool
}

// InstallOptions configure Install.
type InstallOptions struct {
	// Override the manifest's name (so the same dist can be installed twice
	// under different local names).
	Name string
	// MairuVersion is the running binary's semver, passed in so package
	// profile does not depend on a global version constant.
	MairuVersion string
	// Force overwrites an existing profile of the same name.
	Force bool
	// SkipBootstrap suppresses creating user-owned subdirs (used by tests).
	SkipBootstrap bool
}

// UpdateOptions configure Update.
type UpdateOptions struct {
	MairuVersion string
	// ForceConfig replaces config.toml from the new source. By default
	// config.toml is preserved on update (the user may have tuned it).
	ForceConfig bool
}

// Install stages source (git URL or local dir), validates the manifest,
// and copies distribution-owned files into the resolved profile dir.
// Returns the resolved Plan.
func Install(source string, opts InstallOptions) (*Plan, error) {
	workdir, err := os.MkdirTemp("", "mairu_dist_install_")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workdir)

	plan, err := planFromSource(source, workdir, opts.Name, opts.MairuVersion)
	if err != nil {
		return nil, err
	}

	if plan.Existing && !opts.Force {
		return nil, fmt.Errorf("profile %q already exists at %s — use `mairu profile update` or pass --force",
			plan.Manifest.Name, plan.TargetDir)
	}

	plan.PreservesConfig = false // fresh install gets distribution's config.toml

	if !opts.SkipBootstrap {
		if err := bootstrapUserDirs(plan.TargetDir); err != nil {
			return nil, err
		}
	}
	if err := copyPayload(plan.StagedDir, plan.TargetDir, plan.Manifest, plan.PreservesConfig); err != nil {
		return nil, err
	}
	return plan, nil
}

// Update re-pulls a previously-installed profile's distribution and
// re-applies it, preserving config.toml unless ForceConfig.
func Update(name string, opts UpdateOptions) (*Plan, error) {
	canon, err := NormalizeName(name)
	if err != nil {
		return nil, err
	}
	if err := ValidateName(canon); err != nil {
		return nil, err
	}
	target, err := Dir(canon)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("profile %q does not exist", canon)
	}
	existing, err := ReadManifest(target)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("profile %q has no %s — only profiles installed via `mairu profile install` can be updated",
			canon, ManifestFilename)
	}
	if existing.Source == "" {
		return nil, fmt.Errorf("profile %q has no recorded source — reinstall with `mairu profile install <source> --name %s --force`",
			canon, canon)
	}

	workdir, err := os.MkdirTemp("", "mairu_dist_update_")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workdir)

	plan, err := planFromSource(existing.Source, workdir, canon, opts.MairuVersion)
	if err != nil {
		return nil, err
	}
	plan.PreservesConfig = !opts.ForceConfig
	if err := copyPayload(plan.StagedDir, plan.TargetDir, plan.Manifest, plan.PreservesConfig); err != nil {
		return nil, err
	}
	return plan, nil
}

// Describe returns the manifest for a profile, or nil if the profile is
// not a distribution. Errors only on missing profile or read failure.
func Describe(name string) (*Manifest, string, error) {
	canon, err := NormalizeName(name)
	if err != nil {
		return nil, "", err
	}
	if err := ValidateName(canon); err != nil {
		return nil, "", err
	}
	target, err := Dir(canon)
	if err != nil {
		return nil, "", err
	}
	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		return nil, target, fmt.Errorf("profile %q does not exist", canon)
	}
	m, err := ReadManifest(target)
	return m, target, err
}

// ----- internals --------------------------------------------------------

func planFromSource(source, workdir, overrideName, mairuVersion string) (*Plan, error) {
	staged, provenance, err := stageSource(source, workdir)
	if err != nil {
		return nil, err
	}
	if err := rejectSymlinks(staged); err != nil {
		return nil, err
	}
	m, err := ReadManifest(staged)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("no %s at the root of %q — this is not a mairu distribution",
			ManifestFilename, source)
	}
	if err := CheckMairuRequires(m.MairuRequires, mairuVersion); err != nil {
		return nil, err
	}

	targetName := overrideName
	if targetName == "" {
		targetName = m.Name
	}
	canon, err := NormalizeName(targetName)
	if err != nil {
		return nil, err
	}
	if err := ValidateName(canon); err != nil {
		return nil, err
	}
	if canon == DefaultProfileName {
		return nil, fmt.Errorf("cannot install a distribution as %q — that's the built-in root profile; pass --name <name>", DefaultProfileName)
	}
	m.Name = canon
	m.Source = provenance
	m.InstalledAt = time.Now().UTC().Format(time.RFC3339)

	target, err := Dir(canon)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(target)
	existing := err == nil && info.IsDir()

	return &Plan{
		Manifest:  m,
		StagedDir: staged,
		Source:    provenance,
		TargetDir: target,
		Existing:  existing,
	}, nil
}

var bareGithubRE = regexp.MustCompile(`^github\.com/[\w.-]+/[\w.-]+/?$`)

func looksLikeGitURL(s string) bool {
	s = strings.TrimSpace(s)
	switch {
	case strings.HasSuffix(s, ".git"):
		return true
	case strings.HasPrefix(s, "git@"),
		strings.HasPrefix(s, "ssh://"),
		strings.HasPrefix(s, "git://"),
		strings.HasPrefix(s, "https://"),
		strings.HasPrefix(s, "http://"):
		return true
	case bareGithubRE.MatchString(s):
		return true
	}
	return false
}

func stageSource(source, workdir string) (stagedDir, provenance string, err error) {
	src := strings.TrimSpace(source)
	if looksLikeGitURL(src) {
		url := src
		if bareGithubRE.MatchString(url) {
			url = "https://" + strings.TrimRight(url, "/")
		}
		clone := filepath.Join(workdir, "clone")
		cmd := exec.Command("git", "clone", "--depth", "1", url, clone)
		out, runErr := cmd.CombinedOutput()
		if runErr != nil {
			return "", "", fmt.Errorf("git clone failed: %s", strings.TrimSpace(string(out)))
		}
		_ = os.RemoveAll(filepath.Join(clone, ".git"))
		if _, statErr := os.Stat(filepath.Join(clone, ManifestFilename)); statErr != nil {
			return "", "", fmt.Errorf("no %s at the root of %q — not a mairu distribution", ManifestFilename, src)
		}
		return clone, src, nil
	}

	// Local directory
	abs, absErr := filepath.Abs(src)
	if absErr != nil {
		return "", "", absErr
	}
	info, statErr := os.Stat(abs)
	if statErr != nil || !info.IsDir() {
		return "", "", fmt.Errorf("cannot resolve distribution source %q: expected a git URL or local directory", source)
	}
	if _, err := os.Stat(filepath.Join(abs, ManifestFilename)); err != nil {
		return "", "", fmt.Errorf("no %s in %s — local-directory source must contain a manifest", ManifestFilename, abs)
	}
	return abs, abs, nil
}

func rejectSymlinks(root string) error {
	return filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			rel, _ := filepath.Rel(root, p)
			return fmt.Errorf("profile distributions cannot contain symlinks: %s", rel)
		}
		return nil
	})
}

func bootstrapUserDirs(target string) error {
	for _, d := range []string{"logs", "sessions", "local"} {
		if err := os.MkdirAll(filepath.Join(target, d), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// copyPayload copies non-user-owned entries from staged into target.
// On preserveConfig, an existing target/config.toml is left alone.
// .env.EXAMPLE is emitted from the manifest if the staged tree did not ship one.
func copyPayload(staged, target string, m *Manifest, preserveConfig bool) error {
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}

	entries, err := os.ReadDir(staged)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if _, skip := UserOwnedExclude[name]; skip {
			continue
		}
		if name == "config.toml" && preserveConfig {
			if _, err := os.Stat(filepath.Join(target, name)); err == nil {
				continue
			}
		}

		src := filepath.Join(staged, name)
		dst := filepath.Join(target, name)
		if entry.IsDir() {
			if err := os.RemoveAll(dst); err != nil {
				return err
			}
			if err := copyTreeExcludingUserOwned(src, dst); err != nil {
				return err
			}
		} else {
			if err := copyFile(src, dst); err != nil {
				return err
			}
		}
	}

	// Emit .env.EXAMPLE from manifest if not already shipped.
	if len(m.EnvRequires) > 0 {
		envExamplePath := filepath.Join(target, EnvExampleFilename)
		if _, err := os.Stat(envExamplePath); os.IsNotExist(err) {
			if err := os.WriteFile(envExamplePath, []byte(RenderEnvExample(m)), 0o644); err != nil {
				return err
			}
		}
	}

	// Make sure manifest on disk reflects resolved name + source.
	return WriteManifest(target, m)
}

func copyTreeExcludingUserOwned(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, info.Mode().Perm())
		}
		if _, skip := UserOwnedExclude[info.Name()]; skip {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		return copyFile(p, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
