package checks

import "github.com/boshu2/agentops/cli/internal/gates"

// init registers the git-config hygiene gate
// (age-gate-scripts-worktree-gitdir-p62wo): the shared repo config must not
// carry core.bare=true (breaks every linked-worktree git op) or a known
// test/fixture identity (Test/test@test.com, factory@example.invalid,
// pokki-deploy/v@v.com — mis-authors rebased commits). Both poisonings are
// written by hook-leaked GIT_DIR redirecting "scoped" git writes into the
// shared config, so the mutation lives in .git/ where changed-file routing can
// never see it — always-run, no path globs, like workflow.install-drift. The
// backing script is a <150ms config read, so it stays in the fast tier.
func init() {
	gates.Register(gates.Check{
		ID:       "always.git-config-hygiene",
		Tiers:    gates.Fast | gates.Full,
		Blocking: true,
		Backing:  "check-git-config-hygiene.sh",
	})
}
