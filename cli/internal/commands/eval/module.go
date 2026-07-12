// Package eval owns Cobra presentation for the eval command family.
package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	aoeval "github.com/boshu2/agentops/cli/internal/eval"
)

type CoreUseCases interface {
	Run(context.Context, aoeval.CoreRunRequest) (aoeval.CoreRunResult, error)
	Compare(context.Context, aoeval.CoreCompareRequest) (aoeval.CoreCompareResult, error)
	PromoteBaseline(context.Context, aoeval.CoreBaselineRequest) (*aoeval.RunRecord, error)
	AuditBaseline(context.Context, aoeval.CoreBaselineAuditRequest) (*aoeval.BaselineAuditReport, error)
	Scorecard(context.Context, aoeval.CoreScorecardRequest) (*aoeval.Scorecard, error)
	Coverage(context.Context, aoeval.CoreCoverageRequest) (*aoeval.CoverageReport, error)
}

type CleanupUseCases interface {
	Execute(context.Context, aoeval.CleanupRequest) (aoeval.CleanupReport, error)
}

type UseCases struct {
	Core    CoreUseCases
	Cleanup CleanupUseCases
}

type HostOptions struct {
	OutputMode func(*cobra.Command) string
	Verbose    func(*cobra.Command) bool
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
		ID: "ao.eval",
		Profiles: clicontract.ProfileDefault | clicontract.ProfileFlywheel |
			clicontract.ProfileLegacy | clicontract.ProfileCombined,
		Args:        clicontract.ArgsPolicy{Name: "no-args", Validate: cobra.NoArgs},
		Output:      clicontract.OutputNone,
		Effects:     clicontract.EffectFilesystem | clicontract.EffectProcess | clicontract.EffectEnvironment | clicontract.EffectClock,
		ExitClasses: map[int]clicontract.ExitClass{0: clicontract.ExitSuccess, 1: clicontract.ExitFailure},
	}
}

type runOptions struct {
	output, runID, runtime, baseline, baselineMode     string
	contextMode, contextOffDir, contextOnDir, deltaOut string
}
type compareOptions struct {
	output                     string
	maxAggregate, maxDimension float64
}
type baselineOptions struct{ output, promotedBy, rationale string }
type baselineAuditOptions struct{ root, baselineDir string }
type scorecardOptions struct {
	output, kind string
	maxCategory  float64
}
type coverageOptions struct {
	root                                         string
	domains, evidenceKinds, dimensions, runtimes []string
}

func (module Module) Command() *cobra.Command {
	command := &cobra.Command{
		Use: "eval", Short: "Run deterministic local evaluation suites",
		Long: `Run deterministic AgentOps evaluation suites and compare run records.

The eval surface intentionally supports only offline deterministic runs in this
release. Live Claude and Codex adapters are evaluated by a later runtime tier.`,
	}
	command.GroupID = "workflow"
	command.AddCommand(module.runCommand(), module.compareCommand(), module.baselineCommand(), module.baselineAuditCommand(), module.scorecardCommand(), module.coverageCommand())
	if module.useCases.Cleanup != nil {
		command.AddCommand(module.cleanupCommand())
	}
	return command
}

func (module Module) outputMode(command *cobra.Command) string {
	if module.host.OutputMode == nil {
		return ""
	}
	return module.host.OutputMode(command)
}

func (module Module) runCommand() *cobra.Command {
	options := runOptions{baselineMode: string(aoeval.BaselineModeSkillOn), contextMode: string(aoeval.ContextModeNone)}
	command := &cobra.Command{Use: "run <suite.json>", Short: "Run a deterministic eval suite", Args: cobra.ExactArgs(1)}
	flags := command.Flags()
	flags.StringVar(&options.output, "out", "", "write eval run record to path")
	flags.StringVar(&options.runID, "run-id", "", "stable run id to use in the run record")
	flags.StringVar(&options.runtime, "runtime", "", "runtime override (static, mock, shell, claude, codex)")
	flags.StringVar(&options.baseline, "baseline", "", "compare the run against a baseline run record")
	flags.StringVar(&options.baselineMode, "baseline-mode", string(aoeval.BaselineModeSkillOn), "skill-on | skill-off | both — runs the suite once with skills loaded, once with hooks suppressed, or both for a delta scorecard")
	flags.StringVar(&options.contextMode, "context-mode", string(aoeval.ContextModeNone), "none | ab — run context-off/context-on legs over isolated AO_AGENTS_DIR roots")
	flags.StringVar(&options.contextOffDir, "context-off-agents-dir", "", "AO_AGENTS_DIR root for the context-off leg (defaults to suite fixtures)")
	flags.StringVar(&options.contextOnDir, "context-on-agents-dir", "", "AO_AGENTS_DIR root for the context-on leg (defaults to suite fixtures)")
	flags.StringVar(&options.deltaOut, "delta-out", "", "write delta scorecard JSON to path (with --baseline-mode=both or --context-mode=ab)")
	_ = command.RegisterFlagCompletionFunc("runtime", staticCompletion("static", "mock", "shell", "claude", "codex"))
	_ = command.RegisterFlagCompletionFunc("baseline-mode", staticCompletion(aoeval.AllBaselineModes()...))
	_ = command.RegisterFlagCompletionFunc("context-mode", staticCompletion(aoeval.AllContextModes()...))
	command.RunE = func(command *cobra.Command, args []string) error {
		result, err := module.useCases.Core.Run(command.Context(), aoeval.CoreRunRequest{SuitePath: args[0], RunID: options.runID, Runtime: options.runtime, OutputPath: options.output, BaselinePath: options.baseline, BaselineMode: options.baselineMode, ContextMode: options.contextMode, ContextOffDir: options.contextOffDir, ContextOnDir: options.contextOnDir, DeltaOut: options.deltaOut})
		if err != nil {
			return err
		}
		return module.renderRun(command, result)
	}
	return command
}

func (module Module) compareCommand() *cobra.Command {
	var options compareOptions
	command := &cobra.Command{Use: "compare <candidate-run.json> <baseline-run.json>", Short: "Compare an eval run against a baseline", Args: cobra.ExactArgs(2)}
	command.Flags().StringVar(&options.output, "out", "", "write compared eval run record to path")
	command.Flags().Float64Var(&options.maxAggregate, "max-aggregate-regression", 0, "allowed aggregate regression before verdict becomes regression")
	command.Flags().Float64Var(&options.maxDimension, "max-dimension-regression", 0, "allowed per-dimension regression before verdict becomes regression")
	command.RunE = func(command *cobra.Command, args []string) error {
		result, err := module.useCases.Core.Compare(command.Context(), aoeval.CoreCompareRequest{CandidatePath: args[0], BaselinePath: args[1], OutputPath: options.output, MaxAggregateRegression: options.maxAggregate, MaxDimensionRegression: options.maxDimension})
		if err != nil {
			return err
		}
		if module.outputMode(command) == "json" {
			return writeJSON(command, result.Candidate)
		}
		delta := 0.0
		if result.Candidate.BaselineComparison != nil {
			delta = result.Candidate.BaselineComparison.AggregateDelta
		}
		fmt.Fprintf(command.OutOrStdout(), "Eval compare %s vs %s: %s (aggregate delta %.4f)\n", result.Candidate.RunID, result.Baseline.RunID, result.Candidate.Verdict, delta)
		if result.OutputPath != "" {
			fmt.Fprintf(command.OutOrStdout(), "Comparison record: %s\n", result.OutputPath)
		}
		return nil
	}
	return command
}

func (module Module) baselineCommand() *cobra.Command {
	var options baselineOptions
	command := &cobra.Command{Use: "baseline <run.json>", Short: "Promote an eval run record as a baseline", Args: cobra.ExactArgs(1)}
	command.Flags().StringVar(&options.output, "out", "", "write promoted baseline run record to path")
	command.Flags().StringVar(&options.promotedBy, "promoted-by", "", "identity promoting the baseline")
	command.Flags().StringVar(&options.rationale, "rationale", "", "rationale for promoting the baseline")
	command.RunE = func(command *cobra.Command, args []string) error {
		result, err := module.useCases.Core.PromoteBaseline(command.Context(), aoeval.CoreBaselineRequest{RunPath: args[0], OutputPath: options.output, PromotedBy: options.promotedBy, Rationale: options.rationale})
		if err != nil {
			return err
		}
		if module.outputMode(command) == "json" {
			return writeJSON(command, result)
		}
		path := ""
		if result.Baseline != nil {
			path = result.Baseline.BaselinePath
		}
		fmt.Fprintf(command.OutOrStdout(), "Eval baseline promoted: %s\n", path)
		return nil
	}
	return command
}

func (module Module) baselineAuditCommand() *cobra.Command {
	options := baselineAuditOptions{root: "evals/agentops-core", baselineDir: ".agents/evals/baselines"}
	command := &cobra.Command{Use: "baseline-audit [suite.json ...]", Short: "Audit eval suite baseline policy against promoted baselines", Args: cobra.ArbitraryArgs}
	command.Flags().StringVar(&options.root, "root", options.root, "suite root to scan when no suite paths are provided")
	command.Flags().StringVar(&options.baselineDir, "baseline-dir", options.baselineDir, "promoted baseline directory")
	command.RunE = func(command *cobra.Command, args []string) error {
		report, err := module.useCases.Core.AuditBaseline(command.Context(), aoeval.CoreBaselineAuditRequest{SuitePaths: args, Root: options.root, BaselineDir: options.baselineDir})
		if err != nil {
			return err
		}
		if module.outputMode(command) == "json" {
			return writeJSON(command, report)
		}
		fmt.Fprintf(command.OutOrStdout(), "Eval baseline audit: %d suites, %d baselines, %d policy mismatches\n", report.SuiteCount, report.BaselineCount, report.PolicyMismatchCount)
		if len(report.MissingCompareBaselines) > 0 {
			fmt.Fprintf(command.OutOrStdout(), "Missing compare baselines: %d\n", len(report.MissingCompareBaselines))
		}
		if len(report.UnexpectedBaselinesForNone) > 0 {
			fmt.Fprintf(command.OutOrStdout(), "Unexpected baselines for none policy: %d\n", len(report.UnexpectedBaselinesForNone))
		}
		if len(report.OrphanBaselines) > 0 {
			fmt.Fprintf(command.OutOrStdout(), "Orphan baselines: %d\n", len(report.OrphanBaselines))
		}
		if len(report.StaleSuiteHashes) > 0 {
			fmt.Fprintf(command.OutOrStdout(), "Stale suite hashes: %d\n", len(report.StaleSuiteHashes))
		}
		return nil
	}
	return command
}

func (module Module) scorecardCommand() *cobra.Command {
	options := scorecardOptions{kind: string(aoeval.ScorecardKindRPI)}
	command := &cobra.Command{Use: "scorecard <candidate-run.json> [baseline-run.json]", Short: "Build an eval scorecard from run records", Args: cobra.RangeArgs(1, 2)}
	command.Flags().StringVar(&options.output, "out", "", "write scorecard JSON to path")
	command.Flags().StringVar(&options.kind, "kind", options.kind, "scorecard kind (rpi, skill-change)")
	command.Flags().Float64Var(&options.maxCategory, "max-category-regression", 0, "allowed per-category regression before verdict becomes regression")
	_ = command.RegisterFlagCompletionFunc("kind", staticCompletion(string(aoeval.ScorecardKindRPI), string(aoeval.ScorecardKindSkillChange)))
	command.RunE = func(command *cobra.Command, args []string) error {
		baselinePath := ""
		if len(args) == 2 {
			baselinePath = args[1]
		}
		card, err := module.useCases.Core.Scorecard(command.Context(), aoeval.CoreScorecardRequest{CandidatePath: args[0], BaselinePath: baselinePath, OutputPath: options.output, Kind: options.kind, MaxCategoryRegression: options.maxCategory})
		if err != nil {
			return err
		}
		if module.outputMode(command) == "json" {
			return writeJSON(command, card)
		}
		fmt.Fprintf(command.OutOrStdout(), "Eval scorecard %s: %s (%s, categories %d)\n", card.CandidateRunID, card.Verdict, card.Kind, len(card.Categories))
		if options.output != "" {
			fmt.Fprintf(command.OutOrStdout(), "Scorecard: %s\n", options.output)
		}
		return nil
	}
	return command
}

func (module Module) coverageCommand() *cobra.Command {
	options := coverageOptions{root: "evals/agentops-core", domains: append([]string(nil), aoeval.DefaultCoverageDomains...), dimensions: append([]string(nil), aoeval.DefaultCoverageDimensions...), runtimes: append([]string(nil), aoeval.DefaultCoverageRuntimes...)}
	command := &cobra.Command{Use: "coverage [suite.json ...]", Short: "Summarize eval suite coverage", Args: cobra.ArbitraryArgs}
	command.Flags().StringVar(&options.root, "root", options.root, "suite root to scan when no suite paths are provided")
	command.Flags().StringArrayVar(&options.domains, "require-domain", options.domains, "required product domain for missing-domain reporting")
	command.Flags().StringArrayVar(&options.evidenceKinds, "require-evidence-kind", nil, "required evidence kind for missing-evidence-kind reporting")
	command.Flags().StringArrayVar(&options.dimensions, "require-dimension", options.dimensions, "required score dimension for missing-dimension reporting")
	command.Flags().StringArrayVar(&options.runtimes, "require-runtime", options.runtimes, "required deterministic runtime for missing-runtime reporting")
	command.RunE = func(command *cobra.Command, args []string) error {
		report, err := module.useCases.Core.Coverage(command.Context(), aoeval.CoreCoverageRequest{SuitePaths: args, Root: options.root, RequiredDomains: options.domains, RequiredEvidenceKinds: options.evidenceKinds, RequiredDimensions: options.dimensions, RequiredRuntimes: options.runtimes})
		if err != nil {
			return err
		}
		if module.outputMode(command) == "json" {
			return writeJSON(command, report)
		}
		fmt.Fprintf(command.OutOrStdout(), "Eval coverage: %d suites, %d cases, %d critical cases\n", report.SuiteCount, report.CaseCount, report.CriticalCaseCount)
		renderCoverage(command, "domains", report.MissingRequiredDomains, report.RequiredDomains)
		renderCoverage(command, "evidence kinds", report.MissingRequiredEvidenceKinds, report.RequiredEvidenceKinds)
		renderCoverage(command, "dimensions", report.MissingRequiredDimensions, report.RequiredDimensions)
		renderCoverage(command, "runtimes", report.MissingRequiredRuntimes, report.RequiredRuntimes)
		return nil
	}
	return command
}

func (module Module) cleanupCommand() *cobra.Command {
	options := aoeval.CleanupRequest{TmpAgeSeconds: 60}
	command := &cobra.Command{
		Use: "cleanup", Short: "Recover stale Runs (state transitions, --delete, --tmp-files)",
		Long: `Per SCHEMA.md §4 cleanup state-transition rule (rc2):

  Stale pending  (no running transition within 60s)
                                       -> aborted (retraction_reason=never_started)
  Stale running  (no heartbeat within 5min OR Inspect process not alive)
                                       -> failed   (retraction_reason=orphaned_process)

After transitions:
  --delete       Remove Run dirs whose status is failed OR aborted (NEVER retracted).
  --tmp-files    Sweep orphan manifest.json.tmp left from rename-step crashes.
  --dry-run      Print what would be done; no mutations.

The cleanup procedure honors the §4 atomic-write contract on every state
transition (temp + fsync + rename + fsync-parent-dir). Retracted Runs are
never auto-removed — retraction is an audit trail.`,
	}
	command.Flags().BoolVar(&options.Delete, "delete", false, "Remove Run directories whose status is failed or aborted (never retracted)")
	command.Flags().BoolVar(&options.TmpFiles, "tmp-files", false, "Sweep orphan *.tmp files older than --tmp-age")
	command.Flags().Int64Var(&options.TmpAgeSeconds, "tmp-age", 60, "Minimum tmp-file age in seconds before sweep (0 = sweep all)")
	command.Flags().BoolVar(&options.DryRun, "dry-run", false, "Preview without mutations")
	command.RunE = func(command *cobra.Command, _ []string) error {
		report, err := module.useCases.Cleanup.Execute(command.Context(), options)
		if err != nil {
			return err
		}
		if module.outputMode(command) == "json" {
			return writeJSON(command, report)
		}
		fmt.Fprintf(command.OutOrStdout(), "Eval cleanup:\n  transitions->aborted: %d\n  transitions->failed:  %d\n  runs deleted:         %d\n  tmp files swept:      %d\n", report.TransitionsAborted, report.TransitionsFailed, report.RunsDeleted, report.TmpFilesSwept)
		verbose := module.host.Verbose != nil && module.host.Verbose(command)
		if len(report.Touched) > 0 && verbose {
			fmt.Fprintln(command.OutOrStdout(), "Touched:")
			for _, touched := range report.Touched {
				fmt.Fprintf(command.OutOrStdout(), "  %s\n", touched)
			}
		}
		return nil
	}
	return command
}

func (module Module) renderRun(command *cobra.Command, result aoeval.CoreRunResult) error {
	var value any = result.Run
	if result.Mode == aoeval.CoreRunBaselineAB {
		value = result.Delta
	}
	if result.Mode == aoeval.CoreRunContextAB {
		value = result.ContextDelta
	}
	if module.outputMode(command) == "json" {
		return writeJSON(command, value)
	}
	switch result.Mode {
	case aoeval.CoreRunContextAB:
		card := result.ContextDelta
		fmt.Fprintf(command.OutOrStdout(), "Eval context-AB %s: context-off=%.4f (%s) context-on=%.4f (%s) delta=%+.4f cases=%d\n", card.SuiteID, result.FirstRun.AggregateScore, result.FirstRun.Status, result.SecondRun.AggregateScore, result.SecondRun.Status, card.AggregateDelta, len(card.PerCase))
		if result.OutputPath != "" {
			fmt.Fprintf(command.OutOrStdout(), "Context delta scorecard: %s\n", result.OutputPath)
		}
	case aoeval.CoreRunBaselineAB:
		card := result.Delta
		fmt.Fprintf(command.OutOrStdout(), "Eval baseline-AB %s: skill-on=%.4f (%s) skill-off=%.4f (%s) delta=%+.4f cases=%d\n", card.SuiteID, result.FirstRun.AggregateScore, result.FirstRun.Status, result.SecondRun.AggregateScore, result.SecondRun.Status, card.AggregateDelta, len(card.PerCase))
		if result.OutputPath != "" {
			fmt.Fprintf(command.OutOrStdout(), "Delta scorecard: %s\n", result.OutputPath)
		}
	default:
		run := result.Run
		fmt.Fprintf(command.OutOrStdout(), "Eval %s: %s (aggregate %.4f, cases %d)\n", run.RunID, run.Status, run.AggregateScore, len(run.CaseResults))
		if result.OutputPath != "" {
			fmt.Fprintf(command.OutOrStdout(), "Run record: %s\n", result.OutputPath)
		}
	}
	return nil
}

func renderCoverage(command *cobra.Command, label string, missing, required []string) {
	if len(missing) > 0 {
		fmt.Fprintf(command.OutOrStdout(), "Missing required %s: %s\n", label, strings.Join(missing, ", "))
		return
	}
	if len(required) > 0 {
		fmt.Fprintf(command.OutOrStdout(), "Required %s covered\n", label)
	}
}

func writeJSON(command *cobra.Command, value any) error {
	encoder := json.NewEncoder(command.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func staticCompletion(values ...string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return append([]string(nil), values...), cobra.ShellCompDirectiveNoFileComp
	}
}
