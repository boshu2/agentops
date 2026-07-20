package main

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/commands/quickstart"
)

// aoInvocation matches "ao <token> <token> ..." runs in user-facing text. The
// capture is the token run after "ao"; resolution logic decides how many of
// those tokens must exist in the live command tree. "ao" must not be part of a
// path or identifier (e.g. /etc/bash_completion.d/ao) and the token run never
// crosses a line break.
var aoInvocation = regexp.MustCompile(`(?m)(?:^|[^-/\w.])ao[ \t]+([a-z][a-z0-9-]*(?:[ \t][a-z0-9][a-z0-9-]*)*)`)

// proseWords are lowercase words that follow "ao" in help prose without being
// command invocations (e.g. "the ao binary", "ao provides ..."). A word added
// here is a conscious decision that the phrase is prose, not an advertised
// command.
var proseWords = map[string]bool{
	"binary":   true,
	"cli":      true,
	"command":  true,
	"commands": true,
	"provides": true,
	"tool":     true,
	"without":  true,
}

// resolveFirst reports whether token names a direct subcommand of root
// (including aliases and the cobra-generated help/completion commands).
func resolveFirst(root *cobra.Command, token string) (*cobra.Command, bool) {
	if token == "help" || token == "completion" {
		return nil, true
	}
	for _, c := range root.Commands() {
		if c.Name() == token || c.HasAlias(token) {
			return c, true
		}
	}
	return nil, false
}

// checkInvocation validates one extracted token run against the live tree.
// The first token must always resolve. In command-line contexts (Example
// fields, printed output), descent continues: while the current node is a
// group (no Run function), the next plain-word token must resolve as one of
// its subcommands; a leaf's trailing tokens are arguments and are ignored.
func checkInvocation(t *testing.T, source, field, match string, commandContext bool) {
	t.Helper()
	tokens := strings.Fields(match)
	if len(tokens) == 0 {
		return
	}
	if proseWords[tokens[0]] {
		return
	}
	node, ok := resolveFirst(rootCmd, tokens[0])
	if !ok {
		t.Errorf("%s %s advertises %q but %q is not a registered ao command", source, field, "ao "+match, tokens[0])
		return
	}
	if !commandContext || node == nil {
		return
	}
	for _, token := range tokens[1:] {
		if node.Run == nil && node.RunE == nil && len(node.Commands()) > 0 {
			next, ok := resolveFirst(node, token)
			if !ok {
				t.Errorf("%s %s advertises %q but %q is not a subcommand of %q", source, field, "ao "+match, token, node.Name())
				return
			}
			if next == nil {
				return
			}
			node = next
			continue
		}
		return // leaf (or runnable group): remaining tokens are arguments
	}
}

func extractAndCheck(t *testing.T, source, field, text string, commandContext bool) {
	t.Helper()
	for _, m := range aoInvocation.FindAllStringSubmatch(text, -1) {
		checkInvocation(t, source, field, m[1], commandContext)
	}
}

// TestAdvertisedCommandsResolveInHelpTree walks every registered command and
// verifies that each "ao ..." string advertised in its Short, Long, or Example
// help text resolves against the live command tree. This is the guard for the
// defect class where help output advertises a command that was retired.
func TestAdvertisedCommandsResolveInHelpTree(t *testing.T) {
	var walk func(prefix string, cmd *cobra.Command)
	walk = func(prefix string, cmd *cobra.Command) {
		source := strings.TrimSpace(prefix + " " + cmd.Name())
		extractAndCheck(t, source, "Short", cmd.Short, false)
		extractAndCheck(t, source, "Long", cmd.Long, false)
		extractAndCheck(t, source, "Example", cmd.Example, true)
		for _, sub := range cmd.Commands() {
			walk(source, sub)
		}
	}
	walk("", rootCmd)
}

// TestAdvertisedCommandsResolveInQuickstartOutput runs the quick-start command
// (a pure print command) and verifies every "ao ..." string in its printed
// output resolves against the live command tree.
func TestAdvertisedCommandsResolveInQuickstartOutput(t *testing.T) {
	cmd := quickstart.NewModule().Command()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("quick-start execution failed: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("quick-start printed no output")
	}
	extractAndCheck(t, "quick-start", "output", out.String(), true)
}
