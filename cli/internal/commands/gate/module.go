package gate

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	gateapp "github.com/boshu2/agentops/cli/internal/gate"
	"github.com/boshu2/agentops/cli/internal/ports"
	"github.com/boshu2/agentops/cli/internal/search"
)

type ReviewUseCases interface {
	Pending(context.Context, gateapp.PendingRequest) (gateapp.PendingResult, error)
	Approve(context.Context, gateapp.ApproveInput) (gateapp.ApproveResult, error)
	Reject(context.Context, gateapp.RejectInput) (gateapp.RejectResult, error)
	BulkApprove(context.Context, gateapp.BulkApproveInput) (gateapp.BulkApproveResult, error)
}

type RunUseCases interface {
	Execute(context.Context, gateapp.RunRequest) (ports.GateVerdict, error)
}

type CheckUseCases interface {
	Execute(context.Context, gateapp.CheckRequest) (gateapp.CheckResult, error)
}

type UseCases struct {
	Review ReviewUseCases
	Run    RunUseCases
	Check  CheckUseCases
}

type HostOptions struct {
	DryRun       func() bool
	OutputFormat func() string
}

type Module struct {
	useCases UseCases
	host     HostOptions
}

func NewModule(useCases UseCases, host HostOptions) Module {
	return Module{useCases: useCases, host: host}
}

func (Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{
		ID: "ao.gate",
		Profiles: clicontract.ProfileDefault | clicontract.ProfileFlywheel |
			clicontract.ProfileLegacy | clicontract.ProfileCombined,
		Args:    clicontract.ArgsPolicy{Name: "no-args", Validate: cobra.NoArgs},
		Output:  clicontract.OutputNone,
		Effects: clicontract.EffectFilesystem | clicontract.EffectProcess | clicontract.EffectEnvironment | clicontract.EffectClock,
		ExitClasses: map[int]clicontract.ExitClass{
			0: clicontract.ExitSuccess,
			1: clicontract.ExitFailure,
			2: clicontract.ExitUsage,
		},
	}
}

type ExitError struct {
	Code    int
	Message string
}

func (failure *ExitError) Error() string { return failure.Message }
func (failure *ExitError) ExitCode() int { return failure.Code }

func (module Module) Command() *cobra.Command {
	command := &cobra.Command{
		Use:     "gate",
		Short:   "Human review gates",
		GroupID: "core",
		Long: `Manage human review gates for bronze-tier candidates.

Bronze-tier candidates (score 0.50-0.69) require human review
before promotion. The gate command provides the review interface.

Examples:
  ao gate pending
  ao gate approve <candidate-id>
  ao gate reject <candidate-id> --reason="Too vague"`,
	}
	command.AddCommand(
		module.newPendingCommand(),
		module.newApproveCommand(),
		module.newRejectCommand(),
		module.newBulkApproveCommand(),
		module.newRunCommand(),
		module.newCheckCommand(),
	)
	return command
}

func (module Module) newPendingCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "pending",
		Short: "List candidates pending review",
		Long: `List bronze-tier candidates awaiting human review.

Shows age/urgency with oldest items first.
Highlights items approaching 24h auto-promote threshold.

Examples:
  ao gate pending
  ao gate pending --json`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if module.useCases.Review == nil {
				return fmt.Errorf("gate pending: use case not configured")
			}
			result, err := module.useCases.Review.Pending(command.Context(), gateapp.PendingRequest{DryRun: module.dryRun()})
			if err != nil {
				return err
			}
			if result.DryRun {
				fmt.Fprintln(command.OutOrStdout(), "[dry-run] Would list pending gate reviews")
				return nil
			}
			return renderPending(command, result.Entries, module.outputFormat())
		},
	}
}

func renderPending(command *cobra.Command, entries []gateapp.ReviewEntry, format string) error {
	switch format {
	case "json":
		encoder := json.NewEncoder(command.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(entries)
	case "yaml":
		return yaml.NewEncoder(command.OutOrStdout()).Encode(entries)
	default:
		return renderPendingTable(command, entries)
	}
}

func renderPendingTable(command *cobra.Command, entries []gateapp.ReviewEntry) error {
	output := command.OutOrStdout()
	if len(entries) == 0 {
		fmt.Fprintln(output, "No pending reviews")
		return nil
	}
	fmt.Fprintf(output, "Pending Reviews (%d)\n", len(entries))
	fmt.Fprintln(output, "==================")
	fmt.Fprintln(output)
	writer := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "ID\tTIER\tAGE\tUTILITY\tURGENCY")
	fmt.Fprintln(writer, "--\t----\t---\t-------\t-------")
	for _, entry := range entries {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%.2f\t%s\n",
			search.TruncateText(entry.Candidate.ID, 16),
			entry.Candidate.Tier,
			entry.AgeString,
			entry.Candidate.Utility,
			entry.Urgency,
		)
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	fmt.Fprintln(output)
	approaching := 0
	for _, entry := range entries {
		if entry.ApproachingAutoPromote {
			approaching++
		}
	}
	if approaching > 0 {
		fmt.Fprintf(output, "! %d candidate(s) approaching 24h auto-promote threshold\n", approaching)
	}
	return nil
}

func (module Module) newApproveCommand() *cobra.Command {
	var note string
	command := &cobra.Command{
		Use:   "approve <candidate-id>",
		Short: "Approve candidate for promotion",
		Long: `Approve a bronze-tier candidate for promotion.

Records reviewer identity and triggers promotion flow.

Examples:
  ao gate approve cand-abc123
  ao gate approve cand-abc123 --note="Good specificity, approved"`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if module.useCases.Review == nil {
				return fmt.Errorf("gate approve: use case not configured")
			}
			result, err := module.useCases.Review.Approve(command.Context(), gateapp.ApproveInput{CandidateID: args[0], Note: note, DryRun: module.dryRun()})
			if err != nil {
				return err
			}
			output := command.OutOrStdout()
			if result.DryRun {
				fmt.Fprintf(output, "[dry-run] Would approve candidate %s", result.CandidateID)
				if result.Note != "" {
					fmt.Fprintf(output, " with note: %s", result.Note)
				}
				fmt.Fprintln(output)
				return nil
			}
			fmt.Fprintf(output, "Approved: %s\nReviewer: %s\n", result.CandidateID, result.Reviewer)
			if result.Note != "" {
				fmt.Fprintf(output, "Note: %s\n", result.Note)
			}
			fmt.Fprintf(output, "\nTo promote: ao pool promote %s\n", result.CandidateID)
			return nil
		},
	}
	command.Flags().StringVar(&note, "note", "", "Optional approval note")
	return command
}

func (module Module) newRejectCommand() *cobra.Command {
	var reason string
	command := &cobra.Command{
		Use:   "reject <candidate-id>",
		Short: "Reject candidate",
		Long: `Reject a candidate with a required reason.

Records in audit trail for future analysis.

Examples:
  ao gate reject cand-abc123 --reason="Lacks specificity"
  ao gate reject cand-abc123 --reason="Duplicate of existing pattern"`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if module.useCases.Review == nil {
				return fmt.Errorf("gate reject: use case not configured")
			}
			result, err := module.useCases.Review.Reject(command.Context(), gateapp.RejectInput{CandidateID: args[0], Reason: reason, DryRun: module.dryRun()})
			if err != nil {
				return err
			}
			output := command.OutOrStdout()
			if result.DryRun {
				fmt.Fprintf(output, "[dry-run] Would reject candidate %s with reason: %s\n", result.CandidateID, result.Reason)
				return nil
			}
			fmt.Fprintf(output, "Rejected: %s\nReviewer: %s\nReason: %s\n", result.CandidateID, result.Reviewer, result.Reason)
			return nil
		},
	}
	command.Flags().StringVar(&reason, "reason", "", "Required rejection reason")
	_ = command.MarkFlagRequired("reason")
	return command
}

func (module Module) newBulkApproveCommand() *cobra.Command {
	var olderThan, tier string
	command := &cobra.Command{
		Use:   "bulk-approve",
		Short: "Bulk approve silver candidates",
		Long: `Approve all silver-tier candidates older than a threshold.

Silver candidates auto-promote after 24h if not rejected.
This command accelerates the process for reviewed batches.

Examples:
  ao gate bulk-approve
  ao gate bulk-approve --older-than=12h
  ao gate bulk-approve --older-than=24h --dry-run`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if module.useCases.Review == nil {
				return fmt.Errorf("gate bulk-approve: use case not configured")
			}
			result, err := module.useCases.Review.BulkApprove(command.Context(), gateapp.BulkApproveInput{OlderThan: olderThan, Tier: tier, DryRun: module.dryRun()})
			if err != nil {
				return err
			}
			output := command.OutOrStdout()
			if result.DryRun {
				if len(result.Approved) == 0 {
					fmt.Fprintln(output, "[dry-run] No candidates match criteria")
				} else {
					fmt.Fprintf(output, "[dry-run] Would approve %d candidate(s):\n", len(result.Approved))
					printIDs(output, result.Approved)
				}
				return nil
			}
			if len(result.Approved) == 0 {
				fmt.Fprintln(output, "No candidates matched criteria")
			} else {
				fmt.Fprintf(output, "Approved %d candidate(s):\n", len(result.Approved))
				printIDs(output, result.Approved)
			}
			return nil
		},
	}
	command.Flags().StringVar(&olderThan, "older-than", "24h", "Age threshold for bulk approval")
	command.Flags().StringVar(&tier, "tier", "silver", "Tier to bulk approve (default: silver)")
	_ = command.RegisterFlagCompletionFunc("tier", fixedCompletions("bronze", "silver", "gold"))
	return command
}

func printIDs(output interface{ Write([]byte) (int, error) }, ids []string) {
	for _, id := range ids {
		fmt.Fprintf(output, "  - %s\n", id)
	}
}

func (module Module) newRunCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "run <name>",
		Short: "Run a check-*.sh gate via BC2 GateRunnerPort and emit verdict",
		Long: `Invoke a check-*.sh gate via the typed BC2 GateRunnerPort
and emit a GateVerdict (JSON).

The <name> argument is the gate name without the 'check-' prefix
or '.sh' suffix. The runner resolves scripts/check-<name>.sh.

Useful as a typed alternative to 'bash scripts/check-<name>.sh; echo $?'
for scripts that want structured output (status, reason, log tail).

Examples:
  ao gate run compile-health
  ao gate run three-gap-supergate
  ao gate run xxx-does-not-exist`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if module.useCases.Run == nil {
				return fmt.Errorf("gate run: use case not configured")
			}
			verdict, err := module.useCases.Run.Execute(command.Context(), gateapp.RunRequest{Name: args[0]})
			if err != nil {
				return err
			}
			if err := json.NewEncoder(command.OutOrStdout()).Encode(verdict); err != nil {
				return fmt.Errorf("gate run encode: %w", err)
			}
			return nil
		},
	}
}

func (module Module) newCheckCommand() *cobra.Command {
	var fast, full, jsonOutput, githubAnnotations, failFast bool
	var scope, workflowPath string
	var workflowCoverage, requireWorkflowParity bool
	command := &cobra.Command{
		Use:   "check",
		Short: "Run the gate registry (fast cockpit subset or full suite)",
		Long: `Run the declarative gate registry.

  ao gate check            # fast: only checks affected by changed files (cockpit)
  ao gate check --full     # full: every check, routing ignored (CI/refinery)
  ao gate check --json     # machine-readable report (refinery/CI annotations)

Exit code is the verdict: 0 if no blocking check FAILed; 1 if any did
(non-blocking FAIL and WARN/SKIP are advisory).`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			_ = fast
			if module.useCases.Check == nil {
				return &ExitError{Code: 2, Message: "gate check: use case not configured"}
			}
			result, err := module.useCases.Check.Execute(command.Context(), gateapp.CheckRequest{
				Full: full, Scope: scope, FailFast: failFast,
				WorkflowCoverage: workflowCoverage, RequireWorkflowParity: requireWorkflowParity, WorkflowPath: workflowPath,
			})
			if err != nil {
				return &ExitError{Code: 2, Message: err.Error()}
			}
			if jsonOutput {
				raw, jsonErr := result.Report.JSON()
				if jsonErr != nil {
					return &ExitError{Code: 2, Message: jsonErr.Error()}
				}
				fmt.Fprintln(command.OutOrStdout(), string(raw))
			} else {
				result.Report.Human(command.OutOrStdout())
			}
			if githubAnnotations {
				result.Report.GitHubAnnotations(command.ErrOrStderr())
			}
			if result.ExitCode != 0 {
				message := ""
				if result.WorkflowParityMissing > 0 {
					message = fmt.Sprintf("workflow parity missing %d blocking script(s)", result.WorkflowParityMissing)
				}
				return &ExitError{Code: result.ExitCode, Message: message}
			}
			return nil
		},
	}
	flags := command.Flags()
	flags.BoolVar(&fast, "fast", false, "fast cockpit subset routed to changed files (the default; explicit flag for clarity in hooks)")
	flags.BoolVar(&full, "full", false, "run every check (routing ignored); default is the fast changed-file subset")
	flags.BoolVar(&jsonOutput, "json", false, "emit the machine-readable JSON report")
	flags.BoolVar(&githubAnnotations, "github-annotations", false, "emit GitHub Actions annotations for WARN/FAIL checks")
	flags.BoolVar(&failFast, "fail-fast", false, "stop after the first blocking failure")
	flags.StringVar(&scope, "scope", "head", "fast-mode changed-file scope: head|staged|worktree|upstream|range:<base>..<head>")
	flags.BoolVar(&workflowCoverage, "workflow-coverage", false, "include validate.yml-vs-registry script coverage in the report")
	flags.BoolVar(&requireWorkflowParity, "require-workflow-parity", false, "fail if validate.yml references scripts missing from the Go gate registry")
	flags.StringVar(&workflowPath, "workflow-path", ".github/workflows/validate.yml", "workflow path used by --workflow-coverage and --require-workflow-parity")
	return command
}

func (module Module) dryRun() bool {
	return module.host.DryRun != nil && module.host.DryRun()
}

func (module Module) outputFormat() string {
	if module.host.OutputFormat == nil {
		return "table"
	}
	return module.host.OutputFormat()
}

func fixedCompletions(values ...string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	return func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return sorted, cobra.ShellCompDirectiveNoFileComp
	}
}
