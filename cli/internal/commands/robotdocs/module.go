// Package robotdocs owns Cobra presentation for the `ao robot-docs` command.
// The module renders a paste-ready agent handbook whose command surface is
// generated from the live command tree, so it performs no filesystem, process,
// or clock effect — it is a pure read of the Cobra command graph reachable from
// the invoked command's root.
package robotdocs

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

// Module owns Cobra presentation for the robot-docs command.
type Module struct{}

// NewModule constructs the robot-docs command module.
func NewModule() Module {
	return Module{}
}

// Contract declares robot-docs's real behavior: it accepts (and ignores)
// arbitrary positional args exactly as Cobra does today, emits Markdown text to
// stdout, is a pure read of the live command tree (no filesystem, process, or
// clock effect), and exits 0 on success or 1 on an output-write failure.
func (Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{
		ID:       "ao.robot-docs",
		Profiles: clicontract.ProfileDefault | clicontract.ProfileFlywheel | clicontract.ProfileLegacy | clicontract.ProfileCombined,
		Args:     clicontract.ArgsPolicy{Name: "arbitrary", Validate: cobra.ArbitraryArgs},
		Output:   clicontract.OutputText,
		Effects:  clicontract.EffectPure,
		ExitClasses: map[int]clicontract.ExitClass{
			0: clicontract.ExitSuccess,
			1: clicontract.ExitFailure,
		},
	}
}

// Command builds the `ao robot-docs` command.
func (Module) Command() *cobra.Command {
	return &cobra.Command{
		Use:   "robot-docs",
		Short: "Print the paste-ready agent handbook for the ao CLI (Markdown)",
		Long: `Print a paste-ready, agent-targeted handbook for the whole ao CLI.

The handbook covers the output contract, exit codes, machine-readable
surfaces, and the canonical agent workflow — everything an agent needs to
drive ao without an external documentation lookup.`,
		GroupID: "core",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprint(cmd.OutOrStdout(), RenderHandbook(cmd.Root()))
			return nil
		},
	}
}

// RenderHandbook renders the agent handbook. The command list is generated from
// the live command tree rooted at root so the handbook never drifts from
// registration.
func RenderHandbook(root *cobra.Command) string {
	var b strings.Builder
	b.WriteString(`# ao — Agent Handbook

ao is the AgentOps CLI: a validation gate plus a provenance record for
agent work — validated output with proof (no verdict = not done). This
handbook is the contract — read it once, then drive ao without guessing.

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
| 2 | diagnostic: partial result (command-specific) |

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
ao gate check --fast --scope head   # ordinary deterministic repository checks
` + "```" + `

## Environment

- ` + "`NO_COLOR`" + ` disables ANSI styling.
- ` + "`AGENTOPS_CONFIG`" + ` overrides the config file path (same as ` + "`--config`" + `).

## Command surface

`)
	for _, g := range root.Groups() {
		var lines []string
		for _, c := range root.Commands() {
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
