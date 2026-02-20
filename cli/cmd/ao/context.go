package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

var (
	contextSessionID      string
	contextPrompt         string
	contextAgentName      string
	contextMaxTokens      int
	contextWriteHandoff   bool
	contextAutoRestart    bool
	contextWatchdogMinute int

	filenameSanitizer   = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
	contextIssuePattern = regexp.MustCompile(`(?i)\bag-[a-z0-9]+\b`)
)

type transcriptUsage struct {
	InputTokens             int
	CacheCreationInputToken int
	CacheReadInputToken     int
	Model                   string
	Timestamp               time.Time
}

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

type handoffMarker struct {
	SchemaVersion int     `json:"schema_version"`
	ID            string  `json:"id"`
	CreatedAt     string  `json:"created_at"`
	SessionID     string  `json:"session_id"`
	Status        string  `json:"status"`
	UsagePercent  float64 `json:"usage_percent"`
	HandoffFile   string  `json:"handoff_file"`
	Consumed      bool    `json:"consumed"`
	ConsumedAt    string  `json:"consumed_at,omitempty"`
}

const (
	contextReadinessGreen    = "GREEN"
	contextReadinessAmber    = "AMBER"
	contextReadinessRed      = "RED"
	contextReadinessCritical = "CRITICAL"
)

func init() {
	contextCmd := &cobra.Command{
		Use:   "context",
		Short: "Context health telemetry and handoff guardrails",
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show context health across tracked sessions",
		Long: `Aggregate context budget telemetry from .agents/ao/context and classify
sessions into OPTIMAL, WARNING, or CRITICAL with watchdog actions.

Examples:
  ao context status
  ao context status -o json`,
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
  ao context guard --session <id> --write-handoff -o json`,
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
	sessionID := strings.TrimSpace(contextSessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(os.Getenv("CLAUDE_SESSION_ID"))
	}
	if sessionID == "" {
		return errors.New("session id missing: set --session or CLAUDE_SESSION_ID")
	}
	if contextMaxTokens <= 0 {
		contextMaxTokens = contextbudget.DefaultMaxTokens
	}
	watchdog := time.Duration(contextWatchdogMinute) * time.Minute
	if watchdog <= 0 {
		watchdog = defaultWatchdogMinutes * time.Minute
	}
	agentName := strings.TrimSpace(contextAgentName)
	if agentName == "" {
		agentName = strings.TrimSpace(os.Getenv("CLAUDE_AGENT_NAME"))
	}

	status, usage, err := collectSessionStatus(cwd, sessionID, strings.TrimSpace(contextPrompt), contextMaxTokens, watchdog, agentName)
	if err != nil {
		return err
	}
	if contextAutoRestart {
		status = maybeAutoRestartStaleSession(status)
	}
	if err := persistBudget(cwd, status); err != nil {
		return fmt.Errorf("persist budget: %w", err)
	}
	if err := persistAssignment(cwd, status); err != nil {
		return fmt.Errorf("persist assignment: %w", err)
	}

	result := contextGuardResult{
		Session:     status,
		HookMessage: hookMessageForStatus(status),
	}
	if contextWriteHandoff && status.Status == string(contextbudget.StatusCritical) {
		handoffPath, markerPath, hErr := ensureCriticalHandoff(cwd, status, usage)
		if hErr != nil {
			return fmt.Errorf("write critical handoff: %w", hErr)
		}
		result.HandoffFile = handoffPath
		result.PendingMarker = markerPath
		if result.HookMessage != "" {
			result.HookMessage = fmt.Sprintf("%s Handoff saved to %s.", result.HookMessage, handoffPath)
		}
	}

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
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var b contextbudget.BudgetTracker
		if err := json.Unmarshal(data, &b); err != nil || strings.TrimSpace(b.SessionID) == "" {
			continue
		}
		status, _, err := collectSessionStatus(cwd, b.SessionID, "", b.MaxTokens, watchdog, "")
		if err != nil {
			// Keep stale budget rows visible even if transcript is unavailable.
			status = contextSessionStatus{
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
				IsStale:          !b.LastUpdated.IsZero() && time.Since(b.LastUpdated) > watchdog,
				Action:           actionForStatus(string(b.GetStatus()), !b.LastUpdated.IsZero() && time.Since(b.LastUpdated) > watchdog),
			}
		}
		mergePersistedAssignment(cwd, &status)
		statuses = append(statuses, status)
	}
	sort.Slice(statuses, func(i, j int) bool {
		ri := readinessRank(statuses[i].Readiness)
		rj := readinessRank(statuses[j].Readiness)
		if ri != rj {
			return ri < rj
		}
		if statuses[i].Status == statuses[j].Status {
			if statuses[i].IsStale != statuses[j].IsStale {
				return statuses[i].IsStale
			}
			return statuses[i].SessionID < statuses[j].SessionID
		}
		rank := func(s string) int {
			switch s {
			case string(contextbudget.StatusCritical):
				return 0
			case string(contextbudget.StatusWarning):
				return 1
			default:
				return 2
			}
		}
		return rank(statuses[i].Status) < rank(statuses[j].Status)
	})
	return statuses, nil
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
	if err := os.MkdirAll(pendingDir, 0755); err != nil {
		return "", "", fmt.Errorf("create pending dir: %w", err)
	}

	now := time.Now().UTC()
	safeSession := sanitizeForFilename(status.SessionID)
	base := fmt.Sprintf("auto-%s-%s", now.Format("20060102T150405Z"), safeSession)
	handoffPath := filepath.Join(handoffDir, base+".md")
	markerPath := filepath.Join(pendingDir, base+".json")

	changedFiles := gitChangedFiles(cwd, 20)
	activeBead := strings.TrimSpace(runCommand(cwd, 1200*time.Millisecond, "bd", "current"))
	if activeBead == "" {
		activeBead = "none"
	}
	if status.LastTask == "" {
		status.LastTask = "none recorded"
	}

	md := renderHandoffMarkdown(now, status, usage, activeBead, changedFiles)
	if err := os.WriteFile(handoffPath, []byte(md), 0644); err != nil {
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
	if err := os.WriteFile(markerPath, data, 0644); err != nil {
		return "", "", fmt.Errorf("write handoff marker: %w", err)
	}

	return relPath, toRepoRelative(cwd, markerPath), nil
}

func renderHandoffMarkdown(now time.Time, status contextSessionStatus, usage transcriptUsage, activeBead string, changedFiles []string) string {
	hull := strings.TrimSpace(status.Readiness)
	if hull == "" {
		hull = readinessForUsage(status.UsagePercent)
	}
	remaining := status.RemainingPercent
	if remaining <= 0 && status.UsagePercent > 0 {
		remaining = remainingPercent(status.UsagePercent)
	}

	var b strings.Builder
	b.WriteString("# Auto-Handoff (Context Guard)\n\n")
	b.WriteString(fmt.Sprintf("**Timestamp:** %s\n", now.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("**Session:** %s\n", status.SessionID))
	b.WriteString(fmt.Sprintf("**Status:** %s (%.1f%%)\n", status.Status, status.UsagePercent*100))
	b.WriteString(fmt.Sprintf("**Hull:** %s (%.1f%% remaining)\n", hull, remaining*100))
	b.WriteString(fmt.Sprintf("**Action:** %s\n\n", status.Action))

	b.WriteString("## Last Task\n")
	b.WriteString(status.LastTask)
	b.WriteString("\n\n")

	b.WriteString("## Active Work\n")
	b.WriteString(activeBead)
	b.WriteString("\n\n")

	b.WriteString("## Assignment\n")
	b.WriteString(fmt.Sprintf("- agent: %s\n", displayOrDash(status.AgentName)))
	b.WriteString(fmt.Sprintf("- role: %s\n", displayOrDash(status.AgentRole)))
	b.WriteString(fmt.Sprintf("- team: %s\n", displayOrDash(status.TeamName)))
	b.WriteString(fmt.Sprintf("- issue: %s\n", displayOrDash(status.IssueID)))
	b.WriteString(fmt.Sprintf("- tmux target: %s\n\n", displayOrDash(status.TmuxTarget)))

	b.WriteString("## Next Action\n")
	b.WriteString("Start a fresh session, consume this handoff at startup, and continue from the listed task.\n\n")

	b.WriteString("## Modified Files\n")
	if len(changedFiles) == 0 {
		b.WriteString("none\n\n")
	} else {
		for _, f := range changedFiles {
			b.WriteString("- ")
			b.WriteString(f)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("## Blockers\n")
	if status.IsStale {
		b.WriteString("- Session appears stale; watchdog recovery recommended.\n\n")
	} else {
		b.WriteString("none detected\n\n")
	}

	b.WriteString("## Telemetry\n")
	b.WriteString(fmt.Sprintf("- model: %s\n", status.Model))
	b.WriteString(fmt.Sprintf("- input_tokens: %d\n", usage.InputTokens))
	b.WriteString(fmt.Sprintf("- cache_creation_input_tokens: %d\n", usage.CacheCreationInputToken))
	b.WriteString(fmt.Sprintf("- cache_read_input_tokens: %d\n", usage.CacheReadInputToken))
	b.WriteString(fmt.Sprintf("- estimated_usage: %d/%d\n", status.EstimatedUsage, status.MaxTokens))
	b.WriteString(fmt.Sprintf("- recommendation: %s\n", status.Recommendation))
	return b.String()
}

func findPendingHandoffForSession(cwd, sessionID string) (handoffPath string, markerPath string, found bool, err error) {
	pendingDir := filepath.Join(cwd, ".agents", "handoff", "pending")
	entries, err := os.ReadDir(pendingDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", false, nil
		}
		return "", "", false, err
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(pendingDir, e.Name())
		data, rErr := os.ReadFile(path)
		if rErr != nil {
			continue
		}
		var marker handoffMarker
		if jErr := json.Unmarshal(data, &marker); jErr != nil {
			continue
		}
		if marker.SessionID != sessionID || marker.Consumed {
			continue
		}
		return marker.HandoffFile, toRepoRelative(cwd, path), true, nil
	}
	return "", "", false, nil
}

func readSessionTail(path string) (transcriptUsage, string, time.Time, error) {
	tail, err := readFileTail(path, transcriptTailMaxBytes)
	if err != nil {
		return transcriptUsage{}, "", time.Time{}, err
	}

	lines := make([]string, 0, 2048)
	scanner := bufio.NewScanner(bytes.NewReader(tail))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return transcriptUsage{}, "", time.Time{}, err
	}

	type lineEnvelope struct {
		Type      string `json:"type"`
		Timestamp string `json:"timestamp"`
		Message   struct {
			Role    string          `json:"role"`
			Model   string          `json:"model"`
			Usage   json.RawMessage `json:"usage"`
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	}
	type usageEnvelope struct {
		InputTokens             int `json:"input_tokens"`
		CacheCreationInputToken int `json:"cache_creation_input_tokens"`
		CacheReadInputToken     int `json:"cache_read_input_tokens"`
	}

	var usage transcriptUsage
	var lastTask string
	var newestTS time.Time

	for i := len(lines) - 1; i >= 0; i-- {
		raw := strings.TrimSpace(lines[i])
		if raw == "" {
			continue
		}
		var entry lineEnvelope
		if err := json.Unmarshal([]byte(raw), &entry); err != nil {
			continue
		}

		ts := parseTimestamp(entry.Timestamp)
		if newestTS.IsZero() && !ts.IsZero() {
			newestTS = ts
		}

		if usage.Timestamp.IsZero() && len(entry.Message.Usage) > 0 {
			var u usageEnvelope
			if err := json.Unmarshal(entry.Message.Usage, &u); err == nil {
				total := u.InputTokens + u.CacheCreationInputToken + u.CacheReadInputToken
				if total > 0 {
					usage = transcriptUsage{
						InputTokens:             u.InputTokens,
						CacheCreationInputToken: u.CacheCreationInputToken,
						CacheReadInputToken:     u.CacheReadInputToken,
						Model:                   entry.Message.Model,
						Timestamp:               ts,
					}
				}
			}
		}

		if lastTask == "" && entry.Type == "user" && len(entry.Message.Content) > 0 {
			if task := extractTextContent(entry.Message.Content); task != "" {
				lastTask = task
			}
		}

		if !usage.Timestamp.IsZero() && lastTask != "" {
			break
		}
	}
	if newestTS.IsZero() {
		if fi, err := os.Stat(path); err == nil {
			newestTS = fi.ModTime().UTC()
		}
	}
	if usage.Timestamp.IsZero() {
		usage.Timestamp = newestTS
	}
	return usage, lastTask, newestTS, nil
}

func readFileTail(path string, maxBytes int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := fi.Size()
	if size == 0 {
		return []byte{}, nil
	}

	start := int64(0)
	if size > maxBytes {
		start = size - maxBytes
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	if start > 0 {
		if idx := bytes.IndexByte(data, '\n'); idx >= 0 && idx+1 < len(data) {
			data = data[idx+1:]
		}
	}
	return data, nil
}

func parseTimestamp(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	if ts, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return ts.UTC()
	}
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts.UTC()
	}
	return time.Time{}
}

func extractTextContent(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return ""
	}

	var plain string
	if err := json.Unmarshal(raw, &plain); err == nil {
		return normalizeLine(plain)
	}

	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		return ""
	}
	for _, item := range arr {
		txt, ok := item["text"].(string)
		if ok && strings.TrimSpace(txt) != "" {
			return normalizeLine(txt)
		}
	}
	return ""
}

func estimateTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	// Conservative coarse estimate: 1 token ~= 4 chars.
	n := len(text) / 4
	if n < 1 {
		return 1
	}
	return n
}

func actionForStatus(status string, stale bool) string {
	if stale && status != string(contextbudget.StatusOptimal) {
		return "recover_dead_session"
	}
	switch status {
	case string(contextbudget.StatusCritical):
		return "handoff_now"
	case string(contextbudget.StatusWarning):
		return "checkpoint_and_prepare_handoff"
	default:
		if stale {
			return "investigate_stale_session"
		}
		return "continue"
	}
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
	if strings.TrimSpace(assignment.AgentName) != "" {
		status.AgentName = strings.TrimSpace(assignment.AgentName)
	}
	if strings.TrimSpace(assignment.AgentRole) != "" {
		status.AgentRole = strings.TrimSpace(assignment.AgentRole)
	}
	if strings.TrimSpace(assignment.TeamName) != "" {
		status.TeamName = strings.TrimSpace(assignment.TeamName)
	}
	if strings.TrimSpace(assignment.IssueID) != "" {
		status.IssueID = strings.TrimSpace(assignment.IssueID)
	}
	if strings.TrimSpace(assignment.TmuxPaneID) != "" {
		status.TmuxPaneID = strings.TrimSpace(assignment.TmuxPaneID)
	}
	if strings.TrimSpace(assignment.TmuxTarget) != "" {
		status.TmuxTarget = strings.TrimSpace(assignment.TmuxTarget)
	}
	if strings.TrimSpace(assignment.TmuxSession) != "" {
		status.TmuxSession = strings.TrimSpace(assignment.TmuxSession)
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
	return strings.TrimSpace(a.AgentName) == "" &&
		strings.TrimSpace(a.AgentRole) == "" &&
		strings.TrimSpace(a.TeamName) == "" &&
		strings.TrimSpace(a.IssueID) == "" &&
		strings.TrimSpace(a.TmuxPaneID) == "" &&
		strings.TrimSpace(a.TmuxTarget) == "" &&
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
	if err := os.MkdirAll(contextDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(contextDir, "assignment-"+sanitizeForFilename(status.SessionID)+".json")
	return os.WriteFile(path, data, 0644)
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
	if current.AgentName == "" {
		status.AgentName = assignment.AgentName
	}
	if current.AgentRole == "" {
		status.AgentRole = assignment.AgentRole
	}
	if current.TeamName == "" {
		status.TeamName = assignment.TeamName
	}
	if current.IssueID == "" {
		status.IssueID = assignment.IssueID
	}
	if current.TmuxPaneID == "" {
		status.TmuxPaneID = assignment.TmuxPaneID
	}
	if current.TmuxTarget == "" {
		status.TmuxTarget = assignment.TmuxTarget
	}
	if current.TmuxSession == "" {
		status.TmuxSession = assignment.TmuxSession
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

func tmuxTargetAlive(target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	ctx, cancel := contextWithTimeout(1200 * time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tmux", "has-session", "-t", target)
	return cmd.Run() == nil
}

func tmuxStartDetachedSession(sessionName string) error {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return errors.New("missing tmux session name")
	}
	ctx, cancel := contextWithTimeout(1200 * time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tmux", "new-session", "-d", "-s", sessionName)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if strings.TrimSpace(string(out)) != "" {
		return errors.New(normalizeLine(string(out)))
	}
	return err
}

func findTeamMemberByName(agentName string) (string, teamConfigMember, bool) {
	agentName = strings.TrimSpace(agentName)
	if agentName == "" {
		return "", teamConfigMember{}, false
	}
	homeDir := strings.TrimSpace(os.Getenv("HOME"))
	if homeDir == "" {
		return "", teamConfigMember{}, false
	}
	teamsDir := filepath.Join(homeDir, ".claude", "teams")
	entries, err := os.ReadDir(teamsDir)
	if err != nil {
		return "", teamConfigMember{}, false
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		cfgPath := filepath.Join(teamsDir, entry.Name(), "config.json")
		data, err := os.ReadFile(cfgPath)
		if err != nil {
			continue
		}
		var config teamConfigFile
		if err := json.Unmarshal(data, &config); err != nil {
			continue
		}
		for _, member := range config.Members {
			if strings.EqualFold(strings.TrimSpace(member.Name), agentName) {
				return entry.Name(), member, true
			}
		}
	}
	return "", teamConfigMember{}, false
}

func inferAgentRole(agentName, explicitRole string) string {
	if strings.TrimSpace(explicitRole) != "" {
		return strings.TrimSpace(explicitRole)
	}
	agentName = strings.ToLower(strings.TrimSpace(agentName))
	switch {
	case agentName == "":
		return ""
	case strings.Contains(agentName, "admiral"),
		strings.Contains(agentName, "captain"),
		strings.Contains(agentName, "coordinator"),
		strings.Contains(agentName, "orchestrator"),
		strings.Contains(agentName, "quarterback"),
		strings.Contains(agentName, "mayor"),
		strings.Contains(agentName, "leader"),
		strings.Contains(agentName, "lead"):
		return "team-lead"
	case strings.Contains(agentName, "red-cell"),
		strings.Contains(agentName, "navigator"),
		strings.Contains(agentName, "judge"),
		strings.Contains(agentName, "reviewer"):
		return "review"
	case strings.Contains(agentName, "worker"),
		strings.Contains(agentName, "crew"),
		strings.Contains(agentName, "mate"):
		return "worker"
	default:
		return "agent"
	}
}

func remainingPercent(usagePercent float64) float64 {
	remaining := 1 - usagePercent
	switch {
	case remaining < 0:
		return 0
	case remaining > 1:
		return 1
	default:
		return remaining
	}
}

func readinessForUsage(usagePercent float64) string {
	remaining := remainingPercent(usagePercent)
	switch {
	case remaining >= 0.75:
		return contextReadinessGreen
	case remaining >= 0.60:
		return contextReadinessAmber
	case remaining >= 0.40:
		return contextReadinessRed
	default:
		return contextReadinessCritical
	}
}

func readinessAction(readiness string) string {
	switch readiness {
	case contextReadinessGreen:
		return "carry_on"
	case contextReadinessAmber:
		return "finish_current_scope"
	case contextReadinessRed:
		return "relief_on_station"
	default:
		return "immediate_relief"
	}
}

func readinessRank(readiness string) int {
	switch strings.TrimSpace(readiness) {
	case contextReadinessCritical:
		return 0
	case contextReadinessRed:
		return 1
	case contextReadinessAmber:
		return 2
	case contextReadinessGreen:
		return 3
	default:
		return 4
	}
}

func extractIssueID(text string) string {
	m := contextIssuePattern.FindString(strings.TrimSpace(text))
	if m == "" {
		return ""
	}
	return strings.ToLower(m)
}

func tmuxTargetFromPaneID(paneID string) string {
	paneID = strings.TrimSpace(paneID)
	if paneID == "" || paneID == "in-process" {
		return ""
	}
	if idx := strings.LastIndex(paneID, "."); idx > 0 {
		return paneID[:idx]
	}
	return paneID
}

func tmuxSessionFromTarget(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	if idx := strings.Index(target, ":"); idx > 0 {
		return strings.TrimSpace(target[:idx])
	}
	return target
}

func displayOrDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}

func gitChangedFiles(cwd string, limit int) []string {
	out := runCommand(cwd, 1200*time.Millisecond, "git", "diff", "--name-only", "HEAD")
	if strings.TrimSpace(out) == "" {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) > limit {
		lines = lines[:limit]
	}
	trimmed := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			trimmed = append(trimmed, l)
		}
	}
	return trimmed
}

func runCommand(cwd string, timeout time.Duration, name string, args ...string) string {
	ctx, cancel := contextWithTimeout(timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func contextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(context.Background())
	}
	return context.WithTimeout(context.Background(), timeout)
}

func sanitizeForFilename(input string) string {
	out := filenameSanitizer.ReplaceAllString(strings.TrimSpace(input), "-")
	out = strings.Trim(out, "-")
	if out == "" {
		return "session"
	}
	return out
}

func toRepoRelative(cwd, fullPath string) string {
	if fullPath == "" {
		return ""
	}
	rel, err := filepath.Rel(cwd, fullPath)
	if err != nil {
		return fullPath
	}
	return filepath.ToSlash(rel)
}

func normalizeLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func nonZeroOrDefault(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func truncateDisplay(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
