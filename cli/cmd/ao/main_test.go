// practices: [pragmatic-programmer, twelve-factor-app]
package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
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

	tmpHome, err := os.MkdirTemp("", "cmd-ao-test-home-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create test HOME: %v\n", err)
		os.Exit(1)
	}
	origHome, hadOrigHome := os.LookupEnv("HOME")
	os.Setenv("HOME", tmpHome)

	code := m.Run()

	os.RemoveAll(tmpHome)
	if hadOrigHome {
		os.Setenv("HOME", origHome)
	} else {
		os.Unsetenv("HOME")
	}
	os.Exit(code)
}
