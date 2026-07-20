// Package runtimecmd parses and builds the CLI command lines used to invoke a
// coding-agent runtime (e.g. `codex exec <prompt>`). It was extracted from the
// former rpi engine (age-tlj6 teardown): the helpers are a reusable utility with
// no orchestration-engine coupling, consumed by the eval runtime. Pure string
// functions — no I/O, no side effects.
//
// LAW 0 (age-6j9ee.4): a headless `claude -p` / `claude --print` invocation bills
// the Anthropic API / burns the Claude Max quota and is prohibited environment-wide.
// DirectArgs therefore fail-closes: it never emits `-p`/`--print`, and it refuses
// outright to build an argv for the `claude` binary. Route headless work to codex
// or an interactive pane instead.
package runtimecmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// ErrClaudeHeadlessProhibited is returned by DirectArgs when asked to build a
// direct (headless) invocation of the `claude` binary. It is the LAW 0 fail-closed
// error: a headless claude call is never permitted, directly or indirectly.
var ErrClaudeHeadlessProhibited = errors.New(
	"claude headless invocation is prohibited (LAW 0): route this work to codex or an interactive pane",
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

// containsClaudeToken reports whether any whitespace-separated token of command
// resolves (by base name, case-insensitively, with an optional .exe suffix) to
// the `claude` binary. This catches env-wrapped forms like `env -i claude` in
// addition to a bare `claude`, so no framing can smuggle the prohibited binary
// past the LAW 0 gate.
func containsClaudeToken(command string) bool {
	for _, field := range strings.Fields(strings.TrimSpace(command)) {
		base := strings.ToLower(filepath.Base(field))
		base = strings.TrimSuffix(base, ".exe")
		if base == "claude" {
			return true
		}
	}
	return false
}

// DirectArgs builds the argument list for direct (non-stream) runtime execution.
//
// Codex is the only supported direct runtime: it returns `<prefix args...> exec
// <prompt>`. Any command that resolves to the `claude` binary is refused with
// ErrClaudeHeadlessProhibited (LAW 0 — a headless `claude -p` bills the API /
// burns quota). Any other, unrecognized runtime is likewise refused rather than
// falling back to a `-p <prompt>` form, so this function can never emit the
// prohibited flags.
func DirectArgs(command, prompt string) ([]string, error) {
	if containsClaudeToken(command) {
		return nil, ErrClaudeHeadlessProhibited
	}
	_, prefixArgs := Split(command)
	if BinaryName(command) == "codex" {
		args := append([]string{}, prefixArgs...)
		return append(args, "exec", prompt), nil
	}
	return nil, fmt.Errorf("runtimecmd: no direct-invocation form for command %q (only codex is supported for headless execution)", command)
}
