package ingest

import (
	"os"
	"path/filepath"
)

// ResolveProject infers a project name by walking up the directory tree
// from cwd looking for marker files.
//
// Algorithm:
//  1. If cwd is "" or a path that can't be stat'd, return "".
//  2. Walk up from cwd (inclusive) looking for a file or directory named
//     ".mairu". If found, return filepath.Base of the directory CONTAINING
//     the marker (not the marker itself).
//  3. Otherwise walk up looking for ".git". If found, return
//     filepath.Base of the directory containing it.
//  4. Otherwise return filepath.Base(cwd).
//
// The walk stops at the filesystem root (parent == current).
func ResolveProject(cwd string) string {
	if cwd == "" {
		return ""
	}
	if _, err := os.Stat(cwd); err != nil {
		return ""
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return ""
	}

	if dir := findMarker(abs, ".mairu"); dir != "" {
		return filepath.Base(dir)
	}
	if dir := findMarker(abs, ".git"); dir != "" {
		return filepath.Base(dir)
	}

	// Neither marker found. Special-case the filesystem root so we don't
	// return "/" or "" unpredictably.
	if abs == string(filepath.Separator) {
		return ""
	}
	return filepath.Base(abs)
}

func findMarker(start, name string) string {
	d := start
	for {
		if _, err := os.Stat(filepath.Join(d, name)); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return ""
		}
		d = parent
	}
}
