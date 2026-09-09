// Package status owns Cobra presentation for the `ao status` command. The
// module builds its command with host-provided seams and delegates every
// filesystem and clock effect to internal/statusapp.
package status

import (
	"bytes"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/statusapp"
)

// Module owns Cobra presentation for the status command family.
type Module struct {
	host clicontract.HostOptions
}

// NewModule constructs the status command module from its host seams.
func NewModule(host clicontract.HostOptions) Module {
	return Module{host: host}
}

// Contract declares status's real behavior: it accepts (and ignores) arbitrary
// positional args exactly as Cobra does today, emits text (JSON under -o json),
// reads the durable evidence stores and active Git environment boundaries,
// stamps recency from the clock, and exits 0 on success or 1 on invalid input
// or a root-resolution failure.
func (Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{
		ID:       "ao.status",
		Profiles: clicontract.ProfileDefault | clicontract.ProfileLegacy | clicontract.ProfileCombined,
		Args:     clicontract.ArgsPolicy{Name: "arbitrary", Validate: cobra.ArbitraryArgs},
		Output:   clicontract.OutputText,
		Effects:  clicontract.EffectFilesystem | clicontract.EffectEnvironment | clicontract.EffectClock,
		ExitClasses: map[int]clicontract.ExitClass{
			0: clicontract.ExitSuccess,
			1: clicontract.ExitFailure,
		},
	}
}

// Command builds the `ao status` command. The RunE closure delegates entirely
// to statusapp so this module performs no direct effect.
func (module Module) Command() *cobra.Command {
	var evidenceRoot string
	command := &cobra.Command{
		Use:   "status",
		Short: "Show durable AgentOps loop evidence",
		Long: `Display the content-addressed intent and verdict evidence stored by AgentOps.

The command validates artifact names, content identity, and verdict.v2 shape
before counting evidence. It reports recency only; it does not infer an active
runtime phase, elapsed execution time, tool activity, retries, or remaining work.

Without --evidence-root, read .agents/ao under the working directory.
With --evidence-root, inspect only intents/sha256 and verdicts/sha256 in that
existing non-Git directory. Invalid roots fail without fallback or writes;
evidence directory and file symlinks are excluded from explicit-root inspection.

Examples:
  ao status
  ao status --json
  ao status --evidence-root /path/to/evidence --json`,
		GroupID: "core",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts := statusapp.RunOptions{
				JSON: module.host.OutputMode() == "json", Stdout: cmd.OutOrStdout(),
			}
			if cmd.Flags().Changed("evidence-root") {
				opts.EvidenceRoot = &evidenceRoot
			}
			if module.host.OutputMode() == "yaml" {
				// statusapp emits exactly one JSON document to Stdout in JSON
				// mode; capture it and re-emit as YAML so -o yaml is the same
				// data yaml-marshaled rather than a silent human-table fallback.
				var buf bytes.Buffer
				opts.JSON, opts.Stdout = true, &buf
				if err := statusapp.Run(opts); err != nil {
					return err
				}
				return clicontract.JSONToYAML(cmd.OutOrStdout(), buf.Bytes())
			}
			return statusapp.Run(opts)
		},
	}
	command.Flags().StringVar(&evidenceRoot, "evidence-root", "", "Existing explicit non-Git evidence directory (default: working directory's .agents/ao)")
	return command
}
