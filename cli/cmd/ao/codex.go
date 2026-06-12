// practices: [microservices, design-by-contract]
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/adapters/vendorimage/codexruntime"
	"github.com/boshu2/agentops/cli/internal/bridge"
	"github.com/boshu2/agentops/cli/internal/pool"
	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/storage"
	"github.com/boshu2/agentops/cli/internal/types"
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

type codexArtifactRef struct {
	Title      string `json:"title"`
	Path       string `json:"path"`
	ModifiedAt string `json:"modified_at"`
}

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

type codexTaskPacket struct {
	SchemaVersion    int                       `json:"schema_version"`
	PacketID         string                    `json:"packet_id"`
	CreatedAt        string                    `json:"created_at"`
	Objective        string                    `json:"objective"`
	Role             string                    `json:"role"`
	CWD              string                    `json:"cwd"`
	AllowedPaths     []string                  `json:"allowed_paths,omitempty"`
	ForbiddenActions []string                  `json:"forbidden_actions,omitempty"`
	Sandbox          string                    `json:"sandbox"`
	Auth             codexTaskAuthGuard        `json:"auth"`
	Dispatch         codexTaskDispatchPolicy   `json:"dispatch"`
	Execution        codexTaskExecution        `json:"execution"`
	Output           codexTaskOutputContract   `json:"output"`
	Evidence         codexTaskEvidenceContract `json:"evidence"`
	StopCondition    string                    `json:"stop_condition"`
	Notes            []string                  `json:"notes,omitempty"`
}

type codexTaskAuthGuard struct {
	RequiredMode           string   `json:"required_mode"`
	RejectEnv              []string `json:"reject_env"`
	LoginStatusMustContain string   `json:"login_status_must_contain"`
	ForbidAPIKey           bool     `json:"forbid_api_key"`
}

type codexTaskDispatchPolicy struct {
	Mode        string   `json:"mode"`
	MutatesRepo bool     `json:"mutates_repo"`
	Command     []string `json:"command"`
	Notes       string   `json:"notes,omitempty"`
}

type codexTaskExecution struct {
	Argv             []string          `json:"argv"`
	Stdin            codexTaskStdin    `json:"stdin"`
	TimeoutSeconds   int               `json:"timeout_seconds"`
	PromptPath       string            `json:"prompt_path,omitempty"`
	OutputSchemaPath string            `json:"output_schema_path,omitempty"`
	Environment      map[string]string `json:"environment,omitempty"`
}

type codexTaskStdin struct {
	Mode             string `json:"mode"`
	CloseAfterPrompt bool   `json:"close_after_prompt"`
}

type codexTaskOutputContract struct {
	CaptureMode      string `json:"capture_mode"`
	FinalMessagePath string `json:"final_message_path"`
	JSONLPath        string `json:"jsonl_path,omitempty"`
	SchemaPath       string `json:"schema_path,omitempty"`
	ReceiptPath      string `json:"receipt_path"`
}

type codexTaskEvidenceContract struct {
	ReceiptPath      string   `json:"receipt_path"`
	RequiredCommands []string `json:"required_commands"`
	Artifacts        []string `json:"artifacts,omitempty"`
}

type codexRunReceipt struct {
	SchemaVersion     int                  `json:"schema_version"`
	ReceiptID         string               `json:"receipt_id"`
	PacketID          string               `json:"packet_id"`
	CodexSessionID    string               `json:"codex_session_id,omitempty"`
	StartedAt         string               `json:"started_at"`
	EndedAt           string               `json:"ended_at"`
	CWD               string               `json:"cwd"`
	Sandbox           string               `json:"sandbox"`
	AuthMode          string               `json:"auth_mode"`
	AuthStatus        string               `json:"auth_status"`
	Command           codexReceiptCommand  `json:"command"`
	Stdin             codexReceiptStdin    `json:"stdin"`
	TimeoutSeconds    int                  `json:"timeout_seconds"`
	TimedOut          bool                 `json:"timed_out"`
	ExitCode          int                  `json:"exit_code"`
	Outputs           codexReceiptOutputs  `json:"outputs"`
	ChangedFiles      []string             `json:"changed_files"`
	CommandsRun       []codexCommandResult `json:"commands_run"`
	Verdict           codexReceiptVerdict  `json:"verdict"`
	ResumeFromSession string               `json:"resume_from_session,omitempty"`
	Evidence          []codexEvidenceRef   `json:"evidence,omitempty"`
	FailureReason     string               `json:"failure_reason,omitempty"`
}

type codexReceiptCommand struct {
	Argv []string `json:"argv"`
}

type codexReceiptStdin struct {
	Mode         string `json:"mode"`
	ClosedAt     string `json:"closed_at"`
	BytesWritten int    `json:"bytes_written"`
}

type codexReceiptOutputs struct {
	FinalMessagePath string `json:"final_message_path"`
	JSONLPath        string `json:"jsonl_path,omitempty"`
	SchemaPath       string `json:"schema_path,omitempty"`
	ReceiptPath      string `json:"receipt_path"`
}

type codexCommandResult struct {
	Command       string `json:"command"`
	ExitCode      int    `json:"exit_code"`
	OutputExcerpt string `json:"output_excerpt,omitempty"`
}

type codexReceiptVerdict struct {
	Status      string `json:"status"`
	JudgeSource string `json:"judge_source"`
	Summary     string `json:"summary"`
}

type codexEvidenceRef struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
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

func init() {
	codexCmd.GroupID = "workflow"
	rootCmd.AddCommand(codexCmd)
	codexCmd.AddCommand(codexStartCmd, codexEnsureStartCmd, codexStopCmd, codexEnsureStopCmd, codexStatusCmd, codexDispatchCmd)

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

func findTranscriptBySessionID(sessionID string) (string, error) {
	return codexruntime.FindTranscriptBySessionID(sessionID)
}

func synthesizeCodexHistoryTranscript(cwd, sessionID string) (string, error) {
	return codexruntime.SynthesizeCodexHistoryTranscript(cwd, sessionID)
}

func findLastSession() (string, error) {
	return codexruntime.FindLastSession()
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

func loadCodexTaskPacket(path string) (codexTaskPacket, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return codexTaskPacket{}, fmt.Errorf("codex dispatch requires --packet")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return codexTaskPacket{}, fmt.Errorf("read codex task packet: %w", err)
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
	if packet.Dispatch.Mode != "non-mutating" || packet.Dispatch.MutatesRepo {
		return fmt.Errorf("codex dispatch only accepts non-mutating packets")
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

	authStatus, err := validateCodexDispatchAuth(packet)
	if err != nil {
		return codexRunReceipt{}, err
	}
	if err := ensureCodexDispatchOutputDirs(cwd, packet.Output); err != nil {
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

	if err := writeCodexDispatchOutputFiles(cwd, packet.Output, stdout.Bytes()); err != nil {
		return codexRunReceipt{}, err
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
		CommandsRun: []codexCommandResult{{
			Command:       strings.Join(argv, " "),
			ExitCode:      exitCode,
			OutputExcerpt: codexDispatchOutputExcerpt(stdout.String(), stderr.String()),
		}},
		Verdict:       codexDispatchVerdict(cwd, packet.Output, exitCode, timedOut),
		Evidence:      codexDispatchEvidence(packet),
		FailureReason: failureReason,
	}
	if err := writeCodexRunReceipt(cwd, packet.Output.ReceiptPath, receipt); err != nil {
		return codexRunReceipt{}, err
	}

	if timedOut {
		return receipt, fmt.Errorf("codex dispatch timed out after %ds", packet.Execution.TimeoutSeconds)
	}
	if runErr != nil {
		return receipt, fmt.Errorf("codex dispatch failed: %s", failureReason)
	}
	return receipt, nil
}

func validateCodexDispatchAuth(packet codexTaskPacket) (string, error) {
	for _, name := range packet.Auth.RejectEnv {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if os.Getenv(name) != "" {
			return "", fmt.Errorf("codex dispatch refuses %s in environment; use ChatGPT subscription auth", name)
		}
	}
	if packet.Auth.ForbidAPIKey && os.Getenv("OPENAI_API_KEY") != "" {
		return "", fmt.Errorf("codex dispatch refuses OPENAI_API_KEY in environment; use ChatGPT subscription auth")
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

func codexDispatchBinary(packet codexTaskPacket) string {
	if len(packet.Execution.Argv) == 0 || strings.TrimSpace(packet.Execution.Argv[0]) == "" {
		return "codex"
	}
	return packet.Execution.Argv[0]
}

func ensureCodexDispatchOutputDirs(cwd string, out codexTaskOutputContract) error {
	for _, path := range []string{out.FinalMessagePath, out.JSONLPath, out.SchemaPath, out.ReceiptPath} {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		abs := resolveCodexDispatchPath(cwd, path)
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
		data, err := os.ReadFile(resolveCodexDispatchPath(cwd, promptPath))
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

func writeCodexDispatchOutputFiles(cwd string, out codexTaskOutputContract, stdout []byte) error {
	if strings.TrimSpace(out.JSONLPath) != "" && len(stdout) > 0 {
		if err := atomicWriteFile(resolveCodexDispatchPath(cwd, out.JSONLPath), stdout, 0o600); err != nil {
			return fmt.Errorf("write codex jsonl output: %w", err)
		}
	}
	return nil
}

func collectCodexDispatchChangedFiles(cwd string) []string {
	cmd := exec.Command("git", "-C", cwd, "status", "--short", "--untracked-files=all")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var files []string
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

func codexDispatchVerdict(cwd string, out codexTaskOutputContract, exitCode int, timedOut bool) codexReceiptVerdict {
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
	data, err := os.ReadFile(resolveCodexDispatchPath(cwd, finalPath))
	if err != nil {
		return codexReceiptVerdict{Status: "NO_VERDICT", JudgeSource: "codex-dispatch", Summary: "Final message was not readable."}
	}
	text := string(data)
	status := parseCodexVerdictStatus(text)
	if status == "NO_VERDICT" {
		return codexReceiptVerdict{Status: status, JudgeSource: "codex-final-message", Summary: firstLine(text)}
	}
	return codexReceiptVerdict{Status: status, JudgeSource: "codex-final-message", Summary: firstLine(text)}
}

func parseCodexVerdictStatus(text string) string {
	upper := strings.ToUpper(text)
	for _, status := range []string{"PASS", "WARN", "FAIL", "ERROR"} {
		if strings.Contains(upper, "VERDICT: "+status) || strings.Contains(upper, "VERDICT="+status) {
			return status
		}
	}
	return "NO_VERDICT"
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

func writeCodexRunReceipt(cwd, path string, receipt codexRunReceipt) error {
	abs := resolveCodexDispatchPath(cwd, path)
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal codex run receipt: %w", err)
	}
	if err := atomicWriteFile(abs, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write codex run receipt: %w", err)
	}
	return nil
}

func resolveCodexDispatchPath(cwd, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(cwd, path)
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
	nextWork, _ := readUnconsumedItems(filepath.Join(cwd, ".agents", "rpi", "next-work.jsonl"), repoFilter)
	if len(nextWork) > limit {
		nextWork = nextWork[:limit]
	}

	research := collectRecentResearchArtifacts(cwd, query, limit)
	return briefings, learnings, patterns, findings, recentSessions, nextWork, research
}

func collectRecentResearchArtifacts(cwd, query string, limit int) []codexArtifactRef {
	return collectRecentCodexArtifacts(filepath.Join(cwd, ".agents", SectionResearch), query, limit)
}

func collectRecentCodexArtifacts(dir, query string, limit int) []codexArtifactRef {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	type researchFile struct {
		path    string
		modTime time.Time
	}
	var files []researchFile
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, researchFile{
			path:    filepath.Join(dir, entry.Name()),
			modTime: info.ModTime(),
		})
	}
	slices.SortFunc(files, func(a, b researchFile) int {
		return b.modTime.Compare(a.modTime)
	})

	queryLower := strings.ToLower(strings.TrimSpace(query))
	var artifacts []codexArtifactRef
	for _, file := range files {
		if queryLower != "" {
			baseLower := strings.ToLower(filepath.Base(file.path))
			if !strings.Contains(baseLower, queryLower) {
				data, err := os.ReadFile(file.path)
				if err != nil || !strings.Contains(strings.ToLower(string(data)), queryLower) {
					continue
				}
			}
		}
		artifacts = append(artifacts, codexArtifactRef{
			Title:      strings.TrimSuffix(filepath.Base(file.path), filepath.Ext(file.path)),
			Path:       file.path,
			ModifiedAt: file.modTime.UTC().Format(time.RFC3339),
		})
		if len(artifacts) >= limit {
			break
		}
	}
	return artifacts
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

func firstNonEmptyTrimmed(values ...string) string {
	return bridge.FirstNonEmptyTrimmed(values...)
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
	nextWork, _ := readUnconsumedItems(filepath.Join(cwd, ".agents", "rpi", "next-work.jsonl"), repoFilter)
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
