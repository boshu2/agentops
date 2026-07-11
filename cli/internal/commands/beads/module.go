// Package beads owns the Cobra presentation for the beads command family.
// Behavior is delegated through Runner so tracker, filesystem, process, clock,
// and environment effects remain behind driven adapters.
package beads

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	beadsapp "github.com/boshu2/agentops/cli/internal/beads"
	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/epicstatus"
)

type Operation string

const (
	OperationDir              Operation = "dir"
	OperationTracker          Operation = "tracker"
	OperationVerify           Operation = "verify"
	OperationLint             Operation = "lint"
	OperationHarvest          Operation = "harvest"
	OperationAudit            Operation = "audit"
	OperationCluster          Operation = "cluster"
	OperationExec             Operation = "exec"
	OperationResume           Operation = "resume"
	OperationScenariosExtract Operation = "scenarios.extract"
	OperationScenariosCheck   Operation = "scenarios.validate"
	OperationStaleClaims      Operation = "stale-claims"
	OperationEpicStatus       Operation = "epic-status"
	OperationAcceptance       Operation = "verify-acceptance"
)

type Invocation struct {
	Operation Operation
	Args      []string
	Options   Options
}

type Options struct {
	JSON            bool
	Require         bool
	Verbose         bool
	Status          string
	OutputDirectory string
	DryRun          bool
	Strict          bool
	AutoClose       bool
	Apply           bool
	Agent           string
	Ledger          string
	Force           bool
	Write           bool
	ThresholdHours  float64
	Terminal        bool
}

// Runner is the command family's inbound application port. The command module
// owns parsing; the implementation owns use-case orchestration and effects.
type Runner interface {
	Run(*cobra.Command, Invocation) error
}

type Module struct {
	runner    Runner
	resolver  beadsapp.TrackerResolver
	inspector beadsapp.LedgerInspector
	executor  beadsapp.TrackerExecutor
	stale     beadsapp.StaleSource
	claims    beadsapp.ClaimStore
	runtime   beadsapp.ResumeRuntime
	reader    beadsapp.LedgerReader
}

func NewModule(
	runner Runner,
	resolver beadsapp.TrackerResolver,
	inspector beadsapp.LedgerInspector,
	executor beadsapp.TrackerExecutor,
	stale beadsapp.StaleSource,
	claims beadsapp.ClaimStore,
	runtime beadsapp.ResumeRuntime,
	reader beadsapp.LedgerReader,
) Module {
	return Module{
		runner: runner, resolver: resolver, inspector: inspector, executor: executor,
		stale: stale, claims: claims, runtime: runtime, reader: reader,
	}
}

func (Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{
		ID: "ao.beads",
		Profiles: clicontract.ProfileDefault |
			clicontract.ProfileFlywheel |
			clicontract.ProfileLegacy |
			clicontract.ProfileCombined,
		Args:        clicontract.ArgsPolicy{Name: "subcommands-only", Validate: cobra.NoArgs},
		Output:      clicontract.OutputNone,
		Effects:     clicontract.EffectPure,
		ExitClasses: map[int]clicontract.ExitClass{0: clicontract.ExitSuccess, 1: clicontract.ExitFailure},
	}
}

func (module Module) Command() *cobra.Command {
	root := &cobra.Command{
		Use:   "beads",
		Short: "Complementary tooling for the bd (beads) issue tracker",
		Args:  cobra.NoArgs,
		Long: `Commands that help maintain the bd issue tracker alongside the main
bd CLI. These tools focus on catching stale descriptions before a new
session acts on them and harvesting closure reasons into durable learnings.

None of these commands replace bd itself — they complement it.`,
	}
	root.AddCommand(
		module.dirCommand(),
		module.trackerCommand(),
		module.verifyCommand(),
		module.lintCommand(),
		module.harvestCommand(),
		module.auditCommand(),
		module.clusterCommand(),
		module.execCommand(),
		module.resumeCommand(),
		module.scenariosCommand(),
		module.staleCommand(),
		module.epicStatusCommand(),
		module.acceptanceCommand(),
	)
	return root
}

func (module Module) invoke(command *cobra.Command, operation Operation, args []string, options Options) error {
	if module.runner == nil {
		return fmt.Errorf("beads command runner is not configured")
	}
	return module.runner.Run(command, Invocation{Operation: operation, Args: append([]string(nil), args...), Options: options})
}

func (module Module) dirCommand() *cobra.Command {
	var options Options
	command := &cobra.Command{Use: "dir", Short: "Print the resolved live br ledger directory", Args: cobra.NoArgs}
	command.Flags().BoolVar(&options.JSON, "json", false, "Emit {beads_dir, source} as JSON")
	command.Flags().BoolVar(&options.Require, "require", false, "Fail closed: exit non-zero (printing nothing to stdout) unless the resolved directory holds a br ledger")
	command.RunE = func(command *cobra.Command, _ []string) error {
		return module.runDir(command, options)
	}
	return command
}

func (module Module) trackerCommand() *cobra.Command {
	var options Options
	command := &cobra.Command{Use: "tracker", Short: "Print the resolved beads tracker (bd or br) for this environment", Args: cobra.NoArgs}
	command.Flags().BoolVar(&options.JSON, "json", false, "Emit {tracker, binary, ledger_dir, source} as JSON")
	command.RunE = func(command *cobra.Command, _ []string) error {
		return module.runTracker(command, options)
	}
	return command
}

func (module Module) verifyCommand() *cobra.Command {
	var options Options
	command := &cobra.Command{Use: "verify <bead-id>", Short: "Detect stale citations in a bead description (files, functions, symbols)", Args: cobra.ExactArgs(1)}
	command.Flags().BoolVar(&options.JSON, "json", false, "Emit verification report as JSON instead of human-readable text")
	command.Flags().BoolVar(&options.Verbose, "verbose", false, "Include FRESH citations in the output (default: stale only)")
	command.RunE = func(command *cobra.Command, args []string) error {
		return module.invoke(command, OperationVerify, args, options)
	}
	return command
}

func (module Module) lintCommand() *cobra.Command {
	var options Options
	command := &cobra.Command{Use: "lint", Short: "Batch-verify every open bead (or filtered set) against HEAD"}
	command.Flags().StringVar(&options.Status, "status", "open", "bd status filter (open, closed, all)")
	command.Flags().BoolVar(&options.JSON, "json", false, "Emit lint report as JSON")
	command.RunE = func(command *cobra.Command, args []string) error {
		return module.invoke(command, OperationLint, args, options)
	}
	return command
}

func (module Module) harvestCommand() *cobra.Command {
	var options Options
	command := &cobra.Command{Use: "harvest <bead-id>", Short: "Materialize a closed bead's reason as a structured learning file", Args: cobra.ExactArgs(1)}
	command.Flags().StringVar(&options.OutputDirectory, "out-dir", ".agents/learnings", "Directory to write the learning file into")
	command.Flags().BoolVar(&options.DryRun, "dry-run", false, "Print the learning content to stdout without writing a file")
	command.RunE = func(command *cobra.Command, args []string) error {
		return module.invoke(command, OperationHarvest, args, options)
	}
	return command
}

func (module Module) auditCommand() *cobra.Command {
	var options Options
	command := &cobra.Command{Use: "audit", Short: "Audit open beads for likely-fixed, stale, or consolidatable work"}
	command.Flags().BoolVar(&options.JSON, "json", false, "Emit audit report as JSON")
	command.Flags().BoolVar(&options.Strict, "strict", false, "Exit 1 when any likely-fixed, likely-stale, or consolidatable bead is found")
	command.Flags().BoolVar(&options.AutoClose, "auto-close", false, "Close likely-fixed beads when commit or file-change evidence is found")
	command.RunE = func(command *cobra.Command, args []string) error {
		return module.invoke(command, OperationAudit, args, options)
	}
	return command
}

func (module Module) clusterCommand() *cobra.Command {
	var options Options
	command := &cobra.Command{Use: "cluster", Short: "Suggest consolidation clusters for overlapping open beads"}
	command.Flags().BoolVar(&options.JSON, "json", false, "Emit cluster report as JSON")
	command.Flags().BoolVar(&options.Apply, "apply", false, "Reparent non-representative beads under the cluster representative")
	command.RunE = func(command *cobra.Command, args []string) error {
		return module.invoke(command, OperationCluster, args, options)
	}
	return command
}

func (module Module) execCommand() *cobra.Command {
	command := &cobra.Command{
		Use:                "exec [args...]",
		Short:              "Run a bead CRUD command against whichever tracker (bd or br) this environment uses",
		DisableFlagParsing: true,
	}
	command.RunE = func(command *cobra.Command, args []string) error {
		return module.runExec(command, args)
	}
	return command
}

func (module Module) runDir(command *cobra.Command, options Options) error {
	if module.resolver == nil || module.inspector == nil {
		return fmt.Errorf("beads directory ports are not configured")
	}
	if !module.resolver.BeadsDirOverride() {
		if resolved, err := module.resolver.Resolve(); err == nil && resolved.Tracker == beadsapp.TrackerBD {
			if options.Require {
				if reason := beadsapp.LedgerMissing(beadsapp.TrackerBD, module.inspector.InspectLedger(resolved.LedgerDir)); reason != "" {
					return fmt.Errorf("beads dir --require: %s (resolved %s for tracker bd via %s); refusing to print a path a bd write could silently fall back from", reason, resolved.LedgerDir, resolved.Source)
				}
			}
			return writeDirectory(command, options.JSON, resolved.LedgerDir, resolved.Source)
		}
	}
	resolved, err := module.resolver.BRLedger()
	if err != nil {
		return err
	}
	if options.Require {
		if reason := beadsapp.LedgerMissing(beadsapp.TrackerBR, module.inspector.InspectLedger(resolved.Path)); reason != "" {
			return fmt.Errorf("beads dir --require: %s (resolved %s via %s); refusing to print a path a br write could silently fall back from", reason, resolved.Path, resolved.Source)
		}
	}
	return writeDirectory(command, options.JSON, resolved.Path, resolved.Source)
}

func writeDirectory(command *cobra.Command, asJSON bool, path, source string) error {
	if asJSON {
		return json.NewEncoder(command.OutOrStdout()).Encode(map[string]string{"beads_dir": path, "source": source})
	}
	_, err := fmt.Fprintln(command.OutOrStdout(), path)
	return err
}

func (module Module) runTracker(command *cobra.Command, options Options) error {
	if module.resolver == nil {
		return fmt.Errorf("beads tracker resolver is not configured")
	}
	resolved, err := module.resolver.Resolve()
	if err != nil {
		return err
	}
	if options.JSON {
		return json.NewEncoder(command.OutOrStdout()).Encode(resolved)
	}
	output := command.OutOrStdout()
	fmt.Fprintf(output, "tracker     %s\n", resolved.Tracker)
	fmt.Fprintf(output, "binary      %s\n", resolved.Binary)
	fmt.Fprintf(output, "ledger_dir  %s\n", resolved.LedgerDir)
	fmt.Fprintf(output, "source      %s\n", resolved.Source)
	return nil
}

func (module Module) runExec(command *cobra.Command, args []string) error {
	for _, argument := range args {
		if argument == "--help" || argument == "-h" {
			return command.Help()
		}
	}
	if module.executor == nil {
		return fmt.Errorf("beads tracker executor is not configured")
	}
	err := module.executor.Execute(context.Background(), args, beadsapp.ExecStreams{
		Stdin: command.InOrStdin(), Stdout: command.OutOrStdout(), Stderr: command.ErrOrStderr(),
	})
	var exitError interface{ ExitCode() int }
	if errors.As(err, &exitError) {
		command.SilenceUsage = true
		command.SilenceErrors = true
	}
	return err
}

func (module Module) resumeCommand() *cobra.Command {
	var options Options
	command := &cobra.Command{Use: "resume <bead-id>", Short: "Atomically transfer an in_progress claim from a stale agent to this one", Args: cobra.ExactArgs(1)}
	command.Flags().StringVar(&options.Agent, "agent", "", "New claimant id (defaults to BEADS_ACTOR env var, else ao-beads-resume).")
	command.Flags().StringVar(&options.Ledger, "ledger", "docs/provenance/ledger.jsonl", "Path to the provenance ledger (relative to repo root).")
	command.Flags().BoolVar(&options.JSON, "json", false, "Emit the claim_transferred event to stdout (always written to ledger).")
	command.RunE = func(command *cobra.Command, args []string) error {
		return module.runResume(command, args, options)
	}
	return command
}

func (module Module) scenariosCommand() *cobra.Command {
	root := &cobra.Command{Use: "scenarios", Short: "[DEPRECATED — use 'ao beads verify-acceptance'] Convert bead acceptance criteria into Gherkin scenarios", Args: cobra.NoArgs}
	var extract Options
	extractCommand := &cobra.Command{Use: "extract <bead-id>", Short: "Print a candidate Gherkin '## Scenarios' block from a bead's acceptance (dry-run)", Args: cobra.ExactArgs(1)}
	extractCommand.Flags().BoolVar(&extract.JSON, "json", false, "Emit extracted scenarios as JSON (data on stdout) instead of a Gherkin block")
	extractCommand.Flags().BoolVar(&extract.Force, "force", false, "Extract even when the bead already has a '## Scenarios' block")
	extractCommand.Flags().BoolVar(&extract.Write, "write", false, "After printing the block and an operator y/N confirmation, append it to the bead via 'bd update'")
	extractCommand.RunE = func(command *cobra.Command, args []string) error {
		return module.invoke(command, OperationScenariosExtract, args, extract)
	}
	var check Options
	checkCommand := &cobra.Command{Use: "validate <bead-id>", Short: "Check that a bead's authored '## Scenarios' block is well-formed Gherkin", Args: cobra.ExactArgs(1)}
	checkCommand.Flags().BoolVar(&check.JSON, "json", false, "Emit a structured validation verdict as JSON on stdout")
	checkCommand.RunE = func(command *cobra.Command, args []string) error {
		return module.invoke(command, OperationScenariosCheck, args, check)
	}
	root.AddCommand(extractCommand, checkCommand)
	return root
}

func (module Module) staleCommand() *cobra.Command {
	var options Options
	command := &cobra.Command{Use: "stale-claims", Short: "List in_progress beads whose claim looks stale"}
	command.Flags().Float64Var(&options.ThresholdHours, "threshold", 4, "Staleness threshold in hours (claim updated more than N hours ago).")
	command.Flags().BoolVar(&options.JSON, "json", false, "Emit JSON array conforming to stale-claim-event.v1 (event_type: stale_detected).")
	command.RunE = func(command *cobra.Command, args []string) error {
		return module.runStale(command, options)
	}
	return command
}

func (module Module) epicStatusCommand() *cobra.Command {
	var options Options
	command := &cobra.Command{Use: "epic-status <epic-id>", Short: "Deterministic group-terminality verdict for an epic/wave", Args: cobra.ExactArgs(1)}
	command.Flags().BoolVar(&options.Terminal, "terminal", false, "Map the verdict to the process exit code (0 terminal / 2 not-terminal / 3 skipped).")
	command.Flags().BoolVar(&options.JSON, "json", false, "Emit the verdict as a JSON object instead of a human-readable line.")
	command.RunE = func(command *cobra.Command, args []string) error {
		return module.runEpicStatus(command, args, options)
	}
	return command
}

func (module Module) runStale(command *cobra.Command, options Options) error {
	if module.stale == nil || module.runtime == nil {
		return fmt.Errorf("beads stale-claims ports are not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	events, err := beadsapp.DetectStale(ctx, module.stale, module.runtime.Now(), options.ThresholdHours)
	if err != nil {
		return fmt.Errorf("br list: %w", err)
	}
	if options.JSON {
		encoded, err := json.Marshal(events)
		if err != nil {
			return fmt.Errorf("marshal events: %w", err)
		}
		_, err = fmt.Fprintln(command.OutOrStdout(), string(encoded))
		return err
	}
	if len(events) == 0 {
		_, err := fmt.Fprintf(command.OutOrStdout(), "ao beads stale-claims: none — all in_progress beads touched within %.1fh\n", options.ThresholdHours)
		return err
	}
	fmt.Fprintf(command.OutOrStdout(), "ao beads stale-claims: %d in_progress bead(s) stale (threshold %.1fh)\n", len(events), options.ThresholdHours)
	for _, event := range events {
		fmt.Fprintf(command.OutOrStdout(), "  %-22s claim_age=%.1fh last_touch=%s claimant=%s\n", event.BeadID, event.Evidence.ClaimAgeHours, event.Evidence.LastTouchTS, event.OriginalClaimant.ID)
	}
	return nil
}

func (module Module) runResume(command *cobra.Command, args []string, options Options) error {
	if module.claims == nil || module.runtime == nil {
		return fmt.Errorf("beads resume ports are not configured")
	}
	beadID := args[0]
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	prior, err := module.claims.Show(ctx, beadID)
	if err != nil {
		return fmt.Errorf("fetch prior state: %w", err)
	}
	if prior.Status != "in_progress" {
		return fmt.Errorf("bead %s is %q, not in_progress — resume only handles in_progress claims", beadID, prior.Status)
	}
	now := module.runtime.Now().UTC()
	agent := options.Agent
	if agent == "" {
		agent = module.runtime.Actor()
	}
	if agent == "" {
		agent = "ao-beads-resume"
	}
	if err := module.claims.Claim(ctx, beadID, agent); err != nil {
		return fmt.Errorf("claim transfer: %w", err)
	}
	posterior, err := module.claims.Show(ctx, beadID)
	if err != nil {
		posterior = beadsapp.StaleBeadRecord{ID: beadID, Status: "in_progress", Assignee: agent, UpdatedAt: now.Format(time.RFC3339)}
	}
	event := beadsapp.BuildTransferredEvent(beadID, agent, prior, posterior, now)
	ledger, err := module.runtime.ResolveRepoPath(options.Ledger)
	if err != nil {
		return err
	}
	if err := module.runtime.AppendEvent(ledger, event); err != nil {
		return fmt.Errorf("append ledger (claim already transferred): %w", err)
	}
	if options.JSON {
		encoded, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("marshal transferred claim: %w", err)
		}
		_, err = fmt.Fprintln(command.OutOrStdout(), string(encoded))
		return err
	}
	priorAgent := prior.Assignee
	if priorAgent == "" {
		priorAgent = "unknown"
	}
	_, err = fmt.Fprintf(command.OutOrStdout(), "ao beads resume: %s transferred from %q to %q (prior_rev=%s, new_rev=%s)\n", beadID, priorAgent, agent, event.Transfer.PriorRevision, event.Transfer.NewRevision)
	return err
}

func (module Module) runEpicStatus(command *cobra.Command, args []string, options Options) error {
	if module.resolver == nil || module.reader == nil {
		return fmt.Errorf("beads epic-status ports are not configured")
	}
	epic := strings.TrimSpace(args[0])
	ledger, err := module.resolver.BRLedger()
	if err != nil {
		return err
	}
	path := filepath.Join(ledger.Path, "issues.jsonl")
	raw, err := module.reader.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read ledger %s: %w", path, err)
	}
	records, err := beadsapp.ParseLedger(raw)
	if err != nil {
		return fmt.Errorf("parse ledger: %w", err)
	}
	members, present := beadsapp.BuildMembers(epic, records)
	if !present {
		return fmt.Errorf("epic %s not found in ledger %s", epic, ledger.Path)
	}
	result := epicstatus.Evaluate(epic, members)
	if options.JSON {
		encoder := json.NewEncoder(command.OutOrStdout())
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(result); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(command.OutOrStdout(), "%s: %s [%s]\n", result.Group, strings.ToUpper(string(result.Verdict)), result.Code)
		fmt.Fprintf(command.OutOrStdout(), "  %s\n", result.Reason)
		for _, blocker := range result.Blockers {
			fmt.Fprintf(command.OutOrStdout(), "  - blocker %s (%s): %s\n", blocker.ID, blocker.Status, blocker.Class)
		}
	}
	if options.Terminal {
		switch result.Verdict {
		case epicstatus.NotTerminal:
			command.SilenceUsage, command.SilenceErrors = true, true
			return &beadsapp.ExitError{Code: 2}
		case epicstatus.Skipped:
			command.SilenceUsage, command.SilenceErrors = true, true
			return &beadsapp.ExitError{Code: 3}
		}
	}
	return nil
}

func (module Module) acceptanceCommand() *cobra.Command {
	var options Options
	command := &cobra.Command{Use: "verify-acceptance <bead-id>...", Short: "Assert each bead carries the acceptance contract for its type (br-native)", Args: cobra.MinimumNArgs(1)}
	command.Flags().BoolVar(&options.Strict, "strict", false, "Exit non-zero on any FAIL or UNDEFINED verdict")
	command.Flags().BoolVar(&options.JSON, "json", false, "Emit verdicts as JSON")
	command.RunE = func(command *cobra.Command, args []string) error {
		return module.invoke(command, OperationAcceptance, args, options)
	}
	return command
}
