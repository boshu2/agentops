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
