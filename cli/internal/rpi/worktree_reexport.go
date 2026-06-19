package rpi

// The worktree lifecycle (create/merge/remove + git helpers + sentinels) moved
// to cli/internal/worktree (age-tlj6 slice 2) so the live path — ao session
// bootstrap — can create per-session worktrees without depending on this legacy
// rpi engine. These re-exports keep the legacy rpi callers (cmd/ao/rpi_*)
// compiling until the engine is deleted (slice 5); they go away with it.
import "github.com/boshu2/agentops/cli/internal/worktree"

var (
	CreateWorktree       = worktree.CreateWorktree
	MergeWorktree        = worktree.MergeWorktree
	RemoveWorktree       = worktree.RemoveWorktree
	GetRepoRoot          = worktree.GetRepoRoot
	GetCurrentBranch     = worktree.GetCurrentBranch
	EnsureAttachedBranch = worktree.EnsureAttachedBranch
	GenerateRunID        = worktree.GenerateRunID
)
