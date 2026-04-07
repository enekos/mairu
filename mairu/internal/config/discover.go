package config

import (
	"os"
	"path/filepath"
)

const projectConfigName = ".mairu.toml"

// FindProjectConfig walks up from startDir looking for a .mairu.toml file.
// It stops at the filesystem root or at a .git boundary (the directory
// containing .git is included in the search, but its parent is not).
// Returns the absolute path to the config file, or "" if not found.
func FindProjectConfig(startDir string) string {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return ""
	}

	for {
		candidate := filepath.Join(dir, projectConfigName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}

		// If this directory contains .git, don't go higher.
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return ""
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "" // filesystem root
		}
		dir = parent
	}
}
