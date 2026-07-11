//go:build legacy

// practices: [microservices, design-by-contract]
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/adapters/vendorimage/codexruntime"
	"github.com/boshu2/agentops/cli/internal/bridge"
	"github.com/boshu2/agentops/cli/internal/pool"
	"github.com/boshu2/agentops/cli/internal/ratchet"
	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
	"github.com/boshu2/agentops/cli/internal/storage"
	"github.com/boshu2/agentops/cli/internal/types"
	verdictparse "github.com/boshu2/agentops/cli/internal/verdict"
)

var (
	codexStartLimit            int
	codexStartQuery            string
	codexStartNoMaintenance    bool
	codexStopSessionID         string
	codexStopTranscriptPath    string
	codexStopAutoExtract       bool
	codexStopNoHistoryFallback bool
	codexStopCloseLoop         bool
	codexStopNoCloseLoop       bool
	codexStatusDays            int
	codexDispatchPacketPath    string
)

var codexImageHealthRunCheck = runCodexImageHealthCheck

// codexImageHealthDefaultCheckTimeout is the default per-check budget for
// image-health checks. The caller context usually has no deadline, so each
// check gets its own bound to keep a wedged script from hanging the command.
const codexImageHealthDefaultCheckTimeout = 30 * time.Second

// codexImageHealthWaitDelay bounds how long Wait blocks on the stdout/stderr
// pipe copy after the check's context expires. Without it, a grandchild that
// inherits the pipes keeps Run blocked past the per-check budget even though
// the direct child was killed.
const codexImageHealthWaitDelay = 2 * time.Second

var codexImageHealthCheckTimeout = codexImageHealthDefaultCheckTimeout

const (
	runtimeKindClaude   = codexruntime.RuntimeKindClaude
	runtimeKindCodex    = codexruntime.RuntimeKindCodex
	runtimeKindOpenCode = codexruntime.RuntimeKindOpenCode
	runtimeKindUnknown  = codexruntime.RuntimeKindUnknown

	lifecycleModeHookCapable   = codexruntime.LifecycleModeHookCapable
	lifecycleModeCodexHookless = codexruntime.LifecycleModeCodexHookless
	lifecycleModeManual        = codexruntime.LifecycleModeManual
)

type lifecycleRuntimeProfile = codexruntime.LifecycleRuntimeProfile

type codexLifecycleEvent struct {
	SessionID           string `json:"session_id,omitempty"`
	ThreadName          string `json:"thread_name,omitempty"`
	Query               string `json:"query,omitempty"`
	Timestamp           string `json:"timestamp"`
	TranscriptPath      string `json:"transcript_path,omitempty"`
	TranscriptSource    string `json:"transcript_source,omitempty"`
	SyntheticTranscript bool   `json:"synthetic_transcript,omitempty"`
	StartupContextPath  string `json:"startup_context_path,omitempty"`
	MemoryPath          string `json:"memory_path,omitempty"`
	Status              string `json:"status,omitempty"`
	Summary             string `json:"summary,omitempty"`
	HandoffPath         string `json:"handoff_path,omitempty"`
}

type codexLifecycleState struct {
	SchemaVersion int                     `json:"schema_version"`
	Runtime       lifecycleRuntimeProfile `json:"runtime"`
	LastStart     *codexLifecycleEvent    `json:"last_start,omitempty"`
	LastStop      *codexLifecycleEvent    `json:"last_stop,omitempty"`
	UpdatedAt     string                  `json:"updated_at"`
}

type codexStartResult struct {
	Runtime            lifecycleRuntimeProfile  `json:"runtime"`
	ContextQuery       string                   `json:"context_query,omitempty"`
	StartupContextPath string                   `json:"startup_context_path"`
	MemoryPath         string                   `json:"memory_path,omitempty"`
	CloseLoop          *flywheelCloseLoopResult `json:"close_loop,omitempty"`
	Flywheel           *flywheelBrief           `json:"flywheel,omitempty"`
	Briefings          []codexArtifactRef       `json:"briefings,omitempty"`
	Learnings          []learning               `json:"learnings,omitempty"`
	Patterns           []pattern                `json:"patterns,omitempty"`
	Findings           []knowledgeFinding       `json:"findings,omitempty"`
	RecentSessions     []session                `json:"recent_sessions,omitempty"`
	NextWork           []nextWorkItem           `json:"next_work,omitempty"`
	Research           []codexArtifactRef       `json:"research,omitempty"`
	StatePath          string                   `json:"state_path"`
}

type codexEnsureStartResult struct {
	Runtime            lifecycleRuntimeProfile `json:"runtime"`
	Performed          bool                    `json:"performed"`
	Reason             string                  `json:"reason,omitempty"`
	SessionID          string                  `json:"session_id,omitempty"`
	ContextQuery       string                  `json:"context_query,omitempty"`
	StartupContextPath string                  `json:"startup_context_path,omitempty"`
	MemoryPath         string                  `json:"memory_path,omitempty"`
	StatePath          string                  `json:"state_path,omitempty"`
}

type codexStopResult struct {
	Runtime             lifecycleRuntimeProfile  `json:"runtime"`
	TranscriptPath      string                   `json:"transcript_path"`
	TranscriptSource    string                   `json:"transcript_source"`
	SyntheticTranscript bool                     `json:"synthetic_transcript,omitempty"`
	Session             SessionCloseResult       `json:"session"`
	CloseLoop           *flywheelCloseLoopResult `json:"close_loop,omitempty"`
	MemoryPath          string                   `json:"memory_path,omitempty"`
	StatePath           string                   `json:"state_path"`
}

type codexEnsureStopResult struct {
	Runtime             lifecycleRuntimeProfile `json:"runtime"`
	Performed           bool                    `json:"performed"`
	Reason              string                  `json:"reason,omitempty"`
	SessionID           string                  `json:"session_id,omitempty"`
	TranscriptPath      string                  `json:"transcript_path,omitempty"`
	TranscriptSource    string                  `json:"transcript_source,omitempty"`
	SyntheticTranscript bool                    `json:"synthetic_transcript,omitempty"`
	HandoffPath         string                  `json:"handoff_path,omitempty"`
	MemoryPath          string                  `json:"memory_path,omitempty"`
	StatePath           string                  `json:"state_path,omitempty"`
}

type codexCaptureHealth struct {
	SessionsIndexed   int    `json:"sessions_indexed"`
	LastForgeTime     string `json:"last_forge_time,omitempty"`
	LastForgeAge      string `json:"last_forge_age,omitempty"`
	PendingKnowledge  int    `json:"pending_knowledge"`
	PendingQuarantine int    `json:"pending_quarantine"`
}

type codexRetrievalHealth struct {
	Learnings int `json:"learnings"`
	Patterns  int `json:"patterns"`
	Findings  int `json:"findings"`
	NextWork  int `json:"next_work"`
	Briefings int `json:"briefings"`
	Research  int `json:"research"`
}

type codexPromotionHealth struct {
	PendingPool  int `json:"pending_pool"`
	StagedPool   int `json:"staged_pool"`
	RejectedPool int `json:"rejected_pool"`
}

type codexCitationHealth struct {
	WindowDays       int `json:"window_days"`
	Total            int `json:"total"`
	Deduped          int `json:"deduped"`
	UniqueArtifacts  int `json:"unique_artifacts"`
	UniqueSessions   int `json:"unique_sessions"`
	UniqueWorkspaces int `json:"unique_workspaces"`
	Retrieved        int `json:"retrieved"`
	Reference        int `json:"reference"`
	Applied          int `json:"applied"`
}

type codexStatusResult struct {
	Runtime   lifecycleRuntimeProfile `json:"runtime"`
	State     *codexLifecycleState    `json:"state,omitempty"`
	Flywheel  *flywheelBrief          `json:"flywheel,omitempty"`
	Capture   codexCaptureHealth      `json:"capture"`
	Retrieval codexRetrievalHealth    `json:"retrieval"`
	Promotion codexPromotionHealth    `json:"promotion"`
	Citations codexCitationHealth     `json:"citations"`
}

type codexImageHealthResult struct {
	SchemaVersion         int                           `json:"schema_version"`
	Status                string                        `json:"status"`
	CheckedAt             string                        `json:"checked_at"`
	CWD                   string                        `json:"cwd"`
	LifecycleStatePath    string                        `json:"lifecycle_state_path"`
	LifecycleStateMutated bool                          `json:"lifecycle_state_mutated"`
	Summary               codexImageHealthSummary       `json:"summary"`
	Checks                []codexImageHealthCheckResult `json:"checks"`
}

type codexImageHealthSummary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
	Slow    int `json:"slow"`
}

type codexImageHealthCheckSpec struct {
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	Command             []string `json:"command"`
	RepairHint          string   `json:"repair_hint"`
	OptionalWhenMissing bool     `json:"optional_when_missing,omitempty"`
}

type codexImageHealthCheckResult struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Command     []string `json:"command"`
	RepairHint  string   `json:"repair_hint"`
	ExitCode    int      `json:"exit_code"`
	DurationMS  int64    `json:"duration_ms"`
	TimedOut    bool     `json:"timed_out,omitempty"`
	Slow        bool     `json:"slow,omitempty"`
	Stdout      string   `json:"stdout,omitempty"`
	Stderr      string   `json:"stderr,omitempty"`
	Error       string   `json:"error,omitempty"`
	Optional    bool     `json:"optional,omitempty"`
}

var codexCmd = &cobra.Command{
	Use:   "codex",
	Short: "Codex lifecycle commands (fallback for pre-v0.115.0; native hooks preferred)",
	Args:  cobra.NoArgs,
	Long: `Codex lifecycle commands for the AgentOps knowledge flywheel.

Codex CLI v0.115.0+ supports native hooks — prefer those for automatic lifecycle.
These commands remain as a fallback for older Codex versions without native hook support.

  ao codex start   Surface prior context and run safe maintenance
  ao codex stop    Forge the current session and queue closeout learnings
  ao codex status  Show lifecycle health and flywheel status`,
}

var codexStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a Codex session with explicit flywheel maintenance (fallback for pre-v0.115.0)",
	Args:  cobra.NoArgs,
	RunE:  runCodexStart,
}

var codexEnsureStartCmd = &cobra.Command{
	Use:   "ensure-start",
	Short: "Ensure Codex startup context exists once per thread",
	Args:  cobra.NoArgs,
	RunE:  runCodexEnsureStart,
}

var codexStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Close a Codex session explicitly (fallback for pre-v0.115.0)",
	Args:  cobra.NoArgs,
	RunE:  runCodexStop,
}

var codexEnsureStopCmd = &cobra.Command{
	Use:   "ensure-stop",
	Short: "Ensure Codex closeout runs once per thread",
	Args:  cobra.NoArgs,
	RunE:  runCodexEnsureStop,
}

var codexStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Codex lifecycle health (native hooks detected when available)",
	Args:  cobra.NoArgs,
	RunE:  runCodexStatus,
}

var codexDispatchCmd = &cobra.Command{
	Use:     "dispatch --packet <path>",
	Aliases: []string{"run"},
	Short:   "Run a non-mutating Codex task packet and write a receipt",
	Args:    cobra.NoArgs,
	RunE:    runCodexDispatch,
}

var codexImageHealthCmd = &cobra.Command{
	Use:   "image-health",
	Short: "Run read-only Codex image and runtime health checks",
	Args:  cobra.NoArgs,
	RunE:  runCodexImageHealth,
}

func init() {
	codexCmd.GroupID = "workflow"
	rootCmd.AddCommand(codexCmd)
	codexCmd.AddCommand(codexStartCmd, codexEnsureStartCmd, codexStopCmd, codexEnsureStopCmd, codexStatusCmd, codexDispatchCmd, codexImageHealthCmd)

	codexImageHealthCmd.Flags().DurationVar(&codexImageHealthCheckTimeout, "check-timeout", codexImageHealthDefaultCheckTimeout, "Per-check timeout budget for image health checks")

	codexStartCmd.Flags().IntVar(&codexStartLimit, "limit", 3, "Maximum artifacts to surface per category")
	codexStartCmd.Flags().StringVar(&codexStartQuery, "query", "", "Optional startup query (defaults to the current Codex thread name)")
	codexStartCmd.Flags().BoolVar(&codexStartNoMaintenance, "no-maintenance", false, "Skip safe close-loop maintenance on start")
	codexEnsureStartCmd.Flags().IntVar(&codexStartLimit, "limit", 3, "Maximum artifacts to surface per category")
	codexEnsureStartCmd.Flags().StringVar(&codexStartQuery, "query", "", "Optional startup query (defaults to the current Codex thread name)")
	codexEnsureStartCmd.Flags().BoolVar(&codexStartNoMaintenance, "no-maintenance", false, "Skip safe close-loop maintenance on start")

	codexStopCmd.Flags().StringVar(&codexStopSessionID, "session", "", "Codex session ID to close (defaults to the active thread)")
	codexStopCmd.Flags().StringVar(&codexStopTranscriptPath, "transcript", "", "Explicit transcript path to forge instead of runtime discovery")
	codexStopCmd.Flags().BoolVar(&codexStopAutoExtract, "auto-extract", true, "Write lightweight learnings and handoff artifacts during closeout")
	codexStopCmd.Flags().BoolVar(&codexStopNoHistoryFallback, "no-history-fallback", false, "Disable history.jsonl fallback when no archived Codex transcript exists")
	codexStopCmd.Flags().BoolVar(&codexStopCloseLoop, "close-loop", false, "Run mutating flywheel close-loop maintenance after forging")
	codexStopCmd.Flags().BoolVar(&codexStopNoCloseLoop, "no-close-loop", false, "Skip flywheel close-loop maintenance after forging")
	codexEnsureStopCmd.Flags().StringVar(&codexStopSessionID, "session", "", "Codex session ID to close (defaults to the active thread)")
	codexEnsureStopCmd.Flags().StringVar(&codexStopTranscriptPath, "transcript", "", "Explicit transcript path to forge instead of runtime discovery")
	codexEnsureStopCmd.Flags().BoolVar(&codexStopAutoExtract, "auto-extract", true, "Write lightweight learnings and handoff artifacts during closeout")
	codexEnsureStopCmd.Flags().BoolVar(&codexStopNoHistoryFallback, "no-history-fallback", false, "Disable history.jsonl fallback when no archived Codex transcript exists")
	codexEnsureStopCmd.Flags().BoolVar(&codexStopCloseLoop, "close-loop", false, "Run mutating flywheel close-loop maintenance after forging")
	codexEnsureStopCmd.Flags().BoolVar(&codexStopNoCloseLoop, "no-close-loop", false, "Skip flywheel close-loop maintenance after forging")

	codexStatusCmd.Flags().IntVar(&codexStatusDays, "days", 7, "Citation window in days for Codex lifecycle health")

	codexDispatchCmd.Flags().StringVar(&codexDispatchPacketPath, "packet", "", "Path to a Codex task packet JSON file")
	_ = codexDispatchCmd.MarkFlagRequired("packet")
}

func detectLifecycleRuntimeProfile() lifecycleRuntimeProfile {
	return codexruntime.DetectLifecycleRuntimeProfile()
}

func detectCodexLifecycleProfile() lifecycleRuntimeProfile {
	return codexruntime.DetectCodexLifecycleProfile()
}

func synthesizeCodexHistoryTranscript(cwd, sessionID string) (string, error) {
	return codexruntime.SynthesizeCodexHistoryTranscript(cwd, sessionID)
}

func extractSessionIDFromCodexArchivedPath(path string) string {
	return codexruntime.ExtractSessionIDFromCodexArchivedPath(path)
}

func runCodexStart(cmd *cobra.Command, args []string) error {
	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}
	result, err := performCodexStart(cwd)
	if err != nil {
		return err
	}
	return outputCodexStartResult(result)
}

func runCodexEnsureStart(cmd *cobra.Command, args []string) error {
	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}
	if err := ensureCodexLifecycleDirs(cwd); err != nil {
		return err
	}

	profile := detectCodexLifecycleProfile()
	sessionID := profile.SessionID
	if strings.TrimSpace(sessionID) == "" {
		sessionID = resolveSessionID("")
	}
	state, statePath, err := loadOrInitCodexLifecycleState(cwd)
	if err != nil {
		return err
	}
	if codexStartAlreadyStarted(state, sessionID) {
		existingSessionID := sessionID
		if state.LastStart != nil {
			existingSessionID = firstNonEmptyTrimmed(existingSessionID, state.LastStart.SessionID)
		}
		return outputCodexEnsureStartResult(codexEnsureStartResult{
			Runtime:            profile,
			Performed:          false,
			Reason:             "startup already recorded for this Codex thread",
			SessionID:          existingSessionID,
			ContextQuery:       firstNonEmptyTrimmed(codexStartQuery, profile.ThreadName, "codex startup"),
			StartupContextPath: firstNonEmptyLifecycleField(state, func(event *codexLifecycleEvent) string { return event.StartupContextPath }),
			MemoryPath:         firstNonEmptyLifecycleField(state, func(event *codexLifecycleEvent) string { return event.MemoryPath }),
			StatePath:          statePath,
		})
	}

	result, err := performCodexStart(cwd)
	if err != nil {
		return err
	}
	return outputCodexEnsureStartResult(codexEnsureStartResult{
		Runtime:            result.Runtime,
		Performed:          true,
		Reason:             "startup recorded for current Codex thread",
		SessionID:          firstNonEmptyTrimmed(sessionID, result.Runtime.SessionID),
		ContextQuery:       result.ContextQuery,
		StartupContextPath: result.StartupContextPath,
		MemoryPath:         result.MemoryPath,
		StatePath:          result.StatePath,
	})
}

func codexStartAlreadyStarted(state *codexLifecycleState, sessionID string) bool {
	if state == nil || state.LastStart == nil {
		return false
	}
	if strings.TrimSpace(sessionID) == "" || strings.TrimSpace(state.LastStart.SessionID) != strings.TrimSpace(sessionID) {
		return false
	}
	startupContextPath := strings.TrimSpace(state.LastStart.StartupContextPath)
	if startupContextPath == "" {
		return false
	}
	return fileExists(startupContextPath)
}

func performCodexStart(cwd string) (codexStartResult, error) {
	showNewUserWelcome := codexShouldShowNewUserWelcome(cwd)
	if err := ensureCodexLifecycleDirs(cwd); err != nil {
		return codexStartResult{}, err
	}

	profile := detectCodexLifecycleProfile()
	sessionID := profile.SessionID
	if strings.TrimSpace(sessionID) == "" {
		sessionID = resolveSessionID("")
	}

	query := strings.TrimSpace(codexStartQuery)
	if query == "" {
		query = strings.TrimSpace(profile.ThreadName)
	}
	if query == "" {
		query = "codex startup"
	}

	var closeLoop *flywheelCloseLoopResult
	if !codexStartNoMaintenance {
		threshold, err := time.ParseDuration(defaultAutoPromoteThreshold)
		if err != nil {
			return codexStartResult{}, fmt.Errorf("parse default close-loop threshold: %w", err)
		}
		result, err := performFlywheelCloseLoopWithCitationMutation(cwd, filepath.Join(".agents", "knowledge", "pending"), threshold, true, false)
		if err != nil {
			return codexStartResult{}, fmt.Errorf("run codex startup maintenance: %w", err)
		}
		closeLoop = &result
	}

	briefings, learnings, patterns, findings, recentSessions, nextWork, research := collectCodexStartupArtifacts(cwd, query, codexStartLimit)
	recordLookupCitations(cwd, learnings, patterns, findings, sessionID, query, "retrieved")

	memoryPath, err := syncCodexMemory(cwd)
	if err != nil {
		VerbosePrintf("Warning: codex memory sync: %v\n", err)
	}

	startupContextPath, err := writeCodexStartupContext(cwd, profile, query, briefings, learnings, patterns, findings, recentSessions, nextWork, research, showNewUserWelcome)
	if err != nil {
		return codexStartResult{}, fmt.Errorf("write codex startup context: %w", err)
	}

	state, statePath, err := loadOrInitCodexLifecycleState(cwd)
	if err != nil {
		return codexStartResult{}, err
	}
	state.Runtime = profile
	state.LastStart = &codexLifecycleEvent{
		SessionID:          sessionID,
		ThreadName:         profile.ThreadName,
		Query:              query,
		Timestamp:          time.Now().UTC().Format(time.RFC3339),
		StartupContextPath: startupContextPath,
		MemoryPath:         memoryPath,
		Status:             lifecycleModeCodexHookless,
		Summary:            fmt.Sprintf("surfaced %d learnings, %d patterns, %d findings", len(learnings), len(patterns), len(findings)),
	}
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := saveCodexLifecycleState(statePath, state); err != nil {
		return codexStartResult{}, err
	}

	return codexStartResult{
		Runtime:            profile,
		ContextQuery:       query,
		StartupContextPath: startupContextPath,
		MemoryPath:         memoryPath,
		CloseLoop:          closeLoop,
		Flywheel:           loadFlywheelBrief(cwd),
		Briefings:          briefings,
		Learnings:          learnings,
		Patterns:           patterns,
		Findings:           findings,
		RecentSessions:     recentSessions,
		NextWork:           nextWork,
		Research:           research,
		StatePath:          statePath,
	}, nil
}

func runCodexStop(cmd *cobra.Command, args []string) error {
	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}
	result, err := performCodexStop(cwd)
	if err != nil {
		return err
	}
	return outputCodexStopResult(result)
}

func runCodexEnsureStop(cmd *cobra.Command, args []string) error {
	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}
	result, err := performCodexStop(cwd)
	if err != nil {
		return err
	}
	performed := result.Session.Status != "already_closed"
	return outputCodexEnsureStopResult(codexEnsureStopResult{
		Runtime:             result.Runtime,
		Performed:           performed,
		Reason:              ensureStopReason(result),
		SessionID:           result.Session.SessionID,
		TranscriptPath:      result.TranscriptPath,
		TranscriptSource:    result.TranscriptSource,
		SyntheticTranscript: result.SyntheticTranscript,
		HandoffPath:         result.Session.HandoffWritten,
		MemoryPath:          result.MemoryPath,
		StatePath:           result.StatePath,
	})
}

func performCodexStop(cwd string) (codexStopResult, error) {
	if err := ensureCodexLifecycleDirs(cwd); err != nil {
		return codexStopResult{}, err
	}

	profile := detectCodexLifecycleProfile()
	state, statePath, err := loadOrInitCodexLifecycleState(cwd)
	if err != nil {
		return codexStopResult{}, err
	}
	sessionID := strings.TrimSpace(codexStopSessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(profile.SessionID)
	}

	transcriptPath := strings.TrimSpace(codexStopTranscriptPath)
	transcriptSource := "explicit"
	syntheticTranscript := false

	if transcriptPath == "" {
		transcriptPath, transcriptSource, syntheticTranscript, sessionID, err = resolveCodexStopTranscript(cwd, sessionID, codexStopNoHistoryFallback)
		if err != nil {
			return codexStopResult{}, err
		}
	}

	if codexStopAlreadyClosed(state, sessionID, transcriptPath) {
		return buildCodexStopAlreadyClosedResult(profile, state, statePath, sessionID, transcriptPath, transcriptSource, syntheticTranscript), nil
	}

	closeResult, err := forgeExtractReportWithOptions(transcriptPath, cwd, codexStopAutoExtract, false)
	if err != nil {
		return codexStopResult{}, err
	}

	var closeLoop *flywheelCloseLoopResult
	runCloseLoop := codexStopCloseLoop && !codexStopNoCloseLoop
	if runCloseLoop {
		threshold, err := time.ParseDuration(defaultAutoPromoteThreshold)
		if err != nil {
			return codexStopResult{}, fmt.Errorf("parse default close-loop threshold: %w", err)
		}
		result, err := performFlywheelCloseLoopWithCitationMutation(cwd, filepath.Join(".agents", "knowledge", "pending"), threshold, true, true)
		if err != nil {
			return codexStopResult{}, fmt.Errorf("run codex close-loop maintenance: %w", err)
		}
		closeLoop = &result
	}
	if runCloseLoop {
		if err := performHooklessSessionEndMaintenance(cwd); err != nil {
			VerbosePrintf("Warning: codex session-end maintenance: %v\n", err)
		}
	}

	memoryPath, err := syncCodexMemory(cwd)
	if err != nil {
		VerbosePrintf("Warning: codex memory sync: %v\n", err)
	}
	state.Runtime = profile
	state.LastStop = &codexLifecycleEvent{
		SessionID:           closeResult.SessionID,
		ThreadName:          profile.ThreadName,
		Timestamp:           time.Now().UTC().Format(time.RFC3339),
		TranscriptPath:      transcriptPath,
		TranscriptSource:    transcriptSource,
		SyntheticTranscript: syntheticTranscript,
		MemoryPath:          memoryPath,
		Status:              closeResult.Status,
		Summary:             closeResult.Message,
		HandoffPath:         closeResult.HandoffWritten,
	}
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := saveCodexLifecycleState(statePath, state); err != nil {
		return codexStopResult{}, err
	}

	return codexStopResult{
		Runtime:             profile,
		TranscriptPath:      transcriptPath,
		TranscriptSource:    transcriptSource,
		SyntheticTranscript: syntheticTranscript,
		Session:             closeResult,
		CloseLoop:           closeLoop,
		MemoryPath:          memoryPath,
		StatePath:           statePath,
	}, nil
}

func codexStopAlreadyClosed(state *codexLifecycleState, sessionID, transcriptPath string) bool {
	if state == nil || state.LastStop == nil {
		return false
	}
	return bridge.CodexStopAlreadyClosed(
		state.LastStop.SessionID,
		state.LastStop.TranscriptPath,
		sessionID,
		transcriptPath,
	)
}

func buildCodexStopAlreadyClosedResult(profile lifecycleRuntimeProfile, state *codexLifecycleState, statePath, sessionID, transcriptPath, transcriptSource string, syntheticTranscript bool) codexStopResult {
	lastStop := &codexLifecycleEvent{}
	if state != nil && state.LastStop != nil {
		lastStop = state.LastStop
	}

	resolvedSessionID := firstNonEmptyTrimmed(sessionID, lastStop.SessionID)
	resolvedTranscriptPath := firstNonEmptyTrimmed(transcriptPath, lastStop.TranscriptPath)
	resolvedTranscriptSource := firstNonEmptyTrimmed(transcriptSource, lastStop.TranscriptSource)
	if resolvedTranscriptSource == "" {
		resolvedTranscriptSource = "explicit"
	}

	return codexStopResult{
		Runtime:             profile,
		TranscriptPath:      resolvedTranscriptPath,
		TranscriptSource:    resolvedTranscriptSource,
		SyntheticTranscript: syntheticTranscript || lastStop.SyntheticTranscript,
		Session: SessionCloseResult{
			SessionID:      resolvedSessionID,
			Transcript:     resolvedTranscriptPath,
			Status:         "already_closed",
			Message:        "Codex closeout already recorded for this session",
			HandoffWritten: lastStop.HandoffPath,
		},
		MemoryPath: firstNonEmptyTrimmed(lastStop.MemoryPath),
		StatePath:  statePath,
	}
}

func ensureStopReason(result codexStopResult) string {
	return bridge.EnsureStopReason(result.Session.Status)
}

func runCodexStatus(cmd *cobra.Command, args []string) error {
	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}

	profile := detectCodexLifecycleProfile()
	state, _, err := loadOrInitCodexLifecycleState(cwd)
	if err != nil {
		return err
	}

	result := codexStatusResult{
		Runtime:   profile,
		State:     state,
		Flywheel:  loadFlywheelBrief(cwd),
		Capture:   collectCodexCaptureHealth(cwd),
		Retrieval: collectCodexRetrievalHealth(cwd),
		Promotion: collectCodexPromotionHealth(cwd),
		Citations: collectCodexCitationHealth(cwd, codexStatusDays),
	}
	return outputCodexStatusResult(result)
}

func runCodexDispatch(cmd *cobra.Command, args []string) error {
	packet, err := loadCodexTaskPacket(codexDispatchPacketPath)
	if err != nil {
		return err
	}
	receipt, runErr := performCodexDispatch(packet)
	if receipt.ReceiptID != "" {
		if err := outputCodexDispatchResult(receipt); err != nil {
			return err
		}
	}
	return runErr
}

func runCodexImageHealth(cmd *cobra.Command, args []string) error {
	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}
	result := performCodexImageHealth(cmd.Context(), resolveCodexImageHealthRoot(cwd))
	if err := outputCodexImageHealthResult(result); err != nil {
		return err
	}
	if result.Status == "FAIL" {
		return fmt.Errorf("codex image health failed: %d check(s) failed", result.Summary.Failed)
	}
	return nil
}

func performCodexImageHealth(ctx context.Context, cwd string) codexImageHealthResult {
	if ctx == nil {
		ctx = context.Background()
	}
	checkedAt := time.Now().UTC().Format(time.RFC3339)
	lifecycleStatePath := filepath.Join(cwd, ".agents", "ao", "codex", "state.json")
	before := codexImageHealthFileFingerprint(lifecycleStatePath)

	result := codexImageHealthResult{
		SchemaVersion:      1,
		Status:             "PASS",
		CheckedAt:          checkedAt,
		CWD:                cwd,
		LifecycleStatePath: lifecycleStatePath,
	}
	for _, spec := range codexImageHealthCheckSpecs() {
		check := codexImageHealthRunCheck(ctx, cwd, spec)
		result.Checks = append(result.Checks, check)
	}

	after := codexImageHealthFileFingerprint(lifecycleStatePath)
	result.LifecycleStateMutated = before != after
	if result.LifecycleStateMutated {
		result.Checks = append(result.Checks, codexImageHealthCheckResult{
			Name:        "codex-lifecycle-state-nonmutation",
			Description: "Image health must not mutate Codex lifecycle state.",
			Status:      "FAIL",
			RepairHint:  "Run image health without lifecycle start/stop/ensure commands.",
			ExitCode:    1,
			Error:       "Codex lifecycle state changed during image health check.",
		})
	}
	result.Summary = summarizeCodexImageHealthChecks(result.Checks)
	if result.Summary.Failed > 0 {
		result.Status = "FAIL"
	}
	return result
}

func resolveCodexImageHealthRoot(cwd string) string {
	root, err := resolveRepoRoot(cwd)
	if err == nil && strings.TrimSpace(root) != "" {
		return root
	}
	return cwd
}

func codexImageHealthCheckSpecs() []codexImageHealthCheckSpec {
	return []codexImageHealthCheckSpec{
		{
			Name:        "codex-image-verify",
			Description: "Codex image bundle has complete twins and synchronized hashes.",
			Command:     []string{"bash", "images/codex/verify.sh"},
			RepairHint:  "bash images/codex/verify.sh; if hashes drift, run bash scripts/regen-codex-hashes.sh",
		},
		{
			Name:        "codex-parity-audit",
			Description: "Codex artifacts do not contain unsupported Claude-only primitives or stale runtime syntax.",
			Command:     []string{"bash", "scripts/audit-codex-parity.sh"},
			RepairHint:  "bash scripts/audit-codex-parity.sh --skill <name>",
		},
		{
			Name:        "codex-override-coverage",
			Description: "Codex override catalog deliberately covers every skill treatment.",
			Command:     []string{"bash", "scripts/validate-codex-override-coverage.sh"},
			RepairHint:  "bash scripts/validate-codex-override-coverage.sh",
		},
		{
			Name:        "codex-generated-artifacts",
			Description: "Checked-in Codex artifacts and generated metadata are current.",
			Command:     []string{"bash", "scripts/validate-codex-generated-artifacts.sh", "--scope", "worktree"},
			RepairHint:  "bash scripts/refresh-codex-artifacts.sh --scope worktree",
		},
		{
			Name:        "codex-rpi-contract",
			Description: "Codex skill chaining and RPI contract defaults are intact.",
			Command:     []string{"bash", "scripts/validate-codex-rpi-contract.sh"},
			RepairHint:  "bash scripts/validate-codex-rpi-contract.sh",
		},
		{
			Name:        "codex-lifecycle-guards",
			Description: "Codex entry/closeout skills use ensure-start/ensure-stop and current tracker guidance.",
			Command:     []string{"bash", "scripts/validate-codex-lifecycle-guards.sh"},
			RepairHint:  "bash scripts/validate-codex-lifecycle-guards.sh",
		},
		{
			Name:                "codex-headless-runtime-skills",
			Description:         "Headless runtime skill checks are available and pass for Codex.",
			Command:             []string{"bash", "scripts/validate-headless-runtime-skills.sh"},
			RepairHint:          "bash scripts/validate-headless-runtime-skills.sh",
			OptionalWhenMissing: true,
		},
	}
}

func runCodexImageHealthCheck(ctx context.Context, cwd string, spec codexImageHealthCheckSpec) codexImageHealthCheckResult {
	result := codexImageHealthCheckResult{
		Name:        spec.Name,
		Description: spec.Description,
		Command:     append([]string(nil), spec.Command...),
		RepairHint:  spec.RepairHint,
	}
	if len(spec.Command) == 0 {
		result.Status = "FAIL"
		result.ExitCode = -1
		result.Error = "health check command is empty"
		return result
	}
	if missing := codexImageHealthMissingScript(cwd, spec.Command); missing != "" {
		if spec.OptionalWhenMissing {
			result.Status = "SKIP"
			result.Optional = true
			result.ExitCode = 0
			result.Error = "optional health check unavailable: " + missing
			return result
		}
		result.Status = "FAIL"
		result.ExitCode = -1
		result.Error = "required health check unavailable: " + missing
		return result
	}

	budget := codexImageHealthCheckTimeout
	if budget <= 0 {
		budget = codexImageHealthDefaultCheckTimeout
	}
	checkCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	started := time.Now()
	cmd := exec.CommandContext(checkCtx, spec.Command[0], spec.Command[1:]...)
	cmd.Dir = cwd
	cmd.WaitDelay = codexImageHealthWaitDelay
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	elapsed := time.Since(started)
	timedOut := errors.Is(checkCtx.Err(), context.DeadlineExceeded)
	result.DurationMS = elapsed.Milliseconds()
	result.TimedOut = timedOut
	result.Slow = timedOut || elapsed >= budget*4/5
	result.Stdout = codexImageHealthExcerpt(stdout.String())
	result.Stderr = codexImageHealthExcerpt(stderr.String())
	result.ExitCode = codexDispatchExitCode(err, timedOut)
	if timedOut {
		result.Status = "FAIL"
		result.Error = fmt.Sprintf("check timed out after %s (per-check budget %s)", elapsed.Round(time.Millisecond), budget)
		return result
	}
	if err != nil {
		result.Status = "FAIL"
		result.Error = strings.TrimSpace(err.Error())
		if strings.TrimSpace(result.Stderr) != "" {
			result.Error = strings.TrimSpace(result.Stderr)
		}
		return result
	}
	result.Status = "PASS"
	return result
}

func codexImageHealthMissingScript(cwd string, command []string) string {
	if len(command) < 2 || command[0] != "bash" {
		return ""
	}
	scriptPath := command[1]
	if filepath.IsAbs(scriptPath) {
		if !fileExists(scriptPath) {
			return scriptPath
		}
		return ""
	}
	abs := filepath.Join(cwd, scriptPath)
	if !fileExists(abs) {
		return scriptPath
	}
	return ""
}

func summarizeCodexImageHealthChecks(checks []codexImageHealthCheckResult) codexImageHealthSummary {
	summary := codexImageHealthSummary{Total: len(checks)}
	for _, check := range checks {
		switch check.Status {
		case "PASS":
			summary.Passed++
		case "SKIP":
			summary.Skipped++
		default:
			summary.Failed++
		}
		if check.Slow {
			summary.Slow++
		}
	}
	return summary
}

func codexImageHealthFileFingerprint(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return "missing"
	}
	return fmt.Sprintf("exists:%d:%d", info.Size(), info.ModTime().UnixNano())
}

func codexImageHealthExcerpt(text string) string {
	text = strings.TrimSpace(text)
	if len(text) > 2000 {
		return text[:2000]
	}
	return text
}

func loadCodexTaskPacket(path string) (codexTaskPacket, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return codexTaskPacket{}, fmt.Errorf("codex dispatch requires --packet")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return codexTaskPacket{}, fmt.Errorf("read codex task packet: %w", err)
	}
	if err := validateCodexTaskPacketJSON(data); err != nil {
		return codexTaskPacket{}, err
	}
	var packet codexTaskPacket
	if err := json.Unmarshal(data, &packet); err != nil {
		return codexTaskPacket{}, fmt.Errorf("parse codex task packet: %w", err)
	}
	if err := validateCodexTaskPacket(packet); err != nil {
		return codexTaskPacket{}, err
	}
	return packet, nil
}

func validateCodexTaskPacket(packet codexTaskPacket) error {
	if packet.SchemaVersion != 1 {
		return fmt.Errorf("codex task packet schema_version = %d, want 1", packet.SchemaVersion)
	}
	if strings.TrimSpace(packet.PacketID) == "" {
		return fmt.Errorf("codex task packet missing packet_id")
	}
	if strings.TrimSpace(packet.CWD) == "" {
		return fmt.Errorf("codex task packet missing cwd")
	}
	if len(packet.Execution.Argv) == 0 {
		return fmt.Errorf("codex task packet execution.argv is empty")
	}
	if packet.Execution.TimeoutSeconds <= 0 {
		return fmt.Errorf("codex task packet execution.timeout_seconds must be > 0")
	}
	if err := validateCodexTaskResume(packet); err != nil {
		return err
	}
	if err := validateCodexDispatchSandbox(packet); err != nil {
		return err
	}
	if packet.Dispatch.Mode != "non-mutating" || packet.Dispatch.MutatesRepo {
		return fmt.Errorf("codex dispatch only accepts non-mutating packets")
	}
	if !slices.Equal(packet.Dispatch.Command, packet.Execution.Argv) {
		return fmt.Errorf("codex task packet dispatch.command %q does not match execution.argv %q", packet.Dispatch.Command, packet.Execution.Argv)
	}
	if strings.TrimSpace(packet.Output.ReceiptPath) == "" {
		return fmt.Errorf("codex task packet output.receipt_path is required")
	}
	if packet.Execution.Stdin.Mode == "inherit-interactive" {
		return fmt.Errorf("codex dispatch refuses inherit-interactive stdin; use closed or pipe-prompt")
	}
	if packet.Execution.Stdin.Mode == "pipe-prompt" && !packet.Execution.Stdin.CloseAfterPrompt {
		return fmt.Errorf("codex dispatch requires execution.stdin.close_after_prompt for pipe-prompt")
	}
	return nil
}

func validateCodexTaskResume(packet codexTaskPacket) error {
	if packet.Resume == nil {
		return nil
	}
	policy := strings.TrimSpace(packet.Resume.Policy)
	switch policy {
	case "", "none":
		return nil
	case "session-id":
		if !packet.Resume.AllowResume {
			return fmt.Errorf("codex task packet resume.policy session-id requires resume.allow_resume")
		}
		if strings.TrimSpace(packet.Resume.SessionID) == "" {
			return fmt.Errorf("codex task packet resume.policy session-id requires resume.session_id")
		}
		return nil
	case "last-session-in-cwd":
		return fmt.Errorf("codex dispatch refuses resume.policy last-session-in-cwd; use explicit session-id to avoid cwd inheritance")
	default:
		return fmt.Errorf("unsupported codex task packet resume.policy %q", packet.Resume.Policy)
	}
}

func validateCodexDispatchSandbox(packet codexTaskPacket) error {
	switch strings.TrimSpace(packet.Sandbox) {
	case "read-only", "workspace-write", "danger-full-access":
	default:
		return fmt.Errorf("unsupported codex task packet sandbox %q", packet.Sandbox)
	}
	sandboxArg, ok := codexDispatchSandboxArg(packet.Execution.Argv)
	if !ok {
		return fmt.Errorf("codex task packet execution.argv must include --sandbox matching packet sandbox")
	}
	if sandboxArg != packet.Sandbox {
		return fmt.Errorf("codex task packet sandbox %q does not match execution argv sandbox %q", packet.Sandbox, sandboxArg)
	}
	return nil
}

func codexDispatchSandboxArg(argv []string) (string, bool) {
	for i, arg := range argv {
		if arg == "--sandbox" {
			if i+1 >= len(argv) {
				return "", false
			}
			return strings.TrimSpace(argv[i+1]), strings.TrimSpace(argv[i+1]) != ""
		}
		if value, ok := strings.CutPrefix(arg, "--sandbox="); ok {
			value = strings.TrimSpace(value)
			return value, value != ""
		}
	}
	return "", false
}

func performCodexDispatch(packet codexTaskPacket) (codexRunReceipt, error) {
	cwd, err := filepath.Abs(packet.CWD)
	if err != nil {
		return codexRunReceipt{}, fmt.Errorf("resolve packet cwd: %w", err)
	}
	if info, err := os.Stat(cwd); err != nil {
		return codexRunReceipt{}, fmt.Errorf("stat packet cwd: %w", err)
	} else if !info.IsDir() {
		return codexRunReceipt{}, fmt.Errorf("packet cwd is not a directory: %s", cwd)
	}

	if err := validateCodexDispatchPathBounds(cwd, packet); err != nil {
		return codexRunReceipt{}, err
	}
	authStatus, err := validateCodexDispatchAuth(packet)
	if err != nil {
		return codexRunReceipt{}, err
	}
	if err := ensureCodexDispatchOutputDirs(cwd, packet.AllowedPaths, packet.Output); err != nil {
		return codexRunReceipt{}, err
	}

	started := time.Now().UTC()
	stdinBytes, err := codexDispatchStdin(cwd, packet)
	if err != nil {
		return codexRunReceipt{}, err
	}
	stdinClosedAt := time.Now().UTC()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(packet.Execution.TimeoutSeconds)*time.Second)
	defer cancel()

	argv := append([]string(nil), packet.Execution.Argv...)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = cwd
	cmd.Env = codexDispatchEnv(packet)
	if packet.Execution.Stdin.Mode == "pipe-prompt" {
		cmd.Stdin = bytes.NewReader(stdinBytes)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	ended := time.Now().UTC()
	timedOut := ctx.Err() == context.DeadlineExceeded
	exitCode := codexDispatchExitCode(runErr, timedOut)
	failureReason := codexDispatchFailureReason(runErr, timedOut, stderr.String())

	if err := writeCodexDispatchOutputFiles(cwd, packet.AllowedPaths, packet.Output, stdout.Bytes()); err != nil {
		return codexRunReceipt{}, err
	}

	var requiredResults []codexCommandResult
	if !timedOut {
		requiredResults = runCodexRequiredCommands(cwd, packet)
	}

	receipt := codexRunReceipt{
		SchemaVersion:  1,
		ReceiptID:      "codex-receipt-" + started.Format("20060102T150405Z") + "-" + sanitizeCodexReceiptID(packet.PacketID),
		PacketID:       packet.PacketID,
		StartedAt:      started.Format(time.RFC3339),
		EndedAt:        ended.Format(time.RFC3339),
		CWD:            cwd,
		Sandbox:        packet.Sandbox,
		AuthMode:       "chatgpt-subscription",
		AuthStatus:     authStatus,
		Command:        codexReceiptCommand{Argv: argv},
		Stdin:          codexReceiptStdin{Mode: packet.Execution.Stdin.Mode, ClosedAt: stdinClosedAt.Format(time.RFC3339), BytesWritten: len(stdinBytes)},
		TimeoutSeconds: packet.Execution.TimeoutSeconds,
		TimedOut:       timedOut,
		ExitCode:       exitCode,
		Outputs: codexReceiptOutputs{
			FinalMessagePath: packet.Output.FinalMessagePath,
			JSONLPath:        packet.Output.JSONLPath,
			SchemaPath:       packet.Output.SchemaPath,
			ReceiptPath:      packet.Output.ReceiptPath,
		},
		ChangedFiles: collectCodexDispatchChangedFiles(cwd),
		CommandsRun: append([]codexCommandResult{{
			Command:       strings.Join(argv, " "),
			ExitCode:      exitCode,
			OutputExcerpt: codexDispatchOutputExcerpt(stdout.String(), stderr.String()),
		}}, requiredResults...),
		Verdict:       codexDispatchVerdict(cwd, packet.AllowedPaths, packet.Output, exitCode, timedOut),
		Evidence:      codexDispatchEvidence(packet),
		FailureReason: failureReason,
	}
	if packet.Resume != nil && packet.Resume.Policy == "session-id" {
		receipt.ResumeFromSession = strings.TrimSpace(packet.Resume.SessionID)
	}
	receiptValidationErr := errors.Join(
		validateCodexRunReceipt(receipt),
		validateCodexReceiptRequiredCommands(packet.Evidence.RequiredCommands, receipt),
		validateCodexRunReceiptSchema(receipt),
	)
	if receiptValidationErr != nil && receipt.FailureReason == "" {
		receipt.FailureReason = receiptValidationErr.Error()
	}
	if receipt.Verdict.Status == "ERROR" && runErr == nil && !timedOut && receipt.FailureReason == "" {
		receipt.FailureReason = receipt.Verdict.Summary
	}
	if err := writeCodexRunReceipt(cwd, packet.AllowedPaths, packet.Output.ReceiptPath, receipt); err != nil {
		return codexRunReceipt{}, err
	}

	if timedOut {
		return receipt, fmt.Errorf("codex dispatch timed out after %ds", packet.Execution.TimeoutSeconds)
	}
	if runErr != nil {
		return receipt, fmt.Errorf("codex dispatch failed: %s", failureReason)
	}
	if receiptValidationErr != nil {
		return receipt, fmt.Errorf("codex receipt validation failed: %w", receiptValidationErr)
	}
	if receipt.Verdict.Status == "ERROR" {
		return receipt, fmt.Errorf("codex verdict rejected: %s", receipt.Verdict.Summary)
	}
	return receipt, nil
}

func validateCodexDispatchAuth(packet codexTaskPacket) (string, error) {
	for _, name := range codexDispatchForbiddenEnvNames(packet.Auth) {
		if os.Getenv(name) != "" {
			return "", fmt.Errorf("codex dispatch refuses %s in environment; use ChatGPT subscription auth", name)
		}
		if _, injected := packet.Execution.Environment[name]; injected {
			return "", fmt.Errorf("codex dispatch refuses %s in packet execution.environment; use ChatGPT subscription auth", name)
		}
	}

	binary := codexDispatchBinary(packet)
	cmd := exec.Command(binary, "login", "status")
	cmd.Stdin = nil
	out, err := cmd.CombinedOutput()
	status := strings.TrimSpace(string(out))
	if err != nil {
		return "", fmt.Errorf("check Codex login status with %q: %w: %s", binary, err, status)
	}
	want := strings.TrimSpace(packet.Auth.LoginStatusMustContain)
	if want == "" {
		want = "Logged in using ChatGPT"
	}
	if !strings.Contains(status, want) {
		return "", fmt.Errorf("codex dispatch requires ChatGPT subscription auth; login status %q does not contain %q", status, want)
	}
	return status, nil
}

// codexDispatchForbiddenEnvNames returns the deduplicated, ordered set of
// environment variable names the dispatch auth guard must reject in BOTH the
// ambient environment and packet-provided execution.environment.
// OPENAI_API_KEY is always forbidden: dispatch is ChatGPT-subscription-only,
// so a packet cannot opt back into API-key auth by weakening its own guard.
func codexDispatchForbiddenEnvNames(auth codexTaskAuthGuard) []string {
	var names []string
	seen := make(map[string]bool)
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		names = append(names, name)
	}
	for _, name := range auth.RejectEnv {
		add(name)
	}
	add("OPENAI_API_KEY")
	return names
}

func codexDispatchBinary(packet codexTaskPacket) string {
	if len(packet.Execution.Argv) == 0 || strings.TrimSpace(packet.Execution.Argv[0]) == "" {
		return "codex"
	}
	return packet.Execution.Argv[0]
}

func ensureCodexDispatchOutputDirs(cwd string, allowedPaths []string, out codexTaskOutputContract) error {
	for _, path := range []string{out.FinalMessagePath, out.JSONLPath, out.SchemaPath, out.ReceiptPath} {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		abs, err := resolveCodexDispatchPath(cwd, allowedPaths, path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
			return fmt.Errorf("create output dir for %s: %w", path, err)
		}
	}
	return nil
}

func codexDispatchStdin(cwd string, packet codexTaskPacket) ([]byte, error) {
	switch packet.Execution.Stdin.Mode {
	case "closed", "":
		return nil, nil
	case "pipe-prompt":
		promptPath := strings.TrimSpace(packet.Execution.PromptPath)
		if promptPath == "" {
			return []byte(packet.Objective), nil
		}
		abs, err := resolveCodexDispatchPath(cwd, packet.AllowedPaths, promptPath)
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return nil, fmt.Errorf("read Codex prompt path: %w", err)
		}
		return data, nil
	default:
		return nil, fmt.Errorf("unsupported codex dispatch stdin mode %q", packet.Execution.Stdin.Mode)
	}
}

func codexDispatchEnv(packet codexTaskPacket) []string {
	env := os.Environ()
	for k, v := range packet.Execution.Environment {
		env = append(env, k+"="+v)
	}
	return env
}

func codexDispatchExitCode(err error, timedOut bool) int {
	if timedOut {
		return -1
	}
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func codexDispatchFailureReason(err error, timedOut bool, stderr string) string {
	if timedOut {
		return "timeout"
	}
	if err == nil {
		return ""
	}
	stderr = strings.TrimSpace(stderr)
	if stderr != "" {
		return stderr
	}
	return err.Error()
}

func writeCodexDispatchOutputFiles(cwd string, allowedPaths []string, out codexTaskOutputContract, stdout []byte) error {
	if strings.TrimSpace(out.JSONLPath) != "" && len(stdout) > 0 {
		abs, err := resolveCodexDispatchPath(cwd, allowedPaths, out.JSONLPath)
		if err != nil {
			return err
		}
		if err := atomicWriteFile(abs, stdout, 0o600); err != nil {
			return fmt.Errorf("write codex jsonl output: %w", err)
		}
	}
	return nil
}

func collectCodexDispatchChangedFiles(cwd string) []string {
	// Always non-nil: the receipt schema requires changed_files to be an
	// array, and a nil slice marshals to JSON null.
	files := []string{}
	cmd := exec.Command("git", "-C", cwd, "status", "--short", "--untracked-files=all")
	out, err := cmd.Output()
	if err != nil {
		return files
	}
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 4 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		if strings.Contains(path, " -> ") {
			parts := strings.Split(path, " -> ")
			path = strings.TrimSpace(parts[len(parts)-1])
		}
		if path != "" {
			files = append(files, path)
		}
	}
	return files
}

func codexDispatchVerdict(cwd string, allowedPaths []string, out codexTaskOutputContract, exitCode int, timedOut bool) codexReceiptVerdict {
	if timedOut {
		return codexReceiptVerdict{Status: "ERROR", JudgeSource: "codex-dispatch", Summary: "Codex dispatch timed out."}
	}
	if exitCode != 0 {
		return codexReceiptVerdict{Status: "ERROR", JudgeSource: "codex-dispatch", Summary: fmt.Sprintf("Codex exec exited %d.", exitCode)}
	}
	finalPath := strings.TrimSpace(out.FinalMessagePath)
	if finalPath == "" {
		return codexReceiptVerdict{Status: "NO_VERDICT", JudgeSource: "codex-dispatch", Summary: "No final message path was configured."}
	}
	abs, err := resolveCodexDispatchPath(cwd, allowedPaths, finalPath)
	if err != nil {
		return codexReceiptVerdict{Status: "ERROR", JudgeSource: "codex-dispatch", Summary: err.Error()}
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return codexReceiptVerdict{Status: "NO_VERDICT", JudgeSource: "codex-dispatch", Summary: "Final message was not readable."}
	}
	text := string(data)
	return codexFinalMessageVerdict(text)
}

func parseCodexVerdictStatus(text string) string {
	status, _ := parseCodexVerdictStatusStrict(text)
	return status
}

func codexFinalMessageVerdict(text string) codexReceiptVerdict {
	status, err := parseCodexVerdictStatusStrict(text)
	if err != nil {
		return codexReceiptVerdict{Status: "ERROR", JudgeSource: "codex-final-message", Summary: err.Error()}
	}
	if status == "NO_VERDICT" {
		return codexReceiptVerdict{Status: status, JudgeSource: "codex-final-message", Summary: firstLine(text)}
	}
	verdict := codexReceiptVerdict{Status: status, JudgeSource: "codex-final-message", Summary: firstLine(text)}
	if status == "ERROR" {
		return verdict
	}
	if !verdictparse.HasCommandsRun(text) {
		verdict.Status = "ERROR"
		verdict.Summary = "Final verdict missing non-empty COMMANDS RUN body."
		return verdict
	}
	identity, gaps := verdictparse.Identity(text)
	if len(gaps) > 0 {
		verdict.Status = "ERROR"
		verdict.Summary = "Final verdict identity unproven: " + strings.Join(gaps, "; ")
		return verdict
	}
	verdict.AuthorID = identity.Author
	verdict.JudgeName = identity.JudgeName
	verdict.JudgeProgram = identity.JudgeProgram
	verdict.JudgeModelFamily = identity.JudgeModelFamily
	return verdict
}

func parseCodexVerdictStatusStrict(text string) (string, error) {
	re := regexp.MustCompile(`(?i)^\s*VERDICT\s*[:=]\s*(PASS|WARN|FAIL|ERROR)\b`)
	var statuses []string
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		match := re.FindStringSubmatch(scanner.Text())
		if len(match) == 2 {
			statuses = append(statuses, strings.ToUpper(match[1]))
		}
	}
	if len(statuses) == 0 {
		return "NO_VERDICT", nil
	}
	if len(statuses) > 1 {
		unique := make(map[string]bool, len(statuses))
		for _, status := range statuses {
			unique[status] = true
		}
		if len(unique) > 1 {
			return "ERROR", fmt.Errorf("contradictory verdict body: %s", strings.Join(statuses, ", "))
		}
		return "ERROR", fmt.Errorf("malformed verdict body: repeated %s verdict", statuses[0])
	}
	return statuses[0], nil
}

func validateCodexRunReceipt(receipt codexRunReceipt) error {
	var gaps []string
	if strings.TrimSpace(receipt.ReceiptID) == "" {
		gaps = append(gaps, "missing receipt_id")
	}
	if strings.TrimSpace(receipt.PacketID) == "" {
		gaps = append(gaps, "missing packet_id")
	}
	if len(receipt.Command.Argv) == 0 {
		gaps = append(gaps, "missing command.argv")
	}
	if len(receipt.CommandsRun) == 0 {
		gaps = append(gaps, "missing command evidence in commands_run")
	}
	for i, command := range receipt.CommandsRun {
		if strings.TrimSpace(command.Command) == "" {
			gaps = append(gaps, fmt.Sprintf("commands_run[%d] missing command", i))
		}
	}
	status := strings.ToUpper(strings.TrimSpace(receipt.Verdict.Status))
	switch status {
	case "PASS", "WARN", "FAIL", "ERROR", "NO_VERDICT":
	default:
		gaps = append(gaps, "malformed verdict status "+strconv.Quote(receipt.Verdict.Status))
	}
	if receipt.ExitCode == 0 && !receipt.TimedOut && (status == "" || status == "NO_VERDICT") {
		gaps = append(gaps, "successful Codex run did not produce a verifiable verdict")
	}
	if status == "PASS" || status == "WARN" || status == "FAIL" {
		identity := verdictparse.IdentityInfo{
			Author:           receipt.Verdict.AuthorID,
			JudgeName:        receipt.Verdict.JudgeName,
			JudgeProgram:     receipt.Verdict.JudgeProgram,
			JudgeModelFamily: receipt.Verdict.JudgeModelFamily,
		}
		if strings.TrimSpace(identity.Author) == "" {
			gaps = append(gaps, "missing author_id")
		}
		if strings.TrimSpace(identity.JudgeName) == "" {
			gaps = append(gaps, "missing judge_name")
		}
		if strings.TrimSpace(identity.JudgeProgram) == "" {
			gaps = append(gaps, "missing judge_program")
		}
		if strings.TrimSpace(identity.JudgeModelFamily) == "" || verdictparse.UnknownModelFamily(identity.JudgeModelFamily) {
			gaps = append(gaps, "missing or unknown judge_model_family")
		}
		if strings.TrimSpace(identity.Author) != "" && strings.TrimSpace(identity.Author) == strings.TrimSpace(identity.JudgeName) {
			gaps = append(gaps, "author_neq_validator failed: judge_name equals author_id")
		}
	}
	if len(gaps) > 0 {
		return errors.New(strings.Join(gaps, "; "))
	}
	return nil
}

// runCodexRequiredCommands executes the packet's evidence.required_commands
// acceptance commands in cwd and returns one result per command, so the
// receipt carries machine-checkable acceptance evidence instead of only
// proving that Codex itself ran. Each command runs via `sh -c` with the
// packet execution timeout as its own budget; failures are recorded honestly
// (non-zero exit codes do not abort the remaining commands).
func runCodexRequiredCommands(cwd string, packet codexTaskPacket) []codexCommandResult {
	var results []codexCommandResult
	for _, raw := range packet.Evidence.RequiredCommands {
		command := strings.TrimSpace(raw)
		if command == "" {
			continue
		}
		results = append(results, runCodexRequiredCommand(cwd, packet, command))
	}
	return results
}

func runCodexRequiredCommand(cwd string, packet codexTaskPacket, command string) codexCommandResult {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(packet.Execution.TimeoutSeconds)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = cwd
	cmd.Env = codexDispatchEnv(packet)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	timedOut := ctx.Err() == context.DeadlineExceeded
	excerpt := codexDispatchOutputExcerpt(stdout.String(), stderr.String())
	if timedOut {
		excerpt = strings.TrimSpace("required command timed out\n" + excerpt)
	}
	return codexCommandResult{
		Command:       command,
		ExitCode:      codexDispatchExitCode(runErr, timedOut),
		OutputExcerpt: excerpt,
	}
}

// validateCodexReceiptRequiredCommands fails when the packet declares
// evidence.required_commands whose results are absent from the receipt's
// commands_run, so a receipt cannot claim acceptance evidence that was never
// executed and recorded.
func validateCodexReceiptRequiredCommands(required []string, receipt codexRunReceipt) error {
	if len(required) == 0 {
		return nil
	}
	recorded := make(map[string]bool, len(receipt.CommandsRun))
	for _, result := range receipt.CommandsRun {
		recorded[strings.TrimSpace(result.Command)] = true
	}
	var missing []string
	for _, raw := range required {
		command := strings.TrimSpace(raw)
		if command == "" {
			continue
		}
		if !recorded[command] {
			missing = append(missing, command)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("receipt missing required command evidence in commands_run: %s", strings.Join(missing, "; "))
	}
	return nil
}

func codexDispatchEvidence(packet codexTaskPacket) []codexEvidenceRef {
	var evidence []codexEvidenceRef
	for _, path := range packet.Evidence.Artifacts {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		evidence = append(evidence, codexEvidenceRef{Path: path, Kind: codexEvidenceKind(path)})
	}
	if packet.Output.FinalMessagePath != "" {
		evidence = append(evidence, codexEvidenceRef{Path: packet.Output.FinalMessagePath, Kind: "final-message"})
	}
	if packet.Output.JSONLPath != "" {
		evidence = append(evidence, codexEvidenceRef{Path: packet.Output.JSONLPath, Kind: "jsonl"})
	}
	return evidence
}

func codexEvidenceKind(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.Contains(lower, "schema"):
		return "schema"
	case strings.Contains(lower, "test") || strings.Contains(lower, "log"):
		return "test-log"
	case strings.Contains(lower, "fixture") || strings.Contains(lower, "example"):
		return "fixture"
	default:
		return "contract"
	}
}

func writeCodexRunReceipt(cwd string, allowedPaths []string, path string, receipt codexRunReceipt) error {
	abs, err := resolveCodexDispatchPath(cwd, allowedPaths, path)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal codex run receipt: %w", err)
	}
	if err := atomicWriteFile(abs, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write codex run receipt: %w", err)
	}
	return nil
}

// resolveCodexDispatchPath resolves a packet-declared path against cwd and
// enforces the dispatch path boundary: the resolved path must stay inside cwd
// or inside one of the packet's allowed_paths roots (each resolved against cwd
// when relative). Absolute paths and ".." traversal that escape every permitted
// root are rejected so receipts, JSONL, prompts, and final messages cannot be
// written or read outside the declared scope.
func resolveCodexDispatchPath(cwd string, allowedPaths []string, path string) (string, error) {
	cleanCwd := filepath.Clean(cwd)
	candidate := path
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(cleanCwd, candidate)
	}
	candidate = filepath.Clean(candidate)

	roots := []string{cleanCwd}
	for _, root := range allowedPaths {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		if !filepath.IsAbs(root) {
			root = filepath.Join(cleanCwd, root)
		}
		roots = append(roots, filepath.Clean(root))
	}
	for _, root := range roots {
		if candidate == root || strings.HasPrefix(candidate, root+string(filepath.Separator)) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("codex dispatch path %q escapes cwd %s and the packet allowed_paths", path, cleanCwd)
}

// validateCodexDispatchPathBounds rejects a packet up front when any
// dispatcher-managed path (outputs, prompt, schemas, receipts) escapes the
// dispatch path boundary, before auth checks, worker execution, or receipt
// creation.
func validateCodexDispatchPathBounds(cwd string, packet codexTaskPacket) error {
	bounded := []struct {
		label string
		path  string
	}{
		{"output.final_message_path", packet.Output.FinalMessagePath},
		{"output.jsonl_path", packet.Output.JSONLPath},
		{"output.schema_path", packet.Output.SchemaPath},
		{"output.receipt_path", packet.Output.ReceiptPath},
		{"execution.prompt_path", packet.Execution.PromptPath},
		{"execution.output_schema_path", packet.Execution.OutputSchemaPath},
		{"evidence.receipt_path", packet.Evidence.ReceiptPath},
	}
	for _, entry := range bounded {
		path := strings.TrimSpace(entry.path)
		if path == "" {
			continue
		}
		if _, err := resolveCodexDispatchPath(cwd, packet.AllowedPaths, path); err != nil {
			return fmt.Errorf("codex task packet %s: %w", entry.label, err)
		}
	}
	return nil
}

func sanitizeCodexReceiptID(id string) string {
	var b strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "packet"
	}
	return out
}

func codexDispatchOutputExcerpt(stdout, stderr string) string {
	text := strings.TrimSpace(stdout)
	if strings.TrimSpace(stderr) != "" {
		if text != "" {
			text += "\n"
		}
		text += strings.TrimSpace(stderr)
	}
	if len(text) > 500 {
		return text[:500]
	}
	return text
}

func collectCodexStartupArtifacts(cwd, query string, limit int) ([]codexArtifactRef, []learning, []pattern, []knowledgeFinding, []session, []nextWorkItem, []codexArtifactRef) {
	if limit <= 0 {
		limit = 3
	}

	briefings := collectRecentCodexArtifacts(filepath.Join(cwd, ".agents", "briefings"), query, limit)
	learnings, _ := collectLearnings(cwd, query, limit, "", 0)
	patterns, _ := collectPatterns(cwd, query, limit, "", 0)
	findings, _ := collectFindings(cwd, query, limit, "", 0)
	recentSessions, _ := collectRecentSessions(cwd, query, minInt(limit, MaxSessionsToInject))

	repoFilter := filepath.Base(cwd)
	if root := findGitRoot(cwd); root != "" {
		repoFilter = filepath.Base(root)
	}
	nextWork, _ := cliRPI.ReadUnconsumedItems(filepath.Join(cwd, ".agents", "rpi", "next-work.jsonl"), repoFilter)
	if len(nextWork) > limit {
		nextWork = nextWork[:limit]
	}

	research := collectRecentResearchArtifacts(cwd, query, limit)
	return briefings, learnings, patterns, findings, recentSessions, nextWork, research
}

func resolveCodexStopTranscript(cwd, sessionID string, noHistoryFallback bool) (string, string, bool, string, error) {
	if sessionID != "" {
		if path, err := codexruntime.FindTranscriptBySessionID(sessionID); err == nil {
			return path, "archived", false, sessionID, nil
		}
		if !noHistoryFallback {
			path, err := codexruntime.SynthesizeCodexHistoryTranscript(cwd, sessionID)
			if err == nil {
				return path, "history-fallback", true, sessionID, nil
			}
		}
	}

	if path, err := codexruntime.FindLastCodexArchivedTranscript(); err == nil {
		return path, "archived", false, codexruntime.ExtractSessionIDFromCodexArchivedPath(path), nil
	}

	if noHistoryFallback {
		return "", "", false, sessionID, fmt.Errorf("no Codex transcript found and history fallback is disabled")
	}

	fallbackSessionID := sessionID
	if fallbackSessionID == "" {
		fallbackSessionID = resolveCodexSessionIDFromHome()
	}
	if fallbackSessionID == "" {
		return "", "", false, "", fmt.Errorf("no Codex transcript or active history session found")
	}
	path, err := codexruntime.SynthesizeCodexHistoryTranscript(cwd, fallbackSessionID)
	if err != nil {
		return "", "", false, fallbackSessionID, err
	}
	return path, "history-fallback", true, fallbackSessionID, nil
}

func resolveCodexSessionIDFromHome() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return codexruntime.ResolveCodexSessionID(homeDir)
}

func syncCodexMemory(cwd string) (string, error) {
	root := findGitRoot(cwd)
	if root == "" {
		root = cwd
	}
	path := filepath.Join(root, "MEMORY.md")
	if err := syncMemory(cwd, path, 10, true); err != nil {
		return path, err
	}
	return path, nil
}

func codexLifecycleStatePath(cwd string) string {
	return filepath.Join(cwd, ".agents", "ao", "codex", "state.json")
}

func normalizeCodexLifecyclePath(path string) string {
	return bridge.NormalizeCodexLifecyclePath(path)
}

func firstNonEmptyLifecycleField(state *codexLifecycleState, getter func(*codexLifecycleEvent) string) string {
	if state == nil || getter == nil || state.LastStart == nil {
		return ""
	}
	return firstNonEmptyTrimmed(getter(state.LastStart))
}

func ensureCodexLifecycleDirs(cwd string) error {
	for _, dir := range []string{
		filepath.Join(cwd, ".agents", "ao", "codex"),
		filepath.Join(cwd, ".agents", "ao", "codex", "transcripts"),
	} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create codex lifecycle dir %s: %w", dir, err)
		}
	}
	return nil
}

func codexShouldShowNewUserWelcome(cwd string) bool {
	_, err := os.Stat(filepath.Join(cwd, ".agents"))
	return os.IsNotExist(err)
}

func loadOrInitCodexLifecycleState(cwd string) (*codexLifecycleState, string, error) {
	if err := ensureCodexLifecycleDirs(cwd); err != nil {
		return nil, "", err
	}
	path := codexLifecycleStatePath(cwd)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &codexLifecycleState{SchemaVersion: 1}, path, nil
		}
		return nil, "", fmt.Errorf("read codex lifecycle state: %w", err)
	}

	var state codexLifecycleState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, "", fmt.Errorf("parse codex lifecycle state: %w", err)
	}
	if state.SchemaVersion == 0 {
		state.SchemaVersion = 1
	}
	if err := validateCodexLifecycleState(&state); err != nil {
		return nil, "", fmt.Errorf("validating codex lifecycle state: %w", err)
	}
	return &state, path, nil
}

// expectedCodexSchemaVersion is the schema version this code can handle.
const expectedCodexSchemaVersion = 1

// validateCodexLifecycleState checks invariants on a deserialized lifecycle state:
// schema version, timestamp format (RFC3339), and temporal ordering.
func validateCodexLifecycleState(state *codexLifecycleState) error {
	if err := validateCodexLifecycleSchemaVersion(state.SchemaVersion); err != nil {
		return err
	}
	if _, _, err := validateCodexLifecycleTimestamp("updated_at", state.UpdatedAt); err != nil {
		return err
	}
	startTime, startOK, err := validateCodexLifecycleEventTimestamp("last_start", state.LastStart)
	if err != nil {
		return err
	}
	stopTime, stopOK, err := validateCodexLifecycleEventTimestamp("last_stop", state.LastStop)
	if err != nil {
		return err
	}
	return validateCodexLifecycleEventOrdering(state.LastStart, state.LastStop, startTime, startOK, stopTime, stopOK)
}

func validateCodexLifecycleSchemaVersion(schemaVersion int) error {
	if schemaVersion != expectedCodexSchemaVersion {
		return fmt.Errorf("unsupported schema_version %d (expected %d)", schemaVersion, expectedCodexSchemaVersion)
	}
	return nil
}

func validateCodexLifecycleTimestamp(field, value string) (time.Time, bool, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, false, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("invalid %s timestamp %q: %w", field, value, err)
	}
	return parsed, true, nil
}

func validateCodexLifecycleEventTimestamp(field string, event *codexLifecycleEvent) (time.Time, bool, error) {
	if event == nil {
		return time.Time{}, false, nil
	}
	return validateCodexLifecycleTimestamp(field, event.Timestamp)
}

func validateCodexLifecycleEventOrdering(lastStart, lastStop *codexLifecycleEvent, startTime time.Time, startOK bool, stopTime time.Time, stopOK bool) error {
	// If both start and stop exist for the SAME session with timestamps,
	// stop must not precede start unless the stop event has durable closeout
	// evidence. Codex can resume the same thread after explicit closeout, so
	// last_stop may describe the prior closeout for the same thread before a
	// newer last_start.
	if !startOK || !stopOK || lastStart == nil || lastStop == nil {
		return nil
	}
	startSessionID := strings.TrimSpace(lastStart.SessionID)
	if startSessionID == "" || startSessionID != strings.TrimSpace(lastStop.SessionID) {
		return nil
	}
	if stopTime.Before(startTime) && !codexStopHasCloseoutEvidence(lastStop) {
		return fmt.Errorf("last_stop (%s) is before last_start (%s)", lastStop.Timestamp, lastStart.Timestamp)
	}
	return nil
}

func codexStopHasCloseoutEvidence(event *codexLifecycleEvent) bool {
	if event == nil {
		return false
	}
	return strings.TrimSpace(event.TranscriptPath) != "" || strings.TrimSpace(event.HandoffPath) != ""
}

func saveCodexLifecycleState(path string, state *codexLifecycleState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal codex lifecycle state: %w", err)
	}
	if err := atomicWriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write codex lifecycle state: %w", err)
	}
	return nil
}

func writeCodexStartupContext(cwd string, profile lifecycleRuntimeProfile, query string, briefings []codexArtifactRef, learnings []learning, patterns []pattern, findings []knowledgeFinding, recentSessions []session, nextWork []nextWorkItem, research []codexArtifactRef, showNewUserWelcome bool) (string, error) {
	bundle := buildRankedContextBundle(cwd, query, codexStartLimit, learnings, patterns, findings, recentSessions, nextWork, research)
	agentsRoot := knowledgeAgentsRoot(cwd)
	beliefs := codexStartupBeliefs(bundle)
	playbooks := codexStartupPlaybooks(bundle)
	warnings := codexStartupWarnings(bundle, agentsRoot)
	sourceLinks := codexStartupSourceLinks(cwd, agentsRoot, briefings, playbooks)
	content := renderCodexStartupContext(cwd, agentsRoot, profile, query, briefings, beliefs, playbooks, warnings, sourceLinks, showNewUserWelcome)
	return writeCodexStartupContextFile(cwd, content)
}

func codexStartupBeliefs(bundle rankedContextBundle) []string {
	beliefs := append([]string(nil), bundle.Beliefs...)
	if len(beliefs) > 3 {
		beliefs = beliefs[:3]
	}
	return beliefs
}

func codexStartupPlaybooks(bundle rankedContextBundle) []knowledgeContextPlaybook {
	playbooks := append([]knowledgeContextPlaybook(nil), bundle.Playbooks...)
	if len(playbooks) > 1 {
		playbooks = playbooks[:1]
	}
	return playbooks
}

func renderCodexStartupContext(cwd, agentsRoot string, profile lifecycleRuntimeProfile, query string, briefings []codexArtifactRef, beliefs []string, playbooks []knowledgeContextPlaybook, warnings, sourceLinks []string, showNewUserWelcome bool) string {
	var sb strings.Builder
	writeCodexStartupHeader(&sb, profile, query)
	if showNewUserWelcome {
		writeCodexStartupNewUserWelcome(&sb)
	}
	writeCodexStartupBriefings(&sb, query, briefings)
	writeCodexStartupOperatorModel(&sb, cwd, agentsRoot)
	writeCodexStartupSlots(&sb, cwd, beliefs, playbooks, warnings, sourceLinks)
	writeCodexStartupDegradedMode(&sb)
	writeCodexStartupExcludedByDefault(&sb)
	return sb.String()
}

func writeCodexStartupHeader(sb *strings.Builder, profile lifecycleRuntimeProfile, query string) {
	sb.WriteString("# Codex Startup Context\n\n")
	fmt.Fprintf(sb, "- Runtime: %s\n", profile.Runtime)
	fmt.Fprintf(sb, "- Lifecycle mode: %s\n", profile.Mode)
	if profile.ThreadName != "" {
		fmt.Fprintf(sb, "- Thread: %s\n", profile.ThreadName)
	}
	if query != "" {
		fmt.Fprintf(sb, "- Query: %s\n", query)
	}
}

func writeCodexStartupNewUserWelcome(sb *strings.Builder) {
	sb.WriteString("\n## New Here?\n")
	sb.WriteString("- `$research \"how does auth work\"` to understand the repo before changing it\n")
	sb.WriteString("- `$implement \"fix the login bug\"` to run one scoped task end to end\n")
	sb.WriteString("- `$council validate this plan` to pressure-test a plan, PR, or direction before shipping\n")
}

func writeCodexStartupBriefings(sb *strings.Builder, query string, briefings []codexArtifactRef) {
	sb.WriteString("\n## Briefings\n")
	if len(briefings) == 0 {
		fmt.Fprintf(sb, "- No recent knowledge briefing surfaced. Build one with `ao knowledge brief --goal %q` when workspace builders are available.\n", query)
	} else {
		sb.WriteString("- Treat matched knowledge briefings as the primary dynamic surface for this thread; use the ranked context below as supporting operator state.\n")
		for _, item := range briefings {
			fmt.Fprintf(sb, "- %s\n", item.Title)
		}
	}
}

func writeCodexStartupOperatorModel(sb *strings.Builder, cwd, agentsRoot string) {
	sb.WriteString("\n## Operator Model\n")
	sb.WriteString("- Canonical primitives: `fitness gradient`, `stateful environment`, `replaceable actors`, `stigmergic traces`, `selection gates`, `evolutionary promotion`, `governance`\n")
	sb.WriteString("- Treat the control plane as the product; actors are replaceable executors and the environment carries memory, coordination, trust, and adaptation.\n")
	operatorModelPath := filepath.Join(agentsRoot, "knowledge", "operator-model.md")
	if fileExists(operatorModelPath) {
		fmt.Fprintf(sb, "- Doctrine: `%s`\n", displayKnowledgeContextPath(cwd, operatorModelPath))
	}
}

func writeCodexStartupSlots(sb *strings.Builder, cwd string, beliefs []string, playbooks []knowledgeContextPlaybook, warnings, sourceLinks []string) {
	sb.WriteString("\n## Startup Slots\n")
	sb.WriteString("This startup surface is fixed-slot and file-backed: a few beliefs, one healthy playbook, concrete blockers, and source links.\n\n")
	writeCodexStartupStringSection(sb, "Core Beliefs", "- No stable beliefs surfaced yet.", beliefs)
	writeCodexStartupPlaybookSection(sb, cwd, playbooks)
	writeCodexStartupStringSection(sb, "Warnings / Blockers", "- No high-signal blockers surfaced from current operator artifacts.", warnings)
	sb.WriteString("\n")
	writeCodexStartupStringSection(sb, "Source Links", "- No source links surfaced.", sourceLinks)
}

func writeCodexStartupStringSection(sb *strings.Builder, title, emptyLine string, items []string) {
	fmt.Fprintf(sb, "### %s\n", title)
	if len(items) == 0 {
		sb.WriteString(emptyLine)
		sb.WriteString("\n")
		return
	}
	for _, item := range items {
		fmt.Fprintf(sb, "- %s\n", item)
	}
}

func writeCodexStartupPlaybookSection(sb *strings.Builder, cwd string, playbooks []knowledgeContextPlaybook) {
	sb.WriteString("\n### Relevant Playbook\n")
	if len(playbooks) == 0 {
		sb.WriteString("- No healthy playbook matched this thread yet.\n")
	} else {
		for _, playbook := range playbooks {
			summary := strings.TrimSpace(playbook.Summary)
			if summary == "" {
				summary = "Use the healthy operator playbook for bounded execution."
			}
			fmt.Fprintf(sb, "- %s: %s (`%s`)\n", playbook.Title, summary, displayKnowledgeContextPath(cwd, playbook.Path))
		}
	}
	sb.WriteString("\n")
}

func writeCodexStartupDegradedMode(sb *strings.Builder) {
	sb.WriteString("\n## Degraded Mode\n")
	sb.WriteString("- When CAS freshness is unhealthy, file-backed artifacts and lexical probes remain authoritative.\n")
	sb.WriteString("- Startup context assembly stays file-backed and does not silently depend on a healthy CAS index.\n")
}

func writeCodexStartupExcludedByDefault(sb *strings.Builder) {
	sb.WriteString("\n## Excluded By Default\n")
	for _, bullet := range codexStartupExclusionBullets() {
		fmt.Fprintf(sb, "- %s\n", bullet)
	}
}

func writeCodexStartupContextFile(cwd, content string) (string, error) {
	path := filepath.Join(cwd, ".agents", "ao", "codex", "startup-context.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", fmt.Errorf("create codex startup context dir: %w", err)
	}
	if err := atomicWriteFile(path, []byte(content), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func codexStartupWarnings(bundle rankedContextBundle, agentsRoot string) []string {
	warnings := make([]string, 0, 4)
	if warning := knowledgeSourceManifestWarning(agentsRoot); strings.TrimSpace(warning) != "" {
		warnings = append(warnings, warning)
	}
	for _, item := range bundle.NextWork {
		summary := firstNonEmptyTrimmed(strings.TrimSpace(item.Title), strings.TrimSpace(item.Description))
		if summary == "" {
			continue
		}
		warnings = appendKnowledgeCandidate(warnings, summary)
	}
	for _, risk := range bundle.Packet.KnownRisks {
		warnings = appendKnowledgeCandidate(warnings, risk)
	}
	for _, finding := range bundle.Findings {
		summary := firstNonEmptyTrimmed(strings.TrimSpace(finding.Summary), strings.TrimSpace(finding.Title))
		if summary == "" {
			continue
		}
		warnings = appendKnowledgeCandidate(warnings, summary)
	}
	if len(warnings) > 2 {
		warnings = warnings[:2]
	}
	return warnings
}

func codexStartupSourceLinks(cwd, agentsRoot string, briefings []codexArtifactRef, playbooks []knowledgeContextPlaybook) []string {
	links := make([]string, 0, 8)
	operatorModelPath := filepath.Join(agentsRoot, "knowledge", "operator-model.md")
	beliefBookPath := filepath.Join(agentsRoot, "knowledge", "book-of-beliefs.md")
	if fileExists(operatorModelPath) {
		links = append(links, fmt.Sprintf("Doctrine: `%s`", displayKnowledgeContextPath(cwd, operatorModelPath)))
	}
	if fileExists(beliefBookPath) {
		links = append(links, fmt.Sprintf("Beliefs: `%s`", displayKnowledgeContextPath(cwd, beliefBookPath)))
	}
	for _, item := range briefings {
		if strings.TrimSpace(item.Path) != "" {
			links = append(links, fmt.Sprintf("Briefing: `%s`", displayKnowledgeContextPath(cwd, item.Path)))
			continue
		}
		if strings.TrimSpace(item.Title) != "" {
			links = append(links, "Briefing: "+item.Title)
		}
	}
	for _, playbook := range playbooks {
		if strings.TrimSpace(playbook.Path) == "" {
			continue
		}
		links = append(links, fmt.Sprintf("Playbook: `%s`", displayKnowledgeContextPath(cwd, playbook.Path)))
	}
	return dedupeKnowledgeStrings(links)
}

func countGlobMatches(pattern string) int {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return 0
	}
	return len(matches)
}

func collectCodexCaptureHealth(cwd string) codexCaptureHealth {
	result := codexCaptureHealth{
		SessionsIndexed:   countGlobMatches(filepath.Join(cwd, storage.DefaultBaseDir, storage.SessionsDir, "*.jsonl")),
		PendingKnowledge:  countGlobMatches(filepath.Join(cwd, ".agents", "knowledge", "pending", "*.md")),
		PendingQuarantine: countGlobMatches(filepath.Join(cwd, ".agents", "knowledge", "pending", ".quarantine", "*.md")),
	}
	if lastForge := findLastForgeTime(cwd); !lastForge.IsZero() {
		result.LastForgeTime = lastForge.UTC().Format(time.RFC3339)
		result.LastForgeAge = formatDurationBrief(time.Since(lastForge))
	}
	return result
}

func collectCodexRetrievalHealth(cwd string) codexRetrievalHealth {
	repoFilter := filepath.Base(cwd)
	if root := findGitRoot(cwd); root != "" {
		repoFilter = filepath.Base(root)
	}
	nextWork, _ := cliRPI.ReadUnconsumedItems(filepath.Join(cwd, ".agents", "rpi", "next-work.jsonl"), repoFilter)
	return codexRetrievalHealth{
		Learnings: countGlobMatches(filepath.Join(cwd, ".agents", "learnings", "*.md")),
		Patterns:  countGlobMatches(filepath.Join(cwd, ".agents", "patterns", "*.md")),
		Findings:  countGlobMatches(filepath.Join(cwd, ".agents", SectionFindings, "*.md")),
		NextWork:  len(nextWork),
		Briefings: countGlobMatches(filepath.Join(cwd, ".agents", "briefings", "*.md")),
		Research:  countGlobMatches(filepath.Join(cwd, ".agents", SectionResearch, "*.md")),
	}
}

func collectCodexPromotionHealth(cwd string) codexPromotionHealth {
	p := pool.NewPool(cwd)
	pending, _ := p.List(pool.ListOptions{Status: types.PoolStatusPending})
	staged, _ := p.List(pool.ListOptions{Status: types.PoolStatusStaged})
	rejected, _ := p.List(pool.ListOptions{Status: types.PoolStatusRejected})
	return codexPromotionHealth{
		PendingPool:  len(pending),
		StagedPool:   len(staged),
		RejectedPool: len(rejected),
	}
}

func collectCodexCitationHealth(cwd string, days int) codexCitationHealth {
	result := codexCitationHealth{WindowDays: days}
	citations, err := ratchet.LoadCitations(cwd)
	if err != nil {
		return result
	}
	end := time.Now()
	start := end.AddDate(0, 0, -days)
	var filtered []types.CitationEvent
	for _, citation := range citations {
		citation = normalizeCitationEventForRuntime(cwd, citation)
		if citation.CitedAt.Before(start) || citation.CitedAt.After(end) {
			continue
		}
		if !isRetrievableArtifactPath(cwd, citation.ArtifactPath) {
			continue
		}
		filtered = append(filtered, citation)
		result.Total++
		switch canonicalCitationType(citation.CitationType) {
		case types.CitationTypeApplied, types.CitationTypeUsedInFinalArtifact, types.CitationTypeHelpful:
			result.Applied++
		case types.CitationTypeReference:
			result.Reference++
		case types.CitationTypeHarmful, types.CitationTypeRefuted:
			result.Reference++
		default:
			result.Retrieved++
		}
	}
	aggregate := buildCitationAggregate(cwd, filtered)
	result.Deduped = aggregate.DedupedEvents
	result.UniqueArtifacts = aggregate.UniqueArtifacts
	result.UniqueSessions = aggregate.UniqueSessions
	result.UniqueWorkspaces = aggregate.UniqueWorkspaces
	return result
}

func outputCodexStartResult(result codexStartResult) error {
	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Println("Codex Start")
	fmt.Println("===========")
	fmt.Printf("Mode: %s (%s)\n", result.Runtime.Mode, result.Runtime.Runtime)
	if result.Runtime.ThreadName != "" {
		fmt.Printf("Thread: %s\n", result.Runtime.ThreadName)
	}
	fmt.Printf("Startup context: %s\n", result.StartupContextPath)
	if result.MemoryPath != "" {
		fmt.Printf("Memory: %s\n", result.MemoryPath)
	}
	if result.CloseLoop != nil {
		fmt.Printf("Maintenance: ingest=%d promote=%d reward=%d\n",
			result.CloseLoop.Ingest.Added, result.CloseLoop.AutoPromote.Promoted, result.CloseLoop.CitationFeedback.Rewarded)
	}
	fmt.Println()
	printNamedItems("Briefings", result.Briefings, func(item codexArtifactRef) string { return firstLine(item.Title) })
	printNamedItems("Learnings", result.Learnings, func(item learning) string { return firstLine(item.Title) })
	printNamedItems("Patterns", result.Patterns, func(item pattern) string { return firstLine(item.Name) })
	printNamedItems("Findings", result.Findings, func(item knowledgeFinding) string { return firstLine(item.Title) })
	printNamedItems("Next Work", result.NextWork, func(item nextWorkItem) string { return firstLine(item.Title) })
	printNamedItems("Research", result.Research, func(item codexArtifactRef) string { return firstLine(item.Title) })
	return nil
}

func outputCodexEnsureStartResult(result codexEnsureStartResult) error {
	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Println("Codex Ensure Start")
	fmt.Println("==================")
	fmt.Printf("Mode: %s (%s)\n", result.Runtime.Mode, result.Runtime.Runtime)
	if result.Runtime.ThreadName != "" {
		fmt.Printf("Thread: %s\n", result.Runtime.ThreadName)
	}
	if result.SessionID != "" {
		fmt.Printf("Session: %s\n", result.SessionID)
	}
	fmt.Printf("Performed: %t\n", result.Performed)
	if result.Reason != "" {
		fmt.Printf("Reason: %s\n", result.Reason)
	}
	if result.StartupContextPath != "" {
		fmt.Printf("Startup context: %s\n", shortenPath(result.StartupContextPath))
	}
	if result.MemoryPath != "" {
		fmt.Printf("Memory: %s\n", shortenPath(result.MemoryPath))
	}
	return nil
}

func outputCodexStopResult(result codexStopResult) error {
	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Println("Codex Stop")
	fmt.Println("==========")
	fmt.Printf("Mode: %s (%s)\n", result.Runtime.Mode, result.Runtime.Runtime)
	fmt.Printf("Transcript: %s\n", shortenPath(result.TranscriptPath))
	fmt.Printf("Source: %s\n", result.TranscriptSource)
	if result.SyntheticTranscript {
		fmt.Println("Transcript mode: synthesized from Codex history.jsonl")
	}
	fmt.Printf("Session: %s\n", result.Session.SessionID)
	fmt.Printf("Learnings: %d extracted, %d rejected\n", result.Session.LearningsExtracted, result.Session.LearningsRejected)
	if result.Session.HandoffWritten != "" {
		fmt.Printf("Handoff: %s\n", shortenPath(result.Session.HandoffWritten))
	}
	if result.CloseLoop != nil {
		fmt.Printf("Close-loop: ingest=%d promote=%d reward=%d\n",
			result.CloseLoop.Ingest.Added, result.CloseLoop.AutoPromote.Promoted, result.CloseLoop.CitationFeedback.Rewarded)
	}
	return nil
}

func outputCodexEnsureStopResult(result codexEnsureStopResult) error {
	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Println("Codex Ensure Stop")
	fmt.Println("=================")
	fmt.Printf("Mode: %s (%s)\n", result.Runtime.Mode, result.Runtime.Runtime)
	if result.Runtime.ThreadName != "" {
		fmt.Printf("Thread: %s\n", result.Runtime.ThreadName)
	}
	if result.SessionID != "" {
		fmt.Printf("Session: %s\n", result.SessionID)
	}
	fmt.Printf("Performed: %t\n", result.Performed)
	if result.Reason != "" {
		fmt.Printf("Reason: %s\n", result.Reason)
	}
	if result.TranscriptPath != "" {
		fmt.Printf("Transcript: %s\n", shortenPath(result.TranscriptPath))
	}
	if result.TranscriptSource != "" {
		fmt.Printf("Source: %s\n", result.TranscriptSource)
	}
	if result.SyntheticTranscript {
		fmt.Println("Transcript mode: synthesized from Codex history.jsonl")
	}
	if result.HandoffPath != "" {
		fmt.Printf("Handoff: %s\n", shortenPath(result.HandoffPath))
	}
	if result.MemoryPath != "" {
		fmt.Printf("Memory: %s\n", shortenPath(result.MemoryPath))
	}
	return nil
}

func outputCodexStatusResult(result codexStatusResult) error {
	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Println("Codex Lifecycle Status")
	fmt.Println("======================")
	fmt.Printf("Mode: %s (%s)\n", result.Runtime.Mode, result.Runtime.Runtime)
	if result.Runtime.ThreadName != "" {
		fmt.Printf("Thread: %s\n", result.Runtime.ThreadName)
	}
	fmt.Println()
	fmt.Printf("Capture: sessions=%d pending=%d quarantine=%d\n",
		result.Capture.SessionsIndexed, result.Capture.PendingKnowledge, result.Capture.PendingQuarantine)
	if result.Capture.LastForgeAge != "" {
		fmt.Printf("Last forge: %s ago\n", result.Capture.LastForgeAge)
	}
	fmt.Printf("Retrieval: learnings=%d patterns=%d findings=%d next-work=%d briefings=%d research=%d\n",
		result.Retrieval.Learnings, result.Retrieval.Patterns, result.Retrieval.Findings, result.Retrieval.NextWork, result.Retrieval.Briefings, result.Retrieval.Research)
	fmt.Printf("Promotion: pending=%d staged=%d rejected=%d\n",
		result.Promotion.PendingPool, result.Promotion.StagedPool, result.Promotion.RejectedPool)
	fmt.Printf("Citations (%dd): total=%d unique=%d retrieved=%d reference=%d applied=%d\n",
		result.Citations.WindowDays, result.Citations.Total, result.Citations.UniqueArtifacts,
		result.Citations.Retrieved, result.Citations.Reference, result.Citations.Applied)
	if result.Flywheel != nil {
		sign := "+"
		if result.Flywheel.Velocity < 0 {
			sign = ""
		}
		fmt.Printf("Flywheel: %s (%s%.3f/week)\n", result.Flywheel.Status, sign, result.Flywheel.Velocity)
	}
	if result.State != nil {
		if result.State.LastStart != nil {
			fmt.Printf("Last start: %s\n", result.State.LastStart.Timestamp)
		}
		if result.State.LastStop != nil {
			fmt.Printf("Last stop: %s\n", result.State.LastStop.Timestamp)
		}
	}
	return nil
}

func outputCodexDispatchResult(result codexRunReceipt) error {
	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Println("Codex Dispatch")
	fmt.Println("==============")
	fmt.Printf("Packet: %s\n", result.PacketID)
	fmt.Printf("Receipt: %s\n", result.Outputs.ReceiptPath)
	fmt.Printf("Sandbox: %s\n", result.Sandbox)
	fmt.Printf("Auth: %s\n", result.AuthStatus)
	fmt.Printf("Exit: %d\n", result.ExitCode)
	if result.TimedOut {
		fmt.Println("Timed out: true")
	}
	fmt.Printf("Verdict: %s\n", result.Verdict.Status)
	if result.Outputs.FinalMessagePath != "" {
		fmt.Printf("Final message: %s\n", result.Outputs.FinalMessagePath)
	}
	if result.Outputs.JSONLPath != "" {
		fmt.Printf("JSONL: %s\n", result.Outputs.JSONLPath)
	}
	if result.FailureReason != "" {
		fmt.Printf("Failure: %s\n", result.FailureReason)
	}
	return nil
}

func outputCodexImageHealthResult(result codexImageHealthResult) error {
	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Println("Codex Image Health")
	fmt.Println("==================")
	fmt.Printf("Status: %s\n", result.Status)
	fmt.Printf("Checks: %d pass, %d fail, %d skip\n", result.Summary.Passed, result.Summary.Failed, result.Summary.Skipped)
	fmt.Printf("Lifecycle state mutated: %t\n", result.LifecycleStateMutated)
	fmt.Println()
	for _, check := range result.Checks {
		fmt.Printf("- %s: %s\n", check.Name, check.Status)
		if len(check.Command) > 0 {
			fmt.Printf("  command: %s\n", strings.Join(check.Command, " "))
		}
		if check.Error != "" {
			fmt.Printf("  error: %s\n", firstLine(check.Error))
		}
		if check.RepairHint != "" && check.Status != "PASS" {
			fmt.Printf("  repair: %s\n", check.RepairHint)
		}
	}
	return nil
}

func printNamedItems[T any](heading string, items []T, label func(T) string) {
	fmt.Printf("%s:\n", heading)
	if len(items) == 0 {
		fmt.Println("  - none")
		return
	}
	for _, item := range items {
		fmt.Printf("  - %s\n", label(item))
	}
}
