package overnight

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// RecoverFromCrash scans .agents/overnight/ for stale commit markers from a
// crashed previous Dream run and restores a clean state before the current
// process acquires the overnight lock.
//
// This is called from RunLoop's startup path BEFORE acquireOvernightLock.
//
// Recovery decision tree:
//
//	no marker file    -> nothing to do; return nil
//	marker DONE state -> crash after successful commit; clean up marker and
//	                     the accompanying staging/prev directories if they
//	                     still exist
//	marker READY state -> crash between the two os.Rename calls of the
//	                      two-phase commit. Reverse the partial swap: for
//	                      each subpath that exists in prev.<iter>/ but does
//	                      NOT exist (or is empty) in .agents/, os.Rename it
//	                      back. After reversal, delete the staging dir, prev
//	                      dir, and marker.
//	marker malformed   -> log a degraded note via the actions list; do NOT
//	                      touch .agents/; return an error that tells the
//	                      operator to investigate manually.
//
// Multiple markers (from multiple historical crashes) are processed in
// lexicographic order by filename, which approximates chronological order.
// Each marker is processed independently; one bad marker does not block the
// others, but any bad marker causes the aggregated return error to be
// non-nil so callers can surface the degraded state.
//
// Returns the ordered list of human-readable recovery actions taken (for the
// morning report) and a non-nil error if any marker required manual
// intervention.
func RecoverFromCrash(cwd string) ([]string, error) {
	if cwd == "" {
		return nil, errors.New("overnight: RecoverFromCrash requires a non-empty cwd")
	}

	overnightDir := filepath.Join(cwd, ".agents", "overnight")
	if _, err := os.Stat(overnightDir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("overnight: stat overnight dir: %w", err)
	}

	pattern := filepath.Join(overnightDir, "COMMIT-MARKER.*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("overnight: glob commit markers: %w", err)
	}
	// Filter out marker temp files written by checkpoint.go's rename-from-temp.
	filtered := matches[:0]
	for _, m := range matches {
		if strings.HasSuffix(m, ".tmp") {
			continue
		}
		filtered = append(filtered, m)
	}
	matches = filtered
	sort.Strings(matches)

	if len(matches) == 0 {
		return nil, nil
	}

	var actions []string
	var errs []string

	for _, markerPath := range matches {
		base := filepath.Base(markerPath)
		iterationID := strings.TrimPrefix(base, "COMMIT-MARKER.")
		if err := sanitizeIterationID(iterationID); err != nil {
			actions = append(actions, fmt.Sprintf("skipped malformed marker %s: %v", base, err))
			continue
		}

		data, readErr := os.ReadFile(markerPath)
		if readErr != nil {
			errs = append(errs, fmt.Sprintf("read %s: %v", base, readErr))
			actions = append(actions, fmt.Sprintf("skipped unreadable marker %s (manual review required)", base))
			continue
		}

		var body markerBody
		if jsonErr := json.Unmarshal(data, &body); jsonErr != nil {
			errs = append(errs, fmt.Sprintf("parse %s: %v", base, jsonErr))
			actions = append(actions, fmt.Sprintf("skipped malformed marker %s (manual review required)", base))
			continue
		}

		stagingDir := filepath.Join(overnightDir, "staging", iterationID)
		prevDir := filepath.Join(overnightDir, fmt.Sprintf("prev.%s", iterationID))
		liveDir := filepath.Join(cwd, ".agents")

		switch body.State {
		case markerStateDone:
			if rmErr := os.RemoveAll(stagingDir); rmErr != nil {
				errs = append(errs, fmt.Sprintf("remove staging for %s: %v", base, rmErr))
			}
			if rmErr := os.RemoveAll(prevDir); rmErr != nil {
				errs = append(errs, fmt.Sprintf("remove prev for %s: %v", base, rmErr))
			}
			if rmErr := os.Remove(markerPath); rmErr != nil && !os.IsNotExist(rmErr) {
				errs = append(errs, fmt.Sprintf("remove marker %s: %v", base, rmErr))
			}
			actions = append(actions, fmt.Sprintf("cleaned up stale DONE marker %s", base))

		case markerStateReady:
			reversed, revErr := reverseReadySwap(prevDir, liveDir)
			if revErr != nil {
				errs = append(errs, fmt.Sprintf("reverse READY swap for %s: %v", base, revErr))
				actions = append(actions, fmt.Sprintf("partial reversal of READY marker %s (manual review required)", base))
				continue
			}
			if rmErr := os.RemoveAll(stagingDir); rmErr != nil {
				errs = append(errs, fmt.Sprintf("remove staging for %s: %v", base, rmErr))
			}
			if rmErr := os.RemoveAll(prevDir); rmErr != nil {
				errs = append(errs, fmt.Sprintf("remove prev for %s: %v", base, rmErr))
			}
			if rmErr := os.Remove(markerPath); rmErr != nil && !os.IsNotExist(rmErr) {
				errs = append(errs, fmt.Sprintf("remove marker %s: %v", base, rmErr))
			}
			actions = append(actions, fmt.Sprintf("recovered from crash marker %s (state: READY, reversed %d subpaths)", base, reversed))

		default:
			errs = append(errs, fmt.Sprintf("unknown state %q in %s", body.State, base))
			actions = append(actions, fmt.Sprintf("skipped marker %s with unknown state %q (manual review required)", base, body.State))
		}
	}

	if len(errs) > 0 {
		return actions, fmt.Errorf("overnight: RecoverFromCrash encountered %d issue(s) requiring investigation: %s", len(errs), strings.Join(errs, "; "))
	}
	return actions, nil
}

// reverseReadySwap walks CheckpointedSubpaths and, for any subpath that
// exists under prevDir but is missing (or empty) under liveDir, moves the
// prev copy back into place. Returns the number of subpaths actually
// reversed.
//
// A subpath is treated as "missing" from live when os.Stat fails with
// ENOENT; it is treated as "empty" when it is a directory with zero
// entries. In both cases we restore from prev. When live already contains
// non-empty content we leave it alone — that indicates the second rename
// of the two-phase commit completed for this subpath before the crash.
func reverseReadySwap(prevDir, liveDir string) (int, error) {
	var reversed int
	for _, sub := range CheckpointedSubpaths {
		prevPath := filepath.Join(prevDir, sub)
		if _, err := os.Stat(prevPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return reversed, fmt.Errorf("stat prev %s: %w", sub, err)
		}

		livePath := filepath.Join(liveDir, sub)
		liveInfo, liveErr := os.Stat(livePath)
		liveMissing := false
		if liveErr != nil {
			if os.IsNotExist(liveErr) {
				liveMissing = true
			} else {
				return reversed, fmt.Errorf("stat live %s: %w", sub, liveErr)
			}
		}

		if !liveMissing {
			// If live has real content, leave it alone. Only reverse when
			// live is an empty dir (a half-finished state).
			if liveInfo.IsDir() {
				entries, err := os.ReadDir(livePath)
				if err != nil {
					return reversed, fmt.Errorf("readdir live %s: %w", sub, err)
				}
				if len(entries) > 0 {
					continue
				}
				// Remove the empty live dir so rename can replace it.
				if err := os.RemoveAll(livePath); err != nil {
					return reversed, fmt.Errorf("remove empty live %s: %w", sub, err)
				}
			} else {
				// Live is a non-dir file that already exists — don't touch.
				continue
			}
		}

		if err := os.MkdirAll(filepath.Dir(livePath), 0o755); err != nil {
			return reversed, fmt.Errorf("mkdir live parent for %s: %w", sub, err)
		}
		if err := os.Rename(prevPath, livePath); err != nil {
			return reversed, fmt.Errorf("rename prev %s into live: %w", sub, err)
		}
		reversed++
	}
	return reversed, nil
}

// LockIsStale reports whether the lock file at lockPath is safe for the
// caller to reclaim.
//
// Returns true when ALL of the following hold:
//   - the lock file exists,
//   - its mtime is older than maxAge,
//   - the PID inside is zero OR references a process that is no longer
//     alive.
//
// Returns false with nil error when:
//   - the lock file does not exist (no lock to reclaim),
//   - the lock is fresh (mtime within maxAge),
//   - the lock references a live PID.
//
// Returns an error only when the lock file exists but os.Stat fails for a
// reason other than ENOENT.
func LockIsStale(lockPath string, maxAge time.Duration) (bool, error) {
	info, err := os.Stat(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("overnight: stat lock %s: %w", lockPath, err)
	}
	if time.Since(info.ModTime()) < maxAge {
		return false, nil
	}
	pid := ReadLockPID(lockPath)
	if pid > 0 && ProcessAlive(pid) {
		return false, nil
	}
	return true, nil
}

// ReadLockPID parses the lock file at lockPath and returns the PID inside,
// or 0 if the file is missing, unreadable, or does not begin with a
// parseable decimal PID. The expected format is a single line of text
// containing the decimal PID written by WriteLockPID at lock acquisition;
// trailing whitespace and additional lines are tolerated.
func ReadLockPID(lockPath string) int {
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

// WriteLockPID writes the current process PID into the lock file at
// lockPath. It uses O_WRONLY|O_CREATE|O_TRUNC so a fresh acquisition
// replaces any stale content. This is invoked from acquireOvernightLock
// (wired in Wave 4) so every lock file carries its owning PID.
func WriteLockPID(lockPath string) error {
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return fmt.Errorf("overnight: mkdir lock parent: %w", err)
	}
	f, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("overnight: open lock for write: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := fmt.Fprintf(f, "%d\n", os.Getpid()); err != nil {
		return fmt.Errorf("overnight: write pid to lock: %w", err)
	}
	return nil
}

// ProcessAlive reports whether a process with the given PID is currently
// running. It uses os.FindProcess + signal(0) — the standard POSIX liveness
// check — on Unix. On Windows os.FindProcess returns an error when the
// process is definitively gone; we treat any lookup success as "alive"
// because signal(0) is not portable there.
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
	// Best-effort catch-all: treat unknown errors as "not alive" so we
	// never falsely hold a stale lock live.
	return false
}
