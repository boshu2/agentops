// practices: [ai-assisted-dev, pragmatic-programmer]
package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// removedCommand names what replaces a verb the CLI no longer serves. The
// full map with the why lives in docs/MIGRATION.md — these one-liners exist so
// the pointer appears at the moment of failure, where a dev (or an agent
// following error strings) actually hits the wall. Removed verbs are NOT
// registered as commands: an unknown-command error plus this hint is the whole
// mechanism, so no retired lifecycle API survives in the command tree.
type removedCommand struct {
	use string // what to use instead — one clause, plain words
}

// prunedFamilyHint is the shared replacement clause for the 3.2 bookkeeping
// and knowledge family pruned from the default build without individual
// tombstones (docs/MIGRATION.md "Other 3.2 bookkeeping and knowledge verbs").
// There is no in-ao replacement, so every member points at the same escape
// hatches. NOTE: `ao eval` returned in 3.3 as a live command and must never
// appear here — cross-check any addition against the registered command tree
// (TestHintedVerbsAreNotLiveCommands enforces this).
const prunedFamilyHint = "it was pruned from the default build with no in-ao replacement; " +
	"use your own tools, `ao gate check` for deterministic checks, or generic `ao provenance` records"

// removedCommands maps every verb removed from the default `ao` build to its
// replacement hint. Keep in lockstep with docs/MIGRATION.md — the drift test
// (TestRemovedVerbsHaveMigrationRows) fails when a verb here has no row there.
var removedCommands = map[string]removedCommand{
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
	"inject":     {use: "AgentOps no longer retrieves prior knowledge; use the caller's own memory or context tooling"},
	"verify":     {use: "the 3.2 verification front door was removed; semantic judgment is the Validate skill. If `ao verify init` installed a pre-push hook, delete the AGENTOPS-VERIFY-RATCHET block from .git/hooks/pre-push (see docs/UPGRADING.md)"},

	// The pruned 3.2 bookkeeping/knowledge family — one shared clause, per the
	// MIGRATION.md paragraph that lists them together (`eval` is deliberately
	// absent: it returned in 3.3 as a live command).
	"agents":    {use: prunedFamilyHint},
	"beads":     {use: prunedFamilyHint},
	"canon":     {use: prunedFamilyHint},
	"ci":        {use: prunedFamilyHint},
	"citation":  {use: prunedFamilyHint},
	"findings":  {use: prunedFamilyHint},
	"forge":     {use: prunedFamilyHint},
	"knowledge": {use: prunedFamilyHint},
	"mcp":       {use: prunedFamilyHint},
	"metrics":   {use: prunedFamilyHint},
	"notebook":  {use: prunedFamilyHint},
	"patterns":  {use: prunedFamilyHint},
	"pool":      {use: prunedFamilyHint},
	"ratchet":   {use: prunedFamilyHint},
	"registry":  {use: prunedFamilyHint},
	"scope":     {use: prunedFamilyHint},
	"sessions":  {use: prunedFamilyHint},
	"wiki":      {use: prunedFamilyHint},
}

// removedChildCommands covers retired subcommands under retained parents.
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

// requireKnownSubcommand makes a non-runnable parent fail loudly on an
// unknown subcommand instead of printing help with exit 0 (cobra's default
// for parents without a Run function). The error uses cobra's canonical
// unknown-command wording so removedCommandHint can attach the replacement
// pointer for retired children.
func requireKnownSubcommand(parent *cobra.Command) {
	if parent.Annotations == nil {
		parent.Annotations = map[string]string{}
	}
	// Group JSON help treats guarded parents as groups even though the guard
	// makes them technically Runnable (see maybeEmitGroupJSON).
	parent.Annotations[groupGuardAnnotation] = "true"
	parent.Args = cobra.ArbitraryArgs
	parent.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
	}
	parent.SilenceErrors = false
}

// removedCommandHint returns the replacement hint for an "unknown command"
// error whose verb was removed from the default build, or "" when the error
// is anything else. A registered verb always wins so a usage error cannot be
// mislabeled as removal.
func removedCommandHint(root *cobra.Command, err error) string {
	if err == nil {
		return ""
	}
	verb, parent := parseUnknownCommand(err.Error())
	if verb == "" {
		return ""
	}
	tomb, ok := lookupRemovedCommand(verb, parent)
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
	label := verb
	if parent != "" {
		label = parent + " " + verb
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%q was removed from ao — %s.\n", label, tomb.use)
	b.WriteString("Full map of removed surfaces and replacements: docs/MIGRATION.md\n")
	return b.String()
}

// lookupRemovedCommand resolves a removed verb against the root map or, when
// the unknown command surfaced beneath a retained parent, the child map.
func lookupRemovedCommand(verb, parent string) (removedCommand, bool) {
	if parent == "" {
		tomb, ok := removedCommands[verb]
		return tomb, ok
	}
	children, ok := removedChildCommands[parent]
	if !ok {
		return removedCommand{}, false
	}
	tomb, ok := children[verb]
	return tomb, ok
}

// printRemovedCommandHint writes the removed-verb hint (if any) to w. Cobra has
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

// parseUnknownCommand extracts the offending verb and (for nested commands)
// the parent path from a cobra error string such as
// `unknown command "watch" for "ao"` or `unknown command "trace" for "ao goals"`.
// The parent return is "" for root-level verbs and the sub-path without the
// leading "ao " otherwise. Returns "" verb for any other error.
func parseUnknownCommand(msg string) (verb, parent string) {
	const prefix = `unknown command "`
	idx := strings.Index(msg, prefix)
	if idx < 0 {
		return "", ""
	}
	rest := msg[idx+len(prefix):]
	end := strings.Index(rest, `"`)
	if end <= 0 {
		return "", ""
	}
	verb = rest[:end]
	const forPrefix = ` for "`
	rest = rest[end+1:]
	if idx := strings.Index(rest, forPrefix); idx >= 0 {
		tail := rest[idx+len(forPrefix):]
		if end := strings.Index(tail, `"`); end > 0 {
			path := tail[:end]
			if path != "ao" {
				parent = strings.TrimPrefix(path, "ao ")
			}
		}
	}
	return verb, parent
}
