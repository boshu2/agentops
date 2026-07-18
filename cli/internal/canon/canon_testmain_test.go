package canon

import (
	"os"
	"testing"

	"github.com/boshu2/agentops/cli/internal/testsupport"
)

// TestMain scrubs git's hook-injected repository-discovery env
// (GIT_DIR, GIT_WORK_TREE, ...) before any test in this package runs.
// Tests here shell out to git (directly or through production code); with
// those vars leaked in — e.g. when the suite is launched from a git-hook
// context — every fixture git operation is redirected to the REAL repo,
// which is how core.bare=true corrupted the shared .git/config
// (age-cmdao-core-bare-pollution-ek8v, recurred 2026-07-18).
func TestMain(m *testing.M) {
	testsupport.ScrubGitDiscoveryEnv()
	os.Exit(m.Run())
}
