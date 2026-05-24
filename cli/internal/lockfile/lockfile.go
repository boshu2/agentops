// Package lockfile provides generic PID-stamped advisory lock-file primitives:
// staleness detection, PID read/write, and process-liveness checks. These were
// extracted from the retired overnight engine (soc-2rtm0) so KEEP features such
// as `ao harvest` can reclaim stale single-writer locks without depending on
// the overnight package.
package lockfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// IsStale reports whether the lock file at lockPath can be safely reclaimed.
//
// Returns true when ALL of:
//   - the lock file exists,
//   - its mtime is older than maxAge,
//   - the PID inside is zero OR references a process that is no longer alive.
//
// Returns false with nil error when:
//   - the lock file does not exist (no lock to reclaim),
//   - the lock is fresh (mtime within maxAge),
//   - the lock references a live PID.
//
// Returns an error only when the lock file exists but os.Stat fails for a
// reason other than ENOENT.
func IsStale(lockPath string, maxAge time.Duration) (bool, error) {
	info, err := os.Stat(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("lockfile: stat lock %s: %w", lockPath, err)
	}
	if time.Since(info.ModTime()) < maxAge {
		return false, nil
	}
	pid := ReadPID(lockPath)
	if pid > 0 && ProcessAlive(pid) {
		return false, nil
	}
	return true, nil
}

// ReadPID parses the lock file at lockPath and returns the PID inside, or 0 if
// the file is missing, unreadable, or does not begin with a parseable decimal
// PID. The expected format is a single line containing the decimal PID written
// by WritePID at lock acquisition; trailing whitespace and additional lines are
// tolerated.
func ReadPID(lockPath string) int {
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return 0
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return 0
	}
	// Take the first line / first whitespace-delimited token.
	if idx := strings.IndexAny(text, " \t\r\n"); idx >= 0 {
		text = text[:idx]
	}
	pid, err := strconv.Atoi(text)
	if err != nil || pid <= 0 {
		return 0
	}
	return pid
}

// WritePID writes the current process PID into the lock file at lockPath. It
// uses O_WRONLY|O_CREATE|O_TRUNC so a fresh acquisition replaces any stale
// content, and creates the parent directory if needed.
func WritePID(lockPath string) error {
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return fmt.Errorf("lockfile: mkdir lock parent: %w", err)
	}
	f, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("lockfile: open lock for write: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := fmt.Fprintf(f, "%d\n", os.Getpid()); err != nil {
		return fmt.Errorf("lockfile: write pid to lock: %w", err)
	}
	return nil
}

// ProcessAlive reports whether a process with the given PID is currently
// running. It uses os.FindProcess + signal(0) — the standard POSIX liveness
// check — on Unix. On Windows os.FindProcess returns an error when the process
// is definitively gone; we treat any lookup success as "alive" because
// signal(0) is not portable there.
//
// A non-positive PID is never alive.
func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	// signal(0) returns nil if the process exists and the caller has
	// permission to signal it. ESRCH (or os.ErrProcessDone on Darwin /
	// the Go stdlib wrapper) means it is gone. EPERM means it exists but
	// we lack permission — still alive from our POV.
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	if errors.Is(err, syscall.ESRCH) {
		return false
	}
	if errors.Is(err, os.ErrProcessDone) {
		return false
	}
	if errors.Is(err, syscall.EPERM) {
		return true
	}
	// Message-based fallback for Go versions that surface "process already
	// finished" without an underlying errno that Is() can match.
	if strings.Contains(err.Error(), "already finished") {
		return false
	}
	// Best-effort catch-all: treat unknown errors as "not alive" so we never
	// falsely hold a stale lock live.
	return false
}
