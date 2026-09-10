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
		Profiles: clicontract.ProfileDefault | clicontract.ProfileLegacy | clicontract.ProfileCombined,
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
surfaces, and optional AO operations for native execution. Consult it when
an operation is useful; no startup sequence or workflow skills are required.`,
		GroupID: "core",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprint(cmd.OutOrStdout(), RenderHandbook(cmd.Root()))
			return err
		},
	}
}

// RenderHandbook renders the agent handbook. The command list is generated from
// the live command tree rooted at root so the handbook never drifts from
// registration.
func RenderHandbook(root *cobra.Command) string {
	var b strings.Builder
	b.WriteString(`# ao — Agent Handbook

ao provides deterministic repository checks and explicit evidence operations.
Your native coding agent owns the approach and completes accepted work using
repository instructions and ordinary tools. Consult this handbook on demand;
there is no mandatory AO startup sequence and workflow skills are optional.

## Output contract

- stdout is data; stderr is diagnostics. ` + "`ao <cmd> --json | jq ...`" + ` works
  without filtering log lines.
- Commands that advertise structured output support ` + "`--json`" + ` or ` + "`-o json`" + `.
  Use ` + "`-o yaml`" + ` or ` + "`-o table`" + ` where the command documents support.
  This handbook itself is Markdown text.
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
  global flags, exit codes and env vars, when command discovery is useful.
- ` + "`ao robot-docs`" + ` — this handbook.
- ` + "`ao doctor --robot-triage`" + ` — mega-command: health triage JSON in one call.
- ` + "`ao doctor capabilities`" + ` — extended doctor contract (detectors, fixers,
  exit codes).

## Native execution

Read the repository brief, preserve accepted behavior and scope, implement
with native tools, run required checks and repair known failures. Obtain
fresh independent judgment over the exact final change against unchanged acceptance.
The author cannot issue the binding PASS. Missing judgment or unchecked
acceptance remains NOT_PROVEN; passing deterministic checks alone is insufficient.
Report checked and not_checked scope and follow repository delivery policy.

The Validate skill is an optional review method; skills, bootstrap and
initialization are not prerequisites. Evidence persistence is optional too.
Git, work tracking and delivery remain with the caller and repository.

## AO operations on demand

Choose an operation for a current need; these are independent tools, not steps
to run at the beginning of every task.

| Current need | Operation |
|--------------|-----------|
| Read the native execution brief | ` + "`ao quick-start`" + ` |
| Inspect command contracts | ` + "`ao capabilities`" + ` |
| Inspect existing durable evidence | ` + "`ao status --json`" + ` |
| Diagnose an AgentOps health problem | ` + "`ao doctor --robot-triage`" + ` |
| Run deterministic repository checks | ` + "`ao gate check`" + ` |
| Inspect explicit evidence operations | ` + "`ao provenance --help`" + ` |

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
