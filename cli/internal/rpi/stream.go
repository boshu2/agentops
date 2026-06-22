package rpi

import (
	"path/filepath"
	"strings"
)

// Runtime-command helpers — the live remainder of the former stream/phased engine
// (age-tlj6 teardown). The stream-json / phase-orchestration helpers were deleted
// with the engine; only the runtime-command parsing used by the eval runtime
// (cli/internal/eval) remains.

// RuntimeBinaryName extracts the lowercase base binary name from a runtime command string.
func RuntimeBinaryName(command string) string {
	executable, _ := SplitRuntimeCommand(command)
	if executable == "" {
		return ""
	}
	base := strings.ToLower(filepath.Base(executable))
	return strings.TrimSuffix(base, ".exe")
}

// SplitRuntimeCommand splits a runtime command into executable and prefix args.
func SplitRuntimeCommand(command string) (string, []string) {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], fields[1:]
}

// RuntimeDirectCommandArgs builds the argument list for direct runtime execution.
func RuntimeDirectCommandArgs(command, prompt string) []string {
	_, prefixArgs := SplitRuntimeCommand(command)
	args := append([]string{}, prefixArgs...)
	if RuntimeBinaryName(command) == "codex" {
		return append(args, "exec", prompt)
	}
	return append(args, "-p", prompt)
}
