// practices: [pragmatic-programmer, twelve-factor-app]
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/testsupport"
)

// TestMain clears AGENTOPS_RPI_RUNTIME* env vars AND forces HOME to a
// tempdir so no test in this package can silently poison the real
// ~/.agents/ global hub or the real ~/.claude/projects/ transcripts.
//
// Any test that depends on a real $HOME path (e.g., reading real Claude
// Code session transcripts from ~/.claude/projects/) must explicitly
// t.Setenv("HOME", "<specific-path>") to override this package-wide
// isolation. Verified on 2026-04-10: all 6 tests that reference
// .claude/projects in this package are either comments, string literals,
// or tempdir-based and are compatible with HOME=tempdir without override.
func TestMain(m *testing.M) {
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "AGENTOPS_RPI_RUNTIME") {
			key, _, _ := strings.Cut(env, "=")
			os.Unsetenv(key)
		}
	}

	// Scrub git's hook-injected discovery env (GIT_DIR, GIT_WORK_TREE, ...).
	// When this suite is launched from a git-hook context, those vars redirect
	// every fixture `git init`/`git config` to the REAL repo — the ek8v
	// core.bare corruption (recurred 2026-07-18 after the hook-side scrub was
	// retired with the pre-push gate).
	testsupport.ScrubGitDiscoveryEnv()

	tmpHome, err := os.MkdirTemp("", "cmd-ao-test-home-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create test HOME: %v\n", err)
		os.Exit(1)
	}
	origHome, hadOrigHome := os.LookupEnv("HOME")
	// The hermetic binary build (aoBinary) must keep using the real module and
	// build caches: with HOME pointed at the temp dir, Go would resolve an
	// empty GOMODCACHE and re-download modules on every test run (or fail
	// offline). Capture the pre-isolation cache paths for the build env.
	hermeticBuildHome = origHome
	os.Setenv("HOME", tmpHome)

	// Isolate the tmux socket. Several production paths (context-budget
	// session-ensure in internal/context, session spawn in
	// internal/adapters/sessionspawn) shell out to the real `tmux` binary on
	// tmux's DEFAULT socket (/tmp/tmux-$UID/default). Without isolation, a test
	// that reaches one of those paths starts (or attaches to) a tmux SERVER on
	// the developer's real socket — and because a tmux server bakes the process
	// environment ONCE at startup, it captures HOME=<tmpHome above> and then
	// hands that poisoned HOME to every shell the developer opens afterward
	// (bare prompt, $HOME pointing at a since-deleted temp dir). TMUX_TMPDIR
	// relocates the socket dir into a throwaway location so test tmux can never
	// collide with the real server. Diagnosed 2026-06-25 (real session poisoned).
	tmpTmux, err := os.MkdirTemp("", "cmd-ao-test-tmux-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create test TMUX_TMPDIR: %v\n", err)
		os.RemoveAll(tmpHome)
		os.Exit(1)
	}
	origTmuxDir, hadOrigTmuxDir := os.LookupEnv("TMUX_TMPDIR")
	os.Setenv("TMUX_TMPDIR", tmpTmux)
	origTmux, hadOrigTmux := os.LookupEnv("TMUX")
	// TMUX identifies the parent client socket and takes precedence over
	// TMUX_TMPDIR. Clear it so test subprocesses and cleanup can reach only the
	// isolated socket created above, never the NTM or interactive parent server.
	os.Unsetenv("TMUX")

	code := m.Run()

	cleanupHermeticBinary()

	// Tear down any tmux server the tests started on the isolated socket before
	// removing its dir, so no orphan server lingers. Inherits the TMUX_TMPDIR
	// set above, so this targets only the test socket, never the real one.
	_ = exec.Command("tmux", "kill-server").Run()
	os.RemoveAll(tmpTmux)
	if hadOrigTmuxDir {
		os.Setenv("TMUX_TMPDIR", origTmuxDir)
	} else {
		os.Unsetenv("TMUX_TMPDIR")
	}
	if hadOrigTmux {
		os.Setenv("TMUX", origTmux)
	} else {
		os.Unsetenv("TMUX")
	}

	os.RemoveAll(tmpHome)
	if hadOrigHome {
		os.Setenv("HOME", origHome)
	} else {
		os.Unsetenv("HOME")
	}
	os.Exit(code)
}
