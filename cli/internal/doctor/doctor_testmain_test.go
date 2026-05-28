package doctor

import (
	"os"
	"testing"
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
