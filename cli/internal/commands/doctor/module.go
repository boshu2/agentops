// Package doctor owns Cobra presentation for the doctor command family.
package doctor

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	doctorapp "github.com/boshu2/agentops/cli/internal/doctor"
	"github.com/boshu2/agentops/cli/internal/quality"
)

type ReadUseCases interface {
	Diagnose(context.Context, doctorapp.ReadRequest) (*doctorapp.Report, error)
	Triage(context.Context, doctorapp.ReadRequest) (*doctorapp.RobotTriageResult, *doctorapp.Report, error)
	Explain(context.Context, string) (*doctorapp.Finding, error)
	Capabilities(context.Context) *doctorapp.Capabilities
	Health(context.Context) (string, *doctorapp.HealthResult, error)
	RobotDocs(context.Context) string
	List(context.Context) ([]doctorapp.RunSummary, error)
	Diff(context.Context, doctorapp.ReadRequest) (*doctorapp.Report, error)
}

type MutationUseCases interface {
	Fix(context.Context, doctorapp.MutationRequest) (*doctorapp.Report, error)
}

type MaintenanceUseCases interface {
	Undo(context.Context, doctorapp.UndoRequest) (*doctorapp.UndoResult, error)
	GC(context.Context, doctorapp.GCRequest) (doctorapp.GCResult, error)
}

type UseCases struct {
	LegacyChecks  func(context.Context) []quality.Check
	Read          ReadUseCases
	Mutation      MutationUseCases
	Maintenance   MaintenanceUseCases
	DetectorCount func() int
}

type Module struct {
	useCases UseCases
	host     clicontract.HostOptions
}

func NewModule(useCases UseCases, host clicontract.HostOptions) Module {
	return Module{useCases: useCases, host: host}
}

func (Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{
		ID: "ao.doctor",
		Profiles: clicontract.ProfileDefault | clicontract.ProfileFlywheel |
			clicontract.ProfileLegacy | clicontract.ProfileCombined,
		Args:    clicontract.ArgsPolicy{Name: "no-args", Validate: cobra.NoArgs},
		Output:  clicontract.OutputNone,
		Effects: clicontract.EffectFilesystem | clicontract.EffectProcess | clicontract.EffectNetwork | clicontract.EffectEnvironment | clicontract.EffectClock,
		ExitClasses: map[int]clicontract.ExitClass{
			0: clicontract.ExitSuccess,
			1: clicontract.ExitFailure,
		},
	}
}

type rootOptions struct {
	json, fix, dryRun, online, quick, robot, robotTriage bool
	only, skip                                           []string
	since, severity, explain                             string
}

type undoOptions struct{ strict, dryRun bool }
type gcOptions struct {
	before string
	yes    bool
}

func (module Module) Command() *cobra.Command {
	var options rootOptions
	command := &cobra.Command{
		Use: "doctor", Short: "Check AgentOps health", Args: cobra.NoArgs,
		Long: `Run health checks on your AgentOps installation.

The default check is intentionally small: CLI identity, source-skill links,
binary freshness, optional provenance integrity, and host safety. It does not
probe trackers, reviewers, plugin caches, search indexes, or operating-loop
state. Advanced failure-mode diagnostics run only when explicitly selected.

Examples:
  ao doctor
  ao doctor --json`,
	}
	flags := command.Flags()
	flags.BoolVar(&options.json, "json", false, "Output results as JSON")
	flags.BoolVar(&options.fix, "fix", false, "Apply fixers for findings (routes through mutate())")
	flags.BoolVar(&options.dryRun, "dry-run", false, "With --fix: print the plan, change nothing")
	flags.StringSliceVar(&options.only, "only", nil, "Scope to a subset of detectors or subsystems")
	flags.StringSliceVar(&options.skip, "skip", nil, "Inverse of --only")
	flags.StringVar(&options.since, "since", "", "Diff findings against an earlier run")
	flags.BoolVar(&options.online, "online", false, "Enable network probes (default: offline-only)")
	flags.BoolVar(&options.quick, "quick", false, "Run only fast-path detectors (< 200ms)")
	flags.StringVar(&options.severity, "severity", "P3", "Minimum severity to emit (P0|P1|P2|P3)")
	flags.BoolVar(&options.robot, "robot", false, "Alias for --json with structured wrapper")
	flags.BoolVar(&options.robotTriage, "robot-triage", false, "Emit the mega-command triage JSON")
	flags.StringVar(&options.explain, "explain", "", "Expand a single finding by id")
	command.RunE = func(command *cobra.Command, _ []string) error { return module.runRoot(command, options) }
	command.AddCommand(
		module.fixCommand(&options), module.undoCommand(&options), module.explainCommand(&options),
		module.capabilitiesCommand(), module.healthCommand(&options), module.robotDocsCommand(),
		module.gcCommand(&options), module.listCommand(&options), module.diffCommand(&options),
	)
	command.SilenceErrors = true
	command.SilenceUsage = true
	for _, child := range command.Commands() {
		child.SilenceErrors = true
	}
	if module.host.EnrichFlagErr != nil {
		command.SetFlagErrorFunc(func(command *cobra.Command, err error) error {
			err = module.host.EnrichFlagErr(command, err)
			fmt.Fprintln(command.ErrOrStderr(), "Error:", err)
			return err
		})
	}
	return command
}

func (module Module) wantsJSON(_ *cobra.Command, options *rootOptions) bool {
	return options.json || options.robot || module.outputMode() == "json" || module.outputMode() == "yaml"
}

// emitStructured writes a module-built report as JSON, or as YAML when the
// negotiated global output mode is yaml. The doctor render paths reach here only
// after wantsJSON already selected structured output; the local --json and
// --robot flags leave outputMode() at "table"/"json", so YAML is chosen only for
// an explicit global `-o yaml` — never for a robot/local-json json contract.
func (module Module) emitStructured(command *cobra.Command, value any) error {
	if module.outputMode() == "yaml" {
		if err := clicontract.WriteYAML(command.OutOrStdout(), value); err != nil {
			return exit(doctorapp.ExitIOError, err.Error())
		}
		return nil
	}
	return writeJSON(command, value)
}

func (module Module) outputMode() string {
	if module.host.OutputMode == nil {
		return ""
	}
	return module.host.OutputMode()
}

func (module Module) dryRun() bool {
	return module.host.DryRun != nil && module.host.DryRun()
}

func (module Module) effectiveDryRun(_ *cobra.Command, options *rootOptions, local bool) bool {
	return local || options.dryRun || module.dryRun()
}

func readRequest(options rootOptions, dryRun, jsonOutput bool) doctorapp.ReadRequest {
	return doctorapp.ReadRequest{Only: append([]string(nil), options.only...), Skip: append([]string(nil), options.skip...),
		Quick: options.quick, Online: options.online, Severity: options.severity,
		DryRun: dryRun, JSON: jsonOutput, Since: options.since}
}

func mutationRequest(options rootOptions, dryRun, jsonOutput bool) doctorapp.MutationRequest {
	return doctorapp.MutationRequest{Only: append([]string(nil), options.only...), Skip: append([]string(nil), options.skip...),
		Quick: options.quick, Online: options.online, Severity: options.severity, DryRun: dryRun, JSON: jsonOutput}
}

func (module Module) runRoot(command *cobra.Command, options rootOptions) error {
	jsonOutput := module.wantsJSON(command, &options)
	dryRun := module.effectiveDryRun(command, &options, false)
	request := readRequest(options, dryRun, jsonOutput)
	if options.explain != "" {
		return module.runExplain(command, options.explain, jsonOutput)
	}
	if options.robotTriage {
		triage, report, err := module.useCases.Read.Triage(command.Context(), request)
		if err != nil {
			return exit(doctorapp.ExitIOError, err.Error())
		}
		if err := writeJSON(command, triage); err != nil {
			return err
		}
		return resultExit(report.ExitCode, "doctor findings present")
	}
	if options.fix {
		return module.runFix(command, mutationRequest(options, dryRun, jsonOutput), jsonOutput)
	}
	if advancedDiagnosticsRequested(command) {
		report, err := module.useCases.Read.Diagnose(command.Context(), request)
		if err != nil {
			return exit(doctorapp.ExitIOError, err.Error())
		}
		if jsonOutput {
			if err := module.emitStructured(command, report); err != nil {
				return err
			}
		} else {
			renderFindings(command, report)
		}
		return resultExit(report.ExitCode, "doctor findings present")
	}
	checks := module.useCases.LegacyChecks(command.Context())
	if module.outputMode() == "yaml" {
		// quality.RunDoctor emits exactly one JSON document in JSON mode; capture
		// and re-emit as YAML so `ao doctor -o yaml` is the same data
		// yaml-marshaled, not a silent human-summary fallback.
		var buf bytes.Buffer
		if err := quality.RunDoctor(quality.DoctorOptions{JSON: true, Checks: checks, Stdout: &buf}); err != nil {
			return err
		}
		return clicontract.JSONToYAML(command.OutOrStdout(), buf.Bytes())
	}
	return quality.RunDoctor(quality.DoctorOptions{JSON: jsonOutput, Checks: checks, Stdout: command.OutOrStdout()})
}

func advancedDiagnosticsRequested(command *cobra.Command) bool {
	for _, name := range []string{"robot", "robot-triage", "explain", "online", "quick", "since", "severity", "only", "skip"} {
		if command.Flags().Changed(name) {
			return true
		}
	}
	return false
}

func exit(code int, message string) error {
	return &clicontract.ExitError{Code: code, Message: message, Label: "ao doctor"}
}
func resultExit(code int, message string) error {
	if code == doctorapp.ExitHealthy {
		return nil
	}
	return exit(code, message)
}

func writeJSON(command *cobra.Command, value any) error {
	if err := clicontract.WriteJSON(command.OutOrStdout(), value); err != nil {
		return exit(doctorapp.ExitIOError, err.Error())
	}
	return nil
}

func renderFindings(command *cobra.Command, report *doctorapp.Report) {
	if len(report.Findings) == 0 {
		return
	}
	w := command.OutOrStdout()
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Failure-mode findings (%d):\n", len(report.Findings))
	for _, finding := range report.Findings {
		fmt.Fprintf(w, "  [%s] %s — %s\n", finding.Severity, finding.ID, finding.Title)
	}
}

func (module Module) runFix(command *cobra.Command, request doctorapp.MutationRequest, jsonOutput bool) error {
	report, err := module.useCases.Mutation.Fix(command.Context(), request)
	if err != nil {
		return exit(doctorapp.ExitIOError, err.Error())
	}
	if jsonOutput {
		if err := module.emitStructured(command, report); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(command.OutOrStdout(), "ao doctor fix — run %s\n", report.RunID)
		fmt.Fprintf(command.OutOrStdout(), "  findings: %d  actions: %d\n", report.Summary.TotalFindings, report.ActionsTaken)
		if report.UndoCommand != "" {
			fmt.Fprintf(command.OutOrStdout(), "  undo: %s\n", report.UndoCommand)
		}
	}
	return resultExit(report.ExitCode, "doctor fix incomplete")
}

func (module Module) runExplain(command *cobra.Command, id string, jsonOutput bool) error {
	finding, err := module.useCases.Read.Explain(command.Context(), id)
	if err != nil {
		var runtimeFailure *doctorapp.RuntimeError
		if errors.As(err, &runtimeFailure) {
			return exit(doctorapp.ExitIOError, err.Error())
		}
		return exit(doctorapp.ExitNoInput, err.Error())
	}
	if jsonOutput {
		return module.emitStructured(command, finding)
	}
	renderExplain(command, finding)
	return nil
}

// renderExplain prints the full expansion of one finding — a superset of the
// per-finding fields robot-triage emits: identity, confidence, every evidence
// field, remediation, and fixability.
func renderExplain(command *cobra.Command, finding *doctorapp.Finding) {
	w := command.OutOrStdout()
	fmt.Fprintf(w, "%s [%s] (%s)\n%s\n", finding.ID, finding.Severity, finding.Subsystem, finding.Title)
	fmt.Fprintf(w, "Confidence: %.2f\n", finding.Confidence)
	fmt.Fprintln(w, "Evidence:")
	evidence := finding.Evidence
	if evidence.File == "" && len(evidence.Lines) == 0 && evidence.Query == "" && evidence.Hash == "" {
		fmt.Fprintln(w, "  (none recorded)")
	}
	if evidence.File != "" {
		fmt.Fprintf(w, "  file: %s\n", evidence.File)
	}
	if len(evidence.Lines) > 0 {
		fmt.Fprintf(w, "  lines: %v\n", evidence.Lines)
	}
	if evidence.Query != "" {
		fmt.Fprintf(w, "  query: %s\n", evidence.Query)
	}
	if evidence.Hash != "" {
		fmt.Fprintf(w, "  hash: %s\n", evidence.Hash)
	}
	fmt.Fprintf(w, "Remediation: %s\n", finding.Remediation.Command)
	fmt.Fprintf(w, "Auto-fixable: %t (estimated actions: %d)\n",
		finding.Remediation.AutoFixable, finding.Remediation.EstimatedActions)
	if finding.Remediation.ExplainCommand != "" {
		fmt.Fprintf(w, "Explain: %s\n", finding.Remediation.ExplainCommand)
	}
}

func (module Module) fixCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "fix", Short: "Run detectors, then apply fixers (backs up before every mutation)", RunE: func(command *cobra.Command, _ []string) error {
		jsonOutput := module.wantsJSON(command, options)
		dryRun := module.effectiveDryRun(command, options, false)
		return module.runFix(command, mutationRequest(*options, dryRun, jsonOutput), jsonOutput)
	}, Args: cobra.NoArgs}
}

func (module Module) undoCommand(options *rootOptions) *cobra.Command {
	local := undoOptions{strict: true}
	command := &cobra.Command{Use: "undo <run-id>", Short: "Restore from .doctor/runs/<run-id>/backups/ (run-id may be 'latest')", Args: cobra.ExactArgs(1)}
	command.Flags().BoolVar(&local.strict, "strict", true, "Refuse if any backup is missing or hash-mismatched")
	command.Flags().BoolVar(&local.dryRun, "dry-run", false, "Print the restore plan; do not execute")
	command.RunE = func(command *cobra.Command, args []string) error {
		jsonOutput := module.wantsJSON(command, options)
		request := doctorapp.UndoRequest{RunID: args[0], Strict: local.strict, DryRun: module.effectiveDryRun(command, options, local.dryRun)}
		result, err := module.useCases.Maintenance.Undo(command.Context(), request)
		if err != nil {
			if jsonOutput {
				_ = module.emitStructured(command, map[string]any{"run_id": args[0], "exit_code": doctorapp.ExitIOError, "error": err.Error()})
			} else {
				fmt.Fprintf(command.ErrOrStderr(), "undo failed: %v\n", err)
			}
			return exit(doctorapp.ExitIOError, err.Error())
		}
		if jsonOutput {
			return module.emitStructured(command, result)
		}
		fmt.Fprintf(command.OutOrStdout(), "undo %s: restored=%d skipped=%d\n", result.RunID, result.Restored, result.Skipped)
		return nil
	}
	return command
}

func (module Module) explainCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "explain <finding-id>", Short: "Expand a single finding with full evidence", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		return module.runExplain(command, args[0], module.wantsJSON(command, options))
	}}
}

func (module Module) capabilitiesCommand() *cobra.Command {
	return &cobra.Command{Use: "capabilities", Short: "Print the machine-readable doctor contract (JSON)", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		return module.emitStructured(command, module.useCases.Read.Capabilities(command.Context()))
	}}
}

func (module Module) healthCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "health", Short: "Cheap one-line liveness summary", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		line, result, err := module.useCases.Read.Health(command.Context())
		if err != nil {
			return exit(doctorapp.ExitIOError, err.Error())
		}
		if module.wantsJSON(command, options) {
			if err := module.emitStructured(command, result); err != nil {
				return err
			}
		} else {
			fmt.Fprintln(command.OutOrStdout(), line)
		}
		return resultExit(result.ExitCode, "doctor health: not ok")
	}}
}

func (module Module) robotDocsCommand() *cobra.Command {
	return &cobra.Command{Use: "robot-docs", Short: "Print the paste-ready agent handbook (Markdown)", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		_, err := fmt.Fprint(command.OutOrStdout(), module.useCases.Read.RobotDocs(command.Context()))
		return err
	}}
}

func (module Module) gcCommand(options *rootOptions) *cobra.Command {
	var local gcOptions
	command := &cobra.Command{Use: "gc", Short: "Prune old runs (requires --yes and --before <date>)", Args: cobra.NoArgs}
	command.Flags().StringVar(&local.before, "before", "", "Prune runs started before this date (YYYY-MM-DD)")
	command.Flags().BoolVar(&local.yes, "yes", false, "Confirm pruning (required)")
	command.RunE = func(command *cobra.Command, _ []string) error {
		result, err := module.useCases.Maintenance.GC(command.Context(), doctorapp.GCRequest{Before: local.before, Yes: local.yes, DryRun: module.effectiveDryRun(command, options, false)})
		if err != nil {
			var usageFailure *doctorapp.UsageError
			if errors.As(err, &usageFailure) {
				return exit(doctorapp.ExitUsage, err.Error())
			}
			return exit(doctorapp.ExitIOError, err.Error())
		}
		if module.wantsJSON(command, options) {
			return module.emitStructured(command, result)
		}
		if result.DryRun {
			fmt.Fprintf(command.OutOrStdout(), "would prune %d run(s)\n", result.Matched)
		} else {
			fmt.Fprintf(command.OutOrStdout(), "pruned %d run(s)\n", result.Pruned)
		}
		return nil
	}
	return command
}

func (module Module) listCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "ls", Short: "List runs in .doctor/runs/", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		runs, err := module.useCases.Read.List(command.Context())
		if err != nil {
			return exit(doctorapp.ExitIOError, err.Error())
		}
		if module.wantsJSON(command, options) {
			return module.emitStructured(command, map[string]any{"runs": runs})
		}
		if len(runs) == 0 {
			fmt.Fprintln(command.OutOrStdout(), "no doctor runs")
			return nil
		}
		for _, run := range runs {
			fmt.Fprintf(command.OutOrStdout(), "%s  exit=%d  actions=%d\n", run.RunID, run.ExitCode, run.ActionCount)
		}
		return nil
	}}
}

func (module Module) diffCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "diff", Short: "Show what --fix would change (read-only)", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		jsonOutput := module.wantsJSON(command, options)
		report, err := module.useCases.Read.Diff(command.Context(), readRequest(*options, true, jsonOutput))
		if err != nil {
			return exit(doctorapp.ExitIOError, err.Error())
		}
		if jsonOutput {
			return module.emitStructured(command, report)
		}
		renderFixPlan(command, report)
		// Exit code stays 0 with findings present: diff is a read-only preview
		// of the fix plan, not a health verdict — `ao doctor` / `ao doctor
		// health` own the failing exit for findings.
		return nil
	}}
}

// renderFixPlan renders the diff output as an explicit fix PLAN: for each
// finding it states whether --fix would act (and how many actions) or that
// only a manual action exists. This keeps `ao doctor diff` distinct from the
// plain findings list triage renders.
func renderFixPlan(command *cobra.Command, report *doctorapp.Report) {
	w := command.OutOrStdout()
	if len(report.Findings) == 0 {
		fmt.Fprintln(w, "clean diff: --fix would change nothing")
		return
	}
	fmt.Fprintf(w, "Fix plan — what --fix would do (%d finding(s), read-only preview):\n", len(report.Findings))
	for _, finding := range report.Findings {
		fmt.Fprintf(w, "  [%s] %s — %s\n", finding.Severity, finding.ID, finding.Title)
		if finding.Remediation.AutoFixable {
			fmt.Fprintf(w, "      would auto-fix: %d estimated action(s) via %s\n",
				finding.Remediation.EstimatedActions, finding.Remediation.Command)
		} else {
			fmt.Fprintf(w, "      no automatic fix; manual action: %s\n", finding.Remediation.Command)
		}
	}
}
