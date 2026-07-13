// Package claim owns the Cobra presentation for the claim command family.
// Handlers parse, delegate, and render; application policy and effects live
// behind injected ports.
package claim

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	claimapp "github.com/boshu2/agentops/cli/internal/claim"
	"github.com/boshu2/agentops/cli/internal/claimproof"
	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/ports"
)

type UseCases interface {
	Claim(context.Context, string, claimapp.Streams) error
	Bind(context.Context, claimapp.BindRequest) error
	List(context.Context) ([]ports.EvidenceBinding, error)
	Check(context.Context, string, bool) (claimproof.Report, error)
}

type Module struct {
	useCases UseCases
	output   func() string
}

func NewModule(useCases UseCases, output func() string) Module {
	return Module{useCases: useCases, output: output}
}

func (Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{
		ID: "ao.claim",
		Profiles: clicontract.ProfileDefault |
			clicontract.ProfileFlywheel |
			clicontract.ProfileLegacy |
			clicontract.ProfileCombined,
		Args:   clicontract.ArgsPolicy{Name: "range", Validate: cobra.RangeArgs(0, 1)},
		Output: clicontract.OutputNone,
		Effects: clicontract.EffectFilesystem | clicontract.EffectProcess |
			clicontract.EffectTracker | clicontract.EffectEnvironment,
		ExitClasses: map[int]clicontract.ExitClass{0: clicontract.ExitSuccess, 1: clicontract.ExitFailure},
	}
}

func (module Module) Command() *cobra.Command {
	root := &cobra.Command{
		Use:   "claim [id]",
		Short: "Claim a BR bead or manage claim-evidence bindings",
		Args:  cobra.RangeArgs(0, 1),
		Long: `Claim a BR bead for harness-neutral AgentOps loops, or bind/list claim evidence via the typed BC2 ClaimEvidenceBinderPort.

Examples:
  ao claim cp-123
  ao claim bind --claim AOP-CLAIM-X --path .agents/findings/x.md --level PG2
  ao claim list`,
	}
	root.RunE = func(command *cobra.Command, args []string) error {
		if len(args) != 1 {
			return command.Help()
		}
		return module.useCases.Claim(command.Context(), args[0], claimapp.Streams{
			Stdin: command.InOrStdin(), Stdout: command.OutOrStdout(), Stderr: command.ErrOrStderr(),
		})
	}
	root.AddCommand(module.bindCommand(), module.listCommand(), module.checkCommand())
	return root
}

func (module Module) bindCommand() *cobra.Command {
	var request claimapp.BindRequest
	command := &cobra.Command{
		Use:   "bind --claim <AOP-CLAIM-X> --path <evidence-path> [--level PG1|PG2|PG3|PG4] [--anchor ...] [--author-id <id> --judge-id <id>]",
		Short: "Bind a claim to an evidence file at a promotion level",
		Long: `Append (or upgrade) a claim→evidence binding via the typed BC2
ClaimEvidenceBinderPort (productionClaimEvidenceBinder, cycle 116).

The binder is append-only on disk; List replays the file and folds
to the latest per (Claim, Path). The Level can only go UP — attempting
to downgrade (e.g., PG3 → PG1) returns an error.

Examples:
  ao claim bind --claim AOP-CLAIM-X --path .agents/findings/x.md --level PG2
  ao claim bind --claim AOP-CLAIM-Y --path p.md --level PG4 --anchor L10 --anchor L20
  ao claim bind --claim AOP-CLAIM-Z --path verdict.md --level PG4 --author-id worker-1 --judge-id verifier-2`,
		Args: cobra.NoArgs,
	}
	command.Flags().StringVar(&request.Claim, "claim", "", "claim ID (required, e.g. AOP-CLAIM-X)")
	command.Flags().StringVar(&request.Path, "path", "", "evidence file path (required, relative to repo root)")
	command.Flags().StringVar(&request.Level, "level", "PG1", "promotion level: PG1|PG2|PG3|PG4")
	command.Flags().StringArrayVar(&request.Anchors, "anchor", nil, "optional in-file anchors (repeatable)")
	command.Flags().StringVar(&request.AuthorID, "author-id", "", "artifact author identity for reviewer separation checks")
	command.Flags().StringVar(&request.JudgeID, "judge-id", "", "judge/verifier identity for reviewer separation checks")
	_ = command.MarkFlagRequired("claim")
	_ = command.MarkFlagRequired("path")
	command.RunE = func(command *cobra.Command, _ []string) error {
		if err := module.useCases.Bind(command.Context(), request); err != nil {
			return err
		}
		_, err := fmt.Fprintf(command.OutOrStdout(), "bound claim=%q path=%q level=%s\n", request.Claim, request.Path, request.Level)
		return err
	}
	return command
}

func (module Module) listCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "list",
		Short: "List recorded claim→evidence bindings (most-recent first)",
		Long:  `Emit all known claim→evidence bindings via the typed BC2 ClaimEvidenceBinderPort. Output is line-delimited JSON.`,
		Args:  cobra.NoArgs,
	}
	command.RunE = func(command *cobra.Command, _ []string) error {
		bindings, err := module.useCases.List(command.Context())
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(command.OutOrStdout())
		for _, binding := range bindings {
			if err := encoder.Encode(binding); err != nil {
				return fmt.Errorf("claim list encode: %w", err)
			}
		}
		return nil
	}
	return command
}

func (module Module) checkCommand() *cobra.Command {
	var changedOnly bool
	var base string
	command := &cobra.Command{
		Use:   "check --changed [--base <ref>]",
		Short: "Report proof cards for changed public claims",
		Long: `Report read-only proof cards for changed public claim markers.

The checker compares the current branch/worktree to a base ref, finds changed
files containing agentops:claim markers, then reports each claim's registry tier,
evidence citation status, verdict, and next action. It does not bind evidence,
promote tiers, mutate beads, or edit the claim registry.

Examples:
  ao claim check --changed
  ao claim check --changed --base origin/main --json`,
		Args: cobra.NoArgs,
	}
	command.Flags().BoolVar(&changedOnly, "changed", false, "check claim markers in files changed against --base plus worktree changes")
	command.Flags().StringVar(&base, "base", "origin/main", "base ref for --changed comparison")
	command.RunE = func(command *cobra.Command, _ []string) error {
		report, err := module.useCases.Check(command.Context(), base, changedOnly)
		if err != nil {
			return err
		}
		if module.output != nil && module.output() == "json" {
			encoder := json.NewEncoder(command.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(report)
		}
		writeHuman(command, report)
		return nil
	}
	return command
}

func writeHuman(command *cobra.Command, report claimproof.Report) {
	writer := command.OutOrStdout()
	fmt.Fprintf(writer, "claim check: %d changed claim(s) across %d surface(s) (base=%s)\n", report.Summary.Claims, report.Summary.ChangedSurfaces, report.Summary.Base)
	if len(report.Cards) == 0 {
		fmt.Fprintln(writer, "No changed claim markers found.")
		return
	}
	for _, card := range report.Cards {
		fmt.Fprintf(writer, "\n%s\n", card.ClaimID)
		fmt.Fprintf(writer, "  surface: %s\n", card.Surface)
		fmt.Fprintf(writer, "  tier: %s\n", card.Tier)
		if len(card.CiteAllowed) > 0 {
			fmt.Fprintf(writer, "  citation_ok: %t (allowed: %s)\n", card.CitationOK, strings.Join(card.CiteAllowed, ", "))
		}
		fmt.Fprintf(writer, "  verdict: %s\n", card.Verdict)
		if card.EvalBinding != "" {
			fmt.Fprintf(writer, "  eval_binding: %s\n", card.EvalBinding)
		}
		if len(card.Evidence) == 0 {
			fmt.Fprintln(writer, "  evidence: none")
		} else {
			fmt.Fprintln(writer, "  evidence:")
			for _, evidence := range card.Evidence {
				if evidence.Reason == "" {
					fmt.Fprintf(writer, "    - %s [%s]\n", evidence.Path, evidence.Status)
				} else {
					fmt.Fprintf(writer, "    - %s [%s] %s\n", evidence.Path, evidence.Status, evidence.Reason)
				}
			}
		}
		fmt.Fprintf(writer, "  next: %s\n", card.NextAction)
	}
}
