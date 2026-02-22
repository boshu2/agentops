package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/spf13/cobra"

	cliConfig "github.com/boshu2/agentops/cli/internal/config"
	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
	"github.com/boshu2/agentops/cli/internal/types"
)

var (
	phasedFrom         string
	phasedTestFirst    bool
	phasedFastPath     bool
	phasedInteractive  bool
	phasedMaxRetries   int
	phasedPhaseTimeout time.Duration
	phasedStallTimeout time.Duration
	// phasedStreamStartupTimeout bounds how long stream backend can run without
	// receiving its first parsed event before falling back to direct execution.
	phasedStreamStartupTimeout time.Duration
	phasedNoWorktree           bool
	phasedLiveStatus           bool
	phasedSwarmFirst           bool
	phasedAutoCleanStale       bool
	phasedAutoCleanStaleAfter  time.Duration
	phasedRuntimeMode          string
	phasedRuntimeCommand       string
)

// phaseFailureReason classifies why a phase spawn failed.
type phaseFailureReason string

const (
	failReasonTimeout phaseFailureReason = "timeout"
	failReasonStall   phaseFailureReason = "stall"
	failReasonExit    phaseFailureReason = "exit_error"
	failReasonUnknown phaseFailureReason = "unknown"
)

// phasedEngineOptions captures all configurable parameters for runPhasedEngine.
// This allows the loop and other callers to invoke the phased engine programmatically
// without depending on global cobra flag variables.
type phasedEngineOptions struct {
	From                 string
	FastPath             bool
	TestFirst            bool
	Interactive          bool
	MaxRetries           int
	PhaseTimeout         time.Duration
	StallTimeout         time.Duration
	StreamStartupTimeout time.Duration
	NoWorktree           bool
	LiveStatus           bool
	SwarmFirst           bool
	AutoCleanStale       bool
	AutoCleanStaleAfter  time.Duration
	StallCheckInterval   time.Duration
	RuntimeMode          string
	RuntimeCommand       string
	AOCommand            string
	BDCommand            string
	TmuxCommand          string
}

// defaultPhasedEngineOptions returns options matching the default cobra flag values.
func defaultPhasedEngineOptions() phasedEngineOptions {
	return phasedEngineOptions{
		From:                 "discovery",
		MaxRetries:           3,
		PhaseTimeout:         90 * time.Minute,
		StallTimeout:         10 * time.Minute,
		StreamStartupTimeout: 45 * time.Second,
		SwarmFirst:           true,
		AutoCleanStaleAfter:  24 * time.Hour,
		StallCheckInterval:   30 * time.Second,
		RuntimeMode:          "auto",
		RuntimeCommand:       "claude",
		AOCommand:            "ao",
		BDCommand:            "bd",
		TmuxCommand:          "tmux",
	}
}

// runPhasedEngine runs the full phased RPI lifecycle for goal in cwd.
// It is the programmatic entry point used by both the phased cobra command
// and the loop command, ensuring both share the same runtime contracts.
func runPhasedEngine(cwd, goal string, opts phasedEngineOptions) (retErr error) {
	// Temporarily change working directory so runRPIPhasedWithOpts's os.Getwd() call
	// and all path resolution operate in the requested cwd.
	origDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	if cwd != "" && cwd != origDir {
		if err := os.Chdir(cwd); err != nil {
			return fmt.Errorf("chdir to %s: %w", cwd, err)
		}
		defer func() {
			_ = os.Chdir(origDir)
		}()
	}

	args := []string{goal}
	if goal == "" {
		args = nil
	}
	return runRPIPhasedWithOpts(opts, args)
}

func init() {
	phasedCmd := &cobra.Command{
		Use:   "phased <goal>",
		Short: "Run RPI with fresh runtime session per phase",
		Long: `Orchestrate the full RPI lifecycle using 3 consolidated phases.

Each phase gets its own context window (Ralph Wiggum pattern):
  1. Discovery       — research + plan + pre-mortem (shared context, prompt cache hot)
  2. Implementation  — crank (fresh context for heavy work)
  3. Validation      — vibe + post-mortem (fresh eyes, independent of implementer)

This consolidation cuts cold starts from 6 to 3, keeps prompt cache warm
within each phase, and preserves the key isolation boundary: the implementer
session is never the validator session.

Between phases, the CLI reads filesystem artifacts, constructs prompts
via templates, and spawns the next session. Retry loops for gate failures
are handled within the session (discovery) or across sessions (validation).

Examples:
  ao rpi phased "add user authentication"       # full lifecycle (3 sessions)
  ao rpi phased --from=implementation "add auth" # skip to crank (needs epic)
  ao rpi phased --from=validation                # just vibe + post-mortem
  ao rpi phased --dry-run "add auth"             # show prompts without spawning
  ao rpi phased --fast-path "fix typo"           # force --quick for gates`,
		Args: cobra.MaximumNArgs(1),
		RunE: runRPIPhased,
	}

	phasedCmd.Flags().StringVar(&phasedFrom, "from", "discovery", "Start from phase (discovery, implementation, validation; aliases: research, plan, pre-mortem, crank, vibe, post-mortem)")
	phasedCmd.Flags().BoolVar(&phasedTestFirst, "test-first", false, "Pass --test-first to /crank for spec-first TDD")
	phasedCmd.Flags().BoolVar(&phasedFastPath, "fast-path", false, "Force fast path (--quick for gates)")
	phasedCmd.Flags().BoolVar(&phasedInteractive, "interactive", false, "Enable human gates at research and plan phases")
	phasedCmd.Flags().IntVar(&phasedMaxRetries, "max-retries", 3, "Maximum retry attempts per gate (default: 3)")
	phasedCmd.Flags().DurationVar(&phasedPhaseTimeout, "phase-timeout", 90*time.Minute, "Maximum wall-clock runtime per phase (0 disables timeout)")
	phasedCmd.Flags().DurationVar(&phasedStallTimeout, "stall-timeout", 10*time.Minute, "Maximum time without progress before declaring stall (0 disables)")
	phasedCmd.Flags().DurationVar(&phasedStreamStartupTimeout, "stream-startup-timeout", 45*time.Second, "Maximum time to wait for first stream event before falling back to direct execution (0 disables)")
	phasedCmd.Flags().BoolVar(&phasedNoWorktree, "no-worktree", false, "Disable worktree isolation (run in current directory)")
	phasedCmd.Flags().BoolVar(&phasedLiveStatus, "live-status", false, "Stream phase progress to a live-status.md file")
	phasedCmd.Flags().BoolVar(&phasedSwarmFirst, "swarm-first", true, "Default each phase to swarm/agent-team execution; fall back to direct execution if swarm runtime is unavailable")
	phasedCmd.Flags().BoolVar(&phasedAutoCleanStale, "auto-clean-stale", false, "Run stale-run cleanup before starting phased execution")
	phasedCmd.Flags().DurationVar(&phasedAutoCleanStaleAfter, "auto-clean-stale-after", 24*time.Hour, "Only clean stale runs older than this age when auto-clean is enabled")
	phasedCmd.Flags().StringVar(&phasedRuntimeMode, "runtime", "auto", "Phase runtime mode: auto|direct|stream")
	phasedCmd.Flags().StringVar(&phasedRuntimeCommand, "runtime-cmd", "claude", "Runtime command to execute prompts (must support '-p')")

	rpiCmd.AddCommand(phasedCmd)
}

// Phase represents an RPI phase with its index and name.
type phase struct {
	Num  int
	Name string
	Step string // ratchet step name
}

var phases = []phase{
	{1, "discovery", "research"},
	{2, "implementation", "implement"},
	{3, "validation", "validate"},
}

// phasedState persists orchestrator state between phase spawns.
type phasedState struct {
	SchemaVersion   int                 `json:"schema_version"`
	Goal            string              `json:"goal"`
	EpicID          string              `json:"epic_id,omitempty"`
	Phase           int                 `json:"phase"`
	StartPhase      int                 `json:"start_phase"`
	Cycle           int                 `json:"cycle"`
	ParentEpic      string              `json:"parent_epic,omitempty"`
	FastPath        bool                `json:"fast_path"`
	TestFirst       bool                `json:"test_first"`
	SwarmFirst      bool                `json:"swarm_first"`
	Verdicts        map[string]string   `json:"verdicts"`
	Attempts        map[string]int      `json:"attempts"`
	StartedAt       string              `json:"started_at"`
	WorktreePath    string              `json:"worktree_path,omitempty"`
	RunID           string              `json:"run_id,omitempty"`
	OrchestratorPID int                 `json:"orchestrator_pid,omitempty"`
	Backend         string              `json:"backend,omitempty"`
	TerminalStatus  string              `json:"terminal_status,omitempty"` // interrupted, failed, stale, completed
	TerminalReason  string              `json:"terminal_reason,omitempty"`
	TerminatedAt    string              `json:"terminated_at,omitempty"`
	Opts            phasedEngineOptions `json:"opts"`
}

// retryContext holds context for retrying a failed gate.
type retryContext struct {
	Attempt  int
	Findings []finding
	Verdict  string
}

// finding represents a structured finding from a council report.
type finding struct {
	Description string `json:"description"`
	Fix         string `json:"fix"`
	Ref         string `json:"ref"`
}

// PhaseExecutor abstracts the backend used to run a single phase session.
//
// Selection policy (deterministic, logged):
//
//	stream  — stream-json execution with live parsing and fallback semantics.
//	direct  — plain prompt execution without stream parsing.
//
// The chosen backend is recorded in phasedState.Backend, emitted to stdout, and
// appended to the orchestration log so every run has a traceable selection record.
type PhaseExecutor interface {
	// Name returns the backend identifier ("direct" or "stream").
	Name() string
	// Execute runs the phase prompt and blocks until the session completes.
	Execute(prompt, cwd, runID string, phaseNum int) error
}

type directExecutor struct {
	runtimeCommand string
	phaseTimeout   time.Duration
}

func (d *directExecutor) Name() string { return "direct" }
func (d *directExecutor) Execute(prompt, cwd, runID string, phaseNum int) error {
	return spawnRuntimeDirectImpl(d.runtimeCommand, prompt, cwd, phaseNum, d.phaseTimeout)
}

type streamExecutor struct {
	runtimeCommand       string
	statusPath           string
	allPhases            []PhaseProgress
	phaseTimeout         time.Duration
	stallTimeout         time.Duration
	streamStartupTimeout time.Duration
	stallCheckInterval   time.Duration
}

func (s *streamExecutor) Name() string { return "stream" }
func (s *streamExecutor) Execute(prompt, cwd, runID string, phaseNum int) error {
	err := spawnRuntimePhaseWithStream(s.runtimeCommand, prompt, cwd, runID, phaseNum, s.statusPath, s.allPhases, s.phaseTimeout, s.stallTimeout, s.streamStartupTimeout, s.stallCheckInterval)
	if err == nil {
		return nil
	}
	if !shouldFallbackToDirect(err) {
		return err
	}
	fmt.Printf("Stream backend degraded for phase %d; falling back to direct execution (%v)\n", phaseNum, err)
	if directErr := spawnRuntimeDirectImpl(s.runtimeCommand, prompt, cwd, phaseNum, s.phaseTimeout); directErr != nil {
		return fmt.Errorf("stream execution failed: %w; direct fallback failed: %v", err, directErr)
	}
	return nil
}

func shouldFallbackToDirect(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "stream startup timeout") ||
		strings.Contains(msg, "stream parse error") ||
		(strings.Contains(msg, string(failReasonStall)) && strings.Contains(msg, "no stream activity"))
}

// backendCapabilities probes the runtime environment for executor prerequisites.
// All fields are populated by probeBackendCapabilities.
type backendCapabilities struct {
	// LiveStatusEnabled is true when --live-status flag is set.
	LiveStatusEnabled bool
	// RuntimeMode is one of auto|direct|stream.
	RuntimeMode string
}

// probeBackendCapabilities detects available backends in the current environment.
// It is a pure function (no side effects) to keep selectExecutor testable.
func probeBackendCapabilities(liveStatus bool, runtimeMode string) backendCapabilities {
	return backendCapabilities{
		LiveStatusEnabled: liveStatus,
		RuntimeMode:       normalizeRuntimeMode(runtimeMode),
	}
}

// selectExecutorFromCaps resolves an executor from pre-probed capabilities.
// This is the deterministic core of backend selection — testable without env mutation.
// opts provides timeout/interval values for the executors; use defaultPhasedEngineOptions()
// when calling from tests that do not have a full opts available.
//
// Selection order (first match wins):
//  1. runtime=stream — always stream
//  2. runtime=direct — always direct
//  3. runtime=auto   — stream when live-status enabled, otherwise direct
func selectExecutorFromCaps(caps backendCapabilities, statusPath string, allPhases []PhaseProgress, opts phasedEngineOptions) (PhaseExecutor, string) {
	switch caps.RuntimeMode {
	case "stream":
		return &streamExecutor{
			runtimeCommand:       opts.RuntimeCommand,
			statusPath:           statusPath,
			allPhases:            allPhases,
			phaseTimeout:         opts.PhaseTimeout,
			stallTimeout:         opts.StallTimeout,
			streamStartupTimeout: opts.StreamStartupTimeout,
			stallCheckInterval:   opts.StallCheckInterval,
		}, "runtime=stream"
	case "direct":
		return &directExecutor{
			runtimeCommand: opts.RuntimeCommand,
			phaseTimeout:   opts.PhaseTimeout,
		}, "runtime=direct"
	default: // auto
		if caps.LiveStatusEnabled {
			return &streamExecutor{
				runtimeCommand:       opts.RuntimeCommand,
				statusPath:           statusPath,
				allPhases:            allPhases,
				phaseTimeout:         opts.PhaseTimeout,
				stallTimeout:         opts.StallTimeout,
				streamStartupTimeout: opts.StreamStartupTimeout,
				stallCheckInterval:   opts.StallCheckInterval,
			}, "runtime=auto live-status enabled"
		}
		return &directExecutor{
			runtimeCommand: opts.RuntimeCommand,
			phaseTimeout:   opts.PhaseTimeout,
		}, "runtime=auto live-status disabled"
	}
}

// selectExecutor resolves the executor backend based on flags and environment.
// The selection policy, chosen backend, and reason are logged to logPath for
// observability. Pass an empty logPath to skip log writing (e.g., in tests).
//
// Selection order: runtime override (stream/direct) > auto (live-status=>stream, else direct).
func selectExecutor(statusPath string, allPhases []PhaseProgress) PhaseExecutor {
	return selectExecutorWithLog(statusPath, allPhases, "", "", false, defaultPhasedEngineOptions())
}

// selectExecutorWithLog is the log-aware variant used by runRPIPhasedWithOpts.
// logPath and runID are used to append the selection record to the orchestration log.
// liveStatus must be provided explicitly so the function does not read package globals.
// opts provides timeout/interval values embedded into the returned executor.
func selectExecutorWithLog(statusPath string, allPhases []PhaseProgress, logPath, runID string, liveStatus bool, opts phasedEngineOptions) PhaseExecutor {
	caps := probeBackendCapabilities(liveStatus, opts.RuntimeMode)
	executor, reason := selectExecutorFromCaps(caps, statusPath, allPhases, opts)
	msg := fmt.Sprintf("backend=%s reason=%q", executor.Name(), reason)
	fmt.Printf("Executor backend: %s (%s)\n", executor.Name(), reason)
	if logPath != "" {
		logPhaseTransition(logPath, runID, "backend-selection", msg)
	}
	return executor
}

// phaseSummaryInstruction is prepended to each phase prompt so Claude writes a rich summary.
// Placed first so it survives context compaction (early instructions persist longer).
const phaseSummaryInstruction = `PHASE SUMMARY CONTRACT: Before finishing this session, write a concise summary (max 500 tokens) to .agents/rpi/phase-{{.PhaseNum}}-summary.md covering key insights, tradeoffs considered, and risks for subsequent phases. This file is read by the next phase.

`

// contextDisciplineInstruction is prepended to every phase prompt to prevent compaction.
// CONTEXT DISCIPLINE: This constant exists so the CLI can enforce context-aware behavior.
const contextDisciplineInstruction = `CONTEXT DISCIPLINE: You are running inside ao rpi phased (phase {{.PhaseNum}} of 3). Each phase gets a FRESH context window. Stay disciplined:
- Do NOT accumulate large file contents in context. Read files with the Read tool JIT and extract only what you need.
- Do NOT explore broadly when narrow exploration suffices. Be surgical.
- Write findings, plans, and results to DISK (files in .agents/), not just in conversation.
- If you are delegating to workers or spawning agents, do NOT accumulate their full output. Read their result files from disk.
- If you notice context degradation (forgetting earlier instructions, repeating yourself, losing track of the goal), IMMEDIATELY write a handoff to .agents/rpi/phase-{{.PhaseNum}}-handoff.md with: (1) what you accomplished, (2) what remains, (3) key context. Then finish cleanly.
{{.ContextBudget}}
`

// phaseContextBudgets provides phase-specific context guidance.
var phaseContextBudgets = map[int]string{
	1: "BUDGET: This session runs research + plan + pre-mortem. Research: limit to ~15 file reads, write findings to .agents/research/. Plan: write to .agents/plans/, focus on issue creation. Pre-mortem: invoke /council, read the verdict, done. If pre-mortem FAILs, re-plan and re-run pre-mortem within this session (max 3 attempts).",
	2: "BUDGET (CRITICAL): Crank is the highest-risk phase for context. /crank spawns workers internally. Do NOT re-read worker output into your context. Trust /crank to manage its waves. Read only the completion status.",
	3: "BUDGET: This session runs vibe + post-mortem. Vibe: invoke /council on recent changes, read the verdict. Post-mortem: invoke /council + /retro, read output files, write summary. Minimal context for both.",
}

// phasePrompts defines Go templates for each phase's Claude invocation.
// Phase 1 (discovery) chains research + plan + pre-mortem in a single session
// for prompt cache reuse. Phase 2 (implementation) gets a fresh context window.
// Phase 3 (validation) chains vibe + post-mortem with fresh eyes.
var phasePrompts = map[int]string{
	// Discovery: research → plan → pre-mortem (all in one session)
	1: `{{if .SwarmFirst}}SWARM-FIRST EXECUTION CONTRACT:
- Default to /swarm for each step in this phase (research, plan, pre-mortem) using a lead + worker team pattern.
- If /swarm runtime is unavailable, execute the direct commands below in this same session.
- Keep worker outputs on disk and consume thin summaries only.

{{end}}Run these skills IN SEQUENCE. Do not skip any step.

STEP 1 — Research:
{{if .SwarmFirst}}Prefer: execute this step via /swarm with research-focused workers.
Fallback direct command:
{{end}}/research "{{.Goal}}"{{if not .Interactive}} --auto{{end}}

STEP 2 — Plan:
After research completes, run:
{{if .SwarmFirst}}Prefer: execute this step via /swarm with planning/decomposition workers.
Fallback direct command:
{{end}}/plan "{{.Goal}}"{{if not .Interactive}} --auto{{end}}

STEP 3 — Pre-mortem:
After plan completes, run:
{{if .SwarmFirst}}Prefer: execute this step via /swarm (including council/critique workers when available).
Fallback direct command:
{{end}}/pre-mortem{{if .FastPath}} --quick{{end}}

If pre-mortem returns FAIL, re-run /plan with the findings and then /pre-mortem again. Max 3 total attempts. If still FAIL after 3 attempts, stop and report.
	If pre-mortem returns PASS or WARN, proceed.`,

	// Implementation: crank (single skill, fresh context)
	2: `{{if .SwarmFirst}}SWARM-FIRST EXECUTION CONTRACT:
- Run implementation with swarm-managed waves by default (lead + worker teams).
- Prefer crank paths that delegate to /swarm for wave execution.

{{end}}/crank {{.EpicID}}{{if .TestFirst}} --test-first{{end}}`,

	// Validation: vibe → post-mortem (both in one session, fresh eyes)
	3: `{{if .SwarmFirst}}SWARM-FIRST EXECUTION CONTRACT:
- Use swarm/team execution for validation and retrospective steps where available.
- Keep validator and implementer contexts isolated; do not reuse implementation worker context.

{{end}}Run these skills IN SEQUENCE. Do not skip any step.

STEP 1 — Vibe:
{{if .SwarmFirst}}Prefer: execute vibe using /swarm-driven validation workers.
Fallback direct command:
{{end}}/vibe{{if .FastPath}} --quick{{end}} recent

If vibe returns FAIL, STOP and report the findings. Do NOT proceed to post-mortem.
If vibe returns PASS or WARN, proceed.

STEP 2 — Post-mortem:
{{if .SwarmFirst}}Prefer: execute post-mortem using /swarm-driven retro workers.
Fallback direct command:
{{end}}/post-mortem{{if .FastPath}} --quick{{end}} {{.EpicID}}`,
}

// retryPrompts defines templates for retry invocations with feedback context.
// Phase 1 retries are handled WITHIN the session (the prompt instructs Claude to retry).
// Phase 3 (validation) FAIL triggers a fresh phase 2 (implementation) session.
var retryPrompts = map[int]string{
	// Vibe FAIL → re-crank with feedback (spawns fresh implementation session)
	3: `/crank {{.EpicID}}{{if .TestFirst}} --test-first{{end}}` + "\n\n" +
		`Vibe FAIL (attempt {{.RetryAttempt}}/{{.MaxRetries}}). Address these findings:` + "\n" +
		`{{range .Findings}}FINDING: {{.Description}} | FIX: {{.Fix}} | REF: {{.Ref}}` + "\n" + `{{end}}`,
}

// runRPIPhased is the cobra RunE handler for `ao rpi phased`.
// It reads options from package-level cobra flag variables and delegates to runRPIPhasedWithOpts.
func runRPIPhased(cmd *cobra.Command, args []string) error {
	opts := phasedEngineOptions{
		From:                 phasedFrom,
		FastPath:             phasedFastPath,
		TestFirst:            phasedTestFirst,
		Interactive:          phasedInteractive,
		MaxRetries:           phasedMaxRetries,
		PhaseTimeout:         phasedPhaseTimeout,
		StallTimeout:         phasedStallTimeout,
		StreamStartupTimeout: phasedStreamStartupTimeout,
		NoWorktree:           phasedNoWorktree,
		LiveStatus:           phasedLiveStatus,
		SwarmFirst:           phasedSwarmFirst,
		AutoCleanStale:       phasedAutoCleanStale,
		AutoCleanStaleAfter:  phasedAutoCleanStaleAfter,
		StallCheckInterval:   stallCheckInterval,
		RuntimeMode:          phasedRuntimeMode,
		RuntimeCommand:       phasedRuntimeCommand,
	}

	// Apply config-based worktree mode if the --no-worktree flag was not explicitly set.
	if !cmd.Flags().Changed("no-worktree") {
		opts.NoWorktree = resolveWorktreeModeFromConfig(opts.NoWorktree)
	}
	toolchain, err := resolveRPIToolchain(
		cliRPI.Toolchain{
			RuntimeMode:    phasedRuntimeMode,
			RuntimeCommand: phasedRuntimeCommand,
		},
		rpiToolchainFlagSet{
			RuntimeMode:    cmd.Flags().Changed("runtime"),
			RuntimeCommand: cmd.Flags().Changed("runtime-cmd"),
		},
	)
	if err != nil {
		return err
	}
	opts.RuntimeMode = toolchain.RuntimeMode
	opts.RuntimeCommand = toolchain.RuntimeCommand
	opts.AOCommand = toolchain.AOCommand
	opts.BDCommand = toolchain.BDCommand
	opts.TmuxCommand = toolchain.TmuxCommand
	if cmd.Flags().Changed("auto-clean-stale-after") {
		opts.AutoCleanStale = true
	}

	return runRPIPhasedWithOpts(opts, args)
}

// resolveWorktreeModeFromConfig checks the agentops config for rpi.worktree_mode
// and returns the effective NoWorktree value.
func resolveWorktreeModeFromConfig(flagDefault bool) bool {
	cfg, err := cliConfig.Load(nil)
	if err != nil {
		return flagDefault
	}
	switch cfg.RPI.WorktreeMode {
	case "never":
		return true
	case "always":
		return false
	default: // "auto" or empty
		return flagDefault
	}
}

func normalizeRuntimeMode(mode string) string {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	if normalized == "" {
		return "auto"
	}
	return normalized
}

func effectiveRuntimeCommand(command string) string {
	normalized := strings.TrimSpace(command)
	if normalized == "" {
		return "claude"
	}
	return normalized
}

func effectiveAOCommand(command string) string {
	normalized := strings.TrimSpace(command)
	if normalized == "" {
		return "ao"
	}
	return normalized
}

func effectiveBDCommand(command string) string {
	normalized := strings.TrimSpace(command)
	if normalized == "" {
		return "bd"
	}
	return normalized
}

func effectiveTmuxCommand(command string) string {
	normalized := strings.TrimSpace(command)
	if normalized == "" {
		return "tmux"
	}
	return normalized
}

func validateRuntimeMode(mode string) error {
	switch normalizeRuntimeMode(mode) {
	case "auto", "direct", "stream":
		return nil
	default:
		return fmt.Errorf("invalid runtime %q (valid: auto|direct|stream)", mode)
	}
}

// runRPIPhasedWithOpts is the core implementation of the phased RPI lifecycle.
// All configuration is read from opts; no package-level globals are read after
// this point (except test-injection points: lookPath and spawnDirectFn).
func runRPIPhasedWithOpts(opts phasedEngineOptions, args []string) (retErr error) {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	opts.RuntimeMode = normalizeRuntimeMode(opts.RuntimeMode)
	opts.RuntimeCommand = effectiveRuntimeCommand(opts.RuntimeCommand)
	opts.AOCommand = effectiveAOCommand(opts.AOCommand)
	opts.BDCommand = effectiveBDCommand(opts.BDCommand)
	opts.TmuxCommand = effectiveTmuxCommand(opts.TmuxCommand)
	if err := validateRuntimeMode(opts.RuntimeMode); err != nil {
		return err
	}
	if err := preflightRuntimeAvailability(opts.RuntimeCommand); err != nil {
		return err
	}
	if opts.AutoCleanStale {
		minAge := opts.AutoCleanStaleAfter
		if minAge <= 0 {
			minAge = 24 * time.Hour
		}
		fmt.Printf("Auto-cleaning stale runs older than %s before starting\n", minAge)
		if err := executeRPICleanup(cwd, "", true, false, false, GetDryRun(), minAge); err != nil {
			VerbosePrintf("Warning: auto-clean stale runs failed: %v\n", err)
		}
	}

	originalCwd := cwd
	goal, startPhase, err := resolveGoalAndStartPhase(opts, args, cwd)
	if err != nil {
		return err
	}
	state := newPhasedState(opts, startPhase, goal)

	spawnCwd, err := resumePhasedStateIfNeeded(cwd, opts, startPhase, goal, state)
	if err != nil {
		return err
	}

	cleanupSuccess := false
	var logPath string
	spawnCwd, cleanupWorktree, err := setupWorktreeLifecycle(spawnCwd, originalCwd, opts, state)
	if err != nil {
		return err
	}
	defer func() {
		if cleanupErr := cleanupWorktree(cleanupSuccess, logPath); cleanupErr != nil && retErr == nil {
			retErr = cleanupErr
		}
	}()

	ensureStateRunID(state)
	state.OrchestratorPID = os.Getpid()

	_, runLogPath, statusPath, allPhases, err := initializeRunArtifacts(spawnCwd, startPhase, state, opts)
	if err != nil {
		return err
	}
	logPath = runLogPath

	logPhaseTransition(logPath, state.RunID, "start", fmt.Sprintf("goal=%q from=%s", state.Goal, opts.From))

	// Resolve executor backend once for the entire run.
	// selectExecutorWithLog records the selection and reason to the orchestration log.
	executor := selectExecutorWithLog(statusPath, allPhases, logPath, state.RunID, opts.LiveStatus, opts)
	state.Backend = executor.Name()
	if err := savePhasedState(spawnCwd, state); err != nil {
		VerbosePrintf("Warning: could not persist startup state: %v\n", err)
	}
	updateRunHeartbeat(spawnCwd, state.RunID)

	if err := runPhaseLoop(cwd, spawnCwd, state, startPhase, opts, statusPath, allPhases, logPath, executor); err != nil {
		state.TerminalStatus = "failed"
		state.TerminalReason = err.Error()
		state.TerminatedAt = time.Now().Format(time.RFC3339)
		if saveErr := savePhasedState(spawnCwd, state); saveErr != nil {
			VerbosePrintf("Warning: could not persist failed terminal state: %v\n", saveErr)
		}
		return err
	}

	state.TerminalStatus = "completed"
	state.TerminalReason = "all phases completed"
	state.TerminatedAt = time.Now().Format(time.RFC3339)
	if err := savePhasedState(spawnCwd, state); err != nil {
		VerbosePrintf("Warning: could not persist completion state: %v\n", err)
	}

	// All phases completed — mark worktree for merge+cleanup.
	cleanupSuccess = true

	writeFinalPhasedReport(state, logPath)

	return nil
}

// gateFailError signals a gate check failure that may be retried.
type gateFailError struct {
	Phase    int
	Verdict  string
	Findings []finding
	Report   string
}

func (e *gateFailError) Error() string {
	return fmt.Sprintf("gate FAIL at phase %d: %s (report: %s)", e.Phase, e.Verdict, e.Report)
}

// postPhaseProcessing handles phase-specific post-processing.
func postPhaseProcessing(cwd string, state *phasedState, phaseNum int, logPath string) error {
	switch phaseNum {
	case 1: // Discovery — extract epic ID, check pre-mortem verdict
		// Extract epic ID (created by /plan within the discovery session)
		epicID, err := extractEpicID(state.Opts.BDCommand)
		if err != nil {
			return fmt.Errorf("discovery phase: could not extract epic ID (implementation needs this): %w", err)
		}
		state.EpicID = epicID
		fmt.Printf("Epic ID: %s\n", epicID)
		logPhaseTransition(logPath, state.RunID, "discovery", fmt.Sprintf("extracted epic: %s", epicID))

		// Detect fast path
		if !state.Opts.FastPath {
			fast, err := detectFastPath(state.EpicID, state.Opts.BDCommand)
			if err != nil {
				VerbosePrintf("Warning: fast-path detection failed (continuing without): %v\n", err)
			} else if fast {
				state.FastPath = true
				fmt.Println("Micro-epic detected — using fast path (--quick for gates)")
			}
		}

		// Check pre-mortem verdict (run within the discovery session)
		report, err := findLatestCouncilReport(cwd, "pre-mortem", time.Time{}, state.EpicID)
		if err != nil {
			// Pre-mortem may not have run if the session handled retries internally
			// and ultimately gave up. Check if council report exists at all.
			VerbosePrintf("Warning: pre-mortem council report not found (session may have handled retries internally): %v\n", err)
		} else {
			verdict, err := extractCouncilVerdict(report)
			if err != nil {
				VerbosePrintf("Warning: could not extract pre-mortem verdict: %v\n", err)
			} else {
				state.Verdicts["pre_mortem"] = verdict
				fmt.Printf("Pre-mortem verdict: %s\n", verdict)
				logPhaseTransition(logPath, state.RunID, "discovery", fmt.Sprintf("pre-mortem verdict: %s report=%s", verdict, report))

				if verdict == "FAIL" {
					// Discovery session was instructed to retry internally.
					// If we still see FAIL here, it means all retries failed.
					findings, _ := extractCouncilFindings(report, 5)
					return &gateFailError{Phase: 1, Verdict: verdict, Findings: findings, Report: report}
				}
			}
		}

	case 2: // Implementation — check crank completion via bd children
		if state.StartPhase <= 1 {
			if err := validatePriorPhaseResult(cwd, 1); err != nil {
				return fmt.Errorf("phase %d prerequisite not met: %w", phaseNum, err)
			}
		}
		if state.EpicID != "" {
			status, err := checkCrankCompletion(state.EpicID, state.Opts.BDCommand)
			if err != nil {
				VerbosePrintf("Warning: could not check crank completion (continuing to validation): %v\n", err)
			} else {
				fmt.Printf("Crank status: %s\n", status)
				logPhaseTransition(logPath, state.RunID, "implementation", fmt.Sprintf("crank status: %s", status))
				if status == "BLOCKED" || status == "PARTIAL" {
					return &gateFailError{Phase: 2, Verdict: status, Report: "bd children " + state.EpicID}
				}
			}
		}

	case 3: // Validation — check vibe verdict
		if state.StartPhase <= 2 {
			if err := validatePriorPhaseResult(cwd, 2); err != nil {
				return fmt.Errorf("phase %d prerequisite not met: %w", phaseNum, err)
			}
		}
		report, err := findLatestCouncilReport(cwd, "vibe", time.Time{}, state.EpicID)
		if err != nil {
			return fmt.Errorf("validation phase: vibe report not found (phase may not have completed): %w", err)
		}
		verdict, err := extractCouncilVerdict(report)
		if err != nil {
			return fmt.Errorf("validation phase: could not extract vibe verdict from %s: %w", report, err)
		}
		state.Verdicts["vibe"] = verdict
		fmt.Printf("Vibe verdict: %s\n", verdict)
		logPhaseTransition(logPath, state.RunID, "validation", fmt.Sprintf("vibe verdict: %s report=%s", verdict, report))

		if verdict == "FAIL" {
			findings, _ := extractCouncilFindings(report, 5)
			return &gateFailError{Phase: 3, Verdict: verdict, Findings: findings, Report: report}
		}

		// Also extract post-mortem verdict if available (non-blocking)
		pmReport, err := findLatestCouncilReport(cwd, "post-mortem", time.Time{}, state.EpicID)
		if err == nil {
			pmVerdict, err := extractCouncilVerdict(pmReport)
			if err == nil {
				state.Verdicts["post_mortem"] = pmVerdict
				fmt.Printf("Post-mortem verdict: %s\n", pmVerdict)
				logPhaseTransition(logPath, state.RunID, "validation", fmt.Sprintf("post-mortem verdict: %s report=%s", pmVerdict, pmReport))
			}
		}
	}

	return nil
}

func legacyGateAction(attempt, maxRetries int) types.MemRLAction {
	if attempt >= maxRetries {
		return types.MemRLActionEscalate
	}
	return types.MemRLActionRetry
}

func classifyGateFailureClass(phaseNum int, gateErr *gateFailError) types.MemRLFailureClass {
	if gateErr == nil {
		return ""
	}
	verdict := strings.ToUpper(strings.TrimSpace(gateErr.Verdict))
	switch phaseNum {
	case 1:
		if verdict == "FAIL" {
			return types.MemRLFailureClassPreMortemFail
		}
	case 2:
		switch verdict {
		case "BLOCKED":
			return types.MemRLFailureClassCrankBlocked
		case "PARTIAL":
			return types.MemRLFailureClassCrankPartial
		}
	case 3:
		if verdict == "FAIL" {
			return types.MemRLFailureClassVibeFail
		}
	}
	switch verdict {
	case string(failReasonTimeout):
		return types.MemRLFailureClassPhaseTimeout
	case string(failReasonStall):
		return types.MemRLFailureClassPhaseStall
	case string(failReasonExit):
		return types.MemRLFailureClassPhaseExitError
	default:
		return types.MemRLFailureClass(strings.ToLower(verdict))
	}
}

func resolveGateRetryAction(state *phasedState, phaseNum int, gateErr *gateFailError, attempt int) (types.MemRLAction, types.MemRLPolicyDecision) {
	mode := types.GetMemRLMode()
	failureClass := classifyGateFailureClass(phaseNum, gateErr)
	metadataPresent := gateErr != nil && strings.TrimSpace(gateErr.Verdict) != ""

	decision := types.EvaluateDefaultMemRLPolicy(types.MemRLPolicyInput{
		Mode:            mode,
		FailureClass:    failureClass,
		Attempt:         attempt,
		MaxAttempts:     state.Opts.MaxRetries,
		MetadataPresent: metadataPresent,
	})

	legacy := legacyGateAction(attempt, state.Opts.MaxRetries)
	if mode == types.MemRLModeEnforce {
		return decision.Action, decision
	}
	return legacy, decision
}

// handleGateRetry manages retry logic for failed gates.
// spawnCwd is the working directory for spawned claude sessions (may be worktree).
func handleGateRetry(cwd string, state *phasedState, phaseNum int, gateErr *gateFailError, logPath string, spawnCwd string, statusPath string, allPhases []PhaseProgress, executor PhaseExecutor) (bool, error) {
	phaseName := phases[phaseNum-1].Name
	attemptKey := fmt.Sprintf("phase_%d", phaseNum)

	state.Attempts[attemptKey]++
	attempt := state.Attempts[attemptKey]
	if state.Opts.LiveStatus {
		updateLivePhaseStatus(statusPath, allPhases, phaseNum, "retrying after "+gateErr.Verdict, attempt, "")
	}

	action, decision := resolveGateRetryAction(state, phaseNum, gateErr, attempt)
	if decision.Mode != types.MemRLModeOff {
		logPhaseTransition(
			logPath,
			state.RunID,
			phaseName,
			fmt.Sprintf(
				"memrl policy mode=%s failure_class=%s attempt_bucket=%s policy_action=%s selected_action=%s rule=%s",
				decision.Mode,
				decision.FailureClass,
				decision.AttemptBucket,
				decision.Action,
				action,
				decision.RuleID,
			),
		)
	}

	if action == types.MemRLActionEscalate {
		msg := fmt.Sprintf(
			"%s escalated (mode=%s, action=%s, rule=%s, attempt=%d/%d). Last report: %s. Manual intervention needed.",
			phaseName,
			decision.Mode,
			action,
			decision.RuleID,
			attempt,
			state.Opts.MaxRetries,
			gateErr.Report,
		)
		fmt.Println(msg)
		if state.Opts.LiveStatus {
			updateLivePhaseStatus(statusPath, allPhases, phaseNum, "failed after retries", attempt, gateErr.Report)
		}
		logPhaseTransition(logPath, state.RunID, phaseName, msg)
		return false, nil
	}

	fmt.Printf("%s: %s (attempt %d/%d) — retrying\n", phaseName, gateErr.Verdict, attempt, state.Opts.MaxRetries)
	logPhaseTransition(logPath, state.RunID, phaseName, fmt.Sprintf("RETRY attempt %d/%d verdict=%s report=%s", attempt, state.Opts.MaxRetries, gateErr.Verdict, gateErr.Report))

	// Build retry prompt
	retryCtx := &retryContext{
		Attempt:  attempt,
		Findings: gateErr.Findings,
		Verdict:  gateErr.Verdict,
	}

	retryPrompt, err := buildRetryPrompt(cwd, phaseNum, state, retryCtx)
	if err != nil {
		return false, fmt.Errorf("build retry prompt: %w", err)
	}

	if GetDryRun() {
		fmt.Printf("[dry-run] Would spawn retry: %s -p '%s'\n", effectiveRuntimeCommand(state.Opts.RuntimeCommand), retryPrompt)
		return false, nil
	}

	// Spawn retry session
	fmt.Printf("Spawning retry: %s -p '%s'\n", effectiveRuntimeCommand(state.Opts.RuntimeCommand), retryPrompt)
	if state.Opts.LiveStatus {
		updateLivePhaseStatus(statusPath, allPhases, phaseNum, "running retry prompt", attempt, "")
	}
	if err := executor.Execute(retryPrompt, spawnCwd, state.RunID, phaseNum); err != nil {
		if state.Opts.LiveStatus {
			updateLivePhaseStatus(statusPath, allPhases, phaseNum, "retry failed", attempt, err.Error())
		}
		return false, fmt.Errorf("retry failed: %w", err)
	}

	// Re-run the original phase after retry
	rerunPrompt, err := buildPromptForPhase(cwd, phaseNum, state, nil)
	if err != nil {
		return false, fmt.Errorf("build rerun prompt: %w", err)
	}

	fmt.Printf("Re-running phase %d after retry\n", phaseNum)
	if state.Opts.LiveStatus {
		updateLivePhaseStatus(statusPath, allPhases, phaseNum, "re-running phase", attempt, "")
	}
	if err := executor.Execute(rerunPrompt, spawnCwd, state.RunID, phaseNum); err != nil {
		if state.Opts.LiveStatus {
			updateLivePhaseStatus(statusPath, allPhases, phaseNum, "rerun failed", attempt, err.Error())
		}
		return false, fmt.Errorf("rerun failed: %w", err)
	}

	// Check gate again
	if err := postPhaseProcessing(cwd, state, phaseNum, logPath); err != nil {
		if _, ok := err.(*gateFailError); ok {
			// Still failing — recurse
			return handleGateRetry(cwd, state, phaseNum, err.(*gateFailError), logPath, spawnCwd, statusPath, allPhases, executor)
		}
		return false, err
	}
	if state.Opts.LiveStatus {
		updateLivePhaseStatus(statusPath, allPhases, phaseNum, "retry succeeded", attempt, "")
	}

	return true, nil
}

// buildPromptForPhase constructs the Claude invocation prompt for a phase.
func buildPromptForPhase(cwd string, phaseNum int, state *phasedState, _ *retryContext) (string, error) {
	tmplStr, ok := phasePrompts[phaseNum]
	if !ok {
		return "", fmt.Errorf("no prompt template for phase %d", phaseNum)
	}

	tmpl, err := template.New("phase").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	// Get phase-specific context budget guidance
	budget := phaseContextBudgets[phaseNum]

	data := struct {
		Goal          string
		EpicID        string
		FastPath      bool
		TestFirst     bool
		SwarmFirst    bool
		Interactive   bool
		PhaseNum      int
		ContextBudget string
	}{
		Goal:          state.Goal,
		EpicID:        state.EpicID,
		FastPath:      state.FastPath,
		TestFirst:     state.TestFirst,
		SwarmFirst:    state.SwarmFirst,
		Interactive:   state.Opts.Interactive,
		PhaseNum:      phaseNum,
		ContextBudget: budget,
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	skillInvocation := buf.String()

	// Build prompt: summary contract first, then context, then skill invocation.
	// Early instructions survive compaction better than trailing ones.
	var prompt strings.Builder

	// 1. Context discipline instruction (first — survives compaction)
	disciplineTmpl, err := template.New("discipline").Parse(contextDisciplineInstruction)
	if err == nil {
		if err := disciplineTmpl.Execute(&prompt, data); err != nil {
			VerbosePrintf("Warning: could not render context discipline instruction: %v\n", err)
		}
	}

	// 2. Summary instruction
	summaryTmpl, err := template.New("summary").Parse(phaseSummaryInstruction)
	if err == nil {
		if err := summaryTmpl.Execute(&prompt, data); err != nil {
			VerbosePrintf("Warning: could not render summary instruction: %v\n", err)
		}
	}

	// 3. Cross-phase context for phases 3+ (goal, verdicts, prior summaries)
	if phaseNum >= 2 {
		ctx := buildPhaseContext(cwd, state, phaseNum)
		if ctx != "" {
			prompt.WriteString(ctx)
			prompt.WriteString("\n\n")
		}
	}

	// 4. Skill invocation (last — the actual command)
	prompt.WriteString(skillInvocation)

	return prompt.String(), nil
}

// buildPhaseContext constructs a context block from goal, verdicts, and prior phase summaries.
func buildPhaseContext(cwd string, state *phasedState, phaseNum int) string {
	var parts []string

	// Always include the goal
	if state.Goal != "" {
		parts = append(parts, fmt.Sprintf("Goal: %s", state.Goal))
	}

	// Include prior verdicts
	for key, verdict := range state.Verdicts {
		parts = append(parts, fmt.Sprintf("%s verdict: %s", strings.ReplaceAll(key, "_", "-"), verdict))
	}

	// Include prior phase summaries (read from disk)
	if cwd != "" {
		summaries := readPhaseSummaries(cwd, phaseNum)
		if summaries != "" {
			parts = append(parts, summaries)
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return "--- RPI Context (from prior phases) ---\n" + strings.Join(parts, "\n")
}

// readPhaseSummaries reads all phase summary files prior to the given phase.
func readPhaseSummaries(cwd string, currentPhase int) string {
	var summaries []string
	rpiDir := filepath.Join(cwd, ".agents", "rpi")

	for i := 1; i < currentPhase; i++ {
		path := filepath.Join(rpiDir, fmt.Sprintf("phase-%d-summary.md", i))
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		// Cap each summary to prevent context bloat
		if len(content) > 2000 {
			content = content[:2000] + "..."
		}
		phaseName := "unknown"
		if i > 0 && i <= len(phases) {
			phaseName = phases[i-1].Name
		}
		summaries = append(summaries, fmt.Sprintf("[Phase %d: %s]\n%s", i, phaseName, content))
	}

	if len(summaries) == 0 {
		return ""
	}
	return strings.Join(summaries, "\n\n")
}

// buildRetryPrompt constructs a retry prompt with feedback context.
func buildRetryPrompt(cwd string, phaseNum int, state *phasedState, retryCtx *retryContext) (string, error) {
	tmplStr, ok := retryPrompts[phaseNum]
	if !ok {
		// No retry template — fall back to normal prompt
		return buildPromptForPhase(cwd, phaseNum, state, retryCtx)
	}

	tmpl, err := template.New("retry").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parse retry template: %w", err)
	}

	data := struct {
		Goal          string
		EpicID        string
		FastPath      bool
		TestFirst     bool
		RetryAttempt  int
		MaxRetries    int
		Findings      []finding
		PhaseNum      int
		ContextBudget string
	}{
		Goal:          state.Goal,
		EpicID:        state.EpicID,
		FastPath:      state.FastPath,
		TestFirst:     state.TestFirst,
		RetryAttempt:  retryCtx.Attempt,
		MaxRetries:    state.Opts.MaxRetries,
		Findings:      retryCtx.Findings,
		PhaseNum:      phaseNum,
		ContextBudget: phaseContextBudgets[phaseNum],
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute retry template: %w", err)
	}

	skillInvocation := buf.String()

	// Build prompt: context discipline and summary contract first (survive compaction),
	// then the retry skill invocation.
	var prompt strings.Builder

	// 1. Context discipline instruction (first — survives compaction)
	disciplineTmpl, err := template.New("discipline").Parse(contextDisciplineInstruction)
	if err == nil {
		if err := disciplineTmpl.Execute(&prompt, data); err != nil {
			VerbosePrintf("Warning: could not render context discipline instruction: %v\n", err)
		}
	}

	// 2. Summary instruction
	summaryTmpl, err := template.New("summary").Parse(phaseSummaryInstruction)
	if err == nil {
		if err := summaryTmpl.Execute(&prompt, data); err != nil {
			VerbosePrintf("Warning: could not render summary instruction: %v\n", err)
		}
	}

	// 3. Retry skill invocation (last — the actual command with findings)
	prompt.WriteString(skillInvocation)

	return prompt.String(), nil
}

// Exit codes for phased orchestration.
const (
	ExitGateFail  = 10 // Council gate returned FAIL
	ExitUserAbort = 20 // User cancelled the session
	ExitCLIError  = 30 // Runtime CLI error (not found, config issue)
)

// spawnClaudePhase exists for compatibility with legacy direct-spawn tests.
// Production code should use a PhaseExecutor (selectExecutorFromCaps) instead.
func spawnClaudePhase(prompt, cwd, runID string, phaseNum int) error {
	_ = runID
	return spawnDirectFn(prompt, cwd, phaseNum)
}

// spawnRuntimeDirectImpl runs <runtimeCommand> -p directly.
// phaseTimeout controls the maximum runtime; pass 0 to disable the timeout.
func spawnRuntimeDirectImpl(runtimeCommand, prompt, cwd string, phaseNum int, phaseTimeout time.Duration) error {
	command := effectiveRuntimeCommand(runtimeCommand)

	ctx := context.Background()
	cancel := func() {}
	if phaseTimeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), phaseTimeout)
	}
	defer cancel()

	cmd := exec.CommandContext(ctx, command, "-p", prompt)
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = cleanEnvNoClaude()
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("phase %d timed out after %s (set --phase-timeout to increase)", phaseNum, phaseTimeout)
	}
	if err == nil {
		return nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return fmt.Errorf("%s exited with code %d: %w", command, exitErr.ExitCode(), err)
	}
	return fmt.Errorf("%s execution failed: %w", command, err)
}

// spawnClaudeDirectImpl is the legacy wrapper pinned to the default runtime.
func spawnClaudeDirectImpl(prompt, cwd string, phaseNum int, phaseTimeout time.Duration) error {
	return spawnRuntimeDirectImpl("claude", prompt, cwd, phaseNum, phaseTimeout)
}

// spawnRuntimePhaseWithStream spawns a runtime session using --output-format stream-json
// and feeds stdout through ParseStreamEvents for live progress tracking.
// An onUpdate callback calls WriteLiveStatus after every parsed event so that
// external watchers (e.g. ao status) can tail the status file.
// Stderr is passed through to os.Stderr for real-time error visibility.
// phaseTimeout, stallTimeout, streamStartupTimeout, and checkInterval are passed
// explicitly so the function does not read package-level globals.
func spawnRuntimePhaseWithStream(runtimeCommand, prompt, cwd, runID string, phaseNum int, statusPath string, allPhases []PhaseProgress, phaseTimeout, stallTimeout, streamStartupTimeout, checkInterval time.Duration) error {
	command := effectiveRuntimeCommand(runtimeCommand)

	ctx := context.Background()
	cancel := func() {}
	if phaseTimeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), phaseTimeout)
	}
	defer cancel()

	effectiveCheckInterval := checkInterval
	if effectiveCheckInterval <= 0 {
		effectiveCheckInterval = 1 * time.Second
	}

	startedAt := time.Now()

	// Track whether we received at least one parsed event.
	var streamEventCount atomic.Int64

	// Stall detection: track last activity time atomically.
	// Updated on every stream event; a watchdog goroutine checks for staleness.
	var lastActivityUnix atomic.Int64
	lastActivityUnix.Store(startedAt.UnixNano())

	// stallCtx wraps the timeout context with stall-cancellation capability.
	stallCtx, stallCancel := context.WithCancelCause(ctx)
	defer stallCancel(nil)

	// Startup watchdog: fail stream backend if no first event arrives in time.
	// This catches hangs where claude starts but stream-json never yields events.
	if streamStartupTimeout > 0 {
		startupCheckInterval := effectiveCheckInterval
		if startupCheckInterval > 5*time.Second {
			startupCheckInterval = 5 * time.Second
		}
		go func() {
			ticker := time.NewTicker(startupCheckInterval)
			defer ticker.Stop()
			for {
				select {
				case <-stallCtx.Done():
					return
				case <-ticker.C:
					if streamEventCount.Load() > 0 {
						return
					}
					if time.Since(startedAt) > streamStartupTimeout {
						stallCancel(fmt.Errorf("stream startup timeout: no events received after %s", streamStartupTimeout))
						return
					}
				}
			}
		}()
	}

	// Start stall watchdog goroutine (if stall timeout is configured).
	if stallTimeout > 0 {
		go func() {
			ticker := time.NewTicker(effectiveCheckInterval)
			defer ticker.Stop()
			for {
				select {
				case <-stallCtx.Done():
					return
				case <-ticker.C:
					last := time.Unix(0, lastActivityUnix.Load())
					if time.Since(last) > stallTimeout {
						stallCancel(fmt.Errorf("stall detected: no stream activity for %s", stallTimeout))
						return
					}
				}
			}
		}()
	}

	cmd := exec.CommandContext(stallCtx, command, "-p", prompt, "--output-format", "stream-json", "--verbose")
	cmd.Dir = cwd
	cmd.Stderr = os.Stderr
	cmd.Env = cleanEnvNoClaude()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", command, err)
	}

	// phaseIdx is 0-based for allPhases slice.
	phaseIdx := phaseNum - 1

	onUpdate := func(p PhaseProgress) {
		// Record activity for stall detection.
		streamEventCount.Add(1)
		lastActivityUnix.Store(time.Now().UnixNano())

		if phaseIdx >= 0 && phaseIdx < len(allPhases) {
			existing := allPhases[phaseIdx]
			if p.Name == "" {
				p.Name = existing.Name
			}
			if p.CurrentAction == "" {
				p.CurrentAction = existing.CurrentAction
			}
			if p.RetryCount == 0 {
				p.RetryCount = existing.RetryCount
			}
			if p.LastError == "" {
				p.LastError = existing.LastError
			}
			allPhases[phaseIdx] = p
		}
		if writeErr := WriteLiveStatus(statusPath, allPhases, phaseIdx); writeErr != nil {
			VerbosePrintf("Warning: could not write live status: %v\n", writeErr)
		}
	}

	_, parseErr := ParseStreamEvents(stdout, onUpdate)
	waitErr := cmd.Wait()

	// Classify failure reason.
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("phase %d (%s) timed out after %s (set --phase-timeout to increase)", phaseNum, failReasonTimeout, phaseTimeout)
	}
	if cause := context.Cause(stallCtx); cause != nil && stallCtx.Err() != nil && ctx.Err() == nil {
		// Context was cancelled by stall watchdog, not by phase timeout.
		return fmt.Errorf("phase %d (%s): %w", phaseNum, failReasonStall, cause)
	}

	// Prefer wait error (exit code) over parse error.
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			return fmt.Errorf("%s exited with code %d (%s): %w", command, exitErr.ExitCode(), failReasonExit, waitErr)
		}
		return fmt.Errorf("%s execution failed (%s): %w", command, failReasonUnknown, waitErr)
	}
	if parseErr != nil {
		return fmt.Errorf("stream parse error: %w", parseErr)
	}
	if streamEventCount.Load() == 0 {
		return fmt.Errorf("stream startup timeout: stream completed without parseable events")
	}
	return nil
}

// spawnClaudePhaseWithStream is the legacy wrapper pinned to the default runtime.
func spawnClaudePhaseWithStream(prompt, cwd, runID string, phaseNum int, statusPath string, allPhases []PhaseProgress, phaseTimeout, stallTimeout, streamStartupTimeout, checkInterval time.Duration) error {
	return spawnRuntimePhaseWithStream("claude", prompt, cwd, runID, phaseNum, statusPath, allPhases, phaseTimeout, stallTimeout, streamStartupTimeout, checkInterval)
}

func updateLivePhaseStatus(statusPath string, allPhases []PhaseProgress, phaseNum int, action string, retries int, lastErr string) {
	phaseIdx := phaseNum - 1
	if phaseIdx < 0 || phaseIdx >= len(allPhases) {
		return
	}

	p := allPhases[phaseIdx]
	if action != "" {
		p.CurrentAction = summarizeStatusAction(action)
	}
	p.RetryCount = retries
	p.LastError = summarizeStatusAction(lastErr)
	p.LastUpdate = time.Now()
	allPhases[phaseIdx] = p

	if err := WriteLiveStatus(statusPath, allPhases, phaseIdx); err != nil {
		VerbosePrintf("Warning: could not write live status: %v\n", err)
	}
}

// buildAllPhases constructs a []PhaseProgress with Name fields populated
// from the global phases slice, used as the initial state for live status tracking.
func buildAllPhases(phaseDefs []phase) []PhaseProgress {
	all := make([]PhaseProgress, len(phaseDefs))
	for i, p := range phaseDefs {
		all[i] = PhaseProgress{Name: p.Name, CurrentAction: "pending"}
	}
	return all
}

// cleanEnvNoClaude builds a clean env without CLAUDECODE to avoid nesting guard.
func cleanEnvNoClaude() []string {
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "CLAUDECODE=") || strings.HasPrefix(e, "CLAUDE_CODE_") {
			continue
		}
		env = append(env, e)
	}
	return env
}

// --- Verdict extraction helpers ---

// extractCouncilVerdict reads a council report and returns the verdict (PASS/WARN/FAIL).
func extractCouncilVerdict(reportPath string) (string, error) {
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return "", fmt.Errorf("read report: %w", err)
	}

	re := regexp.MustCompile(`(?m)^## Council Verdict:\s*(PASS|WARN|FAIL)`)
	matches := re.FindSubmatch(data)
	if len(matches) < 2 {
		return "", fmt.Errorf("no verdict found in %s", reportPath)
	}
	return string(matches[1]), nil
}

// findLatestCouncilReport finds the most recent council report matching a pattern.
// When epicID is non-empty, reports whose filename contains the epicID are preferred.
// If no epic-scoped report is found, all pattern-matching reports are used as fallback.
func findLatestCouncilReport(cwd string, pattern string, notBefore time.Time, epicID string) (string, error) {
	councilDir := filepath.Join(cwd, ".agents", "council")
	entries, err := os.ReadDir(councilDir)
	if err != nil {
		return "", fmt.Errorf("read council directory: %w", err)
	}

	var matches []string
	var epicMatches []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.Contains(name, pattern) && strings.HasSuffix(name, ".md") {
			if !notBefore.IsZero() {
				info, err := entry.Info()
				if err != nil {
					continue
				}
				if info.ModTime().Before(notBefore) {
					continue
				}
			}
			fullPath := filepath.Join(councilDir, name)
			matches = append(matches, fullPath)
			if epicID != "" && strings.Contains(name, epicID) {
				epicMatches = append(epicMatches, fullPath)
			}
		}
	}

	// Prefer epic-scoped matches when available.
	selected := matches
	if len(epicMatches) > 0 {
		selected = epicMatches
	}

	if len(selected) == 0 {
		return "", fmt.Errorf("no council report matching %q found", pattern)
	}

	sort.Strings(selected)

	return selected[len(selected)-1], nil
}

// extractCouncilFindings extracts structured findings from a council report.
func extractCouncilFindings(reportPath string, max int) ([]finding, error) {
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return nil, fmt.Errorf("read report: %w", err)
	}

	// Look for structured findings: FINDING: ... | FIX: ... | REF: ...
	re := regexp.MustCompile(`(?m)FINDING:\s*(.+?)\s*\|\s*FIX:\s*(.+?)\s*\|\s*REF:\s*(.+?)$`)
	allMatches := re.FindAllSubmatch(data, -1)

	var findings []finding
	for i, m := range allMatches {
		if i >= max {
			break
		}
		findings = append(findings, finding{
			Description: string(m[1]),
			Fix:         string(m[2]),
			Ref:         string(m[3]),
		})
	}

	// Fallback: if no structured findings, extract from "## Shared Findings" section
	if len(findings) == 0 {
		re2 := regexp.MustCompile(`(?m)^\d+\.\s+\*\*(.+?)\*\*\s*[—–-]\s*(.+)$`)
		allMatches2 := re2.FindAllSubmatch(data, -1)
		for i, m := range allMatches2 {
			if i >= max {
				break
			}
			findings = append(findings, finding{
				Description: string(m[1]) + ": " + string(m[2]),
				Fix:         "See council report",
				Ref:         reportPath,
			})
		}
	}

	return findings, nil
}

// --- Epic and completion helpers ---

// extractEpicID finds the most recently created open epic ID via bd CLI.
// bd list returns epics in creation order; we take the LAST match so that
// the epic just created by the plan phase is selected over older ones.
func extractEpicID(bdCommand string) (string, error) {
	command := effectiveBDCommand(bdCommand)

	// Prefer JSON output for prefix-agnostic parsing.
	cmd := exec.Command(command, "list", "--type", "epic", "--status", "open", "--json")
	out, err := cmd.Output()
	if err == nil {
		epicID, parseErr := parseLatestEpicIDFromJSON(out)
		if parseErr == nil {
			return epicID, nil
		}
		VerbosePrintf("Warning: could not parse bd JSON epic list (falling back to text): %v\n", parseErr)
	} else {
		VerbosePrintf("Warning: bd list --json failed (falling back to text): %v\n", err)
	}

	// Fallback for older bd builds that do not support JSON output.
	cmd = exec.Command(command, "list", "--type", "epic", "--status", "open")
	out, err = cmd.Output()
	if err != nil {
		return "", fmt.Errorf("bd list: %w", err)
	}
	return parseLatestEpicIDFromText(string(out))
}

func parseLatestEpicIDFromJSON(data []byte) (string, error) {
	var entries []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return "", fmt.Errorf("parse bd list JSON: %w", err)
	}
	for i := len(entries) - 1; i >= 0; i-- {
		epicID := strings.TrimSpace(entries[i].ID)
		if epicID != "" {
			return epicID, nil
		}
	}
	return "", fmt.Errorf("no epic found in bd list output")
}

func parseLatestEpicIDFromText(output string) (string, error) {
	// Allow custom prefixes (bd-*, ag-*, etc.) and keep the match anchored
	// to issue-like tokens near the start of each line.
	idPattern := regexp.MustCompile(`^[a-z][a-z0-9]*-[a-z0-9][a-z0-9.]*$`)

	var latest string
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		limit := len(fields)
		if limit > 3 {
			limit = 3
		}
		for i := 0; i < limit; i++ {
			field := fields[i]
			token := strings.Trim(field, "[]()")
			if idPattern.MatchString(token) {
				latest = token
				break
			}
		}
	}
	if latest == "" {
		return "", fmt.Errorf("no epic found in bd list output")
	}
	return latest, nil
}

// detectFastPath checks if an epic is a micro-epic (≤2 issues, no blockers).
func detectFastPath(epicID string, bdCommand string) (bool, error) {
	cmd := exec.Command(effectiveBDCommand(bdCommand), "children", epicID)
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("bd children: %w", err)
	}
	return parseFastPath(string(out)), nil
}

// parseFastPath determines if bd children output indicates a micro-epic.
func parseFastPath(output string) bool {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	issueCount := 0
	blockedCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		issueCount++
		if strings.Contains(strings.ToLower(line), "blocked") {
			blockedCount++
		}
	}
	return issueCount <= 2 && blockedCount == 0
}

// checkCrankCompletion checks epic completion via bd children statuses.
// Returns "DONE", "BLOCKED", or "PARTIAL".
func checkCrankCompletion(epicID string, bdCommand string) (string, error) {
	cmd := exec.Command(effectiveBDCommand(bdCommand), "children", epicID)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("bd children: %w", err)
	}
	return parseCrankCompletion(string(out)), nil
}

// parseCrankCompletion determines completion status from bd children output.
func parseCrankCompletion(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	total := 0
	closed := 0
	blocked := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		total++
		lower := strings.ToLower(line)
		if strings.Contains(lower, "closed") || strings.Contains(lower, "✓") {
			closed++
		}
		if strings.Contains(lower, "blocked") {
			blocked++
		}
	}

	if total == 0 {
		return "DONE"
	}
	if closed == total {
		return "DONE"
	}
	if blocked > 0 {
		return "BLOCKED"
	}
	return "PARTIAL"
}

// --- Phase summaries ---

// writePhaseSummary writes a fallback summary only if Claude didn't write one.
func writePhaseSummary(cwd string, state *phasedState, phaseNum int) {
	rpiDir := filepath.Join(cwd, ".agents", "rpi")
	path := filepath.Join(rpiDir, fmt.Sprintf("phase-%d-summary.md", phaseNum))

	// If Claude already wrote a summary, keep it (it's richer than our mechanical one)
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("Phase %d: Claude-written summary found\n", phaseNum)
		return
	}
	fmt.Printf("Phase %d: no Claude summary found, writing fallback\n", phaseNum)

	if err := os.MkdirAll(rpiDir, 0755); err != nil {
		VerbosePrintf("Warning: could not create rpi dir for summary: %v\n", err)
		return
	}

	summary := generatePhaseSummary(state, phaseNum)
	if summary == "" {
		return
	}

	if err := os.WriteFile(path, []byte(summary), 0644); err != nil {
		VerbosePrintf("Warning: could not write phase summary: %v\n", err)
	}
}

// generatePhaseSummary produces a concise summary of what a phase accomplished.
func generatePhaseSummary(state *phasedState, phaseNum int) string {
	switch phaseNum {
	case 1: // Discovery (research + plan + pre-mortem)
		summary := fmt.Sprintf("Discovery completed for goal: %s\n", state.Goal)
		summary += "Research: see .agents/research/ for findings.\n"
		if state.EpicID != "" {
			summary += fmt.Sprintf("Plan: epic %s", state.EpicID)
			if state.FastPath {
				summary += " (micro-epic, fast path)"
			}
			summary += "\n"
		}
		verdict := state.Verdicts["pre_mortem"]
		if verdict != "" {
			summary += fmt.Sprintf("Pre-mortem verdict: %s\nSee .agents/council/*pre-mortem*.md for details.", verdict)
		}
		return summary
	case 2: // Implementation (crank)
		return fmt.Sprintf("Crank completed for epic %s.\nCheck bd children %s for issue statuses.", state.EpicID, state.EpicID)
	case 3: // Validation (vibe + post-mortem)
		summary := ""
		vibeVerdict := state.Verdicts["vibe"]
		if vibeVerdict != "" {
			summary += fmt.Sprintf("Vibe verdict: %s\nSee .agents/council/*vibe*.md for details.\n", vibeVerdict)
		}
		pmVerdict := state.Verdicts["post_mortem"]
		if pmVerdict != "" {
			summary += fmt.Sprintf("Post-mortem verdict: %s\n", pmVerdict)
		}
		summary += "See .agents/council/*post-mortem*.md and .agents/learnings/ for extracted knowledge."
		return summary
	}
	return ""
}

// handoffDetected checks if a phase wrote a handoff file (context degradation signal).
func handoffDetected(cwd string, phaseNum int) bool {
	path := filepath.Join(cwd, ".agents", "rpi", fmt.Sprintf("phase-%d-handoff.md", phaseNum))
	_, err := os.Stat(path)
	return err == nil
}

// cleanPhaseSummaries removes stale phase summaries and handoffs from a prior run.
func cleanPhaseSummaries(stateDir string) {
	for i := 1; i <= len(phases); i++ {
		path := filepath.Join(stateDir, fmt.Sprintf("phase-%d-summary.md", i))
		os.Remove(path) //nolint:errcheck
		handoffPath := filepath.Join(stateDir, fmt.Sprintf("phase-%d-handoff.md", i))
		os.Remove(handoffPath) //nolint:errcheck
		resultPath := filepath.Join(stateDir, fmt.Sprintf("phase-%d-result.json", i))
		os.Remove(resultPath) //nolint:errcheck
	}
}

// --- Worktree isolation ---

// worktreeTimeout is the timeout for git worktree operations (matches Olympus DefaultTimeout).
const worktreeTimeout = 30 * time.Second

// generateRunID returns a 12-char lowercase hex string from crypto/rand.
func generateRunID() string {
	return cliRPI.GenerateRunID()
}

// getCurrentBranch returns the current branch name, or error if detached HEAD.
func getCurrentBranch(repoRoot string) (string, error) {
	return cliRPI.GetCurrentBranch(repoRoot, worktreeTimeout)
}

// createWorktree creates a sibling git worktree for isolated RPI execution.
// Path: ../<repo-basename>-rpi-<runID>/
func createWorktree(cwd string) (worktreePath, runID string, err error) {
	return cliRPI.CreateWorktree(cwd, worktreeTimeout, VerbosePrintf)
}

// mergeWorktree merges the RPI worktree branch back into the original branch.
// Retries the pre-merge dirty check with backoff to handle the race where
// another parallel run is mid-merge (repo momentarily dirty).
func mergeWorktree(repoRoot, worktreePath, runID string) error {
	return cliRPI.MergeWorktree(repoRoot, worktreePath, runID, worktreeTimeout, VerbosePrintf)
}

// removeWorktree removes a worktree directory and any legacy branch marker.
// Modeled on Olympus internal/git/worktree.go Remove().
func removeWorktree(repoRoot, worktreePath, runID string) error {
	return cliRPI.RemoveWorktree(repoRoot, worktreePath, runID, worktreeTimeout)
}

// --- Phase result artifacts ---

// phaseResultFileFmt is the filename pattern for per-phase result artifacts.
// Each phase writes "phase-{N}-result.json" to .agents/rpi/.
// Contract: docs/contracts/rpi-phase-result.schema.json
const phaseResultFileFmt = "phase-%d-result.json"

// phaseResult is a structured artifact written after each phase completes or fails.
// Schema: docs/contracts/rpi-phase-result.schema.json
type phaseResult struct {
	SchemaVersion   int               `json:"schema_version"`
	RunID           string            `json:"run_id"`
	Phase           int               `json:"phase"`
	PhaseName       string            `json:"phase_name"`
	Status          string            `json:"status"`
	Retries         int               `json:"retries,omitempty"`
	Error           string            `json:"error,omitempty"`
	Backend         string            `json:"backend,omitempty"`
	Artifacts       map[string]string `json:"artifacts,omitempty"`
	Verdicts        map[string]string `json:"verdicts,omitempty"`
	StartedAt       string            `json:"started_at"`
	CompletedAt     string            `json:"completed_at,omitempty"`
	DurationSeconds float64           `json:"duration_seconds,omitempty"`
}

// writePhaseResult writes a phase-result.json artifact (named phase-{N}-result.json) atomically (write to .tmp, rename).
func writePhaseResult(cwd string, result *phaseResult) error {
	stateDir := filepath.Join(cwd, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal phase result: %w", err)
	}

	finalPath := filepath.Join(stateDir, fmt.Sprintf(phaseResultFileFmt, result.Phase))
	tmpPath := finalPath + ".tmp"

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write phase result tmp: %w", err)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		return fmt.Errorf("rename phase result: %w", err)
	}

	VerbosePrintf("Phase result written to %s\n", finalPath)
	return nil
}

// validatePriorPhaseResult checks that phase-{expectedPhase}-result.json exists
// and has status "completed". Called at the start of phases 2 and 3.
func validatePriorPhaseResult(cwd string, expectedPhase int) error {
	resultPath := filepath.Join(cwd, ".agents", "rpi", fmt.Sprintf(phaseResultFileFmt, expectedPhase))
	data, err := os.ReadFile(resultPath)
	if err != nil {
		return fmt.Errorf("prior phase %d result not found at %s: %w", expectedPhase, resultPath, err)
	}

	var result phaseResult
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("prior phase %d result is malformed: %w", expectedPhase, err)
	}

	if result.Status != "completed" {
		return fmt.Errorf("prior phase %d has status %q (expected %q)", expectedPhase, result.Status, "completed")
	}

	return nil
}

// --- State persistence ---

const phasedStateFile = "phased-state.json"

// rpiRunRegistryDir returns the per-run registry directory path.
// All per-run artifacts (state, heartbeat) are written here so the registry
// survives interruption and supports resume/status lookup.
// Path: .agents/rpi/runs/<run-id>/
func rpiRunRegistryDir(cwd, runID string) string {
	if runID == "" {
		return ""
	}
	return filepath.Join(cwd, ".agents", "rpi", "runs", runID)
}

// savePhasedState writes orchestrator state to disk atomically.
// The state is written to two locations:
//  1. .agents/rpi/phased-state.json (legacy flat path for backward compatibility)
//  2. .agents/rpi/runs/<run-id>/state.json (per-run registry directory)
//
// Both writes use the tmp+rename pattern to prevent corrupt partial writes.
func savePhasedState(cwd string, state *phasedState) error {
	stateDir := filepath.Join(cwd, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	data = append(data, '\n')

	// Atomic write to flat path (backward compatible).
	flatPath := filepath.Join(stateDir, phasedStateFile)
	if err := writePhasedStateAtomic(flatPath, data); err != nil {
		return fmt.Errorf("write state: %w", err)
	}

	// Also write to per-run registry directory when run ID is available.
	if state.RunID != "" {
		runDir := rpiRunRegistryDir(cwd, state.RunID)
		if mkErr := os.MkdirAll(runDir, 0755); mkErr != nil {
			VerbosePrintf("Warning: create run registry dir: %v\n", mkErr)
		} else {
			registryPath := filepath.Join(runDir, phasedStateFile)
			if wErr := writePhasedStateAtomic(registryPath, data); wErr != nil {
				VerbosePrintf("Warning: write run registry state: %v\n", wErr)
			}
		}
	}

	VerbosePrintf("State saved to %s\n", flatPath)
	return nil
}

// writePhasedStateAtomic writes data to path using a tmp-file+rename pattern.
// This ensures readers never observe a partial write.
func writePhasedStateAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".phased-state-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create tmp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		_ = tmp.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close tmp: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename tmp: %w", err)
	}
	cleanup = false
	return nil
}

// loadPhasedState reads orchestrator state from disk.
// It first tries the per-run registry directory (most recent run), then falls
// back to the flat .agents/rpi/phased-state.json path for backward compatibility.
func loadPhasedState(cwd string) (*phasedState, error) {
	flatPath := filepath.Join(cwd, ".agents", "rpi", phasedStateFile)

	// Try to find the most recently modified state in any run registry directory.
	// This allows resume when the worktree only has the registry (not the flat file).
	runState, runErr := loadLatestRunRegistryState(cwd)
	if runErr == nil && runState != nil {
		// Prefer registry state only when it is newer than (or the same as) the flat file.
		flatInfo, flatStatErr := os.Stat(flatPath)
		if flatStatErr != nil {
			// Flat file does not exist — use registry state.
			return runState, nil
		}
		registryPath := filepath.Join(rpiRunRegistryDir(cwd, runState.RunID), phasedStateFile)
		registryInfo, regStatErr := os.Stat(registryPath)
		if regStatErr == nil && !registryInfo.ModTime().Before(flatInfo.ModTime()) {
			return runState, nil
		}
	}

	// Fall back to flat path.
	data, err := os.ReadFile(flatPath)
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	return parsePhasedState(data)
}

// loadLatestRunRegistryState scans .agents/rpi/runs/ and returns the state
// from the most recently modified run directory, or nil if none exists.
func loadLatestRunRegistryState(cwd string) (*phasedState, error) {
	runsDir := filepath.Join(cwd, ".agents", "rpi", "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return nil, err
	}

	var latestModTime int64
	var latestData []byte

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		statePath := filepath.Join(runsDir, entry.Name(), phasedStateFile)
		info, err := os.Stat(statePath)
		if err != nil {
			continue
		}
		if info.ModTime().UnixNano() > latestModTime {
			data, readErr := os.ReadFile(statePath)
			if readErr != nil {
				continue
			}
			latestModTime = info.ModTime().UnixNano()
			latestData = data
		}
	}

	if latestData == nil {
		return nil, os.ErrNotExist
	}
	return parsePhasedState(latestData)
}

// parsePhasedState parses JSON bytes into a phasedState with nil-safe maps.
func parsePhasedState(data []byte) (*phasedState, error) {
	var state phasedState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal state: %w", err)
	}

	// Ensure maps are never nil after deserialization.
	if state.Verdicts == nil {
		state.Verdicts = make(map[string]string)
	}
	if state.Attempts == nil {
		state.Attempts = make(map[string]int)
	}

	return &state, nil
}

// updateRunHeartbeat writes the current UTC timestamp to
// .agents/rpi/runs/<run-id>/heartbeat.txt atomically.
// It is called during phase execution to signal the run is alive.
// Failures are logged but do not abort the phase.
func updateRunHeartbeat(cwd, runID string) {
	if runID == "" {
		return
	}
	runDir := rpiRunRegistryDir(cwd, runID)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		VerbosePrintf("Warning: create run dir for heartbeat: %v\n", err)
		return
	}
	heartbeatPath := filepath.Join(runDir, "heartbeat.txt")
	ts := time.Now().UTC().Format(time.RFC3339Nano) + "\n"
	if err := writePhasedStateAtomic(heartbeatPath, []byte(ts)); err != nil {
		VerbosePrintf("Warning: update heartbeat: %v\n", err)
	}
}

// readRunHeartbeat returns the last heartbeat timestamp for a run, or zero
// time if the heartbeat file does not exist or cannot be parsed.
func readRunHeartbeat(cwd, runID string) time.Time {
	if runID == "" {
		return time.Time{}
	}
	heartbeatPath := filepath.Join(rpiRunRegistryDir(cwd, runID), "heartbeat.txt")
	data, err := os.ReadFile(heartbeatPath)
	if err != nil {
		return time.Time{}
	}
	ts, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(string(data)))
	if err != nil {
		return time.Time{}
	}
	return ts
}

// --- Ratchet and logging ---

// recordRatchetCheckpoint records a ratchet checkpoint for a phase.
func recordRatchetCheckpoint(step, aoCommand string) {
	cmd := exec.Command(effectiveAOCommand(aoCommand), "ratchet", "record", step)
	if err := cmd.Run(); err != nil {
		VerbosePrintf("Warning: ratchet record %s: %v\n", step, err)
	}
}

// logPhaseTransition appends a log entry to the orchestration log.
func logPhaseTransition(logPath, runID, phase, details string) {
	var entry string
	if runID != "" {
		entry = fmt.Sprintf("[%s] [%s] %s: %s\n", time.Now().Format(time.RFC3339), runID, phase, details)
	} else {
		entry = fmt.Sprintf("[%s] %s: %s\n", time.Now().Format(time.RFC3339), phase, details)
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		VerbosePrintf("Warning: could not write orchestration log: %v\n", err)
		return
	}
	defer f.Close() //nolint:errcheck

	if _, err := f.WriteString(entry); err != nil {
		VerbosePrintf("Warning: could not write log entry: %v\n", err)
		return
	}

	// Mirror transitions into append-only ledger and refresh per-run cache.
	// This keeps mutable status files as cache while preserving provenance.
	maybeAppendRPILedgerTransition(logPath, runID, phase, details)
}

func maybeAppendRPILedgerTransition(logPath, runID, phase, details string) {
	if runID == "" {
		return
	}
	rootDir, ok := deriveRepoRootFromRPIOrchestrationLog(logPath)
	if !ok {
		return
	}

	event := rpiLedgerEvent{
		RunID:  runID,
		Phase:  phase,
		Action: ledgerActionFromDetails(details),
		Details: map[string]any{
			"details": details,
		},
	}

	if _, err := appendRPILedgerEvent(rootDir, event); err != nil {
		VerbosePrintf("Warning: could not append RPI ledger event: %v\n", err)
		return
	}
	if err := materializeRPIRunCache(rootDir, runID); err != nil {
		VerbosePrintf("Warning: could not materialize RPI run cache: %v\n", err)
	}
}

func deriveRepoRootFromRPIOrchestrationLog(logPath string) (string, bool) {
	rpiDir := filepath.Dir(filepath.Clean(logPath))
	if filepath.Base(rpiDir) != "rpi" {
		return "", false
	}
	agentsDir := filepath.Dir(rpiDir)
	if filepath.Base(agentsDir) != ".agents" {
		return "", false
	}
	return filepath.Dir(agentsDir), true
}

func ledgerActionFromDetails(details string) string {
	normalized := strings.ToLower(strings.TrimSpace(details))
	switch {
	case normalized == "":
		return "event"
	case strings.HasPrefix(normalized, "started"):
		return "started"
	case strings.HasPrefix(normalized, "completed"):
		return "completed"
	case strings.HasPrefix(normalized, "failed:"):
		return "failed"
	case strings.HasPrefix(normalized, "fatal:"):
		return "fatal"
	case strings.HasPrefix(normalized, "retry"):
		return "retry"
	case strings.HasPrefix(normalized, "dry-run"):
		return "dry-run"
	case strings.HasPrefix(normalized, "handoff"):
		return "handoff"
	case strings.HasPrefix(normalized, "epic="):
		return "summary"
	}

	fields := strings.Fields(normalized)
	action := strings.Trim(fields[0], ":")
	if action == "" {
		return "event"
	}
	return action
}

// logFailureContext records actionable remediation context when a phase fails.
func logFailureContext(logPath, runID, phase string, err error) {
	logPhaseTransition(logPath, runID, phase, fmt.Sprintf("FAILURE_CONTEXT: %v | action: check .agents/rpi/ for phase artifacts, review .agents/council/ for verdicts", err))
}

// --- Runtime process helpers ---

// lookPath is the function used to resolve binary paths. Package-level for testability.
var lookPath = exec.LookPath

// spawnClaudeDirectGlobal is the package-level wrapper for spawnClaudeDirectImpl that
// reads phasedPhaseTimeout from the global (for spawnClaudePhase / spawnDirectFn fallback paths).
func spawnClaudeDirectGlobal(prompt, cwd string, phaseNum int) error {
	return spawnClaudeDirectImpl(prompt, cwd, phaseNum, phasedPhaseTimeout)
}

// spawnDirectFn is the function used to spawn claude directly. Package-level for testability.
var spawnDirectFn = spawnClaudeDirectGlobal

// stallCheckInterval controls how frequently the stall watchdog goroutine fires in
// spawnClaudePhaseWithStream. Overridable in tests to exercise stall detection quickly.
var stallCheckInterval = 30 * time.Second

// --- Phase name helpers ---

// phaseNameToNum converts a phase name to a consolidated phase number (1-3).
func phaseNameToNum(name string) int {
	normalized := strings.ToLower(strings.TrimSpace(name))
	aliases := map[string]int{
		// Canonical 3-phase names
		"discovery":      1,
		"implementation": 2,
		"validation":     3,
		// Backward-compatible aliases (old 6-phase names map to consolidated phases)
		"research":    1,
		"plan":        1,
		"pre-mortem":  1,
		"premortem":   1,
		"pre_mortem":  1,
		"crank":       2,
		"implement":   2,
		"vibe":        3,
		"validate":    3,
		"post-mortem": 3,
		"postmortem":  3,
		"post_mortem": 3,
	}
	return aliases[normalized]
}
