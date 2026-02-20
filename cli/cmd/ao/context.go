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
	contextMaxTokens      int
	contextWriteHandoff   bool
	contextWatchdogMinute int
)

type transcriptUsage struct {
	InputTokens             int
	CacheCreationInputToken int
	CacheReadInputToken     int
	Model                   string
	Timestamp               time.Time
}

type contextSessionStatus struct {
	SessionID      string  `json:"session_id"`
	TranscriptPath string  `json:"transcript_path,omitempty"`
	Model          string  `json:"model,omitempty"`
	LastTask       string  `json:"last_task,omitempty"`
	InputTokens    int     `json:"input_tokens"`
	CacheCreate    int     `json:"cache_creation_input_tokens"`
	CacheRead      int     `json:"cache_read_input_tokens"`
	EstimatedUsage int     `json:"estimated_usage"`
	MaxTokens      int     `json:"max_tokens"`
	UsagePercent   float64 `json:"usage_percent"`
	Status         string  `json:"status"`
	Recommendation string  `json:"recommendation"`
	LastUpdated    string  `json:"last_updated,omitempty"`
	IsStale        bool    `json:"is_stale"`
	Action         string  `json:"action"`
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
	guardCmd.Flags().IntVar(&contextMaxTokens, "max-tokens", contextbudget.DefaultMaxTokens, "Context window size for percentage calculations")
	guardCmd.Flags().BoolVar(&contextWriteHandoff, "write-handoff", false, "Write auto-handoff marker when status is CRITICAL")
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

	fmt.Printf("%-18s %-10s %-9s %-8s %-20s %s\n", "SESSION", "STATUS", "USAGE", "STALE", "ACTION", "TASK")
	fmt.Println(strings.Repeat("─", 110))
	for _, s := range statuses {
		task := s.LastTask
		if len(task) > 48 {
			task = task[:45] + "..."
		}
		fmt.Printf("%-18s %-10s %6.1f%%   %-8t %-20s %s\n",
			truncateDisplay(s.SessionID, 18),
			s.Status,
			s.UsagePercent*100,
			s.IsStale,
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

	status, usage, err := collectSessionStatus(cwd, sessionID, strings.TrimSpace(contextPrompt), contextMaxTokens, watchdog)
	if err != nil {
		return err
	}
	if err := persistBudget(cwd, status); err != nil {
		return fmt.Errorf("persist budget: %w", err)
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
		status, _, err := collectSessionStatus(cwd, b.SessionID, "", b.MaxTokens, watchdog)
		if err != nil {
			// Keep stale budget rows visible even if transcript is unavailable.
			status = contextSessionStatus{
				SessionID:      b.SessionID,
				EstimatedUsage: b.EstimatedUsage,
				MaxTokens:      nonZeroOrDefault(b.MaxTokens, contextbudget.DefaultMaxTokens),
				UsagePercent:   b.GetUsagePercent(),
				Status:         string(b.GetStatus()),
				Recommendation: b.GetRecommendation(),
				LastUpdated:    b.LastUpdated.Format(time.RFC3339),
				IsStale:        !b.LastUpdated.IsZero() && time.Since(b.LastUpdated) > watchdog,
				Action:         actionForStatus(string(b.GetStatus()), !b.LastUpdated.IsZero() && time.Since(b.LastUpdated) > watchdog),
			}
		}
		statuses = append(statuses, status)
	}
	sort.Slice(statuses, func(i, j int) bool {
		if statuses[i].Status == statuses[j].Status {
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

func collectSessionStatus(cwd, sessionID, prompt string, maxTokens int, watchdog time.Duration) (contextSessionStatus, transcriptUsage, error) {
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

	isStale := !usage.Timestamp.IsZero() && watchdog > 0 && time.Since(usage.Timestamp) > watchdog
	status := contextSessionStatus{
		SessionID:      sessionID,
		TranscriptPath: transcriptPath,
		Model:          usage.Model,
		LastTask:       normalizeLine(lastTask),
		InputTokens:    usage.InputTokens,
		CacheCreate:    usage.CacheCreationInputToken,
		CacheRead:      usage.CacheReadInputToken,
		EstimatedUsage: estimated,
		MaxTokens:      max,
		UsagePercent:   tracker.GetUsagePercent(),
		Status:         string(tracker.GetStatus()),
		Recommendation: tracker.GetRecommendation(),
		LastUpdated:    usage.Timestamp.UTC().Format(time.RFC3339),
		IsStale:        isStale,
		Action:         actionForStatus(string(tracker.GetStatus()), isStale),
	}
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
	var b strings.Builder
	b.WriteString("# Auto-Handoff (Context Guard)\n\n")
	b.WriteString(fmt.Sprintf("**Timestamp:** %s\n", now.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("**Session:** %s\n", status.SessionID))
	b.WriteString(fmt.Sprintf("**Status:** %s (%.1f%%)\n", status.Status, status.UsagePercent*100))
	b.WriteString(fmt.Sprintf("**Action:** %s\n\n", status.Action))

	b.WriteString("## Last Task\n")
	b.WriteString(status.LastTask)
	b.WriteString("\n\n")

	b.WriteString("## Active Work\n")
	b.WriteString(activeBead)
	b.WriteString("\n\n")

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
		return fmt.Sprintf("Context is CRITICAL (%.1f%%). End this session and start a fresh one to avoid compaction loss.", status.UsagePercent*100)
	case "checkpoint_and_prepare_handoff":
		return fmt.Sprintf("Context is WARNING (%.1f%%). Prepare a handoff before continuing long orchestration.", status.UsagePercent*100)
	case "recover_dead_session":
		return "Watchdog: session appears stale with unfinished work. Trigger recovery handoff."
	default:
		return ""
	}
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
	re := regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
	out := re.ReplaceAllString(strings.TrimSpace(input), "-")
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
