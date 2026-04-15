package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/boshu2/agentops/cli/internal/config"
	"github.com/boshu2/agentops/cli/internal/lifecycle"
	ovn "github.com/boshu2/agentops/cli/internal/overnight"
	"github.com/boshu2/agentops/cli/internal/pool"
)

var (
	overnightGoal           string
	overnightOutputDir      string
	overnightRunTimeout     string
	overnightLongHaul       bool
	overnightLongHaulBudget string
	overnightKeepAwake      bool
	overnightNoKeepAwake    bool
	overnightRunners        []string
	overnightModels         string
	overnightCreative       bool
	overnightReportFrom     string

	// Dream nightly compounder loop flags (schema v2).
	overnightQueue           string
	overnightMaxIterations   int
	overnightPlateauEpsilon  float64
	overnightPlateauWindowK  int
	overnightWarnOnly        bool
	overnightCheckpointMaxMB int64
)

var (
	runOvernightLoopFn           = ovn.RunLoop
	writeDreamCloseLoopArtifact  = writeDreamLoopCloseLoopArtifact
	runDreamDefragPreviewFn      = runDreamDefragPreview
	runDreamMetricsHealthFn      = runDreamMetricsHealth
	runDreamRetrievalLiveFn      = runDreamRetrievalLive
	runDreamKnowledgeBriefFn     = runDreamKnowledgeBrief
	runDreamCouncilFn            = runDreamCouncil
	executeDreamMorningPacketsFn = executeDreamMorningPackets
)

type overnightSettings struct {
	OutputDir             string
	RunTimeoutRaw         string
	RunTimeout            time.Duration
	LongHaulEnabled       bool
	LongHaulBudgetRaw     string
	LongHaulBudget        time.Duration
	KeepAwake             bool
	Runners               []string
	RunnerModels          map[string]string
	Consensus             string
	CreativeLane          bool
	CouncilRunnerTimeout  time.Duration
}

type overnightRuntimeSummary struct {
	KeepAwake          bool   `json:"keep_awake" yaml:"keep_awake"`
	KeepAwakeMode      string `json:"keep_awake_mode" yaml:"keep_awake_mode"`
	KeepAwakeNote      string `json:"keep_awake_note,omitempty" yaml:"keep_awake_note,omitempty"`
	RequestedTimeout   string `json:"requested_timeout" yaml:"requested_timeout"`
	EffectiveTimeout   string `json:"effective_timeout" yaml:"effective_timeout"`
	LockPath           string `json:"lock_path" yaml:"lock_path"`
	LogPath            string `json:"log_path" yaml:"log_path"`
	ProcessContractDoc string `json:"process_contract_doc" yaml:"process_contract_doc"`
	ReportContractDoc  string `json:"report_contract_doc" yaml:"report_contract_doc"`
}

type overnightStepSummary struct {
	Name     string `json:"name" yaml:"name"`
	Status   string `json:"status" yaml:"status"`
	Command  string `json:"command,omitempty" yaml:"command,omitempty"`
	Artifact string `json:"artifact,omitempty" yaml:"artifact,omitempty"`
	Note     string `json:"note,omitempty" yaml:"note,omitempty"`
}

type overnightSummary struct {
	SchemaVersion  int                      `json:"schema_version" yaml:"schema_version"`
	Mode           string                   `json:"mode" yaml:"mode"`
	RunID          string                   `json:"run_id" yaml:"run_id"`
	Goal           string                   `json:"goal,omitempty" yaml:"goal,omitempty"`
	RepoRoot       string                   `json:"repo_root" yaml:"repo_root"`
	OutputDir      string                   `json:"output_dir" yaml:"output_dir"`
	Status         string                   `json:"status" yaml:"status"`
	DryRun         bool                     `json:"dry_run" yaml:"dry_run"`
	StartedAt      string                   `json:"started_at" yaml:"started_at"`
	FinishedAt     string                   `json:"finished_at,omitempty" yaml:"finished_at,omitempty"`
	Duration       string                   `json:"duration,omitempty" yaml:"duration,omitempty"`
	Runtime        overnightRuntimeSummary  `json:"runtime" yaml:"runtime"`
	Steps          []overnightStepSummary   `json:"steps" yaml:"steps"`
	Artifacts      map[string]string        `json:"artifacts,omitempty" yaml:"artifacts,omitempty"`
	MetricsHealth  map[string]any           `json:"metrics_health,omitempty" yaml:"metrics_health,omitempty"`
	RetrievalLive  map[string]any           `json:"retrieval_live,omitempty" yaml:"retrieval_live,omitempty"`
	CloseLoop      map[string]any           `json:"close_loop,omitempty" yaml:"close_loop,omitempty"`
	Briefing       map[string]any           `json:"briefing,omitempty" yaml:"briefing,omitempty"`
	Council        *overnightCouncilSummary `json:"council,omitempty" yaml:"council,omitempty"`
	Dreamscape     *overnightDreamscape     `json:"dreamscape,omitempty" yaml:"dreamscape,omitempty"`
	MorningPackets []overnightMorningPacket `json:"morning_packets,omitempty" yaml:"morning_packets,omitempty"`
	Degraded       []string                 `json:"degraded,omitempty" yaml:"degraded,omitempty"`
	Recommended    []string                 `json:"recommended,omitempty" yaml:"recommended,omitempty"`
	NextAction     string                   `json:"next_action,omitempty" yaml:"next_action,omitempty"`
	Yield          *ovn.YieldSummary        `json:"yield,omitempty" yaml:"yield,omitempty"`
	LongHaul       *ovn.LongHaulSummary     `json:"long_haul,omitempty" yaml:"long_haul,omitempty"`

	// Dream-report schema v2 fields. These are populated by the nightly
	// compounder RunLoop (cli/internal/overnight) when v2 mode is active.
	// Marked omitempty so v1 readers tolerate their absence.
	Iterations       []ovn.IterationSummary `json:"iterations,omitempty" yaml:"iterations,omitempty"`
	FitnessDelta     map[string]any         `json:"fitness_delta,omitempty" yaml:"fitness_delta,omitempty"`
	PlateauReason    string                 `json:"plateau_reason,omitempty" yaml:"plateau_reason,omitempty"`
	RegressionReason string                 `json:"regression_reason,omitempty" yaml:"regression_reason,omitempty"`

	// Micro-epic 4 (C3): warn-only ratchet counter. WarnOnlyBudgetInitial
	// is the rescue ceiling in effect for the run; WarnOnlyRemaining is
	// the live counter at loop exit. Zero initial means the ratchet was
	// disabled (e.g. --warn-only=false); the morning report renderer uses
	// the zero-guard to suppress the counter line entirely.
	WarnOnlyBudgetInitial int `json:"warn_only_budget_initial,omitempty" yaml:"warn_only_budget_initial,omitempty"`
	WarnOnlyRemaining     int `json:"warn_only_remaining,omitempty" yaml:"warn_only_remaining,omitempty"`

	// Micro-epic 5 (C4): consecutive MEASURE failure halt signal.
	// MeasureFailureHalt is true when the loop stopped because the
	// configured cap on back-to-back MEASURE failures was reached.
	// FailureReason is a human-readable explanation carrying the
	// iteration index, consecutive count, and configured cap. Both
	// are omitempty so happy-path runs do not emit noise.
	MeasureFailureHalt bool   `json:"measure_failure_halt,omitempty" yaml:"measure_failure_halt,omitempty"`
	FailureReason      string `json:"failure_reason,omitempty" yaml:"failure_reason,omitempty"`

	councilNextActionHint string                              `json:"-" yaml:"-"`
	yieldBaselineCaptured bool                                `json:"-" yaml:"-"`
	packetCorroboration   map[string]dreamPacketCorroboration `json:"-" yaml:"-"`
}

var overnightCmd = &cobra.Command{
	Use:   "overnight",
	Short: "Dream operator mode for private overnight runs",
	Long: `Dream is AgentOps' private overnight operator mode.

It is the local-first counterpart to the public nightly CI proof harness:
  - runs against the real local .agents corpus
  - writes a normalized morning report contract
  - keeps process/runtime behavior explicit and auditable

First slice commands:
  start / run   Start an overnight Dream run now
  report        Read and render an existing Dream report
  setup         Bootstrap Dream config and scheduler guidance`,
}

var overnightStartCmd = &cobra.Command{
	Use:     "start",
	Aliases: []string{"run"},
	Short:   "Run the local Dream maintenance and morning-report path",
	Long: `Start a private overnight Dream run against the local repository.

The v1 path is deliberately bounded:
  - close-loop promotion
  - defrag preview
  - flywheel health report
  - live retrieval proof
  - optional goal briefing

Dream writes both summary.json and summary.md so later layers can consume the
same report contract without re-inventing output shapes.`,
	Args: cobra.NoArgs,
	RunE: runOvernightStart,
}

var overnightReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Render a previously written Dream report",
	Long: `Read a Dream report from summary.json and render it as JSON, YAML, or
terminal text. Pass either a directory containing summary.json or the JSON file
itself via --from.`,
	Args: cobra.NoArgs,
	RunE: runOvernightReport,
}

// overnightWarnOnlyCmd is the parent for warn-only ratchet subcommands.
// Micro-epic 4 (C3): adds the `reset` subcommand that restores the
// rescue budget to a fresh value after an operator has investigated a
// run of consumed rescues.
var overnightWarnOnlyCmd = &cobra.Command{
	Use:   "warn-only",
	Short: "Manage Dream's warn-only rescue budget (C3 ratchet)",
	Long: `Dream's warn-only ratchet protects the first 2-3 production runs
from halting on transient plateau/regression events, but only for a bounded
number of rescues. Once the budget is exhausted the loop falls back to
strict halting behaviour. Use ` + "`ao overnight warn-only reset`" + ` after
investigating a burned-through budget to restore the rescue count.`,
}

var (
	overnightWarnOnlyResetInitial int
	overnightWarnOnlyResetJSON    bool
)

var overnightWarnOnlyResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset the warn-only rescue budget",
	Long: `Reset .agents/overnight/warn-only-budget.json to a fresh state.

By default the budget is restored to the built-in ceiling
(` + fmt.Sprintf("%d", ovn.DefaultWarnOnlyBudget) + `
rescues). Pass --initial to override. The previous state is overwritten.`,
	Args: cobra.NoArgs,
	RunE: runOvernightWarnOnlyReset,
}

func init() {
	overnightCmd.GroupID = "workflow"
	rootCmd.AddCommand(overnightCmd)
	overnightCmd.AddCommand(overnightStartCmd)
	overnightCmd.AddCommand(overnightReportCmd)
	overnightCmd.AddCommand(overnightWarnOnlyCmd)
	overnightWarnOnlyCmd.AddCommand(overnightWarnOnlyResetCmd)
	overnightWarnOnlyResetCmd.Flags().IntVar(&overnightWarnOnlyResetInitial, "initial", 0,
		fmt.Sprintf("Initial rescue ceiling (defaults to %d)", ovn.DefaultWarnOnlyBudget))
	overnightWarnOnlyResetCmd.Flags().BoolVar(&overnightWarnOnlyResetJSON, "json", false,
		"Emit the reset result as JSON instead of human-readable text")

	overnightStartCmd.Flags().StringVar(&overnightGoal, "goal", "", "Optional goal to include in the morning report and briefing step")
	overnightStartCmd.Flags().StringVar(&overnightOutputDir, "output-dir", "", "Directory for overnight artifacts (defaults to dream.report_dir)")
	overnightStartCmd.Flags().StringVar(&overnightRunTimeout, "run-timeout", "", "Maximum duration for the overnight run (defaults to dream.run_timeout)")
	overnightStartCmd.Flags().BoolVar(&overnightLongHaul, "long-haul", false, "Enable the opt-in long-haul Dream controller after the default short path")
	overnightStartCmd.Flags().StringVar(&overnightLongHaulBudget, "long-haul-budget", "1h", "Maximum extra time the long-haul controller may spend after the short path")
	overnightStartCmd.Flags().BoolVar(&overnightKeepAwake, "keep-awake", false, "Force keep-awake assistance on for this run")
	overnightStartCmd.Flags().BoolVar(&overnightNoKeepAwake, "no-keep-awake", false, "Disable keep-awake assistance for this run")
	overnightStartCmd.Flags().StringSliceVar(&overnightRunners, "runner", nil, "Dream runner to execute (repeatable: --runner codex --runner claude)")
	overnightStartCmd.Flags().StringVar(&overnightModels, "models", "", "Deprecated alias for --runner (comma-separated Dream runners)")
	_ = overnightStartCmd.Flags().MarkDeprecated("models", "use --runner instead")
	overnightStartCmd.Flags().BoolVar(&overnightCreative, "creative-lane", false, "Enable the bounded wildcard lane when Dream Council is running")

	// Dream nightly compounder loop flags (schema v2).
	overnightStartCmd.Flags().StringVar(&overnightQueue, "queue", "", "Operator-pinned nightly priorities (markdown file)")
	overnightStartCmd.Flags().IntVar(&overnightMaxIterations, "max-iterations", 0, "Cap iteration count (0 = budget-bounded only)")
	overnightStartCmd.Flags().Float64Var(&overnightPlateauEpsilon, "plateau-epsilon", 0.01, "Plateau threshold: |delta| below this counts as plateau")
	overnightStartCmd.Flags().IntVar(&overnightPlateauWindowK, "plateau-window", 2, "Plateau window K (consecutive sub-epsilon deltas required to halt)")
	overnightStartCmd.Flags().BoolVar(&overnightWarnOnly, "warn-only", true, "First-N-runs mode: warn on plateau/regression, don't halt. Default true; flip to false once thresholds are calibrated.")
	overnightStartCmd.Flags().Int64Var(&overnightCheckpointMaxMB, "checkpoint-max-mb", 512, "Max total MB of checkpoint storage per run")

	overnightReportCmd.Flags().StringVar(&overnightReportFrom, "from", "", "Directory containing summary.json, or the summary.json file itself")
}

func runOvernightStart(cmd *cobra.Command, args []string) error {
	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}

	settings, err := resolveOvernightSettings(cmd, cwd)
	if err != nil {
		return err
	}

	startedAt := time.Now().UTC()
	summary := newOvernightStartSummary(cwd, settings, startedAt)

	if GetDryRun() {
		return finishDryRunOvernightSummary(&summary, startedAt)
	}

	if _, err := os.Stat(filepath.Join(cwd, ".agents")); err != nil {
		return fmt.Errorf("dream run requires a local .agents corpus at %s", filepath.Join(cwd, ".agents"))
	}

	// Crash recovery pass (pm-MISS-01) BEFORE we acquire the lock. Any
	// stale two-phase commit markers from an interrupted previous run are
	// either cleaned up (DONE state) or reversed (READY state) so live
	// .agents/ returns to a consistent shape.
	recoverOvernightStart(cwd, &summary)

	lockFile, err := prepareOvernightLock(&summary)
	if err != nil {
		return err
	}
	defer releaseOvernightLock(lockFile)

	if err := os.MkdirAll(summary.OutputDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	logFile, err := openOvernightLog(&summary)
	if err != nil {
		return err
	}
	defer func() { _ = logFile.Close() }()

	stopKeepAwake, mode, note := startKeepAwakeHelper(logFile, settings.KeepAwake)
	defer stopKeepAwake()
	summary.Runtime.KeepAwakeMode = mode
	if note != "" {
		summary.Runtime.KeepAwakeNote = note
		summary.Degraded = append(summary.Degraded, note)
	}

	ctx, cancel := context.WithTimeout(context.Background(), settings.RunTimeout)
	defer cancel()

	// Schema v2 path: replace the old 5-step linear script with a single
	// call into the bounded INGEST → REDUCE → MEASURE compounder loop.
	// Rollback is `git revert`; there is no --legacy-preview fallback.
	runOpts := newOvernightRunLoopOptions(cwd, settings, summary, logFile)

	// Micro-epic 4 (C3): wire the warn-only ratchet from disk. Only
	// active in warn-only mode — strict mode never consumes rescues
	// because every regression/plateau is a hard halt anyway. ReadBudget
	// implements the rescue matrix (missing/corrupt/out-of-range) and
	// returns a default state rather than erroring, so a broken budget
	// file cannot wedge Dream.
	configureOvernightWarnOnlyBudget(cwd, &summary, &runOpts)

	loopResult, loopErr := runOvernightLoopFn(ctx, runOpts)
	if loopResult != nil {
		applyOvernightLoopResult(&summary, loopResult)
	}
	if loopErr != nil {
		summary.Status = "failed"
		_ = finalizeOvernightSummary(&summary, startedAt)
		return fmt.Errorf("dream run loop failed: %w", loopErr)
	}

	if err := executeOvernightReportSurfaces(cwd, &summary); err != nil {
		summary.Status = "failed"
		_ = finalizeOvernightSummary(&summary, startedAt)
		return err
	}

	// Post-loop: Tier 1 local-LLM forge on recent sessions.
	// Degrades honestly — errors append to summary.Degraded, never abort.
	runPostLoopTier1Forge(ctx, cwd, &summary, settings)
	hydrateOvernightSummaryArtifacts(&summary)

	if settings.LongHaulEnabled {
		executeDreamMorningPacketsFn(cwd, &summary)
		if err := runDreamLongHaul(ctx, cwd, logFile, &summary, settings); err != nil {
			return err
		}
	} else {
		if err := runDreamCouncilFn(ctx, cwd, logFile, &summary, settings); err != nil {
			return err
		}
		executeDreamMorningPacketsFn(cwd, &summary)
	}

	summary.Status = "done"
	if err := finalizeOvernightSummary(&summary, startedAt); err != nil {
		return err
	}
	return outputOvernightSummary(summary)
}

func newOvernightStartSummary(cwd string, settings overnightSettings, startedAt time.Time) overnightSummary {
	summary := overnightSummary{
		SchemaVersion: 2,
		Mode:          "dream.local-bedtime",
		RunID:         startedAt.Format("20060102T150405Z"),
		Goal:          strings.TrimSpace(overnightGoal),
		RepoRoot:      cwd,
		OutputDir:     settings.OutputDir,
		Status:        "planned",
		DryRun:        GetDryRun(),
		StartedAt:     startedAt.Format(time.RFC3339),
		Runtime: overnightRuntimeSummary{
			KeepAwake:          settings.KeepAwake,
			KeepAwakeMode:      "disabled",
			RequestedTimeout:   settings.RunTimeoutRaw,
			EffectiveTimeout:   settings.RunTimeout.String(),
			LockPath:           filepath.Join(filepath.Dir(settings.OutputDir), "run.lock"),
			LogPath:            filepath.Join(settings.OutputDir, "overnight.log"),
			ProcessContractDoc: "docs/contracts/dream-run-contract.md",
			ReportContractDoc:  "docs/contracts/dream-report.md",
		},
		Steps: []overnightStepSummary{
			{Name: "close-loop", Status: "pending", Command: "ao flywheel close-loop --threshold 0h --json"},
			{Name: "defrag-preview", Status: "pending", Command: "ao --dry-run defrag --prune --dedup --oscillation-sweep"},
			{Name: "metrics-health", Status: "pending", Command: "ao metrics health --json"},
			{Name: "retrieval-live", Status: "pending", Command: "ao retrieval-bench --live --json"},
		},
		Artifacts: map[string]string{
			"close_loop":               filepath.Join(settings.OutputDir, "close-loop.json"),
			"defrag_report":            filepath.Join(settings.OutputDir, "defrag", "latest.json"),
			"metrics_health":           filepath.Join(settings.OutputDir, "metrics-health.json"),
			"packet_corroboration":     filepath.Join(settings.OutputDir, "packet-corroboration.json"),
			"retrieval_live":           filepath.Join(settings.OutputDir, "retrieval-bench.json"),
			"morning_packets_json":     filepath.Join(settings.OutputDir, "morning-packets", "index.json"),
			"morning_packets_markdown": filepath.Join(settings.OutputDir, "morning-packets", "index.md"),
			"summary_json":             filepath.Join(settings.OutputDir, "summary.json"),
			"summary_markdown":         filepath.Join(settings.OutputDir, "summary.md"),
		},
		LongHaul: &ovn.LongHaulSummary{
			Enabled: settings.LongHaulEnabled,
			Active:  false,
		},
	}
	if summary.Goal != "" {
		summary.Steps = append(summary.Steps, overnightStepSummary{
			Name:    "knowledge-brief",
			Status:  "pending",
			Command: fmt.Sprintf("ao knowledge brief --goal %q --json", summary.Goal),
		})
		summary.Artifacts["briefing"] = filepath.Join(settings.OutputDir, "briefing.json")
		summary.Artifacts["briefing_fallback"] = filepath.Join(settings.OutputDir, "briefing-fallback.json")
	}
	summary.Steps = append(summary.Steps,
		overnightStepSummary{
			Name:     "morning-packets",
			Status:   "pending",
			Command:  "Dream morning packet synthesis",
			Artifact: summary.Artifacts["morning_packets_json"],
		},
		overnightStepSummary{
			Name:    "bead-sync",
			Status:  "pending",
			Command: "bd create/update linked Dream morning packets",
		},
	)
	appendDreamCouncilPlan(&summary, settings)
	return summary
}

func finishDryRunOvernightSummary(summary *overnightSummary, startedAt time.Time) error {
	summary.Status = "dry-run"
	summary.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	summary.Duration = time.Since(startedAt).Round(time.Millisecond).String()
	ensureOvernightDerivedViews(summary)
	refreshOvernightTelemetry(summary)
	summary.Recommended = recommendedDreamCommands(*summary)
	summary.NextAction = deriveDreamNextAction(*summary)
	return outputOvernightSummary(*summary)
}

func recoverOvernightStart(cwd string, summary *overnightSummary) {
	recoveryActions, recErr := ovn.RecoverFromCrash(cwd)
	if recErr != nil {
		summary.Degraded = append(summary.Degraded, fmt.Sprintf("startup recovery: %v", recErr))
	}
	// Batch recovery actions: if there are many (e.g. 22K stale commit
	// markers from a runaway run), summarize as a single degraded entry
	// instead of spamming one entry per action.
	const batchThreshold = 20
	if len(recoveryActions) > batchThreshold {
		summary.Degraded = append(summary.Degraded,
			fmt.Sprintf("recovery: cleaned up %d stale items (first: %s)",
				len(recoveryActions), recoveryActions[0]))
	} else {
		for _, action := range recoveryActions {
			summary.Degraded = append(summary.Degraded, "recovery: "+action)
		}
	}
}

func prepareOvernightLock(summary *overnightSummary) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(summary.Runtime.LockPath), 0o755); err != nil {
		return nil, fmt.Errorf("create dream lock dir: %w", err)
	}
	// Stale-lock reclaim (pm-MISS-01): 12h matches the documented
	// LockStaleAfter default in internal/overnight.
	if stale, _ := ovn.LockIsStale(summary.Runtime.LockPath, 12*time.Hour); stale {
		_ = os.Remove(summary.Runtime.LockPath)
		summary.Degraded = append(summary.Degraded, "reclaimed stale overnight lock")
	}
	lockFile, err := acquireOvernightLock(summary.Runtime.LockPath)
	if err != nil {
		return nil, err
	}
	_ = ovn.WriteLockPID(summary.Runtime.LockPath)
	return lockFile, nil
}

func openOvernightLog(summary *overnightSummary) (*os.File, error) {
	logFile, err := os.OpenFile(summary.Runtime.LogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open dream log: %w", err)
	}
	return logFile, nil
}

func newOvernightRunLoopOptions(
	cwd string,
	settings overnightSettings,
	summary overnightSummary,
	logWriter io.Writer,
) ovn.RunLoopOptions {
	return ovn.RunLoopOptions{
		Cwd:                cwd,
		OutputDir:          summary.OutputDir,
		RunID:              summary.RunID, // required; namespaces iter-*.json persistence
		RunTimeout:         settings.RunTimeout,
		MaxIterations:      overnightMaxIterations,
		PlateauEpsilon:     overnightPlateauEpsilon,
		PlateauWindowK:     overnightPlateauWindowK,
		WarnOnly:           overnightWarnOnly,
		QueuePath:          overnightQueue,
		CloseLoopCallbacks: newDreamCloseLoopCallbacks(),
		CheckpointMaxBytes: overnightCheckpointMaxMB * 1024 * 1024,
		LogWriter:          logWriter,
	}
}

func newDreamCloseLoopCallbacks() lifecycle.CloseLoopOpts {
	return lifecycle.CloseLoopOpts{
		PendingDir:         filepath.Join(".agents", "knowledge", "pending"),
		Threshold:          0,
		Quiet:              true,
		DryRun:             GetDryRun(),
		IncludeGold:        true,
		ResolveIngestFiles: resolveIngestFiles,
		IngestFilesToPool: func(cwd string, files []string) (lifecycle.CloseLoopIngestResult, error) {
			raw, err := ingestPendingFilesToPool(cwd, files)
			return lifecycle.CloseLoopIngestResult(raw), err
		},
		AutoPromoteFn: func(p *pool.Pool, th time.Duration, includeGold bool) (lifecycle.CloseLoopAutoPromoteResult, error) {
			raw, err := autoPromoteAndPromoteToArtifacts(p, th, includeGold)
			return lifecycle.CloseLoopAutoPromoteResult(raw), err
		},
		ProcessCitationFeedback: processCitationFeedback,
		PromoteCitedLearnings:   promoteCitedLearnings,
		PromoteToMemory:         promoteToMemory,
		StoreIndexUpsertFn:      storeIndexUpsert,
		ApplyMaturityFn: func(cwd string) (lifecycle.MaturityTransitionSummary, error) {
			s, err := applyAllMaturityTransitions(cwd)
			return lifecycle.MaturityTransitionSummary{
				Total:        s.Total,
				Applied:      s.Applied,
				ChangedPaths: s.ChangedPaths,
			}, err
		},
	}
}

func executeOvernightReportSurfaces(cwd string, summary *overnightSummary) error {
	closeLoopArtifact := summary.Artifacts["close_loop"]
	if err := writeDreamCloseLoopArtifact(summary); err != nil {
		setOvernightStepStatus(summary, "close-loop", "failed", closeLoopArtifact, err.Error())
		summary.Degraded = append(summary.Degraded, fmt.Sprintf("close-loop: %v", err))
		return fmt.Errorf("dream close-loop artifact: %w", err)
	}
	setOvernightStepStatus(summary, "close-loop", "done", closeLoopArtifact, "executed inside Dream REDUCE")

	defragArtifact := summary.Artifacts["defrag_report"]
	if err := runDreamDefragPreviewFn(cwd, defragArtifact); err != nil {
		setOvernightStepStatus(summary, "defrag-preview", "soft-fail", defragArtifact, err.Error())
		summary.Degraded = append(summary.Degraded, fmt.Sprintf("defrag-preview: %v", err))
	} else {
		setOvernightStepStatus(summary, "defrag-preview", "done", defragArtifact, "")
	}

	metricsArtifact := summary.Artifacts["metrics_health"]
	if err := runDreamMetricsHealthFn(cwd, metricsArtifact); err != nil {
		setOvernightStepStatus(summary, "metrics-health", "failed", metricsArtifact, err.Error())
		summary.Degraded = append(summary.Degraded, fmt.Sprintf("metrics-health: %v", err))
		return fmt.Errorf("dream metrics health: %w", err)
	}
	setOvernightStepStatus(summary, "metrics-health", "done", metricsArtifact, "")

	retrievalArtifact := summary.Artifacts["retrieval_live"]
	if err := runDreamRetrievalLiveFn(cwd, retrievalArtifact); err != nil {
		setOvernightStepStatus(summary, "retrieval-live", "soft-fail", retrievalArtifact, err.Error())
		summary.Degraded = append(summary.Degraded, fmt.Sprintf("retrieval-live: %v", err))
	} else {
		setOvernightStepStatus(summary, "retrieval-live", "done", retrievalArtifact, "")
	}

	if summary.Goal != "" {
		briefingArtifact := summary.Artifacts["briefing"]
		if err := runDreamKnowledgeBriefFn(cwd, summary.Goal, briefingArtifact); err != nil {
			setOvernightStepStatus(summary, "knowledge-brief", "soft-fail", briefingArtifact, err.Error())
			summary.Degraded = append(summary.Degraded, fmt.Sprintf("knowledge-brief: %v", err))
		} else {
			setOvernightStepStatus(summary, "knowledge-brief", "done", briefingArtifact, "")
		}
	}

	return nil
}

func writeDreamLoopCloseLoopArtifact(summary *overnightSummary) error {
	artifact := buildDreamLoopCloseLoopArtifact(summary.Iterations)
	if artifact == nil {
		return fmt.Errorf("no compounded Dream iteration produced a REDUCE summary")
	}
	return writeJSONFile(summary.Artifacts["close_loop"], artifact)
}

func buildDreamLoopCloseLoopArtifact(iterations []ovn.IterationSummary) map[string]any {
	for i := len(iterations) - 1; i >= 0; i-- {
		iter := iterations[i]
		if !iter.Status.IsCorpusCompounded() || len(iter.Reduce) == 0 {
			continue
		}
		artifact := make(map[string]any, len(iter.Reduce)+4)
		for key, value := range iter.Reduce {
			artifact[key] = value
		}
		artifact["source"] = "dream-loop"
		artifact["iteration_id"] = string(iter.ID)
		artifact["iteration_index"] = iter.Index
		artifact["iteration_status"] = string(iter.Status)
		return artifact
	}
	return nil
}

func runDreamDefragPreview(cwd, artifactPath string) error {
	prevPrune := defragPrune
	prevDedup := defragDedup
	prevOscillation := defragOscillationSweep
	prevOutputDir := defragOutputDir
	prevQuiet := defragQuiet
	defer func() {
		defragPrune = prevPrune
		defragDedup = prevDedup
		defragOscillationSweep = prevOscillation
		defragOutputDir = prevOutputDir
		defragQuiet = prevQuiet
	}()

	defragPrune = true
	defragDedup = true
	defragOscillationSweep = true
	defragOutputDir = filepath.Dir(artifactPath)
	defragQuiet = true

	report := &DefragReport{
		Timestamp: time.Now().UTC(),
		DryRun:    true,
	}
	if err := runDefragPhases(cwd, true, report); err != nil {
		return err
	}
	return writeDefragReport(defragOutputDir, report, io.Discard)
}

func runDreamMetricsHealth(cwd, artifactPath string) error {
	metrics, err := computeHealthMetrics(cwd)
	if err != nil {
		return err
	}
	return writeJSONFile(artifactPath, metrics)
}

func runDreamRetrievalLive(cwd, artifactPath string) error {
	report, err := buildLiveReport(cwd, "", "live-local", 3)
	if err != nil {
		return err
	}
	return writeJSONFile(artifactPath, report)
}

func runDreamKnowledgeBrief(cwd, goal, artifactPath string) error {
	agentsRoot := filepath.Join(cwd, ".agents")
	run, err := runKnowledgeNativeBuilder(cwd, agentsRoot, knowledgeBuilderInvocation{
		Step:           "briefing",
		Implementation: knowledgeBuilderImplementationAONative,
		Args:           []string{"--goal", strings.TrimSpace(goal)},
	})
	if err != nil {
		return err
	}
	result := knowledgeBuilderResult{
		Workspace:  cwd,
		AgentsRoot: agentsRoot,
		Step:       run,
		OutputPath: firstNonEmptyTrimmed(run.Metadata["briefing"], latestKnowledgeBriefing(agentsRoot)),
	}
	if result.OutputPath == "" {
		return fmt.Errorf("briefing builder completed but no briefing output was detected")
	}
	return writeJSONFile(artifactPath, result)
}

func configureOvernightWarnOnlyBudget(cwd string, summary *overnightSummary, runOpts *ovn.RunLoopOptions) {
	if !overnightWarnOnly {
		return
	}
	budgetState, rescueReason := ovn.ReadBudget(cwd)
	if rescueReason != "" {
		summary.Degraded = append(summary.Degraded, rescueReason)
	}
	runOpts.WarnOnlyBudget = &ovn.WarnOnlyRatchet{
		Initial:   budgetState.InitialBudget,
		Remaining: budgetState.Remaining,
		OnConsume: func(newRemaining int) error {
			// Persist via full state write so LastDecrementAt is stamped atomically.
			state, _ := ovn.ReadBudget(cwd)
			state.Remaining = newRemaining
			state.LastDecrementAt = time.Now().UTC().Format(time.RFC3339)
			return ovn.WriteBudget(cwd, state)
		},
	}
	summary.WarnOnlyBudgetInitial = budgetState.InitialBudget
	summary.WarnOnlyRemaining = budgetState.Remaining
}

func applyOvernightLoopResult(summary *overnightSummary, loopResult *ovn.RunLoopResult) {
	summary.Iterations = loopResult.Iterations
	summary.FitnessDelta = loopResult.FitnessDelta
	summary.PlateauReason = loopResult.PlateauReason
	summary.RegressionReason = loopResult.RegressionReason
	summary.Degraded = append(summary.Degraded, loopResult.Degraded...)
	if loopResult.WarnOnlyBudgetInitial > 0 {
		summary.WarnOnlyBudgetInitial = loopResult.WarnOnlyBudgetInitial
		summary.WarnOnlyRemaining = loopResult.WarnOnlyBudgetRemaining
	}
	summary.MeasureFailureHalt = loopResult.MeasureFailureHalt
	summary.FailureReason = loopResult.FailureReason
	appendOvernightIterationSteps(summary, loopResult.Iterations)
}

func appendOvernightIterationSteps(summary *overnightSummary, iterations []ovn.IterationSummary) {
	for i, iter := range iterations {
		summary.Steps = append(summary.Steps, overnightStepSummary{
			Name:   fmt.Sprintf("iteration-%d", i+1),
			Status: string(iter.Status),
		})
	}
}

// runPostLoopTier1Forge queues recent Claude session transcripts for the
// Dream curator worker when configured. Without a worker queue, it falls back
// to the local-LLM summarization pipeline using the curator's model/endpoint
// config. Degrades honestly: all errors land in summary.Degraded; the Dream
// run never aborts on Tier 1 failure.
func runPostLoopTier1Forge(_ context.Context, cwd string, summary *overnightSummary, settings overnightSettings) {
	outDir := filepath.Join(cwd, ".agents", "wiki", "sources")
	result, err := runDreamTier1ForgePostLoop(cwd, outDir, "ao-dream-tier1")
	if err != nil {
		summary.Degraded = append(summary.Degraded, fmt.Sprintf("tier1-forge: %v", err))
		return
	}
	if result == nil {
		return
	}
	if summary.CloseLoop == nil {
		summary.CloseLoop = make(map[string]any)
	}
	summary.CloseLoop["tier1_forge"] = tier1ForgeSummaryMap(*result)
}

func tier1ForgeSummaryMap(summary tier1ForgeSummary) map[string]any {
	if summary.Mode == "dream-worker-queue" {
		return map[string]any{
			"mode":      summary.Mode,
			"queued":    summary.Queued,
			"queue_dir": summary.QueueDir,
		}
	}
	return map[string]any{
		"mode":            summary.Mode,
		"files_processed": summary.FilesProcessed,
		"files_skipped":   summary.FilesSkipped,
		"sessions_wrote":  summary.SessionsWrote,
		"errors":          summary.Errors,
	}
}

// collectRecentSessionJSONL finds Claude session transcripts from the last
// 26 hours (matching the Dream ingest lookback window) under the standard
// Claude projects directory.
func collectRecentSessionJSONL(cwd string) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	projectsDir := filepath.Join(home, ".claude", "projects")
	if _, err := os.Stat(projectsDir); err != nil {
		return nil, nil // No Claude projects dir — nothing to do.
	}
	cutoff := time.Now().Add(-26 * time.Hour)
	var paths []string
	err = filepath.WalkDir(projectsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".jsonl" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().After(cutoff) && info.Size() > 1000 {
			paths = append(paths, path)
		}
		return nil
	})
	return paths, err
}

func runOvernightReport(cmd *cobra.Command, args []string) error {
	summaryPath, err := resolveDreamReportPath(overnightReportFrom)
	if err != nil {
		return err
	}
	summary, err := loadOvernightSummary(summaryPath)
	if err != nil {
		return err
	}
	return outputOvernightSummary(summary)
}

func resolveOvernightSettings(cmd *cobra.Command, cwd string) (overnightSettings, error) {
	cfg, err := config.Load(nil)
	if err != nil {
		return overnightSettings{}, fmt.Errorf("load config: %w", err)
	}

	outputDir := strings.TrimSpace(cfg.Dream.ReportDir)
	if cmd.Flags().Changed("output-dir") {
		outputDir = strings.TrimSpace(overnightOutputDir)
	}
	if outputDir == "" {
		outputDir = ".agents/overnight/latest"
	}
	if !filepath.IsAbs(outputDir) {
		outputDir = filepath.Join(cwd, outputDir)
	}

	runTimeoutRaw := strings.TrimSpace(cfg.Dream.RunTimeout)
	if cmd.Flags().Changed("run-timeout") {
		runTimeoutRaw = strings.TrimSpace(overnightRunTimeout)
	}
	if runTimeoutRaw == "" {
		runTimeoutRaw = "8h"
	}
	runTimeout, err := time.ParseDuration(runTimeoutRaw)
	if err != nil {
		return overnightSettings{}, fmt.Errorf("parse dream timeout %q: %w", runTimeoutRaw, err)
	}
	longHaulBudgetRaw := strings.TrimSpace(overnightLongHaulBudget)
	if longHaulBudgetRaw == "" {
		longHaulBudgetRaw = "1h"
	}
	longHaulBudget, err := time.ParseDuration(longHaulBudgetRaw)
	if err != nil {
		return overnightSettings{}, fmt.Errorf("parse long-haul budget %q: %w", longHaulBudgetRaw, err)
	}

	keepAwake := true
	if cfg.Dream.KeepAwake != nil {
		keepAwake = *cfg.Dream.KeepAwake
	}
	if cmd.Flags().Changed("keep-awake") {
		keepAwake = overnightKeepAwake
	}
	if cmd.Flags().Changed("no-keep-awake") && overnightNoKeepAwake {
		keepAwake = false
	}

	councilRunnerTimeout, err := resolveDreamCouncilRunnerTimeout(cfg.Dream)
	if err != nil {
		return overnightSettings{}, err
	}

	return overnightSettings{
		OutputDir:            outputDir,
		RunTimeoutRaw:        runTimeoutRaw,
		RunTimeout:           runTimeout,
		LongHaulEnabled:      overnightLongHaul,
		LongHaulBudgetRaw:    longHaulBudgetRaw,
		LongHaulBudget:       longHaulBudget,
		KeepAwake:            keepAwake,
		Runners:              resolveDreamRunRunners(cfg.Dream),
		RunnerModels:         resolveDreamRunnerModels(cfg),
		Consensus:            resolveDreamConsensusPolicy(cfg.Dream),
		CreativeLane:         resolveDreamCreativeLane(cfg.Dream),
		CouncilRunnerTimeout: councilRunnerTimeout,
	}, nil
}

// resolveDreamCouncilRunnerTimeout parses DreamConfig.CouncilRunnerTimeout,
// falling back to the package-level default when unset. A zero return means
// "use the built-in default" per withDreamCouncilRunnerTimeout.
func resolveDreamCouncilRunnerTimeout(dcfg config.DreamConfig) (time.Duration, error) {
	raw := strings.TrimSpace(dcfg.CouncilRunnerTimeout)
	if raw == "" {
		return 0, nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("parse dream.council_runner_timeout %q: %w", raw, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("dream.council_runner_timeout must be positive, got %q", raw)
	}
	return parsed, nil
}

func acquireOvernightLock(lockPath string) (*os.File, error) {
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open dream lock: %w", err)
	}
	if err := flockLockNB(file); err != nil {
		_ = file.Close()
		if errors.Is(err, errLockWouldBlock) {
			return nil, fmt.Errorf("another overnight run already holds %s", lockPath)
		}
		return nil, fmt.Errorf("lock dream run: %w", err)
	}
	return file, nil
}

func releaseOvernightLock(file *os.File) {
	if file == nil {
		return
	}
	_ = flockUnlock(file)
	_ = file.Close()
}

func startKeepAwakeHelper(log io.Writer, enabled bool) (func(), string, string) {
	if !enabled {
		return func() {}, "disabled", ""
	}
	if runtime.GOOS != "darwin" {
		return func() {}, "unsupported", "keep-awake requested but v1 Dream only auto-manages sleep on macOS"
	}
	caffeinatePath, err := exec.LookPath("caffeinate")
	if err != nil {
		return func() {}, "missing-caffeinate", "keep-awake requested but caffeinate is unavailable"
	}
	cmd := exec.Command(caffeinatePath, "-i", "-w", strconv.Itoa(os.Getpid()))
	cmd.Stdout = log
	cmd.Stderr = log
	if err := cmd.Start(); err != nil {
		return func() {}, "caffeinate-error", fmt.Sprintf("failed to start caffeinate: %v", err)
	}
	stop := func() {
		if cmd.Process == nil {
			return
		}
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}
	return stop, "caffeinate", ""
}

func setOvernightStepStatus(summary *overnightSummary, name, status, artifact, note string) {
	for i := range summary.Steps {
		if summary.Steps[i].Name == name {
			summary.Steps[i].Status = status
			if artifact != "" {
				summary.Steps[i].Artifact = artifact
			}
			if note != "" {
				summary.Steps[i].Note = note
			}
			return
		}
	}
	summary.Steps = append(summary.Steps, overnightStepSummary{
		Name:     name,
		Status:   status,
		Artifact: artifact,
		Note:     note,
	})
}

func finalizeOvernightSummary(summary *overnightSummary, startedAt time.Time) error {
	summary.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	summary.Duration = time.Since(startedAt).Round(time.Millisecond).String()

	hydrateOvernightSummaryArtifacts(summary)
	ensureOvernightDerivedViews(summary)
	refreshOvernightTelemetry(summary)
	summary.Recommended = recommendedDreamCommands(*summary)
	summary.NextAction = deriveDreamNextAction(*summary)

	if err := os.MkdirAll(summary.OutputDir, 0o755); err != nil {
		return fmt.Errorf("ensure dream output dir: %w", err)
	}

	summaryJSONPath := summary.Artifacts["summary_json"]
	if summaryJSONPath == "" {
		summaryJSONPath = filepath.Join(summary.OutputDir, "summary.json")
	}
	summaryMDPath := summary.Artifacts["summary_markdown"]
	if summaryMDPath == "" {
		summaryMDPath = filepath.Join(summary.OutputDir, "summary.md")
	}
	summary.Artifacts["summary_json"] = summaryJSONPath
	summary.Artifacts["summary_markdown"] = summaryMDPath

	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal dream summary: %w", err)
	}
	if err := os.WriteFile(summaryJSONPath, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", summaryJSONPath, err)
	}
	if err := os.WriteFile(summaryMDPath, []byte(renderOvernightSummaryMarkdown(*summary)), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", summaryMDPath, err)
	}
	return nil
}

func hydrateOvernightSummaryArtifacts(summary *overnightSummary) {
	if artifact := summary.Artifacts["metrics_health"]; artifact != "" {
		if data, err := loadJSONMap(artifact); err == nil {
			summary.MetricsHealth = data
		}
	}
	if artifact := summary.Artifacts["retrieval_live"]; artifact != "" {
		if data, err := loadJSONMap(artifact); err == nil {
			summary.RetrievalLive = data
		}
	}
	if artifact := summary.Artifacts["close_loop"]; artifact != "" {
		if data, err := loadJSONMap(artifact); err == nil {
			summary.CloseLoop = data
		}
	}
	if artifact := summary.Artifacts["briefing"]; artifact != "" {
		if data, err := loadJSONMap(artifact); err == nil {
			summary.Briefing = data
		}
	}
	if summary.Briefing == nil {
		if artifact := summary.Artifacts["briefing_fallback"]; artifact != "" {
			if data, err := loadJSONMap(artifact); err == nil {
				summary.Briefing = data
			}
		}
	}
}

func loadJSONMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

func resolveDreamReportPath(value string) (string, error) {
	target := strings.TrimSpace(value)
	if target == "" {
		target = filepath.Join(".agents", "overnight", "latest")
	}
	info, err := os.Stat(target)
	if err != nil {
		return "", fmt.Errorf("stat dream report target %s: %w", target, err)
	}
	if info.IsDir() {
		return filepath.Join(target, "summary.json"), nil
	}
	return target, nil
}

func loadOvernightSummary(path string) (overnightSummary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return overnightSummary{}, fmt.Errorf("read dream summary %s: %w", path, err)
	}
	var summary overnightSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		return overnightSummary{}, fmt.Errorf("parse dream summary %s: %w", path, err)
	}
	return summary, nil
}

func outputOvernightSummary(summary overnightSummary) error {
	ensureOvernightDerivedViews(&summary)
	refreshOvernightTelemetry(&summary)
	summary.Recommended = recommendedDreamCommands(summary)
	summary.NextAction = deriveDreamNextAction(summary)
	switch GetOutput() {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(summary)
	case "yaml":
		enc := yaml.NewEncoder(os.Stdout)
		if err := enc.Encode(summary); err != nil {
			_ = enc.Close()
			return err
		}
		return enc.Close()
	default:
		fmt.Print(renderOvernightSummaryMarkdown(summary))
		return nil
	}
}

func renderOvernightSummaryMarkdown(summary overnightSummary) string {
	var b strings.Builder
	appendDreamOverview(&b, summary)
	appendDreamscapeSection(&b, summary.Dreamscape)
	appendDreamTerrainSection(&b, summary)
	appendDreamYieldSection(&b, summary.Yield)
	appendDreamLongHaulSection(&b, summary.LongHaul)
	appendDreamStepsSection(&b, summary.Steps)
	appendDreamMorningPacketsSection(&b, summary.MorningPackets)
	appendDreamCouncilSection(&b, summary.Council)
	appendDreamListSection(&b, "Degraded", summary.Degraded, false)
	appendDreamListSection(&b, "First Move", nonEmptySlice(summary.NextAction), false)
	appendDreamListSection(&b, "Recommended", summary.Recommended, true)
	return b.String()
}

func appendDreamOverview(b *strings.Builder, summary overnightSummary) {
	b.WriteString("# Dream Morning Report\n\n")
	fmt.Fprintf(b, "- Status: `%s`\n", summary.Status)
	fmt.Fprintf(b, "- Run ID: `%s`\n", summary.RunID)
	fmt.Fprintf(b, "- Repo: `%s`\n", summary.RepoRoot)
	fmt.Fprintf(b, "- Output: `%s`\n", summary.OutputDir)
	if summary.Goal != "" {
		fmt.Fprintf(b, "- Goal: `%s`\n", summary.Goal)
	}
	fmt.Fprintf(b, "- Started: `%s`\n", summary.StartedAt)
	if summary.FinishedAt != "" {
		fmt.Fprintf(b, "- Finished: `%s`\n", summary.FinishedAt)
	}
	if summary.Duration != "" {
		fmt.Fprintf(b, "- Duration: `%s`\n", summary.Duration)
	}
	fmt.Fprintf(b, "- Keep awake: `%t` via `%s`\n", summary.Runtime.KeepAwake, summary.Runtime.KeepAwakeMode)
	fmt.Fprintf(b, "- Timeout: `%s`\n", summary.Runtime.EffectiveTimeout)
}

func appendDreamscapeSection(b *strings.Builder, dreamscape *overnightDreamscape) {
	if dreamscape == nil {
		return
	}
	b.WriteString("\n## DreamScape\n\n")
	fmt.Fprintf(b, "- Weather: `%s`\n", dreamscape.Weather)
	fmt.Fprintf(b, "- Visibility: `%s`\n", dreamscape.Visibility)
	fmt.Fprintf(b, "- Council: `%s`\n", dreamscape.Council)
	if dreamscape.Tension != "" {
		fmt.Fprintf(b, "- Tension: %s\n", dreamscape.Tension)
	}
	fmt.Fprintf(b, "- First move: %s\n", dreamscape.FirstMove)
}

type dreamMetricLine struct {
	Label string
	Key   string
}

func appendDreamTerrainSection(b *strings.Builder, summary overnightSummary) {
	b.WriteString("\n## Terrain\n\n")
	appendDreamMetricLines(b, summary.MetricsHealth, []dreamMetricLine{
		{Label: "Escape velocity", Key: "escape_velocity"},
		{Label: "Sigma", Key: "sigma"},
		{Label: "Rho", Key: "rho"},
		{Label: "Delta", Key: "delta"},
	})
	appendDreamMetricLines(b, summary.RetrievalLive, []dreamMetricLine{
		{Label: "Retrieval coverage", Key: "coverage"},
	})
	if summary.RetrievalLive != nil {
		fmt.Fprintf(b, "- Queries with hits: `%v/%v`\n", lookupPath(summary.RetrievalLive, "queries_with_hits"), lookupPath(summary.RetrievalLive, "queries"))
	}
}

func appendDreamYieldSection(b *strings.Builder, yield *ovn.YieldSummary) {
	if yield == nil {
		return
	}
	if yield.PacketCountBefore == 0 &&
		yield.PacketCountAfter == 0 &&
		yield.BeadSyncCount == 0 &&
		yield.CouncilCompletedCount == 0 &&
		yield.CouncilFailedCount == 0 &&
		yield.CouncilTimeoutCount == 0 &&
		yield.CouncilActionDelta == "" &&
		yield.CouncilRecommendedKind == "" {
		return
	}
	b.WriteString("\n## Yield\n\n")
	fmt.Fprintf(b, "- Packets: `%d -> %d`\n", yield.PacketCountBefore, yield.PacketCountAfter)
	if yield.TopPacketConfidenceBefore != "" || yield.TopPacketConfidenceAfter != "" {
		fmt.Fprintf(b, "- Top packet confidence: `%s -> %s`\n",
			firstNonEmptyTrimmed(yield.TopPacketConfidenceBefore, "n/a"),
			firstNonEmptyTrimmed(yield.TopPacketConfidenceAfter, "n/a"))
	}
	if yield.QueueBackedCount > 0 || yield.SyntheticCount > 0 {
		fmt.Fprintf(b, "- Queue-backed packets: `%d`\n", yield.QueueBackedCount)
		fmt.Fprintf(b, "- Synthetic packets: `%d`\n", yield.SyntheticCount)
		fmt.Fprintf(b, "- Queue-backed winner: `%t`\n", yield.QueueBackedWon)
	}
	if yield.BeadSyncCount > 0 {
		fmt.Fprintf(b, "- Bead sync count: `%d`\n", yield.BeadSyncCount)
	}
	if len(yield.ConfidenceMix) > 0 {
		fmt.Fprintf(b, "- Confidence mix: `%s`\n", formatDreamConfidenceMix(yield.ConfidenceMix))
	}
	if yield.CouncilCompletedCount > 0 || yield.CouncilFailedCount > 0 || yield.CouncilTimeoutCount > 0 {
		fmt.Fprintf(b, "- Council runners: `%d completed / %d failed`\n", yield.CouncilCompletedCount, yield.CouncilFailedCount)
		fmt.Fprintf(b, "- Council timeouts: `%d`\n", yield.CouncilTimeoutCount)
	}
	if yield.CouncilRecommendedKind != "" {
		fmt.Fprintf(b, "- Council recommended kind: `%s`\n", yield.CouncilRecommendedKind)
	}
	if yield.CouncilActionDelta != "" {
		fmt.Fprintf(b, "- Council action delta: `%s`\n", yield.CouncilActionDelta)
	}
}

func appendDreamLongHaulSection(b *strings.Builder, longHaul *ovn.LongHaulSummary) {
	if longHaul == nil {
		return
	}
	if !longHaul.Enabled &&
		!longHaul.Active &&
		longHaul.TriggerReason == "" &&
		longHaul.ExitReason == "" &&
		longHaul.ProbeCount == 0 &&
		longHaul.ZeroDeltaProbeStreak == 0 {
		return
	}
	b.WriteString("\n## Long-Haul\n\n")
	fmt.Fprintf(b, "- Enabled: `%t`\n", longHaul.Enabled)
	fmt.Fprintf(b, "- Active: `%t`\n", longHaul.Active)
	if longHaul.TriggerReason != "" {
		fmt.Fprintf(b, "- Trigger reason: %s\n", longHaul.TriggerReason)
	}
	if longHaul.ExitReason != "" {
		fmt.Fprintf(b, "- Exit reason: %s\n", longHaul.ExitReason)
	}
	fmt.Fprintf(b, "- Probe count: `%d`\n", longHaul.ProbeCount)
	fmt.Fprintf(b, "- Zero-delta probe streak: `%d`\n", longHaul.ZeroDeltaProbeStreak)
}

func appendDreamMetricLines(b *strings.Builder, values map[string]any, lines []dreamMetricLine) {
	if values == nil {
		return
	}
	for _, line := range lines {
		fmt.Fprintf(b, "- %s: `%v`\n", line.Label, lookupPath(values, line.Key))
	}
}

func appendDreamStepsSection(b *strings.Builder, steps []overnightStepSummary) {
	b.WriteString("\n## What Ran\n\n")
	for _, step := range steps {
		line := fmt.Sprintf("- `%s`: `%s`", step.Name, step.Status)
		if step.Artifact != "" {
			line += fmt.Sprintf(" → `%s`", step.Artifact)
		}
		if step.Note != "" {
			line += fmt.Sprintf(" (%s)", step.Note)
		}
		b.WriteString(line + "\n")
	}
}

func appendDreamCouncilSection(b *strings.Builder, council *overnightCouncilSummary) {
	if council == nil {
		return
	}
	b.WriteString("\n## Dream Council\n\n")
	fmt.Fprintf(b, "- Requested runners: `%s`\n", strings.Join(council.RequestedRunners, ", "))
	if len(council.CompletedRunners) > 0 {
		fmt.Fprintf(b, "- Completed runners: `%s`\n", strings.Join(council.CompletedRunners, ", "))
	}
	if len(council.FailedRunners) > 0 {
		fmt.Fprintf(b, "- Failed runners: `%s`\n", strings.Join(council.FailedRunners, ", "))
	}
	fmt.Fprintf(b, "- Consensus policy: `%s`\n", council.ConsensusPolicy)
	if council.ConsensusKind != "" {
		fmt.Fprintf(b, "- Consensus kind: `%s`\n", council.ConsensusKind)
	}
	if council.RecommendedFirstAction != "" {
		fmt.Fprintf(b, "- Recommended action: %s\n", council.RecommendedFirstAction)
	}
	appendDreamListSection(b, "Disagreements", council.Disagreements, false)
	appendDreamListSection(b, "Wildcards", council.WildcardIdeas, false)
}

func appendDreamListSection(b *strings.Builder, heading string, items []string, quoteItems bool) {
	if len(items) == 0 {
		return
	}
	level := "##"
	if heading == "Disagreements" || heading == "Wildcards" {
		level = "###"
	}
	fmt.Fprintf(b, "\n%s %s\n\n", level, heading)
	for _, item := range items {
		if quoteItems {
			b.WriteString("- `" + item + "`\n")
			continue
		}
		b.WriteString("- " + item + "\n")
	}
}

func nonEmptySlice(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return []string{value}
}

func recommendedDreamCommands(summary overnightSummary) []string {
	recommended := make([]string, 0, len(summary.MorningPackets)+3)
	appendUnique := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, existing := range recommended {
			if existing == value {
				return
			}
		}
		recommended = append(recommended, value)
	}
	for _, packet := range summary.MorningPackets {
		appendUnique(packet.MorningCommand)
		if packet.BeadID != "" {
			appendUnique(fmt.Sprintf("bd show %s", packet.BeadID))
		}
	}
	appendUnique(fmt.Sprintf("ao overnight report --from %q", summary.OutputDir))
	if summary.Goal != "" {
		appendUnique(fmt.Sprintf("ao rpi phased %q", summary.Goal))
	}
	if summary.Artifacts != nil {
		if path := summary.Artifacts["summary_json"]; path != "" {
			appendUnique(fmt.Sprintf("cat %q", path))
		}
	}
	return recommended
}

func deriveDreamNextAction(summary overnightSummary) string {
	if len(summary.MorningPackets) > 0 {
		packet := summary.MorningPackets[0]
		if packet.BeadID != "" {
			return fmt.Sprintf("Start with %q (%s): `%s`.", packet.Title, packet.BeadID, packet.MorningCommand)
		}
		return fmt.Sprintf("Start with %q: `%s`.", packet.Title, packet.MorningCommand)
	}
	if summary.Council != nil && strings.TrimSpace(summary.Council.RecommendedFirstAction) != "" {
		return summary.Council.RecommendedFirstAction
	}
	if coverage, ok := lookupFloat(summary.RetrievalLive, "coverage"); ok && coverage < 0.50 {
		return "Retrieval coverage is weak. Start the day by inspecting misses in retrieval-bench.json and promote one missing learning or pattern."
	}
	if escape, ok := lookupBool(summary.MetricsHealth, "escape_velocity"); ok && !escape {
		return "The flywheel is not at escape velocity. Capture or validate one durable learning before starting new implementation work."
	}
	if summary.Goal != "" {
		return fmt.Sprintf("Use the morning packet to resume the goal: `ao rpi phased %q`.", summary.Goal)
	}
	return "Review the overnight report and pick the highest-signal next action before opening a new implementation lane."
}

func lookupPath(m map[string]any, key string) any {
	if m == nil {
		return "n/a"
	}
	if v, ok := m[key]; ok {
		return v
	}
	return "n/a"
}

func lookupFloat(m map[string]any, key string) (float64, bool) {
	if m == nil {
		return 0, false
	}
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch num := v.(type) {
	case float64:
		return num, true
	case int:
		return float64(num), true
	default:
		return 0, false
	}
}

func lookupBool(m map[string]any, key string) (bool, bool) {
	if m == nil {
		return false, false
	}
	v, ok := m[key]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

func refreshOvernightTelemetry(summary *overnightSummary) {
	ensureOvernightLongHaul(summary)
	if summary.Council == nil && len(summary.MorningPackets) == 0 && !summary.yieldBaselineCaptured {
		return
	}
	yield := ensureOvernightYield(summary)
	yield.PacketCountAfter = len(summary.MorningPackets)
	yield.TopPacketConfidenceAfter = topDreamPacketConfidence(summary.MorningPackets)
	yield.QueueBackedCount = 0
	yield.SyntheticCount = 0
	yield.BeadSyncCount = 0
	confidenceMix := map[string]int{}
	for _, packet := range summary.MorningPackets {
		if packet.QueueBacked {
			yield.QueueBackedCount++
		} else {
			yield.SyntheticCount++
		}
		if packet.BeadID != "" {
			yield.BeadSyncCount++
		}
		if confidence := strings.TrimSpace(packet.Confidence); confidence != "" {
			confidenceMix[confidence]++
		}
	}
	if len(summary.MorningPackets) > 0 {
		yield.QueueBackedWon = summary.MorningPackets[0].QueueBacked
	} else {
		yield.QueueBackedWon = false
	}
	if len(confidenceMix) == 0 {
		yield.ConfidenceMix = nil
	} else {
		yield.ConfidenceMix = confidenceMix
	}
	if summary.Council != nil {
		yield.CouncilCompletedCount = len(summary.Council.CompletedRunners)
		yield.CouncilFailedCount = len(summary.Council.FailedRunners)
		yield.CouncilRecommendedKind = strings.TrimSpace(summary.Council.ConsensusKind)
		yield.CouncilActionDelta = classifyDreamCouncilActionDelta(summary.councilNextActionHint, summary.Council.RecommendedFirstAction)
	} else {
		yield.CouncilCompletedCount = 0
		yield.CouncilFailedCount = 0
		yield.CouncilRecommendedKind = ""
		yield.CouncilActionDelta = ""
	}
	yield.CouncilTimeoutCount = countDreamCouncilTimeouts(summary.Degraded)
}

func ensureOvernightYield(summary *overnightSummary) *ovn.YieldSummary {
	if summary.Yield == nil {
		summary.Yield = &ovn.YieldSummary{}
	}
	return summary.Yield
}

func ensureOvernightLongHaul(summary *overnightSummary) *ovn.LongHaulSummary {
	if summary.LongHaul == nil {
		summary.LongHaul = &ovn.LongHaulSummary{}
	}
	return summary.LongHaul
}

func snapshotDreamPacketYield(summary *overnightSummary) {
	yield := ensureOvernightYield(summary)
	if summary.yieldBaselineCaptured {
		return
	}
	yield.PacketCountBefore = len(summary.MorningPackets)
	yield.TopPacketConfidenceBefore = topDreamPacketConfidence(summary.MorningPackets)
	summary.yieldBaselineCaptured = true
}

func resetDreamPacketYieldBaseline(summary *overnightSummary) {
	yield := ensureOvernightYield(summary)
	yield.PacketCountBefore = len(summary.MorningPackets)
	yield.TopPacketConfidenceBefore = topDreamPacketConfidence(summary.MorningPackets)
	summary.yieldBaselineCaptured = true
}

func topDreamPacketConfidence(packets []overnightMorningPacket) string {
	for _, packet := range packets {
		if confidence := strings.TrimSpace(packet.Confidence); confidence != "" {
			return confidence
		}
	}
	return ""
}

func countDreamCouncilTimeouts(lines []string) int {
	count := 0
	for _, line := range lines {
		if strings.Contains(strings.ToLower(strings.TrimSpace(line)), "council timed out after") {
			count++
		}
	}
	return count
}

func classifyDreamCouncilActionDelta(hint, action string) string {
	hint = strings.TrimSpace(hint)
	action = strings.TrimSpace(action)
	switch {
	case action == "":
		return ""
	case hint == "":
		return "new"
	case strings.EqualFold(hint, action):
		return "unchanged"
	default:
		return "refined"
	}
}

func formatDreamConfidenceMix(m map[string]int) string {
	order := []string{"high", "medium", "low"}
	parts := make([]string, 0, len(m))
	seen := make(map[string]struct{}, len(m))
	for _, key := range order {
		if count, ok := m[key]; ok {
			parts = append(parts, fmt.Sprintf("%s=%d", key, count))
			seen[key] = struct{}{}
		}
	}
	for key, count := range m {
		if _, ok := seen[key]; ok {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%d", key, count))
	}
	return strings.Join(parts, ", ")
}

// runOvernightWarnOnlyReset resets .agents/overnight/warn-only-budget.json
// to a fresh state. Implementation delegates to ovn.ResetBudget which
// writes atomically (CreateTemp → Sync → Rename) so a crash mid-reset
// cannot corrupt the budget file.
//
// Emits either a human-readable confirmation or a JSON payload matching
// the disk shape, depending on --json.
func runOvernightWarnOnlyReset(cmd *cobra.Command, args []string) error {
	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}
	state, err := ovn.ResetBudget(cwd, overnightWarnOnlyResetInitial)
	if err != nil {
		return fmt.Errorf("reset warn-only budget: %w", err)
	}
	path := ovn.WarnOnlyBudgetPath(cwd)
	if overnightWarnOnlyResetJSON {
		payload := map[string]any{
			"path":          path,
			"initial":       state.InitialBudget,
			"remaining":     state.Remaining,
			"last_reset_at": state.LastResetAt,
		}
		raw, marshalErr := json.MarshalIndent(payload, "", "  ")
		if marshalErr != nil {
			return fmt.Errorf("marshal reset payload: %w", marshalErr)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(raw))
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(),
		"warn-only budget reset: remaining=%d initial=%d path=%s\n",
		state.Remaining, state.InitialBudget, path)
	return nil
}
