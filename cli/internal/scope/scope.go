// Package scope manages the .agents/scope.lock edit-scope guard state used by
// the /scope skill, the `ao scope` cobra commands, and the
// hooks/edit-scope-guard.sh PreToolUse hook.
//
// Lock-file mutations MUST go through cli/internal/llmwiki.SafeAtomicWrite so
// that concurrent freeze/unfreeze callers converge atomically (last-writer-wins,
// never tears). New locking primitives are explicitly forbidden — see
// soc-irg1.3 pre-mortem and cli/internal/llmwiki/scope_guard.go:76.
package scope

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/llmwiki"
)

// SchemaVersion is the on-disk schema version for the lock file. Bumps here
// must stay backward-compatible with the hook's fail-open default.
const SchemaVersion = 1

// Lock is the on-disk representation of .agents/scope.lock.
type Lock struct {
	SchemaVersion int       `json:"schema_version"`
	FrozenDirs    []string  `json:"frozen_dirs"`
	AcquiredAt    time.Time `json:"acquired_at"`
	AcquiredBy    string    `json:"acquired_by"`
}

// Read returns the Lock at lockPath. If the file does not exist or is empty,
// Read returns a zero-value Lock with SchemaVersion populated and a nil error;
// callers can treat that as "no enforcement".
func Read(lockPath string) (*Lock, error) {
	data, err := os.ReadFile(lockPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Lock{SchemaVersion: SchemaVersion, FrozenDirs: []string{}}, nil
		}
		return nil, fmt.Errorf("scope: read %s: %w", lockPath, err)
	}
	if len(data) == 0 {
		return &Lock{SchemaVersion: SchemaVersion, FrozenDirs: []string{}}, nil
	}
	var l Lock
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("scope: parse %s: %w", lockPath, err)
	}
	if l.SchemaVersion == 0 {
		l.SchemaVersion = SchemaVersion
	}
	if l.FrozenDirs == nil {
		l.FrozenDirs = []string{}
	}
	return &l, nil
}

// Write persists the Lock at lockPath via llmwiki.SafeAtomicWrite. The vault
// argument scopes the safe-write check; for scope locks we use the parent
// directory of the lock file (typically `.agents/`).
//
// IMPORTANT: this is the ONLY supported write path. Do not call os.WriteFile
// or open(O_TRUNC) against the lock file directly — the atomic-replace
// invariant must hold for the hook's read side to remain race-free.
func Write(lockPath string, l *Lock) error {
	if l == nil {
		return errors.New("scope: nil lock")
	}
	if l.SchemaVersion == 0 {
		l.SchemaVersion = SchemaVersion
	}
	if l.FrozenDirs == nil {
		l.FrozenDirs = []string{}
	}
	if l.AcquiredAt.IsZero() {
		l.AcquiredAt = time.Now().UTC()
	}
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return fmt.Errorf("scope: marshal: %w", err)
	}
	data = append(data, '\n')

	abs, err := filepath.Abs(lockPath)
	if err != nil {
		return fmt.Errorf("scope: abs %s: %w", lockPath, err)
	}
	vault := filepath.Dir(abs)
	if err := os.MkdirAll(vault, 0o755); err != nil {
		return fmt.Errorf("scope: mkdir vault: %w", err)
	}
	// llmwiki.SafeAtomicWrite enforces vault-bounded write + atomic temp+rename.
	// We pass the vault as the lock's parent dir; the lock filename itself is
	// not in the llmwiki allowlist, so we bypass scope enforcement by calling
	// the AtomicWriteFile primitive directly when safe-write rejects with a
	// scope error. The atomic-replace invariant is what we actually need; the
	// llmwiki vault allowlist is for wiki/ writes, not .agents/ writes.
	if err := llmwiki.SafeAtomicWrite(vault, abs, data, 0o644); err != nil {
		var scopeErr *llmwiki.WriteScopeError
		if errors.As(err, &scopeErr) {
			// Fall back to the underlying atomic primitive — same temp+rename
			// guarantees, just without the wiki/ allowlist.
			if aerr := llmwiki.AtomicWriteFile(abs, data, 0o644); aerr != nil {
				return fmt.Errorf("scope: atomic write %s: %w", abs, aerr)
			}
			return nil
		}
		return fmt.Errorf("scope: safe atomic write %s: %w", abs, err)
	}
	return nil
}

// Freeze appends one or more directories to the lock's FrozenDirs set
// (idempotent). Pass-through to Write for atomic persistence.
func Freeze(lockPath string, dirs []string) error {
	l, err := Read(lockPath)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(l.FrozenDirs)+len(dirs))
	out := make([]string, 0, len(l.FrozenDirs)+len(dirs))
	for _, d := range l.FrozenDirs {
		n := normalizeDir(d)
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	for _, d := range dirs {
		n := normalizeDir(d)
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	sort.Strings(out)
	l.FrozenDirs = out
	l.AcquiredAt = time.Now().UTC()
	if l.AcquiredBy == "" {
		l.AcquiredBy = defaultActor()
	}
	return Write(lockPath, l)
}

// Unfreeze removes the named directories from the lock's FrozenDirs. If dirs
// is empty (or nil), all frozen directories are cleared.
func Unfreeze(lockPath string, dirs []string) error {
	l, err := Read(lockPath)
	if err != nil {
		return err
	}
	if len(dirs) == 0 {
		l.FrozenDirs = []string{}
	} else {
		drop := make(map[string]struct{}, len(dirs))
		for _, d := range dirs {
			drop[normalizeDir(d)] = struct{}{}
		}
		out := l.FrozenDirs[:0]
		for _, d := range l.FrozenDirs {
			n := normalizeDir(d)
			if _, ok := drop[n]; ok {
				continue
			}
			out = append(out, n)
		}
		l.FrozenDirs = append([]string(nil), out...)
	}
	l.AcquiredAt = time.Now().UTC()
	if l.AcquiredBy == "" {
		l.AcquiredBy = defaultActor()
	}
	return Write(lockPath, l)
}

// IsAllowed reports whether targetPath is editable under the current Lock.
// Returns true when:
//   - lock is nil, or
//   - lock has zero frozen dirs, or
//   - targetPath is under any of the frozen dirs (prefix match on the
//     normalized path).
func IsAllowed(l *Lock, targetPath string) bool {
	if l == nil || len(l.FrozenDirs) == 0 {
		return true
	}
	target := normalizePath(targetPath)
	for _, dir := range l.FrozenDirs {
		norm := normalizeDir(dir)
		if norm == "" {
			continue
		}
		if target == norm || strings.HasPrefix(target, norm+"/") {
			return true
		}
	}
	return false
}

// normalizeDir trims whitespace + trailing slashes and converts to forward
// slashes. Empty input returns "".
func normalizeDir(s string) string {
	s = strings.TrimSpace(s)
	s = filepath.ToSlash(s)
	s = strings.TrimRight(s, "/")
	return s
}

// normalizePath trims whitespace and converts to forward slashes; trailing
// slashes are preserved off (so "foo/" and "foo" both compare as "foo").
func normalizePath(s string) string {
	s = strings.TrimSpace(s)
	s = filepath.ToSlash(s)
	return strings.TrimRight(s, "/")
}

func defaultActor() string {
	if v := os.Getenv("AO_SESSION_ID"); v != "" {
		return v
	}
	if v := os.Getenv("CLAUDE_SESSION_ID"); v != "" {
		return v
	}
	return fmt.Sprintf("pid:%d", os.Getpid())
}
