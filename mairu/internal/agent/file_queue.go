package agent

import (
	"path/filepath"
	"sync"
)

// fileMutationQueue serializes concurrent writes to the same file. Inspired
// by pi-mono's file-mutation-queue.ts: when the model fires off two edits to
// the same target in parallel, we want them ordered, not racing.
//
// Keyed by realpath (symlinks resolved); falls back to the cleaned input if
// the path doesn't exist yet.
type fileMutationQueue struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func newFileMutationQueue() *fileMutationQueue {
	return &fileMutationQueue{locks: map[string]*sync.Mutex{}}
}

func (q *fileMutationQueue) lockFor(path string) *sync.Mutex {
	key, err := filepath.EvalSymlinks(path)
	if err != nil {
		key = filepath.Clean(path)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	m, ok := q.locks[key]
	if !ok {
		m = &sync.Mutex{}
		q.locks[key] = m
	}
	return m
}

// With runs fn while holding the per-file lock for path.
func (q *fileMutationQueue) With(path string, fn func() error) error {
	m := q.lockFor(path)
	m.Lock()
	defer m.Unlock()
	return fn()
}
