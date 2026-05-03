package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	contextbudget "github.com/boshu2/agentops/cli/internal/context"
	"github.com/spf13/cobra"
)

const (
	transcriptTailMaxBytes = 512 * 1024
	defaultWatchdogMinutes = 20
)

// contextOptions snapshots package-level flag vars into a ContextOptions
// struct so internal/context can be exercised through an explicit options
// value rather than mutating globals.
func contextOptions() contextbudget.ContextOptions {
	return contextbudget.ContextOptions{
		SessionID: contextSessionID, Prompt: contextPrompt, AgentName: contextAgentName,
		MaxTokens: contextMaxTokens, WriteHandoff: contextWriteHandoff, AutoRestart: contextAutoRestart,
	}
}

var (
	contextSessionID      string
	contextPrompt         string
	contextAgentName      string
	contextMaxTokens      int
	contextWriteHandoff   bool
	contextAutoRestart    bool
	contextWatchdogMinute int
)

type transcriptUsage = contextbudget.TranscriptUsage

type contextAssignment struct {
	AgentName   string
	AgentRole   string
	TeamName    string
	IssueID     string
	TmuxPaneID  string
	TmuxTarget  string
	TmuxSession string
}

type contextAssignmentSnapshot struct {
	SessionID   string `json:"session_id"`
	AgentName   string `json:"agent_name,omitempty"`
	AgentRole   string `json:"agent_role,omitempty"`
	TeamName    string `json:"team_name,omitempty"`
	IssueID     string `json:"issue_id,omitempty"`
	TmuxPaneID  string `json:"tmux_pane_id,omitempty"`
	TmuxTarget  string `json:"tmux_target,omitempty"`
	TmuxSession string `json:"tmux_session,omitempty"`
	UpdatedAt   string `json:"updated_at"`
}

type teamConfigFile struct {
	Members []teamConfigMember `json:"members"`
}

type teamConfigMember struct {
	Name      string `json:"name"`
	AgentType string `json:"agentType"`
	TmuxPane  string `json:"tmuxPaneId"`
}

type contextSessionStatus struct {
	SessionID        string  `json:"session_id"`
	TranscriptPath   string  `json:"transcript_path,omitempty"`
	Model            string  `json:"model,omitempty"`
	LastTask         string  `json:"last_task,omitempty"`
	InputTokens      int     `json:"input_tokens"`
	CacheCreate      int     `json:"cache_creation_input_tokens"`
	CacheRead        int     `json:"cache_read_input_tokens"`
	EstimatedUsage   int     `json:"estimated_usage"`
	MaxTokens        int     `json:"max_tokens"`
	UsagePercent     float64 `json:"usage_percent"`
	RemainingPercent float64 `json:"remaining_percent"`
	Status           string  `json:"status"`
	Readiness        string  `json:"readiness"`
	ReadinessAction  string  `json:"readiness_action"`
	Recommendation   string  `json:"recommendation"`
	LastUpdated      string  `json:"last_updated,omitempty"`
	IsStale          bool    `json:"is_stale"`
	Action           string  `json:"action"`
	AgentName        string  `json:"agent_name,omitempty"`
	AgentRole        string  `json:"agent_role,omitempty"`
	TeamName         string  `json:"team_name,omitempty"`
	IssueID          string  `json:"issue_id,omitempty"`
	TmuxPaneID       string  `json:"tmux_pane_id,omitempty"`
	TmuxTarget       string  `json:"tmux_target,omitempty"`
	TmuxSession      string  `json:"tmux_session,omitempty"`
	RestartAttempt   bool    `json:"restart_attempted,omitempty"`
	RestartSuccess   bool    `json:"restart_succeeded,omitempty"`
	RestartMessage   string  `json:"restart_message,omitempty"`
}

type contextGuardResult struct {
	Session       contextSessionStatus `json:"session"`
	HandoffFile   string               `json:"handoff_file,omitempty"`
	PendingMarker string               `json:"pending_marker,omitempty"`
	HookMessage   string               `json:"hook_message,omitempty"`
}

type handoffMarker = contextbudget.HandoffMarker

const (
	contextReadinessGreen    = "GREEN"
	contextReadinessAmber    = "AMBER"
	contextReadinessRed      = "RED"
	contextReadinessCritical = "CRITICAL"
)

// contextCmd is the parent command for context operations (hidden).
var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Context health telemetry and handoff guardrails",
}

func init() {

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show context health across tracked sessions",
		Long: `Aggregate context budget telemetry from .agents/ao/context and classify
sessions into OPTIMAL, WARNING, or CRITICAL with watchdog actions.

Examples:
  ao context status
  ao context status --json`,
		RunE: runContextStatus,
	}
	statusCmd.Flags().IntVar(&contextWatchdogMinute, "watchdog-minutes", defaultWatchdogMinutes, "Mark sessions stale after N minutes without telemetry updates")

	guardCmd := &cobra.Command{
		Use:   "guard",
		Short: "Update one session's telemetry and trigger auto-handoff at CRITICAL",
		Long: `Resolve session telemetry from transcript usage, update budget state,
and optionally write a one-shot auto-handoff marker on CRITICAL.

Examples:
  ao context guard
  ao context guard --session <id> --write-handoff --json`,
		RunE: runContextGuard,
	}
	guardCmd.Flags().StringVar(&contextSessionID, "session", "", "Session ID (default: $CLAUDE_SESSION_ID)")
	guardCmd.Flags().StringVar(&contextPrompt, "prompt", "", "Current user prompt (used as immediate task hint)")
	guardCmd.Flags().StringVar(&contextAgentName, "agent-name", "", "Worker/agent name for assignment mapping (default: $CLAUDE_AGENT_NAME)")
	guardCmd.Flags().IntVar(&contextMaxTokens, "max-tokens", contextbudget.DefaultMaxTokens, "Context window size for percentage calculations")
	guardCmd.Flags().BoolVar(&contextWriteHandoff, "write-handoff", false, "Write auto-handoff marker when status is CRITICAL")
	guardCmd.Flags().BoolVar(&contextAutoRestart, "auto-restart-stale", false, "Attempt tmux restart when stale non-optimal sessions appear dead")
	guardCmd.Flags().IntVar(&contextWatchdogMinute, "watchdog-minutes", defaultWatchdogMinutes, "Mark session stale after N minutes without telemetry updates")

	contextCmd.AddCommand(statusCmd, guardCmd)
	contextCmd.Hidden = true
	contextCmd.GroupID = "workflow"
	rootCmd.AddCommand(contextCmd)
}

func runContextStatus(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	statuses, err := collectTrackedSessionStatuses(cwd, time.Duration(contextWatchdogMinute)*time.Minute)
	if err != nil {
		return err
	}

	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(statuses)
	}

	if len(statuses) == 0 {
		fmt.Println("No context telemetry found. Run `ao context guard` from an active session.")
		return nil
	}

	fmt.Printf("%-18s %-10s %-9s %-9s %-9s %-8s %-14s %-12s %-22s %s\n", "SESSION", "STATUS", "USAGE", "REMAIN", "HULL", "STALE", "AGENT", "ISSUE", "ACTION", "TASK")
	fmt.Println(strings.Repeat("─", 170))
	for _, s := range statuses {
		task := s.LastTask
		if len(task) > 48 {
			task = task[:45] + "..."
		}
		fmt.Printf("%-18s %-10s %6.1f%%   %6.1f%%   %-9s %-8t %-14s %-12s %-22s %s\n",
			truncateDisplay(s.SessionID, 18),
			s.Status,
			s.UsagePercent*100,
			s.RemainingPercent*100,
			s.Readiness,
			s.IsStale,
			truncateDisplay(displayOrDash(s.AgentName), 14),
			truncateDisplay(displayOrDash(s.IssueID), 12),
			s.Action,
			task,
		)
	}
	return nil
}

func runContextGuard(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	sessionID, err := resolveGuardSessionID()
	if err != nil {
		return err
	}
	maxTokens, watchdog, agentName := resolveGuardOptions()

	status, usage, err := collectSessionStatus(cwd, sessionID, strings.TrimSpace(contextPrompt), maxTokens, watchdog, agentName)
	if err != nil {
		return err
	}
	if contextAutoRestart {
		status = maybeAutoRestartStaleSession(status)
	}
	if err := persistGuardState(cwd, status); err != nil {
		return err
	}

	result := contextGuardResult{
		Session:     status,
		HookMessage: hookMessageForStatus(status),
	}
	if err := applyHandoffIfCritical(cwd, status, usage, &result); err != nil {
		return err
	}

	return outputGuardResult(result)
}

func persistGuardState(cwd string, status contextSessionStatus) error {
	if err := persistBudget(cwd, status); err != nil {
		return fmt.Errorf("persist budget: %w", err)
	}
	if err := persistAssignment(cwd, status); err != nil {
		return fmt.Errorf("persist assignment: %w", err)
	}
	return nil
}

func outputGuardResult(result contextGuardResult) error {
	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	fmt.Printf("Session: %s\n", result.Session.SessionID)
	fmt.Printf("Status: %s (%.1f%%)\n", result.Session.Status, result.Session.UsagePercent*100)
	fmt.Printf("Action: %s\n", result.Session.Action)
	if result.HandoffFile != "" {
		fmt.Printf("Handoff: %s\n", result.HandoffFile)
	}
	if result.HookMessage != "" {
		fmt.Println(result.HookMessage)
	}
	return nil
}

// resolveGuardSessionID returns the session ID from the flag or CLAUDE_SESSION_ID env var.
func resolveGuardSessionID() (string, error) {
	sessionID := strings.TrimSpace(contextSessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(os.Getenv("CLAUDE_SESSION_ID"))
	}
	if sessionID == "" {
		return "", errors.New("session id missing: set --session or CLAUDE_SESSION_ID")
	}
	return sessionID, nil
}

// resolveGuardOptions returns maxTokens, watchdog, and agentName from flags/env.
func resolveGuardOptions() (maxTokens int, watchdog time.Duration, agentName string) {
	opts := contextOptions()
	maxTokens = opts.MaxTokens
	if maxTokens <= 0 {
		maxTokens = contextbudget.DefaultMaxTokens
	}
	watchdog = time.Duration(contextWatchdogMinute) * time.Minute
	if watchdog <= 0 {
		watchdog = defaultWatchdogMinutes * time.Minute
	}
	agentName = strings.TrimSpace(opts.AgentName)
	if agentName == "" {
		agentName = strings.TrimSpace(os.Getenv("CLAUDE_AGENT_NAME"))
	}
	return maxTokens, watchdog, agentName
}

// applyHandoffIfCritical writes a handoff file when the session is critical and the flag is set.
func applyHandoffIfCritical(cwd string, status contextSessionStatus, usage transcriptUsage, result *contextGuardResult) error {
	if !contextWriteHandoff || status.Status != string(contextbudget.StatusCritical) {
		return nil
	}
	handoffPath, markerPath, hErr := ensureCriticalHandoff(cwd, status, usage)
	if hErr != nil {
		return fmt.Errorf("write critical handoff: %w", hErr)
	}
	result.HandoffFile = handoffPath
	result.PendingMarker = markerPath
	if result.HookMessage != "" {
		result.HookMessage = fmt.Sprintf("%s Handoff saved to %s.", result.HookMessage, handoffPath)
	}
	return nil
}

func collectTrackedSessionStatuses(cwd string, watchdog time.Duration) ([]contextSessionStatus, error) {
	budgetGlob := filepath.Join(cwd, ".agents", "ao", "context", "budget-*.json")
	files, err := filepath.Glob(budgetGlob)
	if err != nil {
		return nil, fmt.Errorf("glob budgets: %w", err)
	}
	if len(files) == 0 {
		return nil, nil
	}
	sort.Strings(files)

	statuses := make([]contextSessionStatus, 0, len(files))
	for _, path := range files {
		status, ok := collectOneTrackedStatus(cwd, path, watchdog)
		if !ok {
			continue
		}
		mergePersistedAssignment(cwd, &status)
		statuses = append(statuses, status)
	}
	slices.SortFunc(statuses, compareSessionStatuses)
	return statuses, nil
}

func collectOneTrackedStatus(cwd, path string, watchdog time.Duration) (contextSessionStatus, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return contextSessionStatus{}, false
	}
	var b contextbudget.BudgetTracker
	if err := json.Unmarshal(data, &b); err != nil || strings.TrimSpace(b.SessionID) == "" {
		return contextSessionStatus{}, false
	}
	status, _, err := collectSessionStatus(cwd, b.SessionID, "", b.MaxTokens, watchdog, "")
	if err != nil {
		status = staleBudgetFallbackStatus(b, watchdog)
	}
	return status, true
}

func staleBudgetFallbackStatus(b contextbudget.BudgetTracker, watchdog time.Duration) contextSessionStatus {
	isStale := !b.LastUpdated.IsZero() && time.Since(b.LastUpdated) > watchdog
	return contextSessionStatus{
		SessionID:        b.SessionID,
		EstimatedUsage:   b.EstimatedUsage,
		MaxTokens:        nonZeroOrDefault(b.MaxTokens, contextbudget.DefaultMaxTokens),
		UsagePercent:     b.GetUsagePercent(),
		RemainingPercent: remainingPercent(b.GetUsagePercent()),
		Status:           string(b.GetStatus()),
		Readiness:        readinessForUsage(b.GetUsagePercent()),
		ReadinessAction:  readinessAction(readinessForUsage(b.GetUsagePercent())),
		Recommendation:   b.GetRecommendation(),
		LastUpdated:      b.LastUpdated.Format(time.RFC3339),
		IsStale:          isStale,
		Action:           actionForStatus(string(b.GetStatus()), isStale),
	}
}

// compareSessionStatuses orders sessions by readiness rank, then status severity, then stale-first, then ID.
func compareSessionStatuses(a, b contextSessionStatus) int {
	if c := cmp.Compare(readinessRank(a.Readiness), readinessRank(b.Readiness)); c != 0 {
		return c
	}
	statusRank := func(s string) int {
		switch s {
		case string(contextbudget.StatusCritical):
			return 0
		case string(contextbudget.StatusWarning):
			return 1
		default:
			return 2
		}
	}
	if c := cmp.Compare(statusRank(a.Status), statusRank(b.Status)); c != 0 {
		return c
	}
	if a.IsStale != b.IsStale {
		if a.IsStale {
			return -1
		}
		return 1
	}
	return cmp.Compare(a.SessionID, b.SessionID)
}

func collectSessionStatus(cwd, sessionID, prompt string, maxTokens int, watchdog time.Duration, agentName string) (contextSessionStatus, transcriptUsage, error) {
	transcriptPath, err := findTranscriptBySessionID(sessionID)
	if err != nil {
		return contextSessionStatus{}, transcriptUsage{}, fmt.Errorf("find transcript for session %s: %w", sessionID, err)
	}
	usage, lastTask, lastUpdated, err := readSessionTail(transcriptPath)
	if err != nil {
		return contextSessionStatus{}, transcriptUsage{}, fmt.Errorf("read transcript telemetry: %w", err)
	}
	if strings.TrimSpace(prompt) != "" {
		lastTask = strings.TrimSpace(prompt)
	}
	if usage.Timestamp.IsZero() {
		usage.Timestamp = lastUpdated
	}
	estimated := usage.InputTokens + usage.CacheCreationInputToken + usage.CacheReadInputToken
	if estimated <= 0 {
		estimated = estimateTokens(lastTask)
	}
	max := nonZeroOrDefault(maxTokens, contextbudget.DefaultMaxTokens)

	tracker := contextbudget.NewBudgetTracker(sessionID)
	tracker.MaxTokens = max
	tracker.UpdateUsage(estimated)
	usagePercent := tracker.GetUsagePercent()
	readiness := readinessForUsage(usagePercent)

	isStale := !usage.Timestamp.IsZero() && watchdog > 0 && time.Since(usage.Timestamp) > watchdog
	status := contextSessionStatus{
		SessionID:        sessionID,
		TranscriptPath:   transcriptPath,
		Model:            usage.Model,
		LastTask:         normalizeLine(lastTask),
		InputTokens:      usage.InputTokens,
		CacheCreate:      usage.CacheCreationInputToken,
		CacheRead:        usage.CacheReadInputToken,
		EstimatedUsage:   estimated,
		MaxTokens:        max,
		UsagePercent:     usagePercent,
		RemainingPercent: remainingPercent(usagePercent),
		Status:           string(tracker.GetStatus()),
		Readiness:        readiness,
		ReadinessAction:  readinessAction(readiness),
		Recommendation:   tracker.GetRecommendation(),
		LastUpdated:      usage.Timestamp.UTC().Format(time.RFC3339),
		IsStale:          isStale,
		Action:           actionForStatus(string(tracker.GetStatus()), isStale),
	}
	applyContextAssignment(&status, resolveContextAssignment(cwd, status.LastTask, agentName))
	mergePersistedAssignment(cwd, &status)
	return status, usage, nil
}

func persistBudget(cwd string, status contextSessionStatus) error {
	tracker, err := contextbudget.Load(cwd, status.SessionID)
	if err != nil {
		tracker = contextbudget.NewBudgetTracker(status.SessionID)
	}
	tracker.MaxTokens = status.MaxTokens
	tracker.UpdateUsage(status.EstimatedUsage)
	return tracker.Save(cwd)
}

func ensureCriticalHandoff(cwd string, status contextSessionStatus, usage transcriptUsage) (string, string, error) {
	existingPath, existingMarker, found, err := findPendingHandoffForSession(cwd, status.SessionID)
	if err == nil && found {
		return existingPath, existingMarker, nil
	}

	handoffDir := filepath.Join(cwd, ".agents", "handoff")
	pendingDir := filepath.Join(handoffDir, "pending")
	if err := os.MkdirAll(pendingDir, 0750); err != nil {
		return "", "", fmt.Errorf("create pending dir: %w", err)
	}

	now := time.Now().UTC()
	safeSession := sanitizeForFilename(status.SessionID)
	base := fmt.Sprintf("auto-%s-%s", now.Format("20060102T150405Z"), safeSession)
	handoffPath := filepath.Join(handoffDir, base+".md")
	markerPath := filepath.Join(pendingDir, base+".json")

	changedFiles := gitChangedFiles(cwd, 20)
	activeBead := cmp.Or(strings.TrimSpace(runCommand(cwd, 1200*time.Millisecond, "bd", "current")), "none")
	status.LastTask = cmp.Or(status.LastTask, "none recorded")

	md := renderHandoffMarkdown(now, status, usage, activeBead, changedFiles)
	if err := os.WriteFile(handoffPath, []byte(md), 0o600); err != nil {
		return "", "", fmt.Errorf("write handoff markdown: %w", err)
	}

	relPath := toRepoRelative(cwd, handoffPath)
	marker := handoffMarker{
		SchemaVersion: 1,
		ID:            base,
		CreatedAt:     now.Format(time.RFC3339),
		SessionID:     status.SessionID,
		Status:        status.Status,
		UsagePercent:  status.UsagePercent,
		HandoffFile:   relPath,
		Consumed:      false,
	}
	data, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("marshal handoff marker: %w", err)
	}
	if err := os.WriteFile(markerPath, data, 0o600); err != nil {
		return "", "", fmt.Errorf("write handoff marker: %w", err)
	}

	return relPath, toRepoRelative(cwd, markerPath), nil
}

func renderHandoffMarkdown(now time.Time, status contextSessionStatus, usage transcriptUsage, activeBead string, changedFiles []string) string {
	return contextbudget.RenderHandoffMarkdown(contextbudget.HandoffInputs{
		Now: now, SessionID: status.SessionID, Status: status.Status,
		UsagePercent: status.UsagePercent, Readiness: status.Readiness,
		RemainingPercent: status.RemainingPercent, Action: status.Action,
		LastTask: status.LastTask, ActiveBead: activeBead,
		AgentName: status.AgentName, AgentRole: status.AgentRole,
		TeamName: status.TeamName, IssueID: status.IssueID,
		TmuxTarget: status.TmuxTarget, ChangedFiles: changedFiles,
		IsStale: status.IsStale, Model: status.Model,
		InputTokens: usage.InputTokens, CacheCreationInputToken: usage.CacheCreationInputToken,
		CacheReadInputToken: usage.CacheReadInputToken,
		EstimatedUsage:      status.EstimatedUsage, MaxTokens: status.MaxTokens,
		Recommendation: status.Recommendation,
	})
}

func findPendingHandoffForSession(cwd, sessionID string) (string, string, bool, error) {
	return contextbudget.FindPendingHandoffForSession(cwd, sessionID)
}

func matchPendingHandoff(path, cwd, sessionID string) (string, string, bool) {
	return contextbudget.MatchPendingHandoff(path, cwd, sessionID)
}

type tailLineEnvelope = contextbudget.TailLineEnvelope

func readSessionTail(path string) (transcriptUsage, string, time.Time, error) {
	return contextbudget.ReadSessionTail(path)
}

func extractTailUsageAndTask(lines []string) (transcriptUsage, string, time.Time) {
	return contextbudget.ExtractTailUsageAndTask(lines)
}

func fixupTailTimestamps(path string, usage *transcriptUsage, newestTS *time.Time) {
	contextbudget.FixupTailTimestamps(path, usage, newestTS)
}

func scanTailLines(data []byte) ([]string, error) { return contextbudget.ScanTailLines(data) }

func extractUsageFromTailEntry(entry tailLineEnvelope, ts time.Time) transcriptUsage {
	return contextbudget.ExtractUsageFromTailEntry(entry, ts)
}

func extractTaskFromTailEntry(entry tailLineEnvelope) string {
	return contextbudget.ExtractTaskFromTailEntry(entry)
}

func updateTailState(entry tailLineEnvelope, ts time.Time, usage *transcriptUsage, lastTask *string, newestTS *time.Time) bool {
	return contextbudget.UpdateTailState(entry, ts, usage, lastTask, newestTS)
}

func readFileTail(path string, maxBytes int64) ([]byte, error) {
	return contextbudget.ReadFileTail(path, maxBytes)
}

func seekAndReadTail(f *os.File, size, maxBytes int64) ([]byte, error) {
	return contextbudget.SeekAndReadTail(f, size, maxBytes)
}

func parseTimestamp(raw string) time.Time { return contextbudget.ParseTimestamp(raw) }

func extractTextContent(raw json.RawMessage) string { return contextbudget.ExtractTextContent(raw) }

func estimateTokens(text string) int {
	return contextbudget.EstimateTokensFromChars(text, InjectCharsPerToken)
}

func actionForStatus(status string, stale bool) string {
	return contextbudget.ActionForStatus(status, stale, string(contextbudget.StatusOptimal), string(contextbudget.StatusCritical), string(contextbudget.StatusWarning))
}

func hookMessageForStatus(status contextSessionStatus) string {
	switch status.Action {
	case "handoff_now":
		return fmt.Sprintf("Context is CRITICAL (%.1f%% used, %s %.1f%% remaining). End this session and start a fresh one to avoid compaction loss.", status.UsagePercent*100, status.Readiness, status.RemainingPercent*100)
	case "checkpoint_and_prepare_handoff":
		return fmt.Sprintf("Context is WARNING (%.1f%% used, hull %s %.1f%% remaining). Prepare a handoff before continuing long orchestration.", status.UsagePercent*100, status.Readiness, status.RemainingPercent*100)
	case "recover_dead_session":
		if status.RestartAttempt {
			if status.RestartSuccess {
				return fmt.Sprintf("Watchdog: stale session auto-restarted (%s). Verify bootstrap and continue in the fresh session.", status.TmuxSession)
			}
			return fmt.Sprintf("Watchdog: stale session auto-restart failed (%s). Trigger recovery handoff.", status.RestartMessage)
		}
		if status.RestartMessage != "" {
			return fmt.Sprintf("Watchdog: session appears stale with unfinished work (%s). Trigger recovery handoff.", status.RestartMessage)
		}
		return "Watchdog: session appears stale with unfinished work. Trigger recovery handoff."
	default:
		if status.Readiness == contextReadinessRed {
			return fmt.Sprintf("Hull is RED (%.1f%% remaining). Finish current work and prepare relief-on-station handoff.", status.RemainingPercent*100)
		}
		return ""
	}
}

func resolveContextAssignment(cwd, task, agentName string) contextAssignment {
	assignment := contextAssignment{
		AgentName: strings.TrimSpace(agentName),
	}
	assignment.IssueID = extractIssueID(task)
	if assignment.IssueID == "" {
		assignment.IssueID = extractIssueID(runCommand(cwd, 1200*time.Millisecond, "bd", "current"))
	}
	if assignment.AgentName != "" {
		teamName, member, ok := findTeamMemberByName(assignment.AgentName)
		if ok {
			assignment.TeamName = teamName
			assignment.TmuxPaneID = strings.TrimSpace(member.TmuxPane)
			assignment.TmuxTarget = tmuxTargetFromPaneID(assignment.TmuxPaneID)
			assignment.TmuxSession = tmuxSessionFromTarget(assignment.TmuxTarget)
			assignment.AgentRole = normalizeLine(member.AgentType)
		}
	}
	assignment.AgentRole = inferAgentRole(assignment.AgentName, assignment.AgentRole)
	return assignment
}

func applyContextAssignment(status *contextSessionStatus, assignment contextAssignment) {
	if status == nil {
		return
	}
	if v := strings.TrimSpace(assignment.AgentName); v != "" {
		status.AgentName = v
	}
	if v := strings.TrimSpace(assignment.AgentRole); v != "" {
		status.AgentRole = v
	}
	if v := strings.TrimSpace(assignment.TeamName); v != "" {
		status.TeamName = v
	}
	if v := strings.TrimSpace(assignment.IssueID); v != "" {
		status.IssueID = v
	}
	if v := strings.TrimSpace(assignment.TmuxPaneID); v != "" {
		status.TmuxPaneID = v
	}
	if v := strings.TrimSpace(assignment.TmuxTarget); v != "" {
		status.TmuxTarget = v
	}
	if v := strings.TrimSpace(assignment.TmuxSession); v != "" {
		status.TmuxSession = v
	}
}

func assignmentFromStatus(status contextSessionStatus) contextAssignment {
	return contextAssignment{
		AgentName:   strings.TrimSpace(status.AgentName),
		AgentRole:   strings.TrimSpace(status.AgentRole),
		TeamName:    strings.TrimSpace(status.TeamName),
		IssueID:     strings.TrimSpace(status.IssueID),
		TmuxPaneID:  strings.TrimSpace(status.TmuxPaneID),
		TmuxTarget:  strings.TrimSpace(status.TmuxTarget),
		TmuxSession: strings.TrimSpace(status.TmuxSession),
	}
}

func (a contextAssignment) isEmpty() bool {
	return strings.TrimSpace(a.AgentName) == "" && strings.TrimSpace(a.AgentRole) == "" &&
		strings.TrimSpace(a.TeamName) == "" && strings.TrimSpace(a.IssueID) == "" &&
		strings.TrimSpace(a.TmuxPaneID) == "" && strings.TrimSpace(a.TmuxTarget) == "" &&
		strings.TrimSpace(a.TmuxSession) == ""
}

func persistAssignment(cwd string, status contextSessionStatus) error {
	assignment := assignmentFromStatus(status)
	if assignment.isEmpty() {
		return nil
	}
	snapshot := contextAssignmentSnapshot{
		SessionID:   status.SessionID,
		AgentName:   assignment.AgentName,
		AgentRole:   assignment.AgentRole,
		TeamName:    assignment.TeamName,
		IssueID:     assignment.IssueID,
		TmuxPaneID:  assignment.TmuxPaneID,
		TmuxTarget:  assignment.TmuxTarget,
		TmuxSession: assignment.TmuxSession,
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	contextDir := filepath.Join(cwd, ".agents", "ao", "context")
	if err := os.MkdirAll(contextDir, 0750); err != nil {
		return err
	}
	path := filepath.Join(contextDir, "assignment-"+sanitizeForFilename(status.SessionID)+".json")
	return os.WriteFile(path, data, 0o600)
}

func mergePersistedAssignment(cwd string, status *contextSessionStatus) {
	if status == nil || strings.TrimSpace(status.SessionID) == "" {
		return
	}
	assignment, ok := readPersistedAssignment(cwd, status.SessionID)
	if !ok {
		return
	}
	current := assignmentFromStatus(*status)
	mergeAssignmentFields(&current, &assignment, status)
}

func mergeAssignmentFields(current, persisted *contextAssignment, status *contextSessionStatus) {
	if current.AgentName == "" {
		status.AgentName = persisted.AgentName
	}
	if current.AgentRole == "" {
		status.AgentRole = persisted.AgentRole
	}
	if current.TeamName == "" {
		status.TeamName = persisted.TeamName
	}
	if current.IssueID == "" {
		status.IssueID = persisted.IssueID
	}
	if current.TmuxPaneID == "" {
		status.TmuxPaneID = persisted.TmuxPaneID
	}
	if current.TmuxTarget == "" {
		status.TmuxTarget = persisted.TmuxTarget
	}
	if current.TmuxSession == "" {
		status.TmuxSession = persisted.TmuxSession
	}
}

func readPersistedAssignment(cwd, sessionID string) (contextAssignment, bool) {
	path := filepath.Join(cwd, ".agents", "ao", "context", "assignment-"+sanitizeForFilename(sessionID)+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return contextAssignment{}, false
	}
	var snapshot contextAssignmentSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return contextAssignment{}, false
	}
	assignment := contextAssignment{
		AgentName:   strings.TrimSpace(snapshot.AgentName),
		AgentRole:   strings.TrimSpace(snapshot.AgentRole),
		TeamName:    strings.TrimSpace(snapshot.TeamName),
		IssueID:     strings.TrimSpace(snapshot.IssueID),
		TmuxPaneID:  strings.TrimSpace(snapshot.TmuxPaneID),
		TmuxTarget:  strings.TrimSpace(snapshot.TmuxTarget),
		TmuxSession: strings.TrimSpace(snapshot.TmuxSession),
	}
	if assignment.isEmpty() {
		return contextAssignment{}, false
	}
	return assignment, true
}

func maybeAutoRestartStaleSession(status contextSessionStatus) contextSessionStatus {
	if status.Action != "recover_dead_session" {
		return status
	}
	target := strings.TrimSpace(status.TmuxTarget)
	if target == "" {
		status.RestartMessage = "missing tmux target mapping"
		return status
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		status.RestartMessage = "tmux unavailable"
		return status
	}
	if tmuxTargetAlive(target) {
		status.RestartMessage = "tmux target already alive"
		return status
	}
	status.RestartAttempt = true
	sessionName := strings.TrimSpace(status.TmuxSession)
	if sessionName == "" {
		sessionName = tmuxSessionFromTarget(target)
	}
	if sessionName == "" {
		status.RestartMessage = "invalid tmux target"
		return status
	}
	if err := tmuxStartDetachedSession(sessionName); err != nil {
		status.RestartMessage = normalizeLine(err.Error())
		return status
	}
	status.RestartSuccess = true
	status.TmuxSession = sessionName
	status.RestartMessage = "started tmux session " + sessionName
	return status
}

func tmuxTargetAlive(target string) bool { return contextbudget.TmuxTargetAlive(target) }

func tmuxStartDetachedSession(sessionName string) error {
	return contextbudget.TmuxStartDetachedSession(sessionName)
}

func searchTeamConfig(cfgPath, agentName string) (teamConfigMember, bool) {
	m, ok := contextbudget.SearchTeamConfigFile(cfgPath, agentName)
	if !ok {
		return teamConfigMember{}, false
	}
	return teamConfigMember{Name: m.Name, AgentType: m.AgentType, TmuxPane: m.TmuxPane}, true
}

func findTeamMemberByName(agentName string) (string, teamConfigMember, bool) {
	team, m, ok := contextbudget.FindTeamMemberByName(agentName)
	if !ok {
		return "", teamConfigMember{}, false
	}
	return team, teamConfigMember{Name: m.Name, AgentType: m.AgentType, TmuxPane: m.TmuxPane}, true
}

func inferAgentRole(agentName, explicitRole string) string {
	return contextbudget.InferAgentRole(agentName, explicitRole)
}

func remainingPercent(usagePercent float64) float64 {
	return contextbudget.RemainingPercent(usagePercent)
}

func readinessForUsage(usagePercent float64) string {
	return contextbudget.ReadinessForUsage(usagePercent)
}

func readinessAction(readiness string) string { return contextbudget.ReadinessAction(readiness) }

func readinessRank(readiness string) int { return contextbudget.ReadinessRank(readiness) }

func extractIssueID(text string) string { return contextbudget.ExtractIssueID(text) }

func tmuxTargetFromPaneID(paneID string) string { return contextbudget.TmuxTargetFromPaneID(paneID) }

func tmuxSessionFromTarget(target string) string { return contextbudget.TmuxSessionFromTarget(target) }

func displayOrDash(value string) string { return contextbudget.DisplayOrDash(value) }

func gitChangedFiles(cwd string, limit int) []string {
	return contextbudget.GitChangedFiles(cwd, limit)
}

func runCommand(cwd string, timeout time.Duration, name string, args ...string) string {
	return contextbudget.RunCommand(cwd, timeout, name, args...)
}

func contextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return contextbudget.WithTimeout(timeout)
}

func sanitizeForFilename(input string) string { return contextbudget.SanitizeForFilename(input) }

func toRepoRelative(cwd, fullPath string) string { return contextbudget.ToRepoRelative(cwd, fullPath) }

func normalizeLine(s string) string { return contextbudget.NormalizeLine(s) }

func nonZeroOrDefault(value, fallback int) int {
	return contextbudget.NonZeroOrDefault(value, fallback)
}

func truncateDisplay(s string, max int) string { return contextbudget.TruncateDisplay(s, max) }
