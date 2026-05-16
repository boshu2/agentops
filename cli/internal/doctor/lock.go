package doctor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ErrLockHeld is the sentinel error returned when an advisory lock is already
// held by another process. Callers map it to exit code 5 (concurrency_lost).
var ErrLockHeld = errors.New("doctor: lock held by another process")

// Guard represents an acquired advisory lock. Release must be called (typically
// via defer) to drop the lock and close the underlying file descriptor.
type Guard struct {
	file     *os.File
	path     string
	mgr      *LockManager
	released bool
	mu       sync.Mutex
}

// Release drops the advisory lock and closes the lockfile descriptor. It is
// idempotent and safe to call multiple times.
func (g *Guard) Release() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.released || g.file == nil {
		return nil
	}
	g.released = true
	if g.mgr != nil {
		g.mgr.forget(g.path)
	}
	_ = unlockFile(g.file)
	return g.file.Close()
}

// LockManager hands out per-path advisory locks backed by flock(2). Lockfiles
// live under <locksDir> with a path-derived name so a lockfile can never
// collide with a real target file.
type LockManager struct {
	locksDir string
	mu       sync.Mutex
	held     map[string]struct{}
}

// NewLockManager creates a LockManager whose lockfiles live under locksDir.
// The directory is created lazily on first Acquire.
func NewLockManager(locksDir string) *LockManager {
	return &LockManager{
		locksDir: locksDir,
		held:     make(map[string]struct{}),
	}
}

// lockNameFor derives a stable, collision-free lockfile name for a target path.
func lockNameFor(target string) string {
	abs, err := filepath.Abs(target)
	if err != nil {
		abs = target
	}
	return sha256Hex([]byte(abs))[len("sha256:"):] + ".lock"
}

// Acquire takes a non-blocking exclusive advisory lock for path. If the lock is
// already held — by this process or another — it returns ErrLockHeld so the
// caller can map it to exit code 5.
func (lm *LockManager) Acquire(path string) (*Guard, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if _, ok := lm.held[path]; ok {
		return nil, ErrLockHeld
	}

	if err := os.MkdirAll(lm.locksDir, 0o755); err != nil {
		return nil, fmt.Errorf("doctor: create locks dir: %w", err)
	}
	lockPath := filepath.Join(lm.locksDir, lockNameFor(path))
	f, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("doctor: open lockfile: %w", err)
	}
	if err := tryLockExclusive(f); err != nil {
		_ = f.Close()
		return nil, ErrLockHeld
	}
	lm.held[path] = struct{}{}
	return &Guard{file: f, path: path, mgr: lm}, nil
}

// forget removes a path from the in-process held set; called by Guard.Release.
func (lm *LockManager) forget(path string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	delete(lm.held, path)
}
