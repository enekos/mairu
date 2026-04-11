package fsutil

import (
	"os"
	"path/filepath"
	"sync"

	ignore "github.com/sabhiram/go-gitignore"
)

type ProjectIgnorer struct {
	cache map[string]*ignore.GitIgnore
	mu    sync.RWMutex
}

func NewProjectIgnorer(startDir string) *ProjectIgnorer {
	pi := &ProjectIgnorer{
		cache: make(map[string]*ignore.GitIgnore),
	}

	absDir, err := filepath.Abs(startDir)
	if err == nil {
		current := absDir
		for {
			pi.load(current)
			if info, err := os.Stat(filepath.Join(current, ".git")); err == nil && info.IsDir() {
				break
			}
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
			current = parent
		}
	}
	return pi
}

func (pi *ProjectIgnorer) load(dir string) *ignore.GitIgnore {
	pi.mu.RLock()
	gi, exists := pi.cache[dir]
	pi.mu.RUnlock()
	if exists {
		return gi
	}

	pi.mu.Lock()
	defer pi.mu.Unlock()

	if gi, exists := pi.cache[dir]; exists {
		return gi
	}

	gi, _ = ignore.CompileIgnoreFile(filepath.Join(dir, ".gitignore"))
	pi.cache[dir] = gi
	return gi
}

func (pi *ProjectIgnorer) IsIgnored(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	currentDir := filepath.Dir(absPath)

	for {
		if gi := pi.load(currentDir); gi != nil {
			rel, err := filepath.Rel(currentDir, absPath)
			if err == nil && gi.MatchesPath(rel) {
				return true
			}
		}

		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			break
		}
		currentDir = parent
	}

	return false
}
