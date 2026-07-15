// practices: [ai-assisted-dev, pragmatic-programmer]
package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// removedCommand is the tombstone for a verb the default build no longer
// serves and what replaces it. The full map with the why lives in
// docs/MIGRATION.md —
// these one-liners exist so the pointer appears at the moment of failure,
// where a dev (or an agent following error strings) actually hits the wall.
type removedCommand struct {
	use string // what to use instead — one clause, plain words
}

// removedCommands maps every verb removed from the default `ao` build to its
// tombstone. Keep in lockstep with docs/MIGRATION.md — the drift test
// (TestRemovedVerbsHaveMigrationRows) fails when a verb here has no row there.
var removedCommands = map[string]removedCommand{
	// Cathedral Cut lifecycle removals. These are executable tombstones for one
	// release: each fails without importing, forwarding to, or mutating old state.
	"pawl":       {use: "semantic review no longer controls admission; invoke the Validate skill for an independent verdict"},
	"plan-pawl":  {use: "plan admission was removed; invoke premortem when the caller wants an advisory plan challenge"},
	"land":       {use: "AgentOps no longer owns delivery; use the repository's Git or CI process"},
	"done":       {use: "AgentOps no longer owns work closure; report the result to the caller"},
	"close":      {use: "AgentOps no longer owns work closure; report the result to the caller"},
	"governor":   {use: "AgentOps no longer owns retries or budgets; the caller decides whether to invoke RPI again"},
	"yield":      {use: "AgentOps no longer controls throughput budgets; use substrate-native observation"},
	"claim":      {use: "AgentOps no longer owns work claims; use your tracker or execution substrate directly"},
	"next-work":  {use: "AgentOps no longer selects work; the caller supplies a bead or other intent source"},
	"state":      {use: "AgentOps no longer admits lifecycle state; inspect the intent source, derived subject manifest, and verdict directly"},
	"worktree":   {use: "AgentOps no longer owns Git worktrees; use Git directly"},
	"validate":   {use: "semantic validation is the Validate skill; deterministic repository checks remain under `ao gate check`"},
	"converge":   {use: "AgentOps no longer retries toward convergence; start a caller-directed revision invocation"},
	"reconcile":  {use: "AgentOps no longer reconciles lifecycle state; use read-only provenance verify or trace"},
	"membrane":   {use: "admission was removed; record observations as verdict findings or generic provenance"},
	"crank":      {use: "factory control was removed; call an executor directly or use optional `dispatch_once`"},
	"constraint": {use: "AgentOps no longer promotes findings into blocking policy; encode accepted rules in repository-owned checks"},
}

var removedChildCommands = map[string]map[string]removedCommand{
	"goals": {
		"trace": {use: "the directive-to-bead lifecycle chain was retired; inspect current goal and scenario artifacts directly"},
	},
	"session": {
		"memory": {use: "automatic repository-memory synchronization was removed; write caller-authored handoff evidence instead"},
	},
	"skills": {
		"edit": {use: "AgentOps no longer commits skill edits; edit canonical skill sources and use repository Git policy directly"},
	},
}

type removedCommandExitError struct{}

func (removedCommandExitError) Error() string { return "" }
func (removedCommandExitError) ExitCode() int { return 1 }

func newRemovedCommand(name string, tomb removedCommand) *cobra.Command {
	command := &cobra.Command{
		Use:           name,
		Short:         "Removed in the AgentOps Cathedral Cut",
		Args:          cobra.ArbitraryArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintf(cmd.ErrOrStderr(), "%s no longer exists: %s.\n", name, tomb.use)
			return removedCommandExitError{}
		},
	}
	command.FParseErrWhitelist.UnknownFlags = true
	return command
}

func newRemovedChildCommand(parent, name string, tomb removedCommand) *cobra.Command {
	command := newRemovedCommand(name, tomb)
	command.RunE = func(cmd *cobra.Command, _ []string) error {
		fmt.Fprintf(cmd.ErrOrStderr(), "%s %s no longer exists: %s.\n", parent, name, tomb.use)
		return removedCommandExitError{}
	}
	return command
}

// installRemovedCommandTombstones replaces any legacy registrations with the
// centralized inert stubs. No tombstone forwards to old implementation code.
func installRemovedCommandTombstones(root *cobra.Command, exceptions ...string) {
	excluded := map[string]struct{}{}
	for _, name := range exceptions {
		excluded[name] = struct{}{}
	}
	for name, tomb := range removedCommands {
		if _, cathedralCut := cathedralCutCommands[name]; !cathedralCut {
			continue
		}
		if _, skip := excluded[name]; skip {
			continue
		}
		for _, command := range append([]*cobra.Command(nil), root.Commands()...) {
			if command.Name() == name || command.HasAlias(name) {
				root.RemoveCommand(command)
			}
		}
		root.AddCommand(newRemovedCommand(name, tomb))
	}
}

var cathedralCutCommands = map[string]struct{}{
	"pawl": {}, "plan-pawl": {}, "land": {}, "done": {}, "close": {},
	"governor": {}, "yield": {}, "claim": {}, "next-work": {}, "state": {},
	"worktree": {}, "validate": {}, "converge": {}, "reconcile": {},
	"membrane": {}, "crank": {},
	"constraint": {},
}

// removedCommandHint returns the tombstone hint for an "unknown command"
// error whose verb was removed from the default build, or "" when the error
// is anything else. A registered verb always wins so a usage error cannot be
// mislabeled as removal.
func removedCommandHint(root *cobra.Command, err error) string {
	if err == nil {
		return ""
	}
	verb := parseUnknownCommand(err.Error())
	if verb == "" {
		return ""
	}
	tomb, ok := removedCommands[verb]
	if !ok {
		return ""
	}
	if root != nil {
		for _, c := range root.Commands() {
			if c.Name() == verb || c.HasAlias(verb) {
				return ""
			}
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%q was removed from ao — %s.\n", verb, tomb.use)
	b.WriteString("Full map of removed surfaces and replacements: docs/MIGRATION.md\n")
	return b.String()
}

// printRemovedCommandHint writes the tombstone hint (if any) to w. Cobra has
// already printed the bare "Error: unknown command ..." line; this appends
// the actionable part. Reports whether a hint was written.
func printRemovedCommandHint(w io.Writer, root *cobra.Command, err error) bool {
	hint := removedCommandHint(root, err)
	if hint == "" {
		return false
	}
	fmt.Fprint(w, "\n"+hint)
	return true
}

// parseUnknownCommand extracts the offending verb from a cobra error string
// such as `unknown command "watch" for "ao"`. Returns "" for any other error.
func parseUnknownCommand(msg string) string {
	const prefix = `unknown command "`
	idx := strings.Index(msg, prefix)
	if idx < 0 {
		return ""
	}
	rest := msg[idx+len(prefix):]
	end := strings.Index(rest, `"`)
	if end <= 0 {
		return ""
	}
	return rest[:end]
}
