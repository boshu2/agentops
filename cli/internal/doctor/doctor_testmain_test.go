package doctor

import (
	"os"
	"testing"

	"github.com/boshu2/agentops/cli/internal/testsupport"
)

// TestMain isolates HOME for every test in the cli/internal/doctor package.
//
// Doctor tests build temp repo + HOME directories and pass them explicitly via
// DetectEnv.HomeDir, so they do not depend on the real $HOME. But the fix_*
// detectors and fixers resolve $HOME-rooted paths (~/.claude, ~/.codex,
// ~/.agents), and any code path that falls back to os.UserHomeDir() would
// otherwise touch the operator's real home tree. Setting HOME to a throwaway
// directory before m.Run() is the cheapest defense-in-depth and satisfies the
// check-test-home-isolation gate for the whole package (soc-z3qo.4).
func TestMain(m *testing.M) {
	// Also scrub git's hook-injected discovery env: doctor's engine shells out
	// to git, and a leaked GIT_DIR/GIT_WORK_TREE would redirect those ops to
	// the real repo (age-cmdao-core-bare-pollution-ek8v class).
	testsupport.ScrubGitDiscoveryEnv()

	tmp, err := os.MkdirTemp("", "doctor-testmain-home-*")
	if err != nil {
		panic("doctor TestMain: failed to create tmpdir: " + err.Error())
	}

	oldHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmp)

	code := m.Run()

	_ = os.Setenv("HOME", oldHome)
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}
