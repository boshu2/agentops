// Package runtimecmd parses and builds the CLI command lines used to invoke a
// coding-agent runtime (e.g. `codex exec <prompt>`, `claude -p <prompt>`). It was
// extracted from the former rpi engine (age-tlj6 teardown): the helpers are a
// reusable utility with no orchestration-engine coupling, consumed by the eval
// runtime. Pure string functions — no I/O, no side effects.
package runtimecmd

import (
	"path/filepath"
	"strings"
)

// BinaryName extracts the lowercase base binary name from a runtime command string.
func BinaryName(command string) string {
	executable, _ := Split(command)
	if executable == "" {
		return ""
	}
	base := strings.ToLower(filepath.Base(executable))
	return strings.TrimSuffix(base, ".exe")
}

// Split splits a runtime command into its executable and prefix args.
func Split(command string) (string, []string) {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], fields[1:]
}

// DirectArgs builds the argument list for direct (non-stream) runtime execution.
func DirectArgs(command, prompt string) []string {
	_, prefixArgs := Split(command)
	args := append([]string{}, prefixArgs...)
	if BinaryName(command) == "codex" {
		return append(args, "exec", prompt)
	}
	return append(args, "-p", prompt)
}
