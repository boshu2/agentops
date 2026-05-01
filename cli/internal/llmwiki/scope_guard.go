package llmwiki

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// allowedWritePaths lists the prefixes (relative to vault root) that llmwiki
// stages are allowed to write to. Hardcoded per pre-mortem amendment C5.
//
// Roadmap: future expansion to wiki/threads/, wiki/clips/, wiki/reviewed/,
// wiki/knowledge/ when those subdirs are formally adopted by llmwiki.loop.
// Operating on .agents/ is FORBIDDEN — that tree is owned by internal skills
// (compile, knowledge-activation, etc.). The internal/external split is
// architecture.md load-bearing (research §2.4).
var allowedWritePaths = []string{
	"wiki/sources/",
	"wiki/entities/",
	"wiki/concepts/",
	"wiki/synthesis/",
	"wiki/INDEX.md",
	"wiki/LOG.md",
}

// WriteScopeError indicates a stage tried to write outside its allowlist.
type WriteScopeError struct {
	Vault string
	Path  string
}

func (e *WriteScopeError) Error() string {
	return fmt.Sprintf("write outside llmwiki scope: vault=%s path=%s (allowed: %s)",
		e.Vault, e.Path, strings.Join(allowedWritePaths, ", "))
}

// CheckWriteScope returns nil if path (relative to vault) is in the allowlist,
// or a *WriteScopeError otherwise. ALL stage writes MUST go through this check.
//
// The check rejects:
//   - paths outside the vault entirely (filepath.Rel returns "..").
//   - paths that resolve into the vault but outside the allowlist.
//   - relative paths that escape the vault via "..".
func CheckWriteScope(vault, path string) error {
	if vault == "" || path == "" {
		return &WriteScopeError{Vault: vault, Path: path}
	}
	rel, err := filepath.Rel(vault, path)
	if err != nil {
		return &WriteScopeError{Vault: vault, Path: path}
	}
	rel = filepath.ToSlash(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return &WriteScopeError{Vault: vault, Path: path}
	}
	for _, prefix := range allowedWritePaths {
		// Exact-file allowlist entries (no trailing slash) match by equality.
		if !strings.HasSuffix(prefix, "/") {
			if rel == prefix {
				return nil
			}
			continue
		}
		// Directory prefixes match the directory itself or anything under it.
		trimmed := strings.TrimSuffix(prefix, "/")
		if rel == trimmed || strings.HasPrefix(rel, prefix) {
			return nil
		}
	}
	return &WriteScopeError{Vault: vault, Path: path}
}

// SafeAtomicWrite is the wrapper to use from stages: enforces scope, then
// atomic-writes. Stages MUST go through this helper for every wiki write.
func SafeAtomicWrite(vault, absPath string, contents []byte, mode os.FileMode) error {
	if err := CheckWriteScope(vault, absPath); err != nil {
		return err
	}
	return AtomicWriteFile(absPath, contents, mode)
}
