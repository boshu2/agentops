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

	status, usage, err := collectSessionStatus(cwd, sessionID, "new high-priority task", contextbudget.DefaultMaxTokens, 20*time.Minute)
	if err != nil {
		t.Fatalf("collectSessionStatus: %v", err)
	}
	if status.Status != string(contextbudget.StatusCritical) {
		t.Fatalf("status = %q, want %q", status.Status, contextbudget.StatusCritical)
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

	status, _, err := collectSessionStatus(cwd, sessionID, "", contextbudget.DefaultMaxTokens, 10*time.Minute)
	if err != nil {
		t.Fatalf("collectSessionStatus: %v", err)
	}
	if !status.IsStale {
		t.Fatal("expected stale session")
	}
	if status.Action != "recover_dead_session" {
		t.Fatalf("action = %q, want recover_dead_session", status.Action)
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
