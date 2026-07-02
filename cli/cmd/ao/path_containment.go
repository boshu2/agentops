// practices: [design-by-contract, code-complete]
package main

import (
	"os"
	"path/filepath"
	"strings"
)

// realpathOrSelf returns p as an absolute, symlink-resolved path. When p (or a leaf of it)
// doesn't exist, it resolves symlinks on the LONGEST existing prefix and keeps the rest, so
// a non-existent entry still compares consistently against an existing root (e.g. macOS
// /var -> /private/var) — required for correct containment checks.
func realpathOrSelf(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = filepath.Clean(p)
	}
	cur, remaining := abs, ""
	for {
		if resolved, err := filepath.EvalSymlinks(cur); err == nil {
			if remaining == "" {
				return resolved
			}
			return filepath.Join(resolved, remaining)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return abs
		}
		remaining = filepath.Join(filepath.Base(cur), remaining)
		cur = parent
	}
}

// pathInside reports whether child is at or below root (both should be realpath'd).
func pathInside(child, root string) bool {
	rel, err := filepath.Rel(root, child)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel))
}

// pathInsideRepo reports whether path is at or below repo, resolving symlinks on
// both sides so a symlinked binary or checkout compares correctly. It is the ONE
// containment predicate the "no repo-internal-absolute binary is ever trusted"
// invariant rests on: trustedLookPath applies it to exclude a repo-planted git
// from PATH resolution (runtime), and `ao verify init` applies it to refuse
// baking a repo-internal ao path into the installed hook (install time).
func pathInsideRepo(path, repo string) bool {
	return pathInside(realpathOrSelf(path), realpathOrSelf(repo))
}
