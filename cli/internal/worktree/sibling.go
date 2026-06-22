package worktree

import (
	"fmt"
	"path/filepath"
)

// ValidateWorktreeSibling checks that worktreePath is a sibling of repoRoot (shares
// the same parent directory) and is not the repo root itself — a safety guard
// before removing a worktree, so a path-confusion can never delete the canonical
// checkout or an unrelated directory. Moved from the former rpi cleanup engine
// (age-tlj6) to live with the rest of the worktree lifecycle.
func ValidateWorktreeSibling(repoRoot, worktreePath string) error {
	repoParent := filepath.Dir(filepath.Clean(repoRoot))
	wtParent := filepath.Dir(filepath.Clean(worktreePath))
	if wtParent != repoParent {
		return fmt.Errorf("worktree path %q is not a sibling of repo %q; refusing removal", worktreePath, repoRoot)
	}
	cleanWT := filepath.Clean(worktreePath)
	if cleanWT == filepath.Clean(repoRoot) {
		return fmt.Errorf("worktree path %q is the repo root; refusing removal", worktreePath)
	}
	return nil
}
