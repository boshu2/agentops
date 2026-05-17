// practices: [twelve-factor-app, agent-ergonomics]
package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var robotDocsCmd = &cobra.Command{
	Use:   "robot-docs",
	Short: "Print the paste-ready agent handbook for the ao CLI (Markdown)",
	Long: `Print a paste-ready, agent-targeted handbook for the whole ao CLI.

The handbook covers the output contract, exit codes, machine-readable
surfaces, and the canonical agent workflow — everything an agent needs to
drive ao without an external documentation lookup.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		fmt.Fprint(cmd.OutOrStdout(), robotDocsText())
		return nil
	},
}

func init() {
	robotDocsCmd.GroupID = "core"
	rootCmd.AddCommand(robotDocsCmd)
}

// robotDocsText renders the agent handbook. The command list is generated
// from the live command tree so the handbook never drifts from registration.
func robotDocsText() string {
	var b strings.Builder
	b.WriteString(`# ao — Agent Handbook

ao is the AgentOps CLI: a software-factory control plane for repo-native
agent work. This handbook is the contract — read it once, then drive ao
without guessing.

## Output contract

- stdout is data; stderr is diagnostics. ` + "`ao <cmd> --json | jq ...`" + ` works
  without filtering log lines.
- Append ` + "`--json`" + ` (or ` + "`-o json`" + `) to any read-side command for a stable,
  parseable structure. ` + "`-o yaml`" + ` and the default ` + "`-o table`" + ` are also available.
- Output is deterministic where possible: stable ordering, no timestamp
  leakage into free text.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | success |
| 1 | error: usage error, runtime failure, or (for diagnostic commands) findings present |
| 2 | diagnostic: partial result or bead claimed (command-specific) |

Diagnostic commands extend this dictionary. Read the precise codes with
` + "`ao doctor capabilities`" + ` (doctor surface) or a command's own ` + "`--help`" + `.

## Machine-readable surfaces

- ` + "`ao capabilities`" + ` — the full CLI contract as JSON: command surface,
  global flags, exit codes, env vars. Run this first.
- ` + "`ao robot-docs`" + ` — this handbook.
- ` + "`ao doctor --robot-triage`" + ` — mega-command: health triage JSON in one call.
- ` + "`ao doctor capabilities`" + ` — extended doctor contract (detectors, fixers,
  exit codes).

## Canonical agent workflow

` + "```" + `
ao capabilities                 # discover the contract
ao status --json                # where am I, what's initialized
ao doctor --robot-triage        # one-call health + remediation
ao inject "<topic>"             # pull relevant prior knowledge
ao rpi phased "<goal>"          # run the Research-Plan-Implement loop
` + "```" + `

## Environment

- ` + "`NO_COLOR`" + ` disables ANSI styling.
- ` + "`AGENTOPS_CONFIG`" + ` overrides the config file path (same as ` + "`--config`" + `).

## Command surface

`)
	for _, g := range rootCmd.Groups() {
		var lines []string
		for _, c := range rootCmd.Commands() {
			if c.Hidden || c.GroupID != g.ID {
				continue
			}
			lines = append(lines, fmt.Sprintf("  %-15s %s", c.Name(), c.Short))
		}
		if len(lines) == 0 {
			continue
		}
		b.WriteString(g.Title + "\n")
		b.WriteString("```\n")
		for _, l := range lines {
			b.WriteString(l + "\n")
		}
		b.WriteString("```\n\n")
	}
	b.WriteString("Run `ao <command> --help` for the flags and arguments of any command.\n")
	return b.String()
}
