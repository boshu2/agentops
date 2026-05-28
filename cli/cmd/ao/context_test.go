// practices: [ai-assisted-dev, distributed-tracing]
package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

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

// --- resolveGuardSessionID ---

func TestResolveGuardSessionID(t *testing.T) {
	// Save and restore package-level var
	oldVal := contextSessionID
	defer func() { contextSessionID = oldVal }()

	t.Run("returns flag value when set", func(t *testing.T) {
		contextSessionID = "flag-session-123"
		t.Setenv("CLAUDE_SESSION_ID", "env-session-456")
		got, err := resolveGuardSessionID()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "flag-session-123" {
			t.Errorf("got %q, want %q", got, "flag-session-123")
		}
	})

	t.Run("falls back to env var when flag empty", func(t *testing.T) {
		contextSessionID = ""
		t.Setenv("CLAUDE_SESSION_ID", "env-session-456")
		got, err := resolveGuardSessionID()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "env-session-456" {
			t.Errorf("got %q, want %q", got, "env-session-456")
		}
	})

	t.Run("returns error when both empty", func(t *testing.T) {
		contextSessionID = ""
		t.Setenv("CLAUDE_SESSION_ID", "")
		_, err := resolveGuardSessionID()
		if err == nil {
			t.Fatal("expected error when both flag and env are empty")
		}
		if !strings.Contains(err.Error(), "session id missing") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("trims whitespace from flag", func(t *testing.T) {
		contextSessionID = "  trimmed-session  "
		got, err := resolveGuardSessionID()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "trimmed-session" {
			t.Errorf("got %q, want %q", got, "trimmed-session")
		}
	})
}

// --- resolveGuardOptions ---

func TestResolveGuardOptions(t *testing.T) {
	oldMaxTokens := contextMaxTokens
	oldWatchdog := contextWatchdogMinute
	oldAgentName := contextAgentName
	defer func() {
		contextMaxTokens = oldMaxTokens
		contextWatchdogMinute = oldWatchdog
		contextAgentName = oldAgentName
	}()

	t.Run("uses flag values when set", func(t *testing.T) {
		contextMaxTokens = 300000
		contextWatchdogMinute = 30
		contextAgentName = "my-agent"
		t.Setenv("CLAUDE_AGENT_NAME", "env-agent")

		maxTok, watchdog, name := resolveGuardOptions()
		if maxTok != 300000 {
			t.Errorf("maxTokens = %d, want 300000", maxTok)
		}
		if watchdog != 30*time.Minute {
			t.Errorf("watchdog = %v, want 30m", watchdog)
		}
		if name != "my-agent" {
			t.Errorf("agentName = %q, want my-agent", name)
		}
	})

	t.Run("falls back to defaults for zero values", func(t *testing.T) {
		contextMaxTokens = 0
		contextWatchdogMinute = 0
		contextAgentName = ""
		t.Setenv("CLAUDE_AGENT_NAME", "env-agent")

		maxTok, watchdog, name := resolveGuardOptions()
		if maxTok != contextbudget.DefaultMaxTokens {
			t.Errorf("maxTokens = %d, want %d", maxTok, contextbudget.DefaultMaxTokens)
		}
		if watchdog != defaultWatchdogMinutes*time.Minute {
			t.Errorf("watchdog = %v, want %v", watchdog, defaultWatchdogMinutes*time.Minute)
		}
		if name != "env-agent" {
			t.Errorf("agentName = %q, want env-agent", name)
		}
	})

	t.Run("negative maxTokens uses default", func(t *testing.T) {
		contextMaxTokens = -100
		contextWatchdogMinute = -5
		contextAgentName = ""
		t.Setenv("CLAUDE_AGENT_NAME", "")

		maxTok, watchdog, name := resolveGuardOptions()
		if maxTok != contextbudget.DefaultMaxTokens {
			t.Errorf("maxTokens = %d, want default", maxTok)
		}
		if watchdog != defaultWatchdogMinutes*time.Minute {
			t.Errorf("watchdog = %v, want default", watchdog)
		}
		if name != "" {
			t.Errorf("agentName = %q, want empty", name)
		}
	})
}

// --- outputGuardResult ---

func TestOutputGuardResult(t *testing.T) {
	oldOutput := output
	defer func() { output = oldOutput }()

	t.Run("json mode encodes to stdout", func(t *testing.T) {
		output = "json"
		result := contextGuardResult{
			Session: contextSessionStatus{
				SessionID: "test-json-out",
				Status:    "OPTIMAL",
			},
		}
		// Just verify it doesn't error — stdout capture is complex in tests
		err := outputGuardResult(result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("table mode prints text", func(t *testing.T) {
		output = "table"
		result := contextGuardResult{
			Session: contextSessionStatus{
				SessionID:    "test-table-out",
				Status:       "WARNING",
				UsagePercent: 0.65,
				Action:       "checkpoint_and_prepare_handoff",
			},
			HandoffFile: "some/handoff.md",
			HookMessage: "Context is WARNING",
		}
		err := outputGuardResult(result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// --- applyHandoffIfCritical ---

func TestApplyHandoffIfCritical(t *testing.T) {
	oldWriteHandoff := contextWriteHandoff
	defer func() { contextWriteHandoff = oldWriteHandoff }()

	t.Run("no-op when write-handoff flag is false", func(t *testing.T) {
		contextWriteHandoff = false
		cwd := t.TempDir()
		status := contextSessionStatus{
			Status: string(contextbudget.StatusCritical),
		}
		usage := transcriptUsage{}
		result := &contextGuardResult{}

		err := applyHandoffIfCritical(cwd, status, usage, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.HandoffFile != "" {
			t.Errorf("expected empty handoff file, got %q", result.HandoffFile)
		}
	})

	t.Run("no-op when status is not critical", func(t *testing.T) {
		contextWriteHandoff = true
		cwd := t.TempDir()
		status := contextSessionStatus{
			Status: string(contextbudget.StatusWarning),
		}
		usage := transcriptUsage{}
		result := &contextGuardResult{}

		err := applyHandoffIfCritical(cwd, status, usage, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.HandoffFile != "" {
			t.Errorf("expected empty handoff file, got %q", result.HandoffFile)
		}
	})

	t.Run("writes handoff when critical and flag set", func(t *testing.T) {
		contextWriteHandoff = true
		cwd := t.TempDir()
		status := contextSessionStatus{
			SessionID:      "critical-handoff-session",
			Status:         string(contextbudget.StatusCritical),
			UsagePercent:   0.92,
			EstimatedUsage: 184000,
			MaxTokens:      200000,
			Action:         "handoff_now",
			LastTask:       "test task",
		}
		usage := transcriptUsage{
			InputTokens:             1000,
			CacheCreationInputToken: 83000,
			CacheReadInputToken:     100000,
			Model:                   "claude-opus",
			Timestamp:               time.Now().UTC(),
		}
		result := &contextGuardResult{
			HookMessage: "initial message",
		}

		err := applyHandoffIfCritical(cwd, status, usage, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.HandoffFile == "" {
			t.Fatal("expected non-empty handoff file")
		}
		if result.PendingMarker == "" {
			t.Fatal("expected non-empty pending marker")
		}
		if !strings.Contains(result.HookMessage, "Handoff saved to") {
			t.Errorf("hook message should contain handoff path, got %q", result.HookMessage)
		}
	})
}

// --- inferAgentRole ---

func TestInferAgentRole(t *testing.T) {
	tests := []struct {
		name         string
		agentName    string
		explicitRole string
		wantRole     string
	}{
		{name: "explicit role wins", agentName: "worker-1", explicitRole: "custom-role", wantRole: "custom-role"},
		{name: "empty name returns empty", agentName: "", explicitRole: "", wantRole: ""},
		{name: "admiral is team-lead", agentName: "fleet-admiral-01", explicitRole: "", wantRole: "team-lead"},
		{name: "captain is team-lead", agentName: "captain-kirk", explicitRole: "", wantRole: "team-lead"},
		{name: "coordinator is team-lead", agentName: "build-coordinator", explicitRole: "", wantRole: "team-lead"},
		{name: "orchestrator is team-lead", agentName: "test-orchestrator", explicitRole: "", wantRole: "team-lead"},
		{name: "quarterback is team-lead", agentName: "quarterback-7", explicitRole: "", wantRole: "team-lead"},
		{name: "mayor is team-lead", agentName: "mayor-of-town", explicitRole: "", wantRole: "team-lead"},
		{name: "leader is team-lead", agentName: "team-leader", explicitRole: "", wantRole: "team-lead"},
		{name: "lead is team-lead", agentName: "tech-lead", explicitRole: "", wantRole: "team-lead"},
		{name: "red-cell is review", agentName: "red-cell-alpha", explicitRole: "", wantRole: "review"},
		{name: "navigator is review", agentName: "navigator-01", explicitRole: "", wantRole: "review"},
		{name: "judge is review", agentName: "judge-dredd", explicitRole: "", wantRole: "review"},
		{name: "reviewer is review", agentName: "code-reviewer", explicitRole: "", wantRole: "review"},
		{name: "worker is worker", agentName: "worker-42", explicitRole: "", wantRole: "worker"},
		{name: "crew is worker", agentName: "crew-member", explicitRole: "", wantRole: "worker"},
		{name: "mate is worker", agentName: "first-mate", explicitRole: "", wantRole: "worker"},
		{name: "unknown is agent", agentName: "random-name", explicitRole: "", wantRole: "agent"},
		{name: "whitespace-only explicit returns empty for empty name", agentName: "", explicitRole: "  ", wantRole: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferAgentRole(tt.agentName, tt.explicitRole)
			if got != tt.wantRole {
				t.Errorf("inferAgentRole(%q, %q) = %q, want %q", tt.agentName, tt.explicitRole, got, tt.wantRole)
			}
		})
	}
}

// --- remainingPercent ---

func TestRemainingPercent(t *testing.T) {
	tests := []struct {
		name    string
		usage   float64
		want    float64
		wantMin float64
		wantMax float64
	}{
		{name: "zero usage", usage: 0, wantMin: 1.0, wantMax: 1.0},
		{name: "half usage", usage: 0.5, wantMin: 0.5, wantMax: 0.5},
		{name: "full usage", usage: 1.0, wantMin: 0.0, wantMax: 0.0},
		{name: "over 100% usage clamps to 0", usage: 1.5, wantMin: 0.0, wantMax: 0.0},
		{name: "negative usage clamps to 1", usage: -0.5, wantMin: 1.0, wantMax: 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := remainingPercent(tt.usage)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("remainingPercent(%f) = %f, want in [%f, %f]", tt.usage, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// --- readinessRank ---

func TestReadinessRank(t *testing.T) {
	tests := []struct {
		readiness string
		wantRank  int
	}{
		{contextReadinessCritical, 0},
		{contextReadinessRed, 1},
		{contextReadinessAmber, 2},
		{contextReadinessGreen, 3},
		{"UNKNOWN", 4},
		{"", 4},
		{"  CRITICAL  ", 0},
	}
	for _, tt := range tests {
		t.Run(tt.readiness, func(t *testing.T) {
			got := readinessRank(tt.readiness)
			if got != tt.wantRank {
				t.Errorf("readinessRank(%q) = %d, want %d", tt.readiness, got, tt.wantRank)
			}
		})
	}
}

// --- tmuxTargetFromPaneID ---

func TestTmuxTargetFromPaneID(t *testing.T) {
	tests := []struct {
		name   string
		paneID string
		want   string
	}{
		{name: "empty returns empty", paneID: "", want: ""},
		{name: "in-process returns empty", paneID: "in-process", want: ""},
		{name: "session:window.pane strips pane", paneID: "my-session:0.2", want: "my-session:0"},
		{name: "no dot returns as-is", paneID: "my-session:0", want: "my-session:0"},
		{name: "whitespace only returns empty", paneID: "   ", want: ""},
		{name: "multiple dots uses last", paneID: "a.b.c", want: "a.b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tmuxTargetFromPaneID(tt.paneID)
			if got != tt.want {
				t.Errorf("tmuxTargetFromPaneID(%q) = %q, want %q", tt.paneID, got, tt.want)
			}
		})
	}
}

// --- tmuxSessionFromTarget ---

func TestTmuxSessionFromTarget(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   string
	}{
		{name: "empty returns empty", target: "", want: ""},
		{name: "session:window extracts session", target: "my-session:0", want: "my-session"},
		{name: "no colon returns as-is", target: "my-session", want: "my-session"},
		{name: "whitespace trimmed", target: "  my-session:0  ", want: "my-session"},
		{name: "multiple colons uses first", target: "a:b:c", want: "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tmuxSessionFromTarget(tt.target)
			if got != tt.want {
				t.Errorf("tmuxSessionFromTarget(%q) = %q, want %q", tt.target, got, tt.want)
			}
		})
	}
}

// --- toRepoRelative ---

func TestToRepoRelative(t *testing.T) {
	tests := []struct {
		name     string
		cwd      string
		fullPath string
		want     string
	}{
		{name: "empty path returns empty", cwd: "/a/b", fullPath: "", want: ""},
		{name: "child path returns relative", cwd: "/a/b", fullPath: "/a/b/c/d.txt", want: "c/d.txt"},
		{name: "same dir returns dot file", cwd: "/a/b", fullPath: "/a/b/file.txt", want: "file.txt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toRepoRelative(tt.cwd, tt.fullPath)
			if got != tt.want {
				t.Errorf("toRepoRelative(%q, %q) = %q, want %q", tt.cwd, tt.fullPath, got, tt.want)
			}
		})
	}
}

// --- contextWithTimeout ---

func TestContextWithTimeout(t *testing.T) {
	t.Run("positive timeout creates context with deadline", func(t *testing.T) {
		ctx, cancel := contextWithTimeout(5 * time.Second)
		defer cancel()
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("expected deadline to be set")
		}
		if time.Until(deadline) <= 0 {
			t.Fatal("expected future deadline")
		}
	})

	t.Run("zero timeout creates cancel-only context", func(t *testing.T) {
		ctx, cancel := contextWithTimeout(0)
		defer cancel()
		_, ok := ctx.Deadline()
		if ok {
			t.Fatal("expected no deadline for zero timeout")
		}
	})

	t.Run("negative timeout creates cancel-only context", func(t *testing.T) {
		ctx, cancel := contextWithTimeout(-1 * time.Second)
		defer cancel()
		_, ok := ctx.Deadline()
		if ok {
			t.Fatal("expected no deadline for negative timeout")
		}
	})
}

// --- truncateDisplay ---

func TestTruncateDisplay(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{name: "short string unchanged", s: "abc", max: 10, want: "abc"},
		{name: "exact length unchanged", s: "abcde", max: 5, want: "abcde"},
		{name: "long string truncated with ellipsis", s: "abcdefghij", max: 7, want: "abcd..."},
		{name: "max 3 no ellipsis", s: "abcdefgh", max: 3, want: "abc"},
		{name: "max 2 no ellipsis", s: "abcdefgh", max: 2, want: "ab"},
		{name: "empty string", s: "", max: 5, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateDisplay(tt.s, tt.max)
			if got != tt.want {
				t.Errorf("truncateDisplay(%q, %d) = %q, want %q", tt.s, tt.max, got, tt.want)
			}
		})
	}
}

// --- estimateTokens ---

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{name: "empty string", text: "", want: 0},
		{name: "whitespace only", text: "   ", want: 0},
		{name: "short text returns 1", text: "ab", want: 1},
		{name: "exactly 4 chars", text: "abcd", want: 1},
		{name: "longer text", text: "this is a longer text for token estimation test", want: len("this is a longer text for token estimation test") / 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateTokens(tt.text)
			if got != tt.want {
				t.Errorf("estimateTokens(%q) = %d, want %d", tt.text, got, tt.want)
			}
		})
	}
}

// --- actionForStatus ---

func TestActionForStatus(t *testing.T) {
	tests := []struct {
		name   string
		status string
		stale  bool
		want   string
	}{
		{name: "critical", status: string(contextbudget.StatusCritical), stale: false, want: "handoff_now"},
		{name: "warning", status: string(contextbudget.StatusWarning), stale: false, want: "checkpoint_and_prepare_handoff"},
		{name: "optimal", status: string(contextbudget.StatusOptimal), stale: false, want: "continue"},
		{name: "stale critical recovers", status: string(contextbudget.StatusCritical), stale: true, want: "recover_dead_session"},
		{name: "stale warning recovers", status: string(contextbudget.StatusWarning), stale: true, want: "recover_dead_session"},
		{name: "stale optimal investigates", status: string(contextbudget.StatusOptimal), stale: true, want: "investigate_stale_session"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := actionForStatus(tt.status, tt.stale)
			if got != tt.want {
				t.Errorf("actionForStatus(%q, %v) = %q, want %q", tt.status, tt.stale, got, tt.want)
			}
		})
	}
}

// --- hookMessageForStatus ---

func TestHookMessageForStatus(t *testing.T) {
	tests := []struct {
		name   string
		status contextSessionStatus
		want   string // substring to find in result
		empty  bool   // if true, expect empty string
	}{
		{
			name: "handoff_now action",
			status: contextSessionStatus{
				Action:           "handoff_now",
				UsagePercent:     0.92,
				Readiness:        contextReadinessCritical,
				RemainingPercent: 0.08,
			},
			want: "CRITICAL",
		},
		{
			name: "checkpoint action",
			status: contextSessionStatus{
				Action:           "checkpoint_and_prepare_handoff",
				UsagePercent:     0.65,
				Readiness:        contextReadinessAmber,
				RemainingPercent: 0.35,
			},
			want: "WARNING",
		},
		{
			name: "recover with restart attempt success",
			status: contextSessionStatus{
				Action:         "recover_dead_session",
				RestartAttempt: true,
				RestartSuccess: true,
				TmuxSession:    "my-session",
			},
			want: "auto-restarted",
		},
		{
			name: "recover with restart attempt failure",
			status: contextSessionStatus{
				Action:         "recover_dead_session",
				RestartAttempt: true,
				RestartSuccess: false,
				RestartMessage: "tmux unavailable",
			},
			want: "auto-restart failed",
		},
		{
			name: "recover without restart attempt but with message",
			status: contextSessionStatus{
				Action:         "recover_dead_session",
				RestartAttempt: false,
				RestartMessage: "missing tmux target",
			},
			want: "stale with unfinished work",
		},
		{
			name: "recover without restart attempt and no message",
			status: contextSessionStatus{
				Action: "recover_dead_session",
			},
			want: "stale with unfinished work",
		},
		{
			name: "continue with RED readiness",
			status: contextSessionStatus{
				Action:           "continue",
				Readiness:        contextReadinessRed,
				RemainingPercent: 0.45,
			},
			want: "Hull is RED",
		},
		{
			name:   "continue with GREEN readiness returns empty",
			status: contextSessionStatus{Action: "continue", Readiness: contextReadinessGreen},
			empty:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hookMessageForStatus(tt.status)
			if tt.empty {
				if got != "" {
					t.Errorf("expected empty, got %q", got)
				}
				return
			}
			if !strings.Contains(got, tt.want) {
				t.Errorf("hookMessageForStatus() = %q, want substring %q", got, tt.want)
			}
		})
	}
}

// --- readPersistedAssignment ---

func TestReadPersistedAssignment(t *testing.T) {
	t.Run("returns false for missing file", func(t *testing.T) {
		dir := t.TempDir()
		_, ok := readPersistedAssignment(dir, "nonexistent-session")
		if ok {
			t.Error("expected false for missing file")
		}
	})

	t.Run("returns false for invalid JSON", func(t *testing.T) {
		dir := t.TempDir()
		contextDir := filepath.Join(dir, ".agents", "ao", "context")
		if err := os.MkdirAll(contextDir, 0755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(contextDir, "assignment-bad-json.json")
		if err := os.WriteFile(path, []byte("{invalid}"), 0644); err != nil {
			t.Fatal(err)
		}
		_, ok := readPersistedAssignment(dir, "bad-json")
		if ok {
			t.Error("expected false for invalid JSON")
		}
	})

	t.Run("returns false for empty assignment", func(t *testing.T) {
		dir := t.TempDir()
		contextDir := filepath.Join(dir, ".agents", "ao", "context")
		if err := os.MkdirAll(contextDir, 0755); err != nil {
			t.Fatal(err)
		}
		snapshot := contextAssignmentSnapshot{SessionID: "empty-assign"}
		data, _ := json.MarshalIndent(snapshot, "", "  ")
		path := filepath.Join(contextDir, "assignment-empty-assign.json")
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
		_, ok := readPersistedAssignment(dir, "empty-assign")
		if ok {
			t.Error("expected false for empty assignment fields")
		}
	})

	t.Run("returns assignment for valid file", func(t *testing.T) {
		dir := t.TempDir()
		contextDir := filepath.Join(dir, ".agents", "ao", "context")
		if err := os.MkdirAll(contextDir, 0755); err != nil {
			t.Fatal(err)
		}
		snapshot := contextAssignmentSnapshot{
			SessionID: "valid-session",
			AgentName: "worker-7",
			IssueID:   "ag-123",
			TeamName:  "alpha",
		}
		data, _ := json.MarshalIndent(snapshot, "", "  ")
		path := filepath.Join(contextDir, "assignment-valid-session.json")
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
		assignment, ok := readPersistedAssignment(dir, "valid-session")
		if !ok {
			t.Fatal("expected true for valid file")
		}
		if assignment.AgentName != "worker-7" {
			t.Errorf("AgentName = %q, want worker-7", assignment.AgentName)
		}
		if assignment.IssueID != "ag-123" {
			t.Errorf("IssueID = %q, want ag-123", assignment.IssueID)
		}
		if assignment.TeamName != "alpha" {
			t.Errorf("TeamName = %q, want alpha", assignment.TeamName)
		}
	})
}

// --- mergePersistedAssignment ---

func TestMergePersistedAssignment(t *testing.T) {
	t.Run("nil status is safe", func(t *testing.T) {
		dir := t.TempDir()
		mergePersistedAssignment(dir, nil) // should not panic
	})

	t.Run("empty session ID is safe", func(t *testing.T) {
		dir := t.TempDir()
		status := &contextSessionStatus{}
		mergePersistedAssignment(dir, status) // should not panic
	})

	t.Run("merges persisted fields into status", func(t *testing.T) {
		dir := t.TempDir()
		contextDir := filepath.Join(dir, ".agents", "ao", "context")
		if err := os.MkdirAll(contextDir, 0755); err != nil {
			t.Fatal(err)
		}
		snapshot := contextAssignmentSnapshot{
			SessionID: "merge-test",
			AgentName: "persisted-agent",
			TeamName:  "persisted-team",
		}
		data, _ := json.MarshalIndent(snapshot, "", "  ")
		if err := os.WriteFile(filepath.Join(contextDir, "assignment-merge-test.json"), data, 0644); err != nil {
			t.Fatal(err)
		}

		status := &contextSessionStatus{SessionID: "merge-test"}
		mergePersistedAssignment(dir, status)
		if status.AgentName != "persisted-agent" {
			t.Errorf("AgentName = %q, want persisted-agent", status.AgentName)
		}
		if status.TeamName != "persisted-team" {
			t.Errorf("TeamName = %q, want persisted-team", status.TeamName)
		}
	})
}

// --- findTeamMemberByName ---

func TestFindTeamMemberByName(t *testing.T) {
	t.Run("empty name returns false", func(t *testing.T) {
		_, _, ok := findTeamMemberByName("")
		if ok {
			t.Error("expected false for empty name")
		}
	})

	t.Run("empty HOME returns false", func(t *testing.T) {
		t.Setenv("HOME", "")
		_, _, ok := findTeamMemberByName("worker-1")
		if ok {
			t.Error("expected false for empty HOME")
		}
	})

	t.Run("no teams dir returns false", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)
		_, _, ok := findTeamMemberByName("worker-1")
		if ok {
			t.Error("expected false when no teams dir exists")
		}
	})

	t.Run("finds member across multiple teams", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)

		// Create two team dirs
		team1Dir := filepath.Join(tmpHome, ".claude", "teams", "alpha-team")
		team2Dir := filepath.Join(tmpHome, ".claude", "teams", "beta-team")
		if err := os.MkdirAll(team1Dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(team2Dir, 0755); err != nil {
			t.Fatal(err)
		}

		// Write configs
		cfg1 := `{"members":[{"name":"alice","agentType":"lead","tmuxPaneId":"sess:0.1"}]}`
		cfg2 := `{"members":[{"name":"bob","agentType":"worker","tmuxPaneId":"sess:0.2"}]}`
		if err := os.WriteFile(filepath.Join(team1Dir, "config.json"), []byte(cfg1), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(team2Dir, "config.json"), []byte(cfg2), 0644); err != nil {
			t.Fatal(err)
		}

		teamName, member, ok := findTeamMemberByName("bob")
		if !ok {
			t.Fatal("expected to find bob")
		}
		if teamName != "beta-team" {
			t.Errorf("teamName = %q, want beta-team", teamName)
		}
		if member.AgentType != "worker" {
			t.Errorf("agentType = %q, want worker", member.AgentType)
		}
	})

	t.Run("skips non-directory entries in teams dir", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)

		teamsDir := filepath.Join(tmpHome, ".claude", "teams")
		if err := os.MkdirAll(teamsDir, 0755); err != nil {
			t.Fatal(err)
		}
		// Create a regular file (not a directory)
		if err := os.WriteFile(filepath.Join(teamsDir, "not-a-team.txt"), []byte("junk"), 0644); err != nil {
			t.Fatal(err)
		}

		_, _, ok := findTeamMemberByName("anyone")
		if ok {
			t.Error("expected false when only non-dir entries exist")
		}
	})
}

// --- maybeAutoRestartStaleSession additional cases ---

func TestMaybeAutoRestartStaleSession_EdgeCases(t *testing.T) {
	t.Run("non-recover action returns unchanged", func(t *testing.T) {
		status := contextSessionStatus{
			Action:     "continue",
			TmuxTarget: "some:target",
		}
		result := maybeAutoRestartStaleSession(status)
		if result.RestartAttempt {
			t.Error("should not attempt restart for non-recover action")
		}
	})

	t.Run("empty tmux target sets message", func(t *testing.T) {
		status := contextSessionStatus{
			Action:     "recover_dead_session",
			TmuxTarget: "",
		}
		result := maybeAutoRestartStaleSession(status)
		if result.RestartMessage != "missing tmux target mapping" {
			t.Errorf("message = %q, want 'missing tmux target mapping'", result.RestartMessage)
		}
	})

	t.Run("empty session name from target sets message", func(t *testing.T) {
		// tmuxSessionFromTarget("") returns "" and tmuxSessionFromTarget(":0") returns ""
		// because idx=0 is not > 0
		status := contextSessionStatus{
			Action:     "recover_dead_session",
			TmuxTarget: ":0",
		}
		// tmux may not be available, so check what branch we land in
		result := maybeAutoRestartStaleSession(status)
		// Either tmux unavailable or invalid target
		if result.RestartMessage == "" {
			t.Error("expected non-empty restart message for edge case target")
		}
	})
}

// --- parseTimestamp additional coverage ---

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantZero bool
	}{
		{name: "empty string", input: "", wantZero: true},
		{name: "whitespace only", input: "   ", wantZero: true},
		{name: "RFC3339Nano", input: "2026-02-20T10:00:00.123456789Z", wantZero: false},
		{name: "RFC3339", input: "2026-02-20T10:00:00Z", wantZero: false},
		{name: "invalid format", input: "not-a-timestamp", wantZero: true},
		{name: "date only", input: "2026-02-20", wantZero: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTimestamp(tt.input)
			if tt.wantZero && !got.IsZero() {
				t.Errorf("expected zero time for %q, got %v", tt.input, got)
			}
			if !tt.wantZero && got.IsZero() {
				t.Errorf("expected non-zero time for %q", tt.input)
			}
		})
	}
}

// --- readFileTail ---

func TestReadFileTail(t *testing.T) {
	t.Run("reads empty file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "empty.jsonl")
		if err := os.WriteFile(path, []byte{}, 0644); err != nil {
			t.Fatal(err)
		}
		data, err := readFileTail(path, 1024)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(data) != 0 {
			t.Errorf("expected empty data, got %d bytes", len(data))
		}
	})

	t.Run("returns error for nonexistent file", func(t *testing.T) {
		_, err := readFileTail("/nonexistent/path/file.jsonl", 1024)
		if err == nil {
			t.Fatal("expected error for nonexistent file")
		}
	})
}

// --- collectTrackedSessionStatuses ---

func TestCollectTrackedSessionStatuses(t *testing.T) {
	t.Run("returns nil when no budget files", func(t *testing.T) {
		dir := t.TempDir()
		statuses, err := collectTrackedSessionStatuses(dir, 20*time.Minute)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if statuses != nil {
			t.Errorf("expected nil, got %d statuses", len(statuses))
		}
	})

	t.Run("loads budget files and returns statuses", func(t *testing.T) {
		dir := t.TempDir()
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)

		// Create a budget file
		contextDir := filepath.Join(dir, ".agents", "ao", "context")
		if err := os.MkdirAll(contextDir, 0755); err != nil {
			t.Fatal(err)
		}

		sessionID := "tracked-session-01"
		// Create transcript so collectSessionStatus can find it
		transcriptDir := filepath.Join(tmpHome, ".claude", "projects", "proj", "conversations")
		if err := os.MkdirAll(transcriptDir, 0755); err != nil {
			t.Fatal(err)
		}
		transcriptLines := []map[string]any{
			{
				"type":      "user",
				"timestamp": time.Now().Add(-1 * time.Minute).UTC().Format(time.RFC3339),
				"message": map[string]any{
					"role":    "user",
					"content": "test task",
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
						"cache_creation_input_tokens": 2000,
						"cache_read_input_tokens":     3000,
					},
				},
			},
		}
		var b strings.Builder
		for _, line := range transcriptLines {
			data, _ := json.Marshal(line)
			b.Write(data)
			b.WriteByte('\n')
		}
		if err := os.WriteFile(filepath.Join(transcriptDir, sessionID+".jsonl"), []byte(b.String()), 0644); err != nil {
			t.Fatal(err)
		}

		tracker := contextbudget.NewBudgetTracker(sessionID)
		tracker.MaxTokens = contextbudget.DefaultMaxTokens
		tracker.UpdateUsage(6000)
		data, _ := json.MarshalIndent(tracker, "", "  ")
		if err := os.WriteFile(filepath.Join(contextDir, "budget-"+sessionID+".json"), data, 0644); err != nil {
			t.Fatal(err)
		}

		statuses, err := collectTrackedSessionStatuses(dir, 20*time.Minute)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(statuses) == 0 {
			t.Fatal("expected at least one status")
		}
		if statuses[0].SessionID != sessionID {
			t.Errorf("SessionID = %q, want %q", statuses[0].SessionID, sessionID)
		}
	})
}

// --- gitChangedFiles ---

func TestGitChangedFiles_WithChanges(t *testing.T) {
	dir := t.TempDir()

	// Initialize a git repo, commit a file, then modify it
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "commit.gpgsign", "false"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}

	// Create and commit a file
	testFile := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(testFile, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"git", "add", "hello.txt"},
		{"git", "commit", "-m", "init"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}

	// Modify the file (unstaged change against HEAD)
	if err := os.WriteFile(testFile, []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}

	changed := gitChangedFiles(dir, 10)
	if len(changed) != 1 {
		t.Fatalf("expected 1 changed file, got %d: %v", len(changed), changed)
	}
	if changed[0] != "hello.txt" {
		t.Errorf("expected 'hello.txt', got %q", changed[0])
	}
}

func TestGitChangedFiles_CleanRepo(t *testing.T) {
	dir := t.TempDir()

	// Initialize a git repo with a committed file, no uncommitted changes
	for _, args := range [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "commit.gpgsign", "false"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}
	testFile := filepath.Join(dir, "clean.txt")
	if err := os.WriteFile(testFile, []byte("committed"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"git", "add", "clean.txt"},
		{"git", "commit", "-m", "init"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}

	changed := gitChangedFiles(dir, 10)
	if len(changed) != 0 {
		t.Errorf("expected no changed files, got %d: %v", len(changed), changed)
	}
}

func TestGitChangedFiles_NotGitRepo(t *testing.T) {
	dir := t.TempDir() // plain directory, not a git repo

	changed := gitChangedFiles(dir, 10)
	if len(changed) != 0 {
		t.Errorf("expected no changed files for non-git dir, got %d: %v", len(changed), changed)
	}
}

// --- runContextStatus ---

func TestRunContextStatus_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	// Save and restore cwd
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Save and restore the output format
	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	cmd := &cobra.Command{}
	err = runContextStatus(cmd, nil)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runContextStatus returned error: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	got := string(buf[:n])

	if !strings.Contains(got, "No context telemetry found") {
		t.Errorf("expected 'No context telemetry found' in output, got: %q", got)
	}
}

func TestRunContextStatus_JSONOutput(t *testing.T) {
	dir := t.TempDir()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Set JSON output mode
	oldOutput := output
	output = "json"
	defer func() { output = oldOutput }()

	cmd := &cobra.Command{}
	err = runContextStatus(cmd, nil)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runContextStatus returned error: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	got := string(buf[:n])

	// JSON output should be valid JSON (null or empty array for no sessions)
	var parsed interface{}
	if jsonErr := json.Unmarshal([]byte(strings.TrimSpace(got)), &parsed); jsonErr != nil {
		t.Errorf("expected valid JSON output, got parse error: %v\noutput: %q", jsonErr, got)
	}
}

// --- runContextGuard ---

func TestRunContextGuard_NoCritical(t *testing.T) {
	dir := t.TempDir()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Create a transcript file with low usage (well below CRITICAL threshold)
	sessionID := "guard-test-session"
	t.Setenv("CLAUDE_SESSION_ID", sessionID)

	// Create transcript directory structure
	projDir := filepath.Join(tmpHome, ".claude", "projects", "guard-proj", "conversations")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a transcript with minimal usage
	transcriptLines := []map[string]any{
		{
			"type":      "user",
			"timestamp": time.Now().Add(-30 * time.Second).UTC().Format(time.RFC3339),
			"message": map[string]any{
				"role":    "user",
				"content": "simple task",
			},
		},
		{
			"type":      "assistant",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"message": map[string]any{
				"role":  "assistant",
				"model": "claude-sonnet",
				"usage": map[string]any{
					"input_tokens":                500,
					"cache_creation_input_tokens": 1000,
					"cache_read_input_tokens":     2000,
				},
			},
		},
	}
	var b strings.Builder
	for _, line := range transcriptLines {
		data, _ := json.Marshal(line)
		b.Write(data)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(projDir, sessionID+".jsonl"), []byte(b.String()), 0644); err != nil {
		t.Fatal(err)
	}

	// Save and restore flag state
	oldSessionID := contextSessionID
	oldMaxTokens := contextMaxTokens
	oldWatchdog := contextWatchdogMinute
	oldAutoRestart := contextAutoRestart
	oldWriteHandoff := contextWriteHandoff
	contextSessionID = sessionID
	contextMaxTokens = contextbudget.DefaultMaxTokens
	contextWatchdogMinute = defaultWatchdogMinutes
	contextAutoRestart = false
	contextWriteHandoff = false
	defer func() {
		contextSessionID = oldSessionID
		contextMaxTokens = oldMaxTokens
		contextWatchdogMinute = oldWatchdog
		contextAutoRestart = oldAutoRestart
		contextWriteHandoff = oldWriteHandoff
	}()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldOutput := output
	output = "json"
	defer func() { output = oldOutput }()

	cmd := &cobra.Command{}
	err = runContextGuard(cmd, nil)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runContextGuard returned error: %v", err)
	}

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	got := strings.TrimSpace(string(buf[:n]))

	// Parse the JSON result
	var result contextGuardResult
	if jsonErr := json.Unmarshal([]byte(got), &result); jsonErr != nil {
		t.Fatalf("expected valid JSON, got parse error: %v\noutput: %q", jsonErr, got)
	}

	// With 3500 total tokens out of 200000 default, status should not be CRITICAL
	if result.Session.Status == "CRITICAL" {
		t.Errorf("expected non-CRITICAL status with low usage, got %q", result.Session.Status)
	}
	if result.Session.Action == "handoff_now" {
		t.Error("expected no handoff_now action with low usage")
	}
	if result.HandoffFile != "" {
		t.Errorf("expected no handoff file, got %q", result.HandoffFile)
	}
	// Verify session ID propagates
	if result.Session.SessionID != sessionID {
		t.Errorf("SessionID = %q, want %q", result.Session.SessionID, sessionID)
	}
	// Verify usage is reflected
	if result.Session.EstimatedUsage != 3500 {
		t.Errorf("EstimatedUsage = %d, want 3500", result.Session.EstimatedUsage)
	}
}

// --- renderHandoffMarkdown ---

func TestRenderHandoffMarkdown(t *testing.T) {
	now := time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC)
	status := contextSessionStatus{
		SessionID:        "render-test",
		Status:           "CRITICAL",
		UsagePercent:     0.92,
		RemainingPercent: 0.08,
		Readiness:        contextReadinessCritical,
		Action:           "handoff_now",
		LastTask:         "test rendering",
		AgentName:        "worker-1",
		AgentRole:        "worker",
		TeamName:         "alpha",
		IssueID:          "ag-xyz",
		TmuxTarget:       "sess:0",
		Model:            "claude-opus",
		EstimatedUsage:   184000,
		MaxTokens:        200000,
		Recommendation:   "CRITICAL: Context nearly full.",
		IsStale:          true,
	}
	usage := transcriptUsage{
		InputTokens:             1000,
		CacheCreationInputToken: 83000,
		CacheReadInputToken:     100000,
	}

	md := renderHandoffMarkdown(now, status, usage, "ag-xyz", []string{"file1.go", "file2.go"})

	checks := []string{
		"# Auto-Handoff",
		"render-test",
		"CRITICAL",
		"handoff_now",
		"test rendering",
		"ag-xyz",
		"worker-1",
		"worker",
		"alpha",
		"file1.go",
		"file2.go",
		"stale",
		"claude-opus",
		"184000",
	}
	for _, check := range checks {
		if !strings.Contains(md, check) {
			t.Errorf("handoff markdown missing %q", check)
		}
	}

	// Test with no changed files
	md2 := renderHandoffMarkdown(now, status, usage, "none", nil)
	if !strings.Contains(md2, "none\n") {
		t.Error("expected 'none' for no changed files")
	}

	// Test with non-stale status
	status.IsStale = false
	md3 := renderHandoffMarkdown(now, status, usage, "none", nil)
	if !strings.Contains(md3, "none detected") {
		t.Error("expected 'none detected' for non-stale blockers")
	}
}

// === Additional context.go coverage tests ===

func TestRunContextStatus_TableWithStatuses(t *testing.T) {
	dir := t.TempDir()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	sessionID := "table-status-session"
	transcriptDir := filepath.Join(tmpHome, ".claude", "projects", "proj", "conversations")
	if err := os.MkdirAll(transcriptDir, 0755); err != nil {
		t.Fatal(err)
	}
	longTask := "a really long task description that exceeds forty-eight characters to test truncation behavior in the output"
	writeTranscriptLines(t, filepath.Join(transcriptDir, sessionID+".jsonl"), []map[string]any{
		{"type": "user", "timestamp": time.Now().Add(-1 * time.Minute).UTC().Format(time.RFC3339), "message": map[string]any{"role": "user", "content": longTask}},
		{"type": "assistant", "timestamp": time.Now().UTC().Format(time.RFC3339), "message": map[string]any{"role": "assistant", "model": "claude-sonnet", "usage": map[string]any{"input_tokens": 1000, "cache_creation_input_tokens": 2000, "cache_read_input_tokens": 3000}}},
	})
	ctxDir := filepath.Join(dir, ".agents", "ao", "context")
	if err := os.MkdirAll(ctxDir, 0755); err != nil {
		t.Fatal(err)
	}
	tracker := contextbudget.NewBudgetTracker(sessionID)
	tracker.MaxTokens = contextbudget.DefaultMaxTokens
	tracker.UpdateUsage(6000)
	bdata, _ := json.MarshalIndent(tracker, "", "  ")
	os.WriteFile(filepath.Join(ctxDir, "budget-"+sessionID+".json"), bdata, 0644)
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()
	oldWd := contextWatchdogMinute
	contextWatchdogMinute = defaultWatchdogMinutes
	defer func() { contextWatchdogMinute = oldWd }()
	err := runContextStatus(&cobra.Command{}, nil)
	w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	got := string(buf[:n])
	if !strings.Contains(got, "SESSION") {
		t.Errorf("expected SESSION header, got: %q", got)
	}
}

func TestRunContextStatus_EmptyTelemetry(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()
	t.Setenv("HOME", t.TempDir())
	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()
	oldWd := contextWatchdogMinute
	contextWatchdogMinute = defaultWatchdogMinutes
	defer func() { contextWatchdogMinute = oldWd }()
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := runContextStatus(&cobra.Command{}, nil)
	w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	if !strings.Contains(string(buf[:n]), "No context telemetry") {
		t.Errorf("expected empty message")
	}
}

func TestRunContextGuard_TableOutput(t *testing.T) {
	dir := t.TempDir()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()
	sessionID := "guard-table-session"
	projDir := filepath.Join(tmpHome, ".claude", "projects", "proj", "conversations")
	os.MkdirAll(projDir, 0755)
	writeTranscriptLines(t, filepath.Join(projDir, sessionID+".jsonl"), []map[string]any{
		{"type": "user", "timestamp": time.Now().Add(-30 * time.Second).UTC().Format(time.RFC3339), "message": map[string]any{"role": "user", "content": "task"}},
		{"type": "assistant", "timestamp": time.Now().UTC().Format(time.RFC3339), "message": map[string]any{"role": "assistant", "model": "claude-sonnet", "usage": map[string]any{"input_tokens": 500, "cache_creation_input_tokens": 1000, "cache_read_input_tokens": 2000}}},
	})
	old1, old2, old3, old4, old5, old6 := contextSessionID, contextMaxTokens, contextWatchdogMinute, contextAutoRestart, contextWriteHandoff, output
	contextSessionID = sessionID
	contextMaxTokens = contextbudget.DefaultMaxTokens
	contextWatchdogMinute = defaultWatchdogMinutes
	contextAutoRestart = false
	contextWriteHandoff = false
	output = "table"
	defer func() {
		contextSessionID = old1
		contextMaxTokens = old2
		contextWatchdogMinute = old3
		contextAutoRestart = old4
		contextWriteHandoff = old5
		output = old6
	}()
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := runContextGuard(&cobra.Command{}, nil)
	w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	if !strings.Contains(string(buf[:n]), "Session:") {
		t.Errorf("expected 'Session:' in output")
	}
}

func TestRunContextGuard_MissingSession(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()
	old := contextSessionID
	contextSessionID = ""
	t.Setenv("CLAUDE_SESSION_ID", "")
	defer func() { contextSessionID = old }()
	err := runContextGuard(&cobra.Command{}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "session id missing") {
		t.Errorf("unexpected: %v", err)
	}
}

func TestRunContextGuard_AutoRestart(t *testing.T) {
	dir := t.TempDir()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()
	sessionID := "guard-ar"
	projDir := filepath.Join(tmpHome, ".claude", "projects", "proj", "conversations")
	os.MkdirAll(projDir, 0755)
	writeTranscriptLines(t, filepath.Join(projDir, sessionID+".jsonl"), []map[string]any{
		{"type": "user", "timestamp": time.Now().Add(-30 * time.Second).UTC().Format(time.RFC3339), "message": map[string]any{"role": "user", "content": "task"}},
		{"type": "assistant", "timestamp": time.Now().UTC().Format(time.RFC3339), "message": map[string]any{"role": "assistant", "model": "claude-sonnet", "usage": map[string]any{"input_tokens": 500, "cache_creation_input_tokens": 500, "cache_read_input_tokens": 500}}},
	})
	old1, old2, old3, old4, old5, old6 := contextSessionID, contextMaxTokens, contextWatchdogMinute, contextAutoRestart, contextWriteHandoff, output
	contextSessionID = sessionID
	contextMaxTokens = contextbudget.DefaultMaxTokens
	contextWatchdogMinute = defaultWatchdogMinutes
	contextAutoRestart = true
	contextWriteHandoff = false
	output = "json"
	defer func() {
		contextSessionID = old1
		contextMaxTokens = old2
		contextWatchdogMinute = old3
		contextAutoRestart = old4
		contextWriteHandoff = old5
		output = old6
	}()
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := runContextGuard(&cobra.Command{}, nil)
	w.Close()
	os.Stdout = oldStdout
	_, _ = io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
}

func TestRunContextGuard_WithPromptOverride(t *testing.T) {
	dir := t.TempDir()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()
	sessionID := "guard-prompt"
	projDir := filepath.Join(tmpHome, ".claude", "projects", "proj", "conversations")
	os.MkdirAll(projDir, 0755)
	writeTranscriptLines(t, filepath.Join(projDir, sessionID+".jsonl"), []map[string]any{
		{"type": "user", "timestamp": time.Now().Add(-30 * time.Second).UTC().Format(time.RFC3339), "message": map[string]any{"role": "user", "content": "original"}},
		{"type": "assistant", "timestamp": time.Now().UTC().Format(time.RFC3339), "message": map[string]any{"role": "assistant", "model": "claude-sonnet", "usage": map[string]any{"input_tokens": 500, "cache_creation_input_tokens": 500, "cache_read_input_tokens": 500}}},
	})
	old1, old2, old3, old4, old5, old6, old7 := contextSessionID, contextMaxTokens, contextWatchdogMinute, contextAutoRestart, contextWriteHandoff, contextPrompt, output
	contextSessionID = sessionID
	contextMaxTokens = contextbudget.DefaultMaxTokens
	contextWatchdogMinute = defaultWatchdogMinutes
	contextAutoRestart = false
	contextWriteHandoff = false
	contextPrompt = "override prompt"
	output = "json"
	defer func() {
		contextSessionID = old1
		contextMaxTokens = old2
		contextWatchdogMinute = old3
		contextAutoRestart = old4
		contextWriteHandoff = old5
		contextPrompt = old6
		output = old7
	}()
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := runContextGuard(&cobra.Command{}, nil)
	w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	var result contextGuardResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(buf[:n]))), &result); err != nil {
		t.Fatalf("json: %v", err)
	}
	if result.Session.LastTask != "override prompt" {
		t.Errorf("LastTask = %q", result.Session.LastTask)
	}
}

func TestRunContextGuard_CollectError(t *testing.T) {
	dir := t.TempDir()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()
	old1, old2, old3, old4, old5, old6 := contextSessionID, contextMaxTokens, contextWatchdogMinute, contextAutoRestart, contextWriteHandoff, output
	contextSessionID = "nonexistent-session-xyz"
	contextMaxTokens = contextbudget.DefaultMaxTokens
	contextWatchdogMinute = defaultWatchdogMinutes
	contextAutoRestart = false
	contextWriteHandoff = false
	output = "json"
	defer func() {
		contextSessionID = old1
		contextMaxTokens = old2
		contextWatchdogMinute = old3
		contextAutoRestart = old4
		contextWriteHandoff = old5
		output = old6
	}()
	err := runContextGuard(&cobra.Command{}, nil)
	if err == nil {
		t.Fatal("expected error for missing transcript")
	}
	if !strings.Contains(err.Error(), "find transcript") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunContextGuard_WriteHandoffCritical(t *testing.T) {
	dir := t.TempDir()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()
	sessionID := "guard-handoff-crit"
	projDir := filepath.Join(tmpHome, ".claude", "projects", "proj", "conversations")
	os.MkdirAll(projDir, 0755)
	writeTranscriptLines(t, filepath.Join(projDir, sessionID+".jsonl"), []map[string]any{
		{"type": "user", "timestamp": time.Now().Add(-30 * time.Second).UTC().Format(time.RFC3339), "message": map[string]any{"role": "user", "content": "critical task"}},
		{"type": "assistant", "timestamp": time.Now().UTC().Format(time.RFC3339), "message": map[string]any{"role": "assistant", "model": "claude-sonnet", "usage": map[string]any{"input_tokens": 180000, "cache_creation_input_tokens": 10000, "cache_read_input_tokens": 10000}}},
	})
	old1, old2, old3, old4, old5, old6 := contextSessionID, contextMaxTokens, contextWatchdogMinute, contextAutoRestart, contextWriteHandoff, output
	contextSessionID = sessionID
	contextMaxTokens = 200000
	contextWatchdogMinute = defaultWatchdogMinutes
	contextAutoRestart = false
	contextWriteHandoff = true
	output = "json"
	defer func() {
		contextSessionID = old1
		contextMaxTokens = old2
		contextWatchdogMinute = old3
		contextAutoRestart = old4
		contextWriteHandoff = old5
		output = old6
	}()
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := runContextGuard(&cobra.Command{}, nil)
	w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	buf := make([]byte, 16384)
	n, _ := r.Read(buf)
	var result contextGuardResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(buf[:n]))), &result); err != nil {
		t.Fatalf("json: %v\nbody: %s", err, string(buf[:n]))
	}
	if result.Session.Status != string(contextbudget.StatusCritical) {
		t.Errorf("status=%q, want CRITICAL", result.Session.Status)
	}
	if result.HandoffFile == "" {
		t.Error("expected handoff file to be set")
	}
	if !strings.Contains(result.HookMessage, "Handoff saved") {
		t.Errorf("hook message should mention handoff: %q", result.HookMessage)
	}
}

func TestPersistGuardState_FullCoverage(t *testing.T) {
	dir := t.TempDir()
	err := persistGuardState(dir, contextSessionStatus{SessionID: "persist-test", EstimatedUsage: 5000, MaxTokens: 200000, AgentName: "w1", AgentRole: "worker"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if _, e := os.Stat(filepath.Join(dir, ".agents", "ao", "context", "budget-persist-test.json")); os.IsNotExist(e) {
		t.Error("no budget")
	}
	if _, e := os.Stat(filepath.Join(dir, ".agents", "ao", "context", "assignment-persist-test.json")); os.IsNotExist(e) {
		t.Error("no assignment")
	}
}

func TestPersistGuardState_EmptyAssignment(t *testing.T) {
	dir := t.TempDir()
	if err := persistGuardState(dir, contextSessionStatus{SessionID: "pe", EstimatedUsage: 5000, MaxTokens: 200000}); err != nil {
		t.Fatal(err)
	}
	if _, e := os.Stat(filepath.Join(dir, ".agents", "ao", "context", "assignment-pe.json")); !os.IsNotExist(e) {
		t.Error("unexpected assignment")
	}
}

func TestSeekAndReadTail_LargeFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.jsonl")
	var b strings.Builder
	for i := 0; i < 100; i++ {
		b.WriteString(`{"t":"u"}` + strings.Repeat("x", 100) + "\n")
	}
	os.WriteFile(path, []byte(b.String()), 0644)
	data, err := readFileTail(path, 200)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("empty")
	}
	if len(data) > 200 {
		t.Errorf("too big: %d", len(data))
	}
}

func TestExtractTextContent_ArrayContent(t *testing.T) {
	if got := extractTextContent(json.RawMessage(`[{"type":"text","text":"hello"},{"type":"img"}]`)); got != "hello" {
		t.Errorf("got %q", got)
	}
}

func TestExtractTextContent_ArrayEmptyText(t *testing.T) {
	if got := extractTextContent(json.RawMessage(`[{"type":"text","text":""},{"type":"text","text":"  "}]`)); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestExtractTextContent_InvalidJSON(t *testing.T) {
	if got := extractTextContent(json.RawMessage(`{bad}`)); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestExtractTextContent_NilRaw(t *testing.T) {
	if got := extractTextContent(json.RawMessage(nil)); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestExtractTextContent_WhitespaceOnly(t *testing.T) {
	if got := extractTextContent(json.RawMessage(`   `)); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestExtractIssueID_Variants(t *testing.T) {
	for _, tt := range []struct{ n, i, w string }{{"std", "ag-gjw work", "ag-gjw"}, {"upper", "AG-XYZ", "ag-xyz"}, {"none", "task", ""}, {"empty", "", ""}} {
		t.Run(tt.n, func(t *testing.T) {
			if got := extractIssueID(tt.i); got != tt.w {
				t.Errorf("%q->%q want %q", tt.i, got, tt.w)
			}
		})
	}
}

func TestResolveContextAssignment_Variants(t *testing.T) {
	t.Run("no agent", func(t *testing.T) {
		a := resolveContextAssignment(t.TempDir(), "task", "")
		if a.AgentName != "" {
			t.Errorf("name=%q", a.AgentName)
		}
	})
	t.Run("issue in task", func(t *testing.T) {
		if a := resolveContextAssignment(t.TempDir(), "ag-xyz feature", ""); a.IssueID != "ag-xyz" {
			t.Errorf("issue=%q", a.IssueID)
		}
	})
	t.Run("agent no team", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		a := resolveContextAssignment(t.TempDir(), "task", "some-agent")
		if a.AgentRole != "agent" {
			t.Errorf("role=%q", a.AgentRole)
		}
	})
}

func TestPersistAssignment_Variants(t *testing.T) {
	t.Run("writes", func(t *testing.T) {
		dir := t.TempDir()
		persistAssignment(dir, contextSessionStatus{SessionID: "aw", AgentName: "w5", TeamName: "b"})
		data, err := os.ReadFile(filepath.Join(dir, ".agents", "ao", "context", "assignment-aw.json"))
		if err != nil {
			t.Fatal(err)
		}
		var s contextAssignmentSnapshot
		json.Unmarshal(data, &s)
		if s.AgentName != "w5" {
			t.Errorf("name=%q", s.AgentName)
		}
	})
	t.Run("skips empty", func(t *testing.T) {
		dir := t.TempDir()
		persistAssignment(dir, contextSessionStatus{SessionID: "as"})
		if _, e := os.Stat(filepath.Join(dir, ".agents", "ao", "context", "assignment-as.json")); !os.IsNotExist(e) {
			t.Error("unexpected")
		}
	})
}

func TestReadSessionTail_NoUsageEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nu.jsonl")
	writeTranscriptLines(t, path, []map[string]any{{"type": "user", "timestamp": "2026-02-20T10:00:00Z", "message": map[string]any{"role": "user", "content": "some task"}}})
	u, task, _, err := readSessionTail(path)
	if err != nil {
		t.Fatal(err)
	}
	if task != "some task" {
		t.Errorf("task=%q", task)
	}
	if u.InputTokens != 0 {
		t.Errorf("tokens=%d", u.InputTokens)
	}
}

func TestReadSessionTail_FileNotFound(t *testing.T) {
	_, _, _, err := readSessionTail(filepath.Join(t.TempDir(), "nonexistent.jsonl"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReadSessionTail_EmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.jsonl")
	os.WriteFile(path, []byte{}, 0644)
	usage, task, _, err := readSessionTail(path)
	if err != nil {
		t.Fatal(err)
	}
	if task != "" {
		t.Errorf("task=%q", task)
	}
	if usage.InputTokens != 0 {
		t.Errorf("tokens=%d", usage.InputTokens)
	}
}

func TestCollectSessionStatus_ZeroUsageFallback(t *testing.T) {
	dir, h := t.TempDir(), t.TempDir()
	t.Setenv("HOME", h)
	sid := "zu"
	td := filepath.Join(h, ".claude", "projects", "p", "conversations")
	os.MkdirAll(td, 0755)
	writeTranscriptLines(t, filepath.Join(td, sid+".jsonl"), []map[string]any{
		{"type": "user", "timestamp": time.Now().UTC().Format(time.RFC3339), "message": map[string]any{"role": "user", "content": "a task with no usage data"}},
		{"type": "assistant", "timestamp": time.Now().UTC().Format(time.RFC3339), "message": map[string]any{"role": "assistant", "model": "s", "usage": map[string]any{"input_tokens": 0, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0}}},
	})
	s, _, err := collectSessionStatus(dir, sid, "", contextbudget.DefaultMaxTokens, 20*time.Minute, "")
	if err != nil {
		t.Fatal(err)
	}
	if s.EstimatedUsage <= 0 {
		t.Errorf("usage=%d", s.EstimatedUsage)
	}
}

func TestCollectSessionStatus_Stale(t *testing.T) {
	dir, h := t.TempDir(), t.TempDir()
	t.Setenv("HOME", h)
	sid := "st"
	td := filepath.Join(h, ".claude", "projects", "p", "conversations")
	os.MkdirAll(td, 0755)
	writeTranscriptLines(t, filepath.Join(td, sid+".jsonl"), []map[string]any{
		{"type": "user", "timestamp": time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339), "message": map[string]any{"role": "user", "content": "old"}},
		{"type": "assistant", "timestamp": time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339), "message": map[string]any{"role": "assistant", "model": "s", "usage": map[string]any{"input_tokens": 1000, "cache_creation_input_tokens": 1000, "cache_read_input_tokens": 1000}}},
	})
	s, _, err := collectSessionStatus(dir, sid, "", contextbudget.DefaultMaxTokens, 20*time.Minute, "")
	if err != nil {
		t.Fatal(err)
	}
	if !s.IsStale {
		t.Error("expected stale")
	}
}

func TestCollectSessionStatus_ZeroTimestamp(t *testing.T) {
	dir, h := t.TempDir(), t.TempDir()
	t.Setenv("HOME", h)
	sid := "zt"
	td := filepath.Join(h, ".claude", "projects", "p", "conversations")
	os.MkdirAll(td, 0755)
	writeTranscriptLines(t, filepath.Join(td, sid+".jsonl"), []map[string]any{
		{"type": "user", "message": map[string]any{"role": "user", "content": "task"}},
		{"type": "assistant", "message": map[string]any{"role": "assistant", "model": "s", "usage": map[string]any{"input_tokens": 1000, "cache_creation_input_tokens": 1000, "cache_read_input_tokens": 1000}}},
	})
	s, _, err := collectSessionStatus(dir, sid, "", contextbudget.DefaultMaxTokens, 20*time.Minute, "")
	if err != nil {
		t.Fatal(err)
	}
	if s.LastUpdated == "" {
		t.Error("expected LastUpdated")
	}
}

func TestCompareSessionStatuses_Extra(t *testing.T) {
	t.Run("id tiebreak", func(t *testing.T) {
		a := contextSessionStatus{Readiness: contextReadinessGreen, Status: string(contextbudget.StatusOptimal), SessionID: "b"}
		b := contextSessionStatus{Readiness: contextReadinessGreen, Status: string(contextbudget.StatusOptimal), SessionID: "a"}
		if compareSessionStatuses(a, b) <= 0 {
			t.Error("a should be after b")
		}
	})
	t.Run("stale first", func(t *testing.T) {
		a := contextSessionStatus{Readiness: contextReadinessGreen, Status: string(contextbudget.StatusOptimal), IsStale: false, SessionID: "a"}
		b := contextSessionStatus{Readiness: contextReadinessGreen, Status: string(contextbudget.StatusOptimal), IsStale: true, SessionID: "b"}
		if compareSessionStatuses(a, b) <= 0 {
			t.Error("stale b before a")
		}
	})
}

func TestFindPendingHandoff_Extra(t *testing.T) {
	t.Run("skips dirs and non-json", func(t *testing.T) {
		dir := t.TempDir()
		pd := filepath.Join(dir, ".agents", "handoff", "pending")
		os.MkdirAll(filepath.Join(pd, "sub.json"), 0755)
		os.WriteFile(filepath.Join(pd, "r.md"), []byte("hi"), 0644)
		_, _, f, err := findPendingHandoffForSession(dir, "x")
		if err != nil {
			t.Fatal(err)
		}
		if f {
			t.Error("found")
		}
	})
	t.Run("no dir", func(t *testing.T) {
		_, _, f, err := findPendingHandoffForSession(t.TempDir(), "x")
		if err != nil {
			t.Fatal(err)
		}
		if f {
			t.Error("found")
		}
	})
	t.Run("match found", func(t *testing.T) {
		dir := t.TempDir()
		pd := filepath.Join(dir, ".agents", "handoff", "pending")
		os.MkdirAll(pd, 0755)
		marker := handoffMarker{SessionID: "match-me", HandoffFile: "handoff.md"}
		data, _ := json.Marshal(marker)
		os.WriteFile(filepath.Join(pd, "test.json"), data, 0644)
		hp, mp, found, err := findPendingHandoffForSession(dir, "match-me")
		if err != nil {
			t.Fatal(err)
		}
		if !found {
			t.Error("not found")
		}
		if hp != "handoff.md" {
			t.Errorf("hp=%q", hp)
		}
		if mp == "" {
			t.Error("empty marker path")
		}
	})
}

func TestTmuxStartDetachedSession_Variants(t *testing.T) {
	if err := tmuxStartDetachedSession(""); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Errorf("err=%v", err)
	}
	if err := tmuxStartDetachedSession("   "); err == nil {
		t.Error("expected error")
	}
}

func TestTmuxStartDetachedSession_CmdFail_NoOutput(t *testing.T) {
	bd := filepath.Join(t.TempDir(), "bin")
	os.MkdirAll(bd, 0755)
	os.WriteFile(filepath.Join(bd, "tmux"), []byte("#!/bin/sh\nexit 1\n"), 0755)
	t.Setenv("PATH", bd+string(os.PathListSeparator)+os.Getenv("PATH"))
	err := tmuxStartDetachedSession("test-session")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "exit status") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReadinessForUsage_Extra(t *testing.T) {
	for _, tt := range []struct {
		u float64
		w string
	}{
		{0.75, contextReadinessCritical}, {0.60, contextReadinessRed}, {0.40, contextReadinessAmber}, {0.25, contextReadinessGreen}, {1.5, contextReadinessCritical},
	} {
		if got := readinessForUsage(tt.u); got != tt.w {
			t.Errorf("readinessForUsage(%f)=%q want %q", tt.u, got, tt.w)
		}
	}
}

func TestReadinessAction_Extra(t *testing.T) {
	for _, tt := range []struct{ i, w string }{
		{contextReadinessGreen, "carry_on"}, {contextReadinessAmber, "finish_current_scope"}, {contextReadinessRed, "relief_on_station"}, {contextReadinessCritical, "immediate_relief"}, {"X", "immediate_relief"},
	} {
		if got := readinessAction(tt.i); got != tt.w {
			t.Errorf("readinessAction(%q)=%q want %q", tt.i, got, tt.w)
		}
	}
}

func TestToRepoRelative_Extra(t *testing.T) {
	if got := toRepoRelative("", "/a/b.txt"); got == "" {
		t.Error("empty")
	}
}

func TestGitChangedFiles_LimitExtra(t *testing.T) {
	dir := t.TempDir()
	for _, a := range [][]string{{"git", "init"}, {"git", "config", "user.email", "t@t"}, {"git", "config", "user.name", "T"}, {"git", "config", "commit.gpgsign", "false"}} {
		c := exec.Command(a[0], a[1:]...)
		c.Dir = dir
		c.CombinedOutput()
	}
	for i := 0; i < 5; i++ {
		os.WriteFile(filepath.Join(dir, strings.Repeat("a", i+1)+".txt"), []byte("v1"), 0644)
	}
	for _, a := range [][]string{{"git", "add", "."}, {"git", "commit", "-m", "i"}} {
		c := exec.Command(a[0], a[1:]...)
		c.Dir = dir
		c.CombinedOutput()
	}
	for i := 0; i < 5; i++ {
		os.WriteFile(filepath.Join(dir, strings.Repeat("a", i+1)+".txt"), []byte("v2"), 0644)
	}
	ch := gitChangedFiles(dir, 2)
	if len(ch) > 2 {
		t.Errorf("got %d", len(ch))
	}
	if len(ch) == 0 {
		t.Error("none")
	}
}

func TestRunCommand_Extra(t *testing.T) {
	if got := runCommand("/tmp", 5*time.Second, "echo", "hi"); got != "hi" {
		t.Errorf("got %q", got)
	}
	if got := runCommand("/tmp", 5*time.Second, "false"); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestCollectOneTrackedStatus_Extra(t *testing.T) {
	dir := t.TempDir()
	cd := filepath.Join(dir, ".agents", "ao", "context")
	os.MkdirAll(cd, 0755)
	os.WriteFile(filepath.Join(cd, "budget-bad.json"), []byte("{bad}"), 0644)
	if _, ok := collectOneTrackedStatus(dir, filepath.Join(cd, "budget-bad.json"), 20*time.Minute); ok {
		t.Error("ok for bad json")
	}
	os.WriteFile(filepath.Join(cd, "budget-es.json"), []byte(`{"session_id":"  "}`), 0644)
	if _, ok := collectOneTrackedStatus(dir, filepath.Join(cd, "budget-es.json"), 20*time.Minute); ok {
		t.Error("ok for empty sid")
	}
}

func TestMergeAssignmentFields_Extra(t *testing.T) {
	s := &contextSessionStatus{SessionID: "m", AgentName: "cur"}
	mergeAssignmentFields(&contextAssignment{AgentName: "cur"}, &contextAssignment{AgentName: "p", AgentRole: "pr", TeamName: "pt", IssueID: "pi", TmuxPaneID: "pp", TmuxTarget: "ptg", TmuxSession: "ps"}, s)
	if s.AgentName != "cur" {
		t.Error("overwritten")
	}
	if s.AgentRole != "pr" {
		t.Errorf("role=%q", s.AgentRole)
	}
	if s.TmuxSession != "ps" {
		t.Errorf("session=%q", s.TmuxSession)
	}
}

func TestApplyContextAssignment_Extra(t *testing.T) {
	applyContextAssignment(nil, contextAssignment{AgentName: "x"})
	s := &contextSessionStatus{}
	applyContextAssignment(s, contextAssignment{AgentName: "w", AgentRole: "r", TeamName: "t", IssueID: "i", TmuxPaneID: "p", TmuxTarget: "tg", TmuxSession: "ts"})
	if s.AgentName != "w" {
		t.Errorf("name=%q", s.AgentName)
	}
	if s.TmuxSession != "ts" {
		t.Errorf("session=%q", s.TmuxSession)
	}
}

func TestRenderHandoffMarkdown_Extra(t *testing.T) {
	md := renderHandoffMarkdown(time.Now(), contextSessionStatus{SessionID: "fb", UsagePercent: 0.5}, transcriptUsage{}, "none", nil)
	if !strings.Contains(md, "50.0%") {
		t.Error("missing 50%")
	}
}

func TestSearchTeamConfig_Extra(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{bad"), 0644)
	if _, ok := searchTeamConfig(filepath.Join(dir, "bad.json"), "x"); ok {
		t.Error("ok for bad")
	}
	os.WriteFile(filepath.Join(dir, "g.json"), []byte(`{"members":[{"name":"a"}]}`), 0644)
	if _, ok := searchTeamConfig(filepath.Join(dir, "g.json"), "b"); ok {
		t.Error("ok for no match")
	}
}

func TestMatchPendingHandoff_Extra(t *testing.T) {
	dir := t.TempDir()
	d1, _ := json.Marshal(handoffMarker{SessionID: "s1", Consumed: true})
	os.WriteFile(filepath.Join(dir, "c.json"), d1, 0644)
	if _, _, ok := matchPendingHandoff(filepath.Join(dir, "c.json"), dir, "s1"); ok {
		t.Error("consumed")
	}
	d2, _ := json.Marshal(handoffMarker{SessionID: "s2"})
	os.WriteFile(filepath.Join(dir, "w.json"), d2, 0644)
	if _, _, ok := matchPendingHandoff(filepath.Join(dir, "w.json"), dir, "s3"); ok {
		t.Error("wrong session")
	}
	if _, _, ok := matchPendingHandoff("/no/f.json", "/tmp", "x"); ok {
		t.Error("nonexistent")
	}
}

func TestFixupTailTimestamps_Extra(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.jsonl")
	os.WriteFile(path, []byte("d\n"), 0644)
	u := transcriptUsage{}
	ts := time.Time{}
	fixupTailTimestamps(path, &u, &ts)
	if ts.IsZero() {
		t.Error("zero ts")
	}
	if u.Timestamp.IsZero() {
		t.Error("zero usage ts")
	}
	e := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	u2 := transcriptUsage{Timestamp: e}
	ts2 := e
	fixupTailTimestamps(path, &u2, &ts2)
	if !ts2.Equal(e) {
		t.Error("ts changed")
	}
}

func TestMaybeAutoRestartStaleSession_Extra(t *testing.T) {
	t.Run("tmux unavailable", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		r := maybeAutoRestartStaleSession(contextSessionStatus{Action: "recover_dead_session", TmuxTarget: "s:0"})
		if r.RestartMessage != "tmux unavailable" {
			t.Errorf("msg=%q", r.RestartMessage)
		}
	})
	t.Run("target alive", func(t *testing.T) {
		bd := filepath.Join(t.TempDir(), "bin")
		os.MkdirAll(bd, 0755)
		os.WriteFile(filepath.Join(bd, "tmux"), []byte("#!/bin/sh\nexit 0\n"), 0755)
		t.Setenv("PATH", bd+string(os.PathListSeparator)+os.Getenv("PATH"))
		r := maybeAutoRestartStaleSession(contextSessionStatus{Action: "recover_dead_session", TmuxTarget: "l:0"})
		if r.RestartMessage != "tmux target already alive" {
			t.Errorf("msg=%q", r.RestartMessage)
		}
	})
	t.Run("session from target", func(t *testing.T) {
		bd := filepath.Join(t.TempDir(), "bin")
		os.MkdirAll(bd, 0755)
		os.WriteFile(filepath.Join(bd, "tmux"), []byte("#!/bin/sh\nif [ \"$1\" = \"has-session\" ]; then exit 1; fi\nexit 0\n"), 0755)
		t.Setenv("PATH", bd+string(os.PathListSeparator)+os.Getenv("PATH"))
		r := maybeAutoRestartStaleSession(contextSessionStatus{Action: "recover_dead_session", TmuxTarget: "w:0"})
		if !r.RestartSuccess {
			t.Errorf("fail: %q", r.RestartMessage)
		}
		if r.TmuxSession != "w" {
			t.Errorf("session=%q", r.TmuxSession)
		}
	})
	t.Run("start fails", func(t *testing.T) {
		bd := filepath.Join(t.TempDir(), "bin")
		os.MkdirAll(bd, 0755)
		os.WriteFile(filepath.Join(bd, "tmux"), []byte("#!/bin/sh\nif [ \"$1\" = \"has-session\" ]; then exit 1; fi\necho fail >&2; exit 1\n"), 0755)
		t.Setenv("PATH", bd+string(os.PathListSeparator)+os.Getenv("PATH"))
		r := maybeAutoRestartStaleSession(contextSessionStatus{Action: "recover_dead_session", TmuxTarget: "f:0", TmuxSession: "f"})
		if !r.RestartAttempt {
			t.Error("no attempt")
		}
		if r.RestartSuccess {
			t.Error("should fail")
		}
	})
}

func TestTmuxTargetAlive_Extra(t *testing.T) {
	if tmuxTargetAlive("") {
		t.Error("empty")
	}
	if tmuxTargetAlive("   ") {
		t.Error("ws")
	}
}

func TestParseTimestamp_RFC3339WithOffset(t *testing.T) {
	if parseTimestamp("2026-02-20T10:00:00+05:00").IsZero() {
		t.Error("zero")
	}
}

func TestParseTimestamp_UnparsableReturnsZero(t *testing.T) {
	if ts := parseTimestamp("not-a-timestamp"); !ts.IsZero() {
		t.Errorf("expected zero, got %v", ts)
	}
}

func TestParseTimestamp_EmptyString(t *testing.T) {
	if ts := parseTimestamp(""); !ts.IsZero() {
		t.Errorf("expected zero, got %v", ts)
	}
}

func TestEnsureCriticalHandoff_ExistingHandoff(t *testing.T) {
	dir := t.TempDir()
	pd := filepath.Join(dir, ".agents", "handoff", "pending")
	os.MkdirAll(pd, 0755)
	marker := handoffMarker{SessionID: "existing-ho", HandoffFile: "existing-handoff.md"}
	data, _ := json.Marshal(marker)
	os.WriteFile(filepath.Join(pd, "existing.json"), data, 0644)
	hp, mp, err := ensureCriticalHandoff(dir, contextSessionStatus{SessionID: "existing-ho"}, transcriptUsage{})
	if err != nil {
		t.Fatal(err)
	}
	if hp != "existing-handoff.md" {
		t.Errorf("hp=%q", hp)
	}
	if mp == "" {
		t.Error("empty marker path")
	}
}

func TestEnsureCriticalHandoff_NewHandoff(t *testing.T) {
	dir := t.TempDir()
	status := contextSessionStatus{SessionID: "new-ho-test", Status: string(contextbudget.StatusCritical), UsagePercent: 0.95, RemainingPercent: 0.05, Readiness: contextReadinessCritical, LastTask: "critical task"}
	hp, mp, err := ensureCriticalHandoff(dir, status, transcriptUsage{InputTokens: 190000})
	if err != nil {
		t.Fatal(err)
	}
	if hp == "" {
		t.Error("empty handoff path")
	}
	if mp == "" {
		t.Error("empty marker path")
	}
	matches, _ := filepath.Glob(filepath.Join(dir, ".agents", "handoff", "*.md"))
	if len(matches) == 0 {
		t.Error("no handoff markdown files created")
	}
	markers, _ := filepath.Glob(filepath.Join(dir, ".agents", "handoff", "pending", "*.json"))
	if len(markers) == 0 {
		t.Error("no marker files created")
	}
}
