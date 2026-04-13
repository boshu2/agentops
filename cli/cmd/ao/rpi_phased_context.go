package main

import (
	"cmp"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"
	"time"

	cliConfig "github.com/boshu2/agentops/cli/internal/config"
	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
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
	TmuxWorkers          int
	GCCityPath           string   // explicit city.toml directory for gc backend; empty = auto-discover
	ExecCommand          gcExecFn `json:"-"` // nil = exec.Command; injectable for testing
	LookPath             gcLookFn `json:"-"` // nil = exec.LookPath; injectable for testing
	Mixed                bool     // opt-in cross-vendor mixed-model execution
	NoBudget             bool
	BudgetSpec           string
	WorkingDir           string `json:"-"` // runtime-only; base directory for repo/worktree resolution
	RunID                string // Pre-seeded run ID (serve mode); empty = auto-generate
	NoDashboard          bool
	DiscoveryArtifact    string                // path to pre-validated discovery artifact; skips Phase 1 when set with --from=implementation
	StdoutWriter         io.Writer             `json:"-"` // runtime-only; suppresses raw Claude output when dashboard active
	OnSpawnCwdReady      func(spawnCwd string) `json:"-"` // called after worktree resolved; serve mode uses this to update mux root
}

// defaultPhasedEngineOptions returns options matching the default cobra flag values.
func defaultPhasedEngineOptions() phasedEngineOptions {
	return phasedEngineOptions{
		From:                 "discovery",
		TestFirst:            true,
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
		TmuxWorkers:          1,
		NoBudget:             false,
		BudgetSpec:           "",
	}
}

// phase is a thin alias for the internal Phase type used in cmd/ao.
type phase = cliRPI.Phase

var phases = cliRPI.Phases

// phasedState persists orchestrator state between phase spawns.
type phasedState struct {
	SchemaVersion   int                 `json:"schema_version"`
	Goal            string              `json:"goal"`
	EpicID          string              `json:"epic_id,omitempty"`
	TrackerMode     string              `json:"tracker_mode,omitempty"`
	TrackerReason   string              `json:"tracker_reason,omitempty"`
	Phase           int                 `json:"phase"`
	StartPhase      int                 `json:"start_phase"`
	Cycle           int                 `json:"cycle"`
	ParentEpic      string              `json:"parent_epic,omitempty"`
	FastPath        bool                `json:"fast_path"`
	TestFirst       bool                `json:"test_first"`
	SwarmFirst      bool                `json:"swarm_first"`
	Complexity      ComplexityLevel     `json:"complexity,omitempty"` // fast, standard, full
	ProgramPath     string              `json:"program_path,omitempty"`
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

// retryContext is a thin alias for the internal RetryContext type.
type retryContext = cliRPI.RetryContext

// finding is a thin alias for the internal Finding type.
type finding = cliRPI.Finding

// Instruction constants and phase context budgets delegate to internal/rpi.
var (
	phaseSummaryInstruction      = cliRPI.PhaseSummaryInstruction
	contextDisciplineInstruction = cliRPI.ContextDisciplineInstruction
	autodevProgramInstruction    = cliRPI.AutodevProgramInstruction
	phaseContextBudgets          = cliRPI.PhaseContextBudgets
)

// phasePrompts delegates to internal/rpi.
var phasePrompts = cliRPI.PhasePrompts

// Retry instruction constants and templates delegate to internal/rpi.
var (
	retryContextDisciplineInstruction = cliRPI.RetryContextDisciplineInstruction
	retryPhaseSummaryInstruction      = cliRPI.RetryPhaseSummaryInstruction
	retryPrompts                      = cliRPI.RetryPrompts
)

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
	return cmp.Or(strings.ToLower(strings.TrimSpace(mode)), "auto")
}

func effectiveRuntimeCommand(command string) string {
	return cmp.Or(strings.TrimSpace(command), "claude")
}

func effectiveAOCommand(command string) string {
	return cmp.Or(strings.TrimSpace(command), "ao")
}

func effectiveBDCommand(command string) string {
	return cmp.Or(strings.TrimSpace(command), "bd")
}

func effectiveTmuxCommand(command string) string {
	return cmp.Or(strings.TrimSpace(command), "tmux")
}

func validateRuntimeMode(mode string) error {
	switch normalizeRuntimeMode(mode) {
	case "auto", "direct", "stream", "tmux", "gc":
		return nil
	default:
		return fmt.Errorf("invalid runtime %q (valid: auto|direct|stream|tmux|gc)", mode)
	}
}

// phaseNameToNum delegates to internal/rpi.
func phaseNameToNum(name string) int { return cliRPI.PhaseNameToNum(name) }

// parsePhaseBudgetSpec delegates to internal/rpi.
func parsePhaseBudgetSpec(spec string) (map[int]time.Duration, error) {
	return cliRPI.ParsePhaseBudgetSpec(spec)
}

// defaultPhaseBudgetForComplexity delegates to internal/rpi.
func defaultPhaseBudgetForComplexity(complexity ComplexityLevel, phaseNum int) time.Duration {
	return cliRPI.DefaultPhaseBudgetForComplexity(complexity, phaseNum)
}

func budgetComplexityLevel(state *phasedState) ComplexityLevel {
	if state == nil {
		return ComplexityStandard
	}
	return cliRPI.BudgetComplexityLevel(state.FastPath, state.Complexity)
}

// resolvePhaseBudget delegates to internal/rpi.
func resolvePhaseBudget(state *phasedState, phaseNum int) (budget time.Duration, hasBudget bool, err error) {
	if state == nil {
		return 0, false, nil
	}
	return cliRPI.ResolvePhaseBudget(state.Opts.NoBudget, state.Opts.BudgetSpec, state.FastPath, state.Complexity, phaseNum)
}

// renderPreambleInstructions delegates to internal/rpi.
func renderPreambleInstructions(prompt *strings.Builder, data any) {
	cliRPI.RenderPreambleInstructions(prompt, data, VerbosePrintf)
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

	data := struct {
		Goal          string
		EpicID        string
		FastPath      bool
		TestFirst     bool
		SwarmFirst    bool
		Interactive   bool
		Mixed         bool
		PhaseNum      int
		ContextBudget string
		ProgramPath   string
		PlanFileMode  bool
		PlanFilePath  string
		TasklistMode  bool
	}{
		Goal:          state.Goal,
		EpicID:        state.EpicID,
		FastPath:      state.FastPath,
		TestFirst:     state.TestFirst,
		SwarmFirst:    state.SwarmFirst,
		Interactive:   state.Opts.Interactive,
		Mixed:         state.Opts.Mixed,
		PhaseNum:      phaseNum,
		ContextBudget: phaseContextBudgets[phaseNum],
		ProgramPath:   state.ProgramPath,
		PlanFileMode:  isPlanFileEpic(state.EpicID),
		PlanFilePath:  planFileFromEpic(state.EpicID),
		TasklistMode:  state.TrackerMode == "tasklist" && strings.TrimSpace(state.EpicID) == "",
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	var prompt strings.Builder
	renderPreambleInstructions(&prompt, data)

	// Cross-phase context for phases 2+ — prefer structured handoffs, fall back to raw summaries
	if phaseNum >= 2 {
		handoffs, _ := readAllHandoffs(cwd, phaseNum)
		if len(handoffs) > 0 {
			manifest := defaultPhaseManifests[phaseNum]
			ctx := buildHandoffContext(handoffs, manifest)
			prompt.WriteString(ctx)
			prompt.WriteString("\n\n")
		} else {
			// Fallback: legacy summary-based context (for runs that predate structured handoffs)
			ctx := buildPhaseContext(cwd, state, phaseNum)
			if ctx != "" {
				prompt.WriteString(ctx)
				prompt.WriteString("\n\n")
			}
		}
	}

	prompt.WriteString(buf.String())

	// Write provenance audit trail
	if state.RunID != "" {
		if err := writePromptAuditTrail(cwd, state.RunID, phaseNum, prompt.String()); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to write prompt audit trail for phase %d: %v\n", phaseNum, err)
		}
	}

	return prompt.String(), nil
}

// buildPhaseContext delegates to internal/rpi.
func buildPhaseContext(cwd string, state *phasedState, phaseNum int) string {
	return cliRPI.BuildPhaseContext(cwd, state.Goal, state.Verdicts, phaseNum)
}

// readPhaseSummaries delegates to internal/rpi.
func readPhaseSummaries(cwd string, currentPhase int) string {
	return cliRPI.ReadPhaseSummaries(cwd, currentPhase)
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
		ProgramPath   string
		PlanFileMode  bool
		PlanFilePath  string
		TasklistMode  bool
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
		ProgramPath:   state.ProgramPath,
		PlanFileMode:  isPlanFileEpic(state.EpicID),
		PlanFilePath:  planFileFromEpic(state.EpicID),
		TasklistMode:  state.TrackerMode == "tasklist" && strings.TrimSpace(state.EpicID) == "",
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

	// 3. Retry-specific context discipline (avoid repeating prior work)
	prompt.WriteString("\n")
	prompt.WriteString(retryContextDisciplineInstruction)
	prompt.WriteString("\n\n")

	// 4. Retry phase summary instruction (include prior phase outcomes)
	prompt.WriteString(retryPhaseSummaryInstruction)
	prompt.WriteString("\n\n")

	// 5. Retry skill invocation (last — the actual command with findings)
	prompt.WriteString(skillInvocation)

	// Write provenance audit trail for retry
	if state.RunID != "" {
		if err := writePromptAuditTrail(cwd, state.RunID, phaseNum, prompt.String()); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to write retry audit trail for phase %d: %v\n", phaseNum, err)
		}
	}

	return prompt.String(), nil
}

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
