// practices: [ai-assisted-dev, pragmatic-programmer]
package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// removedCommand is the tombstone for a verb the default build no longer
// serves: what replaces it, and (when a build tag restores it) how to get the
// old surface back. The full map with the why lives in docs/MIGRATION.md —
// these one-liners exist so the pointer appears at the moment of failure,
// where a dev (or an agent following error strings) actually hits the wall.
type removedCommand struct {
	use     string // what to use instead — one clause, plain words
	restore string // exact restore command, or "" when nothing brings it back
}

const (
	restoreFlywheel = "make build-flywheel"
	restoreLegacy   = "AGENTOPS_LEGACY=1 make build"
)

// removedCommands maps every verb removed from the default `ao` build to its
// tombstone. Keep in lockstep with docs/MIGRATION.md — the drift test
// (TestRemovedVerbsHaveMigrationRows) fails when a verb here has no row there.
var removedCommands = map[string]removedCommand{
	// Daemon + scheduling lane (ADR-0009: delete, not deprecate; no restore).
	"daemon":    {use: "out-of-session work moved to an external substrate (NTM, `ao mcp serve`, or `ao agent`)"},
	"schedule":  {use: "scheduling moved to your substrate (cron-triggered NTM dispatch or a managed-agents schedule)"},
	"plans":     {use: "scheduling moved to your substrate (cron-triggered NTM dispatch or a managed-agents schedule)"},
	"watch":     {use: "scheduling moved to your substrate (cron-triggered NTM dispatch or a managed-agents schedule)"},
	"overnight": {use: "scheduling moved to your substrate (cron-triggered NTM dispatch or a managed-agents schedule)"},
	"cron":      {use: "scheduling moved to your substrate (cron-triggered NTM dispatch or a managed-agents schedule)"},

	// Loop verbs deleted outright (no build tag brings these back).
	"rpi":     {use: "run the in-session operating loop instead (the /rpi skill drives one turn of it)"},
	"evolve":  {use: "run the /evolve skill flow instead"},
	"factory": {use: "the factory is the loop itself — /crank and /swarm in-session, or substrate dispatch out-of-session"},

	// Memory/recall moved to external tools (ADR-0010 kept only session-log mining native).
	"recall": {use: "use cass (coding_agent_session_search) to search past sessions"},

	// Corpus/flywheel surface, archived behind //go:build flywheel (ADR-0012).
	"corpus":   {use: "archived flywheel surface; consume knowledge via cass + cm", restore: restoreFlywheel},
	"curate":   {use: "archived flywheel surface; consume knowledge via cass + cm", restore: restoreFlywheel},
	"defrag":   {use: "archived flywheel surface; consume knowledge via cass + cm", restore: restoreFlywheel},
	"harvest":  {use: "archived flywheel surface; consume knowledge via cass + cm", restore: restoreFlywheel},
	"mind":     {use: "archived flywheel surface; consume knowledge via cass + cm", restore: restoreFlywheel},
	"refinery": {use: "archived flywheel surface; consume knowledge via cass + cm", restore: restoreFlywheel},

	// RPI/factory machinery, archived behind //go:build legacy (ADR-0012).
	"autodev":     {use: "archived RPI/factory machinery; the operating loop replaces it", restore: restoreLegacy},
	"codex":       {use: "archived RPI/factory machinery; the operating loop replaces it", restore: restoreLegacy},
	"loop":        {use: "archived RPI/factory machinery; the operating loop replaces it", restore: restoreLegacy},
	"orchestrate": {use: "archived RPI/factory machinery; the operating loop replaces it", restore: restoreLegacy},
	"operator":    {use: "archived RPI/factory machinery; the operating loop replaces it", restore: restoreLegacy},
	"tick":        {use: "archived RPI/factory machinery; the operating loop replaces it", restore: restoreLegacy},
	"turn_verify": {use: "archived RPI/factory machinery; the operating loop replaces it", restore: restoreLegacy},
	"harness":     {use: "archived RPI/factory machinery; the operating loop replaces it", restore: restoreLegacy},
}

// removedCommandHint returns the tombstone hint for an "unknown command"
// error whose verb was removed from the default build, or "" when the error
// is anything else. A verb the running binary actually registers (a flywheel
// or legacy build) never hints: the command exists there, so a usage error
// must not claim it was removed.
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
	if tomb.restore != "" {
		fmt.Fprintf(&b, "Restore the old surface with: %s\n", tomb.restore)
	}
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
