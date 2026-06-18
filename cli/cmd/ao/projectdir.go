// practices: [pragmatic-programmer, twelve-factor-app]
package main

import (
	"fmt"
	"os"
)

// testProjectDir allows tests to override the working directory used by
// command handlers without calling os.Chdir. When non-empty, resolveProjectDir
// returns this value instead of os.Getwd(). Production code never sets this
// variable; only test code should.
//
// NOTE: Because this is a package-level variable, tests that set it must NOT
// use t.Parallel() unless all concurrent tests agree on the same value or use
// their own synchronization.
var testProjectDir string

// resolveProjectDir returns the effective project directory. If testProjectDir
// is set (by tests), it is returned directly. Otherwise os.Getwd() is called.
func resolveProjectDir() (string, error) {
	if testProjectDir != "" {
		return testProjectDir, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	return cwd, nil
}

// repoRootOrCwd returns the git repository top-level — correct from ANY
// subdirectory (e.g. cli/) and inside linked worktrees — for commands that read
// repo-rooted artifacts like docs/contracts/claim-registry.yaml. It resolves the
// effective project dir first (honoring the testProjectDir override), then asks
// git for that dir's top-level, and falls back to the dir itself when it is not
// inside a repo or git is unavailable.
//
// Prefer this over resolveProjectDir for any handler that joins a repo-relative
// path: resolveProjectDir returns the raw cwd, which breaks when the command is
// run from a subdirectory (age-6sg.1).
func repoRootOrCwd() (string, error) {
	cwd, err := resolveProjectDir()
	if err != nil {
		return "", err
	}
	if root, err := resolveRepoRoot(cwd); err == nil && root != "" {
		return root, nil
	}
	return cwd, nil
}
