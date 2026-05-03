package overnight

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain isolates HOME for every test in the cli/internal/overnight
// package. This is a belt-and-suspenders backup to the per-test
// t.Setenv("HOME", ...) guards added to the L2 e2e tests and the
// Wave 3 stages_test.go fixtures.
//
// Without this, any test that calls harvest.Promote (directly or
// transitively via RunIngest) would write to $HOME/.agents/learnings/
// by default — the exact bug caught in Phase 3 validation on
// 2026-04-09 when 150 synthetic fixtures leaked into the operator's
// real global hub.
//
// Note: even after Issue 1 adds WalkOptions.SkipGlobalHub and the
// overnight package opts in, a third-party test that forgets the
// flag would still leak. TestMain is the cheapest defense-in-depth.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "overnight-testmain-home-*")
	if err != nil {
		panic("overnight TestMain: failed to create tmpdir: " + err.Error())
	}
	// Pre-create the .agents/learnings tree so any harvest.Promote
	// call has a target to write to (non-nil path avoids confusing
	// mkdir errors inside tests).
	_ = os.MkdirAll(filepath.Join(tmp, ".agents", "learnings"), 0o755)

	oldHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmp)
	restorePathEnv := clearAgentPathEnv()

	code := m.Run()

	restorePathEnv()
	_ = os.Setenv("HOME", oldHome)
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}

func clearAgentPathEnv() func() {
	keys := []string{
		"AO_HOME", "CLAUDE_PLUGIN_DATA",
		"AO_AGENTS_DIR", "AO_KNOWLEDGE_ROOT", "AO_HOOKS_DIR", "AO_SCOPE_LOCK",
		"AO_RPI_DIR", "AO_FINDINGS_DIR", "AO_PLANS_DIR", "AO_COUNCIL_DIR",
		"AO_LEARNINGS_DIR", "AO_PATTERNS_DIR", "AO_DECISIONS_DIR",
	}
	prev := make(map[string]string, len(keys))
	prevSet := make(map[string]bool, len(keys))
	for _, key := range keys {
		value, ok := os.LookupEnv(key)
		prev[key] = value
		prevSet[key] = ok
		_ = os.Unsetenv(key)
	}
	return func() {
		for _, key := range keys {
			if prevSet[key] {
				_ = os.Setenv(key, prev[key])
			} else {
				_ = os.Unsetenv(key)
			}
		}
	}
}
