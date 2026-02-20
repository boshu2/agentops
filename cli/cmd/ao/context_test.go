package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	contextbudget "github.com/boshu2/agentops/cli/internal/context"
)

func TestReadSessionTailParsesUsageAndTask(t *testing.T) {
	tmp := t.TempDir()
	transcript := filepath.Join(tmp, "session.jsonl")

	lines := []map[string]any{
		{
			"type":      "user",
			"timestamp": "2026-02-20T10:00:00Z",
			"message": map[string]any{
				"role":    "user",
				"content": "previous task",
			},
		},
		{
			"type":      "assistant",
			"timestamp": "2026-02-20T10:00:05Z",
			"message": map[string]any{
				"role":  "assistant",
				"model": "claude-opus",
				"usage": map[string]any{
					"input_tokens":                   1500,
					"cache_creation_input_tokens":    25000,
					"cache_read_input_tokens":        64000,
					"unused_field_should_be_ignored": 1,
				},
			},
		},
	}
	writeTranscriptLines(t, transcript, lines)

	usage, task, lastUpdated, err := readSessionTail(transcript)
	if err != nil {
		t.Fatalf("readSessionTail: %v", err)
	}
	if usage.InputTokens != 1500 {
		t.Fatalf("input tokens = %d, want 1500", usage.InputTokens)
	}
	if usage.CacheCreationInputToken != 25000 {
		t.Fatalf("cache creation tokens = %d, want 25000", usage.CacheCreationInputToken)
	}
	if usage.CacheReadInputToken != 64000 {
		t.Fatalf("cache read tokens = %d, want 64000", usage.CacheReadInputToken)
	}
	if usage.Model != "claude-opus" {
		t.Fatalf("model = %q, want claude-opus", usage.Model)
	}
	if task != "previous task" {
		t.Fatalf("task = %q, want %q", task, "previous task")
	}
	if lastUpdated.IsZero() {
		t.Fatal("expected non-zero lastUpdated timestamp")
	}
}

func TestCollectSessionStatusPromptOverrideAndCritical(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	cwd := t.TempDir()

	sessionID := "abc-session-123"
	transcript := filepath.Join(tmpHome, ".claude", "projects", "proj", "conversations", sessionID+".jsonl")
	if err := os.MkdirAll(filepath.Dir(transcript), 0755); err != nil {
		t.Fatalf("mkdir transcript dir: %v", err)
	}

	lines := []map[string]any{
		{
			"type":      "user",
			"timestamp": time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339),
			"message": map[string]any{
				"role":    "user",
				"content": "old task",
			},
		},
		{
			"type":      "assistant",
			"timestamp": time.Now().Add(-1 * time.Minute).UTC().Format(time.RFC3339),
			"message": map[string]any{
				"role":  "assistant",
				"model": "claude-opus",
				"usage": map[string]any{
					"input_tokens":                10000,
					"cache_creation_input_tokens": 90000,
					"cache_read_input_tokens":     90000,
				},
			},
		},
	}
	writeTranscriptLines(t, transcript, lines)

	status, usage, err := collectSessionStatus(cwd, sessionID, "new high-priority task", contextbudget.DefaultMaxTokens, 20*time.Minute, "")
	if err != nil {
		t.Fatalf("collectSessionStatus: %v", err)
	}
	if status.Status != string(contextbudget.StatusCritical) {
		t.Fatalf("status = %q, want %q", status.Status, contextbudget.StatusCritical)
	}
	if status.Readiness != contextReadinessCritical {
		t.Fatalf("readiness = %q, want %q", status.Readiness, contextReadinessCritical)
	}
	if status.ReadinessAction != "immediate_relief" {
		t.Fatalf("readiness action = %q, want immediate_relief", status.ReadinessAction)
	}
	if status.Action != "handoff_now" {
		t.Fatalf("action = %q, want handoff_now", status.Action)
	}
	if status.LastTask != "new high-priority task" {
		t.Fatalf("last task = %q, want prompt override", status.LastTask)
	}
	if usage.InputTokens != 10000 {
		t.Fatalf("usage input tokens = %d, want 10000", usage.InputTokens)
	}
}

func TestCollectSessionStatusStaleWatchdogAction(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	cwd := t.TempDir()

	sessionID := "stale-session-001"
	transcript := filepath.Join(tmpHome, ".claude", "projects", "proj", "conversations", sessionID+".jsonl")
	if err := os.MkdirAll(filepath.Dir(transcript), 0755); err != nil {
		t.Fatalf("mkdir transcript dir: %v", err)
	}

	oldTS := time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339)
	lines := []map[string]any{
		{
			"type":      "user",
			"timestamp": oldTS,
			"message": map[string]any{
				"role":    "user",
				"content": "stale work item",
			},
		},
		{
			"type":      "assistant",
			"timestamp": oldTS,
			"message": map[string]any{
				"role":  "assistant",
				"model": "claude-sonnet",
				"usage": map[string]any{
					"input_tokens":                1000,
					"cache_creation_input_tokens": 59000,
					"cache_read_input_tokens":     60000,
				},
			},
		},
	}
	writeTranscriptLines(t, transcript, lines)

	status, _, err := collectSessionStatus(cwd, sessionID, "", contextbudget.DefaultMaxTokens, 10*time.Minute, "")
	if err != nil {
		t.Fatalf("collectSessionStatus: %v", err)
	}
	if !status.IsStale {
		t.Fatal("expected stale session")
	}
	if status.Readiness != contextReadinessRed {
		t.Fatalf("readiness = %q, want %q", status.Readiness, contextReadinessRed)
	}
	if status.ReadinessAction != "relief_on_station" {
		t.Fatalf("readiness action = %q, want relief_on_station", status.ReadinessAction)
	}
	if status.Action != "recover_dead_session" {
		t.Fatalf("action = %q, want recover_dead_session", status.Action)
	}
}

func TestCollectSessionStatusResolvesAssignmentFromTeamConfig(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	cwd := t.TempDir()

	teamsDir := filepath.Join(tmpHome, ".claude", "teams", "alpha-team")
	if err := os.MkdirAll(teamsDir, 0755); err != nil {
		t.Fatalf("mkdir teams dir: %v", err)
	}
	cfg := `{"members":[{"name":"worker-7","agentType":"general-purpose","tmuxPaneId":"convoy-20260220:0.2"}]}`
	if err := os.WriteFile(filepath.Join(teamsDir, "config.json"), []byte(cfg), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	sessionID := "session-assignment-01"
	transcript := filepath.Join(tmpHome, ".claude", "projects", "proj", "conversations", sessionID+".jsonl")
	if err := os.MkdirAll(filepath.Dir(transcript), 0755); err != nil {
		t.Fatalf("mkdir transcript dir: %v", err)
	}
	lines := []map[string]any{
		{
			"type":      "user",
			"timestamp": time.Now().Add(-1 * time.Minute).UTC().Format(time.RFC3339),
			"message": map[string]any{
				"role":    "user",
				"content": "continue ag-gjw with mapping updates",
			},
		},
		{
			"type":      "assistant",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"message": map[string]any{
				"role":  "assistant",
				"model": "claude-sonnet",
				"usage": map[string]any{
					"input_tokens":                1000,
					"cache_creation_input_tokens": 1000,
					"cache_read_input_tokens":     1000,
				},
			},
		},
	}
	writeTranscriptLines(t, transcript, lines)

	status, _, err := collectSessionStatus(cwd, sessionID, "", contextbudget.DefaultMaxTokens, 20*time.Minute, "worker-7")
	if err != nil {
		t.Fatalf("collectSessionStatus: %v", err)
	}
	if status.AgentName != "worker-7" {
		t.Fatalf("agent name = %q, want worker-7", status.AgentName)
	}
	if status.AgentRole != "general-purpose" {
		t.Fatalf("agent role = %q, want general-purpose", status.AgentRole)
	}
	if status.TeamName != "alpha-team" {
		t.Fatalf("team name = %q, want alpha-team", status.TeamName)
	}
	if status.IssueID != "ag-gjw" {
		t.Fatalf("issue id = %q, want ag-gjw", status.IssueID)
	}
	if status.TmuxPaneID != "convoy-20260220:0.2" {
		t.Fatalf("tmux pane id = %q, want convoy-20260220:0.2", status.TmuxPaneID)
	}
	if status.TmuxTarget != "convoy-20260220:0" {
		t.Fatalf("tmux target = %q, want convoy-20260220:0", status.TmuxTarget)
	}
	if status.TmuxSession != "convoy-20260220" {
		t.Fatalf("tmux session = %q, want convoy-20260220", status.TmuxSession)
	}
	if status.Readiness != contextReadinessGreen {
		t.Fatalf("readiness = %q, want %q", status.Readiness, contextReadinessGreen)
	}
	if status.ReadinessAction != "carry_on" {
		t.Fatalf("readiness action = %q, want carry_on", status.ReadinessAction)
	}
}

func TestReadinessForUsage(t *testing.T) {
	tests := []struct {
		name          string
		usagePercent  float64
		wantReadiness string
		wantAction    string
	}{
		{name: "green", usagePercent: 0.20, wantReadiness: contextReadinessGreen, wantAction: "carry_on"},
		{name: "amber", usagePercent: 0.30, wantReadiness: contextReadinessAmber, wantAction: "finish_current_scope"},
		{name: "red", usagePercent: 0.55, wantReadiness: contextReadinessRed, wantAction: "relief_on_station"},
		{name: "critical", usagePercent: 0.70, wantReadiness: contextReadinessCritical, wantAction: "immediate_relief"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotReadiness := readinessForUsage(tt.usagePercent)
			if gotReadiness != tt.wantReadiness {
				t.Fatalf("readinessForUsage(%0.2f) = %q, want %q", tt.usagePercent, gotReadiness, tt.wantReadiness)
			}
			gotAction := readinessAction(gotReadiness)
			if gotAction != tt.wantAction {
				t.Fatalf("readinessAction(%q) = %q, want %q", gotReadiness, gotAction, tt.wantAction)
			}
		})
	}
}

func TestMaybeAutoRestartStaleSession(t *testing.T) {
	tmp := t.TempDir()
	tmuxLog := filepath.Join(tmp, "tmux.log")
	tmuxBinDir := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(tmuxBinDir, 0755); err != nil {
		t.Fatalf("mkdir tmux bin dir: %v", err)
	}
	tmuxScript := `#!/bin/sh
if [ "$1" = "has-session" ]; then
  exit 1
fi
if [ "$1" = "new-session" ] && [ "$2" = "-d" ] && [ "$3" = "-s" ]; then
  echo "$4" >> "$TMUX_TEST_LOG"
  exit 0
fi
exit 2
`
	tmuxPath := filepath.Join(tmuxBinDir, "tmux")
	if err := os.WriteFile(tmuxPath, []byte(tmuxScript), 0755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", tmuxBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TMUX_TEST_LOG", tmuxLog)

	status := contextSessionStatus{
		Action:      "recover_dead_session",
		TmuxTarget:  "worker-123:0",
		TmuxSession: "worker-123",
	}
	updated := maybeAutoRestartStaleSession(status)
	if !updated.RestartAttempt {
		t.Fatal("expected restart attempt")
	}
	if !updated.RestartSuccess {
		t.Fatalf("expected restart success, message=%q", updated.RestartMessage)
	}
	logData, err := os.ReadFile(tmuxLog)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	if strings.TrimSpace(string(logData)) != "worker-123" {
		t.Fatalf("tmux new-session target = %q, want worker-123", strings.TrimSpace(string(logData)))
	}
}

func TestEnsureCriticalHandoffWritesMarkerAndDeduplicates(t *testing.T) {
	cwd := t.TempDir()
	status := contextSessionStatus{
		SessionID:      "session-dup-1",
		Status:         string(contextbudget.StatusCritical),
		UsagePercent:   0.92,
		EstimatedUsage: 184000,
		MaxTokens:      contextbudget.DefaultMaxTokens,
		LastTask:       "orchestrate workers",
		Action:         "handoff_now",
	}
	usage := transcriptUsage{
		InputTokens:             1000,
		CacheCreationInputToken: 83000,
		CacheReadInputToken:     100000,
		Model:                   "claude-opus",
		Timestamp:               time.Now().UTC(),
	}

	handoff1, marker1, err := ensureCriticalHandoff(cwd, status, usage)
	if err != nil {
		t.Fatalf("ensureCriticalHandoff first call: %v", err)
	}
	if handoff1 == "" || marker1 == "" {
		t.Fatalf("expected non-empty handoff and marker paths, got %q / %q", handoff1, marker1)
	}

	handoff2, marker2, err := ensureCriticalHandoff(cwd, status, usage)
	if err != nil {
		t.Fatalf("ensureCriticalHandoff second call: %v", err)
	}
	if handoff1 != handoff2 {
		t.Fatalf("handoff path mismatch: first=%q second=%q", handoff1, handoff2)
	}
	if marker1 != marker2 {
		t.Fatalf("marker path mismatch: first=%q second=%q", marker1, marker2)
	}

	pendingDir := filepath.Join(cwd, ".agents", "handoff", "pending")
	markers, err := filepath.Glob(filepath.Join(pendingDir, "*.json"))
	if err != nil {
		t.Fatalf("glob markers: %v", err)
	}
	if len(markers) != 1 {
		t.Fatalf("expected exactly one pending marker, got %d", len(markers))
	}
}

func writeTranscriptLines(t *testing.T, path string, lines []map[string]any) {
	t.Helper()
	var b strings.Builder
	for _, line := range lines {
		data, err := json.Marshal(line)
		if err != nil {
			t.Fatalf("marshal transcript line: %v", err)
		}
		b.Write(data)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}
}
