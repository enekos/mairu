package agent

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFileMutationQueue_SerializesSamePath(t *testing.T) {
	q := newFileMutationQueue()
	tmp := filepath.Join(t.TempDir(), "x.txt")
	if err := os.WriteFile(tmp, nil, 0644); err != nil {
		t.Fatal(err)
	}

	var inFlight, maxInFlight int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = q.With(tmp, func() error {
				cur := atomic.AddInt32(&inFlight, 1)
				for {
					m := atomic.LoadInt32(&maxInFlight)
					if cur <= m || atomic.CompareAndSwapInt32(&maxInFlight, m, cur) {
						break
					}
				}
				time.Sleep(2 * time.Millisecond)
				atomic.AddInt32(&inFlight, -1)
				return nil
			})
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&maxInFlight); got != 1 {
		t.Fatalf("expected serialized access (max=1), got max=%d", got)
	}
}

func TestFileMutationQueue_DistinctPathsAreParallel(t *testing.T) {
	q := newFileMutationQueue()
	dir := t.TempDir()

	var insideA, insideB int32
	var sawOverlap int32
	var wg sync.WaitGroup

	work := func(path string, mine *int32, other *int32) {
		defer wg.Done()
		_ = q.With(path, func() error {
			atomic.StoreInt32(mine, 1)
			defer atomic.StoreInt32(mine, 0)
			// Wait briefly for the other goroutine to overlap.
			deadline := time.Now().Add(100 * time.Millisecond)
			for time.Now().Before(deadline) {
				if atomic.LoadInt32(other) == 1 {
					atomic.StoreInt32(&sawOverlap, 1)
					return nil
				}
				time.Sleep(time.Millisecond)
			}
			return nil
		})
	}

	wg.Add(2)
	go work(filepath.Join(dir, "a"), &insideA, &insideB)
	go work(filepath.Join(dir, "b"), &insideB, &insideA)
	wg.Wait()

	if atomic.LoadInt32(&sawOverlap) != 1 {
		t.Fatal("expected distinct-path operations to run concurrently")
	}
}

func TestFileMutationQueue_ResolvesSymlinks(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported here: %v", err)
	}

	q := newFileMutationQueue()
	if q.lockFor(target) != q.lockFor(link) {
		t.Fatal("expected symlink and real path to share a lock")
	}
}
