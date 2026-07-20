package workflowsapp

import (
	"os"
	"testing"

	"github.com/boshu2/agentops/cli/internal/testsupport"
)

// TestMain scrubs git discovery env vars (GIT_DIR, GIT_WORK_TREE, ...) before
// any test shells out to git. Git injects these into hook-launched processes;
// with GIT_DIR pointing at a linked worktree's gitdir, a fixture `git init`
// would rewrite the SHARED .git/config (ek8v). Required by .claude/rules/go.md
// for any package whose tests shell out to git.
func TestMain(m *testing.M) {
	testsupport.ScrubGitDiscoveryEnv()
	os.Exit(m.Run())
}
