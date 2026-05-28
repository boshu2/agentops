// practices: [ai-assisted-dev, distributed-tracing]
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

// --- scanTailLines ---

func TestContext_ScanTailLines(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		wantCount int
		wantFirst string
		wantLast  string
	}{
		{
			name:      "empty input yields no lines",
			input:     []byte{},
			wantCount: 0,
		},
		{
			name:      "single line without newline",
			input:     []byte("hello"),
			wantCount: 1,
			wantFirst: "hello",
			wantLast:  "hello",
		},
		{
			name:      "two lines with trailing newline",
			input:     []byte("first\nsecond\n"),
			wantCount: 2, // scanner yields "first", "second" (no trailing empty)
			wantFirst: "first",
			wantLast:  "second",
		},
		{
			name:      "multiple lines",
			input:     []byte("a\nb\nc"),
			wantCount: 3,
			wantFirst: "a",
			wantLast:  "c",
		},
		{
			name:      "blank lines preserved",
			input:     []byte("\n\n"),
			wantCount: 2,
			wantFirst: "",
			wantLast:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines, err := scanTailLines(tt.input)
			if err != nil {
				t.Fatalf("scanTailLines error: %v", err)
			}
			if len(lines) != tt.wantCount {
				t.Fatalf("got %d lines, want %d; lines=%v", len(lines), tt.wantCount, lines)
			}
			if tt.wantCount > 0 {
				if lines[0] != tt.wantFirst {
					t.Errorf("first line = %q, want %q", lines[0], tt.wantFirst)
				}
				if lines[len(lines)-1] != tt.wantLast {
					t.Errorf("last line = %q, want %q", lines[len(lines)-1], tt.wantLast)
				}
			}
		})
	}
}

// --- extractUsageFromTailEntry ---

func TestContext_ExtractUsageFromTailEntry(t *testing.T) {
	ts := time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC)

	t.Run("valid usage parsed", func(t *testing.T) {
		entry := tailLineEnvelope{}
		entry.Message.Model = "claude-opus"
		entry.Message.Usage = json.RawMessage(`{"input_tokens":500,"cache_creation_input_tokens":1000,"cache_read_input_tokens":2000}`)

		got := extractUsageFromTailEntry(entry, ts)
		if got.InputTokens != 500 {
			t.Errorf("InputTokens = %d, want 500", got.InputTokens)
		}
		if got.CacheCreationInputToken != 1000 {
			t.Errorf("CacheCreationInputToken = %d, want 1000", got.CacheCreationInputToken)
		}
		if got.CacheReadInputToken != 2000 {
			t.Errorf("CacheReadInputToken = %d, want 2000", got.CacheReadInputToken)
		}
		if got.Model != "claude-opus" {
			t.Errorf("Model = %q, want %q", got.Model, "claude-opus")
		}
		if !got.Timestamp.Equal(ts) {
			t.Errorf("Timestamp = %v, want %v", got.Timestamp, ts)
		}
	})

	t.Run("nil usage returns zero value", func(t *testing.T) {
		entry := tailLineEnvelope{}
		got := extractUsageFromTailEntry(entry, ts)
		if got.InputTokens != 0 || got.Model != "" {
			t.Errorf("expected zero-value usage, got %+v", got)
		}
	})

	t.Run("zero total tokens returns zero value", func(t *testing.T) {
		entry := tailLineEnvelope{}
		entry.Message.Usage = json.RawMessage(`{"input_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}`)
		got := extractUsageFromTailEntry(entry, ts)
		if got.InputTokens != 0 {
			t.Errorf("expected zero-value usage for zero totals, got %+v", got)
		}
	})

	t.Run("invalid JSON returns zero value", func(t *testing.T) {
		entry := tailLineEnvelope{}
		entry.Message.Usage = json.RawMessage(`{invalid}`)
		got := extractUsageFromTailEntry(entry, ts)
		if got.InputTokens != 0 {
			t.Errorf("expected zero-value usage for invalid JSON, got %+v", got)
		}
	})
}

// --- extractTaskFromTailEntry ---

func TestContext_ExtractTaskFromTailEntry(t *testing.T) {
	t.Run("user message with string content", func(t *testing.T) {
		entry := tailLineEnvelope{Type: "user"}
		entry.Message.Content = json.RawMessage(`"hello world"`)
		got := extractTaskFromTailEntry(entry)
		if got != "hello world" {
			t.Errorf("got %q, want %q", got, "hello world")
		}
	})

	t.Run("non-user type returns empty", func(t *testing.T) {
		entry := tailLineEnvelope{Type: "assistant"}
		entry.Message.Content = json.RawMessage(`"hello"`)
		got := extractTaskFromTailEntry(entry)
		if got != "" {
			t.Errorf("got %q, want empty for non-user type", got)
		}
	})

	t.Run("user type with empty content returns empty", func(t *testing.T) {
		entry := tailLineEnvelope{Type: "user"}
		got := extractTaskFromTailEntry(entry)
		if got != "" {
			t.Errorf("got %q, want empty for nil content", got)
		}
	})

	t.Run("user message with array content", func(t *testing.T) {
		entry := tailLineEnvelope{Type: "user"}
		entry.Message.Content = json.RawMessage(`[{"type":"text","text":"array task"}]`)
		got := extractTaskFromTailEntry(entry)
		if got != "array task" {
			t.Errorf("got %q, want %q", got, "array task")
		}
	})
}

// --- updateTailState ---

func TestContext_UpdateTailState(t *testing.T) {
	t.Run("sets newestTS from first non-zero timestamp", func(t *testing.T) {
		ts := time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC)
		entry := tailLineEnvelope{Timestamp: ts.Format(time.RFC3339)}
		var usage transcriptUsage
		var lastTask string
		var newestTS time.Time

		_ = updateTailState(entry, ts, &usage, &lastTask, &newestTS)
		if !newestTS.Equal(ts) {
			t.Errorf("newestTS = %v, want %v", newestTS, ts)
		}
	})

	t.Run("returns true when both usage and task found", func(t *testing.T) {
		ts := time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC)
		// Create entry with usage
		entry := tailLineEnvelope{Type: "assistant"}
		entry.Message.Model = "claude-opus"
		entry.Message.Usage = json.RawMessage(`{"input_tokens":100,"cache_creation_input_tokens":200,"cache_read_input_tokens":300}`)

		var usage transcriptUsage
		lastTask := "already set"
		var newestTS time.Time

		done := updateTailState(entry, ts, &usage, &lastTask, &newestTS)
		if !done {
			t.Error("expected true when both usage and task are set")
		}
	})

	t.Run("returns false when only task found", func(t *testing.T) {
		ts := time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC)
		entry := tailLineEnvelope{Type: "user"}
		entry.Message.Content = json.RawMessage(`"my task"`)

		var usage transcriptUsage
		var lastTask string
		var newestTS time.Time

		done := updateTailState(entry, ts, &usage, &lastTask, &newestTS)
		if done {
			t.Error("expected false when only task found, no usage")
		}
		if lastTask != "my task" {
			t.Errorf("lastTask = %q, want %q", lastTask, "my task")
		}
	})

	t.Run("does not overwrite newestTS once set", func(t *testing.T) {
		ts1 := time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC)
		ts2 := time.Date(2026, 2, 20, 13, 0, 0, 0, time.UTC)

		entry := tailLineEnvelope{}
		var usage transcriptUsage
		var lastTask string
		newestTS := ts1

		_ = updateTailState(entry, ts2, &usage, &lastTask, &newestTS)
		if !newestTS.Equal(ts1) {
			t.Errorf("newestTS should not change once set, got %v want %v", newestTS, ts1)
		}
	})
}

// --- extractTailUsageAndTask ---

func TestContext_ExtractTailUsageAndTask(t *testing.T) {
	t.Run("extracts from reverse order", func(t *testing.T) {
		lines := []string{
			`{"type":"user","timestamp":"2026-02-20T10:00:00Z","message":{"role":"user","content":"first task"}}`,
			`{"type":"assistant","timestamp":"2026-02-20T10:01:00Z","message":{"role":"assistant","model":"claude-opus","usage":{"input_tokens":100,"cache_creation_input_tokens":200,"cache_read_input_tokens":300}}}`,
			`{"type":"user","timestamp":"2026-02-20T10:02:00Z","message":{"role":"user","content":"second task"}}`,
		}

		usage, task, ts := extractTailUsageAndTask(lines)
		if task != "second task" {
			t.Errorf("task = %q, want %q", task, "second task")
		}
		if usage.InputTokens != 100 {
			t.Errorf("InputTokens = %d, want 100", usage.InputTokens)
		}
		if ts.IsZero() {
			t.Error("expected non-zero timestamp")
		}
	})

	t.Run("empty lines returns zero values", func(t *testing.T) {
		usage, task, ts := extractTailUsageAndTask(nil)
		if task != "" || usage.InputTokens != 0 || !ts.IsZero() {
			t.Errorf("expected zero values for empty input, got task=%q usage=%+v ts=%v", task, usage, ts)
		}
	})

	t.Run("skips invalid JSON lines", func(t *testing.T) {
		lines := []string{
			"not json at all",
			`{"type":"user","timestamp":"2026-02-20T10:00:00Z","message":{"role":"user","content":"valid task"}}`,
		}
		_, task, _ := extractTailUsageAndTask(lines)
		if task != "valid task" {
			t.Errorf("task = %q, want %q", task, "valid task")
		}
	})

	t.Run("skips blank lines", func(t *testing.T) {
		lines := []string{
			"",
			"   ",
			`{"type":"user","timestamp":"2026-02-20T10:00:00Z","message":{"role":"user","content":"the task"}}`,
		}
		_, task, _ := extractTailUsageAndTask(lines)
		if task != "the task" {
			t.Errorf("task = %q, want %q", task, "the task")
		}
	})
}

// --- fixupTailTimestamps ---

func TestContext_FixupTailTimestamps(t *testing.T) {
	t.Run("uses file mod time when newestTS is zero", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.jsonl")
		if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}

		var usage transcriptUsage
		newestTS := time.Time{}

		fixupTailTimestamps(path, &usage, &newestTS)
		if newestTS.IsZero() {
			t.Error("expected newestTS to be set from file mod time")
		}
	})

	t.Run("does not overwrite non-zero newestTS", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.jsonl")
		if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}

		var usage transcriptUsage
		original := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		newestTS := original

		fixupTailTimestamps(path, &usage, &newestTS)
		if !newestTS.Equal(original) {
			t.Errorf("newestTS should not change, got %v want %v", newestTS, original)
		}
	})

	t.Run("sets usage.Timestamp from newestTS when zero", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.jsonl")
		if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}

		var usage transcriptUsage
		newestTS := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

		fixupTailTimestamps(path, &usage, &newestTS)
		if !usage.Timestamp.Equal(newestTS) {
			t.Errorf("usage.Timestamp = %v, want %v", usage.Timestamp, newestTS)
		}
	})

	t.Run("does not overwrite non-zero usage.Timestamp", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.jsonl")
		if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}

		usageTS := time.Date(2026, 1, 15, 8, 0, 0, 0, time.UTC)
		usage := transcriptUsage{Timestamp: usageTS}
		newestTS := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

		fixupTailTimestamps(path, &usage, &newestTS)
		if !usage.Timestamp.Equal(usageTS) {
			t.Errorf("usage.Timestamp should not change, got %v want %v", usage.Timestamp, usageTS)
		}
	})
}

// --- seekAndReadTail ---

func TestContext_SeekAndReadTail(t *testing.T) {
	t.Run("reads full file when smaller than maxBytes", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "small.txt")
		content := "line1\nline2\nline3\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = f.Close() }()

		fi, _ := f.Stat()
		got, err := seekAndReadTail(f, fi.Size(), 10000)
		if err != nil {
			t.Fatalf("seekAndReadTail: %v", err)
		}
		if string(got) != content {
			t.Errorf("got %q, want %q", string(got), content)
		}
	})

	t.Run("truncates and aligns to newline when exceeding maxBytes", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "large.txt")
		// 5 lines of 10 chars each = 55 bytes total
		content := "aaaaaaaaaa\nbbbbbbbbbb\ncccccccccc\ndddddddddd\neeeeeeeeee\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = f.Close() }()

		fi, _ := f.Stat()
		got, err := seekAndReadTail(f, fi.Size(), 25)
		if err != nil {
			t.Fatalf("seekAndReadTail: %v", err)
		}
		// Should start after the first newline in the truncated portion
		gotStr := string(got)
		if strings.Contains(gotStr, "aaaaaaaaaa") {
			t.Errorf("expected head content to be truncated, got %q", gotStr)
		}
		// Should contain the tail
		if !strings.Contains(gotStr, "eeeeeeeeee") {
			t.Errorf("expected tail content, got %q", gotStr)
		}
	})
}

// --- compareSessionStatuses ---

func TestContext_CompareSessionStatuses(t *testing.T) {
	t.Run("CRITICAL sorts before GREEN", func(t *testing.T) {
		a := contextSessionStatus{Readiness: contextReadinessCritical, Status: string(contextbudget.StatusCritical)}
		b := contextSessionStatus{Readiness: contextReadinessGreen, Status: string(contextbudget.StatusOptimal)}
		if compareSessionStatuses(a, b) >= 0 {
			t.Error("CRITICAL should sort before GREEN")
		}
	})

	t.Run("same readiness, CRITICAL status before WARNING", func(t *testing.T) {
		a := contextSessionStatus{Readiness: contextReadinessCritical, Status: string(contextbudget.StatusCritical)}
		b := contextSessionStatus{Readiness: contextReadinessCritical, Status: string(contextbudget.StatusWarning)}
		if compareSessionStatuses(a, b) >= 0 {
			t.Error("CRITICAL status should sort before WARNING")
		}
	})

	t.Run("same readiness and status, stale before non-stale", func(t *testing.T) {
		a := contextSessionStatus{Readiness: contextReadinessRed, Status: string(contextbudget.StatusWarning), IsStale: true, SessionID: "z"}
		b := contextSessionStatus{Readiness: contextReadinessRed, Status: string(contextbudget.StatusWarning), IsStale: false, SessionID: "a"}
		if compareSessionStatuses(a, b) >= 0 {
			t.Error("stale should sort before non-stale")
		}
	})

	t.Run("identical everything sorts by session ID", func(t *testing.T) {
		a := contextSessionStatus{Readiness: contextReadinessGreen, Status: string(contextbudget.StatusOptimal), SessionID: "aaa"}
		b := contextSessionStatus{Readiness: contextReadinessGreen, Status: string(contextbudget.StatusOptimal), SessionID: "zzz"}
		if compareSessionStatuses(a, b) >= 0 {
			t.Error("aaa should sort before zzz")
		}
	})

	t.Run("equal sessions return 0", func(t *testing.T) {
		a := contextSessionStatus{Readiness: contextReadinessGreen, Status: string(contextbudget.StatusOptimal), SessionID: "same"}
		b := contextSessionStatus{Readiness: contextReadinessGreen, Status: string(contextbudget.StatusOptimal), SessionID: "same"}
		if compareSessionStatuses(a, b) != 0 {
			t.Error("identical sessions should compare as equal")
		}
	})
}

// --- staleBudgetFallbackStatus ---

func TestContext_StaleBudgetFallbackStatus(t *testing.T) {
	t.Run("zero-value budget yields OPTIMAL", func(t *testing.T) {
		b := contextbudget.BudgetTracker{
			SessionID: "test-session",
			MaxTokens: contextbudget.DefaultMaxTokens,
		}
		watchdog := 20 * time.Minute

		status := staleBudgetFallbackStatus(b, watchdog)
		if status.SessionID != "test-session" {
			t.Errorf("SessionID = %q, want %q", status.SessionID, "test-session")
		}
		if status.Status != string(contextbudget.StatusOptimal) {
			t.Errorf("Status = %q, want OPTIMAL", status.Status)
		}
		if status.Readiness != contextReadinessGreen {
			t.Errorf("Readiness = %q, want GREEN", status.Readiness)
		}
		if status.IsStale {
			t.Error("expected not stale for zero LastUpdated")
		}
	})

	t.Run("old budget is marked stale", func(t *testing.T) {
		b := contextbudget.BudgetTracker{
			SessionID:      "stale-session",
			MaxTokens:      contextbudget.DefaultMaxTokens,
			EstimatedUsage: 150000,
			LastUpdated:    time.Now().Add(-2 * time.Hour),
		}
		watchdog := 20 * time.Minute

		status := staleBudgetFallbackStatus(b, watchdog)
		if !status.IsStale {
			t.Error("expected stale for old LastUpdated")
		}
	})

	t.Run("recent budget not stale", func(t *testing.T) {
		b := contextbudget.BudgetTracker{
			SessionID:      "fresh-session",
			MaxTokens:      contextbudget.DefaultMaxTokens,
			EstimatedUsage: 100000,
			LastUpdated:    time.Now().Add(-5 * time.Minute),
		}
		watchdog := 20 * time.Minute

		status := staleBudgetFallbackStatus(b, watchdog)
		if status.IsStale {
			t.Error("expected not stale for recent LastUpdated")
		}
	})

	t.Run("zero MaxTokens uses default", func(t *testing.T) {
		b := contextbudget.BudgetTracker{
			SessionID: "no-max-session",
			MaxTokens: 0,
		}
		status := staleBudgetFallbackStatus(b, 20*time.Minute)
		if status.MaxTokens != contextbudget.DefaultMaxTokens {
			t.Errorf("MaxTokens = %d, want %d", status.MaxTokens, contextbudget.DefaultMaxTokens)
		}
	})
}

// --- applyContextAssignment ---

func TestContext_ApplyContextAssignment(t *testing.T) {
	t.Run("nil status is safe", func(t *testing.T) {
		// Should not panic
		applyContextAssignment(nil, contextAssignment{AgentName: "test"})
	})

	t.Run("applies all non-empty fields", func(t *testing.T) {
		status := &contextSessionStatus{}
		assignment := contextAssignment{
			AgentName:   "worker-01",
			AgentRole:   "worker",
			TeamName:    "team-alpha",
			IssueID:     "ag-123",
			TmuxPaneID:  "session:0.1",
			TmuxTarget:  "session:0",
			TmuxSession: "session",
		}
		applyContextAssignment(status, assignment)
		if status.AgentName != "worker-01" {
			t.Errorf("AgentName = %q, want %q", status.AgentName, "worker-01")
		}
		if status.AgentRole != "worker" {
			t.Errorf("AgentRole = %q, want %q", status.AgentRole, "worker")
		}
		if status.TeamName != "team-alpha" {
			t.Errorf("TeamName = %q, want %q", status.TeamName, "team-alpha")
		}
		if status.IssueID != "ag-123" {
			t.Errorf("IssueID = %q, want %q", status.IssueID, "ag-123")
		}
		if status.TmuxPaneID != "session:0.1" {
			t.Errorf("TmuxPaneID = %q, want %q", status.TmuxPaneID, "session:0.1")
		}
		if status.TmuxTarget != "session:0" {
			t.Errorf("TmuxTarget = %q, want %q", status.TmuxTarget, "session:0")
		}
		if status.TmuxSession != "session" {
			t.Errorf("TmuxSession = %q, want %q", status.TmuxSession, "session")
		}
	})

	t.Run("does not overwrite with empty fields", func(t *testing.T) {
		status := &contextSessionStatus{
			AgentName: "existing-agent",
			IssueID:   "ag-existing",
		}
		assignment := contextAssignment{} // all empty
		applyContextAssignment(status, assignment)
		if status.AgentName != "existing-agent" {
			t.Errorf("AgentName should not change, got %q", status.AgentName)
		}
		if status.IssueID != "ag-existing" {
			t.Errorf("IssueID should not change, got %q", status.IssueID)
		}
	})

	t.Run("whitespace-only fields do not overwrite", func(t *testing.T) {
		status := &contextSessionStatus{
			AgentName: "existing",
		}
		assignment := contextAssignment{
			AgentName: "   ",
		}
		applyContextAssignment(status, assignment)
		if status.AgentName != "existing" {
			t.Errorf("AgentName should not change for whitespace-only, got %q", status.AgentName)
		}
	})
}

// --- mergeAssignmentFields ---

func TestContext_MergeAssignmentFields(t *testing.T) {
	t.Run("fills empty current fields from persisted", func(t *testing.T) {
		current := &contextAssignment{}
		persisted := &contextAssignment{
			AgentName:   "persisted-worker",
			AgentRole:   "worker",
			TeamName:    "team-a",
			IssueID:     "ag-111",
			TmuxPaneID:  "sess:0.1",
			TmuxTarget:  "sess:0",
			TmuxSession: "sess",
		}
		status := &contextSessionStatus{}

		mergeAssignmentFields(current, persisted, status)

		if status.AgentName != "persisted-worker" {
			t.Errorf("AgentName = %q, want %q", status.AgentName, "persisted-worker")
		}
		if status.AgentRole != "worker" {
			t.Errorf("AgentRole = %q, want %q", status.AgentRole, "worker")
		}
		if status.TeamName != "team-a" {
			t.Errorf("TeamName = %q, want %q", status.TeamName, "team-a")
		}
		if status.IssueID != "ag-111" {
			t.Errorf("IssueID = %q, want %q", status.IssueID, "ag-111")
		}
		if status.TmuxPaneID != "sess:0.1" {
			t.Errorf("TmuxPaneID = %q, want %q", status.TmuxPaneID, "sess:0.1")
		}
		if status.TmuxTarget != "sess:0" {
			t.Errorf("TmuxTarget = %q, want %q", status.TmuxTarget, "sess:0")
		}
		if status.TmuxSession != "sess" {
			t.Errorf("TmuxSession = %q, want %q", status.TmuxSession, "sess")
		}
	})

	t.Run("does not overwrite non-empty current fields", func(t *testing.T) {
		current := &contextAssignment{
			AgentName: "current-worker",
			IssueID:   "ag-current",
		}
		persisted := &contextAssignment{
			AgentName: "persisted-worker",
			IssueID:   "ag-persisted",
			TeamName:  "persisted-team",
		}
		status := &contextSessionStatus{}

		mergeAssignmentFields(current, persisted, status)

		// These should NOT be set (current is non-empty)
		if status.AgentName != "" {
			t.Errorf("AgentName should remain empty when current is non-empty, got %q", status.AgentName)
		}
		if status.IssueID != "" {
			t.Errorf("IssueID should remain empty when current is non-empty, got %q", status.IssueID)
		}
		// TeamName should be set (current is empty)
		if status.TeamName != "persisted-team" {
			t.Errorf("TeamName = %q, want %q", status.TeamName, "persisted-team")
		}
	})
}

// --- searchTeamConfig ---

func TestContext_SearchTeamConfig(t *testing.T) {
	t.Run("finds member by exact name", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.json")
		config := teamConfigFile{
			Members: []teamConfigMember{
				{Name: "alice", AgentType: "general", TmuxPane: "sess:0.1"},
				{Name: "bob", AgentType: "lead", TmuxPane: "sess:0.2"},
			},
		}
		data, _ := json.Marshal(config)
		if err := os.WriteFile(cfgPath, data, 0644); err != nil {
			t.Fatal(err)
		}

		member, ok := searchTeamConfig(cfgPath, "alice")
		if !ok {
			t.Fatal("expected true for alice")
		}
		if member.Name != "alice" {
			t.Errorf("Name = %q, want %q", member.Name, "alice")
		}
		if member.AgentType != "general" {
			t.Errorf("AgentType = %q, want %q", member.AgentType, "general")
		}
		if member.TmuxPane != "sess:0.1" {
			t.Errorf("TmuxPane = %q, want %q", member.TmuxPane, "sess:0.1")
		}
	})

	t.Run("case-insensitive match", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.json")
		config := teamConfigFile{
			Members: []teamConfigMember{
				{Name: "Alice"},
			},
		}
		data, _ := json.Marshal(config)
		os.WriteFile(cfgPath, data, 0644) //nolint:errcheck

		_, ok := searchTeamConfig(cfgPath, "alice")
		if !ok {
			t.Error("expected case-insensitive match")
		}
	})

	t.Run("nonexistent member returns false", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.json")
		config := teamConfigFile{
			Members: []teamConfigMember{{Name: "alice"}},
		}
		data, _ := json.Marshal(config)
		os.WriteFile(cfgPath, data, 0644) //nolint:errcheck

		_, ok := searchTeamConfig(cfgPath, "charlie")
		if ok {
			t.Error("expected false for nonexistent member")
		}
	})

	t.Run("nonexistent file returns false", func(t *testing.T) {
		_, ok := searchTeamConfig("/nonexistent/path/config.json", "alice")
		if ok {
			t.Error("expected false for nonexistent file")
		}
	})

	t.Run("invalid JSON returns false", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.json")
		os.WriteFile(cfgPath, []byte("{invalid}"), 0644) //nolint:errcheck

		_, ok := searchTeamConfig(cfgPath, "alice")
		if ok {
			t.Error("expected false for invalid JSON")
		}
	})
}

// --- persistAssignment ---

func TestContext_PersistAssignment(t *testing.T) {
	t.Run("writes assignment file for non-empty assignment", func(t *testing.T) {
		dir := t.TempDir()
		status := contextSessionStatus{
			SessionID: "persist-test-001",
			AgentName: "worker-01",
			AgentRole: "worker",
			IssueID:   "ag-555",
		}

		if err := persistAssignment(dir, status); err != nil {
			t.Fatalf("persistAssignment: %v", err)
		}

		// Verify file was written
		expectedPath := filepath.Join(dir, ".agents", "ao", "context", "assignment-persist-test-001.json")
		data, err := os.ReadFile(expectedPath)
		if err != nil {
			t.Fatalf("read assignment file: %v", err)
		}

		var snapshot contextAssignmentSnapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			t.Fatalf("unmarshal snapshot: %v", err)
		}
		if snapshot.SessionID != "persist-test-001" {
			t.Errorf("SessionID = %q, want %q", snapshot.SessionID, "persist-test-001")
		}
		if snapshot.AgentName != "worker-01" {
			t.Errorf("AgentName = %q, want %q", snapshot.AgentName, "worker-01")
		}
		if snapshot.IssueID != "ag-555" {
			t.Errorf("IssueID = %q, want %q", snapshot.IssueID, "ag-555")
		}
		if snapshot.UpdatedAt == "" {
			t.Error("expected non-empty UpdatedAt")
		}
	})

	t.Run("skips write for empty assignment", func(t *testing.T) {
		dir := t.TempDir()
		status := contextSessionStatus{
			SessionID: "empty-assign-session",
		}

		if err := persistAssignment(dir, status); err != nil {
			t.Fatalf("persistAssignment: %v", err)
		}

		// Verify no file was written
		expectedPath := filepath.Join(dir, ".agents", "ao", "context", "assignment-empty-assign-session.json")
		if _, err := os.Stat(expectedPath); !os.IsNotExist(err) {
			t.Error("expected no assignment file for empty assignment")
		}
	})
}

// --- persistBudget ---

func TestContext_PersistBudget(t *testing.T) {
	t.Run("creates budget file", func(t *testing.T) {
		dir := t.TempDir()
		status := contextSessionStatus{
			SessionID:      "budget-test-001",
			MaxTokens:      contextbudget.DefaultMaxTokens,
			EstimatedUsage: 50000,
		}

		if err := persistBudget(dir, status); err != nil {
			t.Fatalf("persistBudget: %v", err)
		}

		expectedPath := filepath.Join(dir, ".agents", "ao", "context", "budget-budget-test-001.json")
		data, err := os.ReadFile(expectedPath)
		if err != nil {
			t.Fatalf("read budget file: %v", err)
		}

		var tracker contextbudget.BudgetTracker
		if err := json.Unmarshal(data, &tracker); err != nil {
			t.Fatalf("unmarshal tracker: %v", err)
		}
		if tracker.SessionID != "budget-test-001" {
			t.Errorf("SessionID = %q, want %q", tracker.SessionID, "budget-test-001")
		}
		if tracker.MaxTokens != contextbudget.DefaultMaxTokens {
			t.Errorf("MaxTokens = %d, want %d", tracker.MaxTokens, contextbudget.DefaultMaxTokens)
		}
		if tracker.EstimatedUsage != 50000 {
			t.Errorf("EstimatedUsage = %d, want 50000", tracker.EstimatedUsage)
		}
	})

	t.Run("updates existing budget file", func(t *testing.T) {
		dir := t.TempDir()
		// Create initial budget
		status1 := contextSessionStatus{
			SessionID:      "budget-update-001",
			MaxTokens:      contextbudget.DefaultMaxTokens,
			EstimatedUsage: 30000,
		}
		if err := persistBudget(dir, status1); err != nil {
			t.Fatal(err)
		}

		// Update
		status2 := contextSessionStatus{
			SessionID:      "budget-update-001",
			MaxTokens:      contextbudget.DefaultMaxTokens,
			EstimatedUsage: 80000,
		}
		if err := persistBudget(dir, status2); err != nil {
			t.Fatalf("persistBudget update: %v", err)
		}

		tracker, err := contextbudget.Load(dir, "budget-update-001")
		if err != nil {
			t.Fatalf("load tracker: %v", err)
		}
		if tracker.EstimatedUsage != 80000 {
			t.Errorf("EstimatedUsage = %d, want 80000", tracker.EstimatedUsage)
		}
	})
}

// --- persistGuardState ---

func TestContext_PersistGuardState(t *testing.T) {
	t.Run("persists both budget and assignment", func(t *testing.T) {
		dir := t.TempDir()
		status := contextSessionStatus{
			SessionID:      "guard-state-001",
			MaxTokens:      contextbudget.DefaultMaxTokens,
			EstimatedUsage: 60000,
			AgentName:      "worker-99",
			IssueID:        "ag-guard",
		}

		if err := persistGuardState(dir, status); err != nil {
			t.Fatalf("persistGuardState: %v", err)
		}

		// Verify budget file
		budgetPath := filepath.Join(dir, ".agents", "ao", "context", "budget-guard-state-001.json")
		if _, err := os.Stat(budgetPath); err != nil {
			t.Errorf("budget file not created: %v", err)
		}

		// Verify assignment file
		assignPath := filepath.Join(dir, ".agents", "ao", "context", "assignment-guard-state-001.json")
		if _, err := os.Stat(assignPath); err != nil {
			t.Errorf("assignment file not created: %v", err)
		}
	})

	t.Run("skips assignment for empty assignment fields", func(t *testing.T) {
		dir := t.TempDir()
		status := contextSessionStatus{
			SessionID:      "guard-no-assign",
			MaxTokens:      contextbudget.DefaultMaxTokens,
			EstimatedUsage: 10000,
		}

		if err := persistGuardState(dir, status); err != nil {
			t.Fatalf("persistGuardState: %v", err)
		}

		// Budget should exist
		budgetPath := filepath.Join(dir, ".agents", "ao", "context", "budget-guard-no-assign.json")
		if _, err := os.Stat(budgetPath); err != nil {
			t.Errorf("budget file should exist: %v", err)
		}

		// Assignment should not exist
		assignPath := filepath.Join(dir, ".agents", "ao", "context", "assignment-guard-no-assign.json")
		if _, err := os.Stat(assignPath); !os.IsNotExist(err) {
			t.Error("assignment file should not exist for empty assignment")
		}
	})
}

// --- findPendingHandoffForSession ---

func TestContext_FindPendingHandoffForSession(t *testing.T) {
	t.Run("returns false when pending dir does not exist", func(t *testing.T) {
		dir := t.TempDir()
		_, _, found, err := findPendingHandoffForSession(dir, "any-session")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found {
			t.Error("expected not found for missing pending dir")
		}
	})

	t.Run("returns false when no matching marker", func(t *testing.T) {
		dir := t.TempDir()
		pendingDir := filepath.Join(dir, ".agents", "handoff", "pending")
		if err := os.MkdirAll(pendingDir, 0755); err != nil {
			t.Fatal(err)
		}
		// Write a marker for a different session
		marker := handoffMarker{
			SessionID:   "other-session",
			HandoffFile: "some/path.md",
		}
		data, _ := json.Marshal(marker)
		os.WriteFile(filepath.Join(pendingDir, "marker.json"), data, 0644) //nolint:errcheck

		_, _, found, err := findPendingHandoffForSession(dir, "my-session")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found {
			t.Error("expected not found for non-matching session")
		}
	})

	t.Run("finds matching unconsumed marker", func(t *testing.T) {
		dir := t.TempDir()
		pendingDir := filepath.Join(dir, ".agents", "handoff", "pending")
		if err := os.MkdirAll(pendingDir, 0755); err != nil {
			t.Fatal(err)
		}
		marker := handoffMarker{
			SessionID:   "my-session",
			HandoffFile: ".agents/handoff/auto-test.md",
			Consumed:    false,
		}
		data, _ := json.Marshal(marker)
		markerFile := filepath.Join(pendingDir, "auto-test.json")
		os.WriteFile(markerFile, data, 0644) //nolint:errcheck

		handoff, markerPath, found, err := findPendingHandoffForSession(dir, "my-session")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !found {
			t.Fatal("expected to find matching marker")
		}
		if handoff != ".agents/handoff/auto-test.md" {
			t.Errorf("handoff = %q, want %q", handoff, ".agents/handoff/auto-test.md")
		}
		if markerPath == "" {
			t.Error("expected non-empty marker path")
		}
	})

	t.Run("skips consumed markers", func(t *testing.T) {
		dir := t.TempDir()
		pendingDir := filepath.Join(dir, ".agents", "handoff", "pending")
		if err := os.MkdirAll(pendingDir, 0755); err != nil {
			t.Fatal(err)
		}
		marker := handoffMarker{
			SessionID:   "my-session",
			HandoffFile: "some/path.md",
			Consumed:    true,
		}
		data, _ := json.Marshal(marker)
		os.WriteFile(filepath.Join(pendingDir, "consumed.json"), data, 0644) //nolint:errcheck

		_, _, found, err := findPendingHandoffForSession(dir, "my-session")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found {
			t.Error("expected not found for consumed marker")
		}
	})

	t.Run("skips directories and non-json files", func(t *testing.T) {
		dir := t.TempDir()
		pendingDir := filepath.Join(dir, ".agents", "handoff", "pending")
		if err := os.MkdirAll(filepath.Join(pendingDir, "subdir"), 0755); err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(pendingDir, "notes.txt"), []byte("not json"), 0644) //nolint:errcheck

		_, _, found, err := findPendingHandoffForSession(dir, "any")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found {
			t.Error("expected not found for non-json entries")
		}
	})
}

// --- matchPendingHandoff ---

func TestContext_MatchPendingHandoff(t *testing.T) {
	t.Run("matches unconsumed marker for correct session", func(t *testing.T) {
		dir := t.TempDir()
		marker := handoffMarker{
			SessionID:   "target-session",
			HandoffFile: "handoff.md",
			Consumed:    false,
		}
		data, _ := json.Marshal(marker)
		path := filepath.Join(dir, "test.json")
		os.WriteFile(path, data, 0644) //nolint:errcheck

		hp, _, ok := matchPendingHandoff(path, dir, "target-session")
		if !ok {
			t.Fatal("expected match")
		}
		if hp != "handoff.md" {
			t.Errorf("handoff path = %q, want %q", hp, "handoff.md")
		}
	})

	t.Run("no match for wrong session", func(t *testing.T) {
		dir := t.TempDir()
		marker := handoffMarker{
			SessionID:   "other-session",
			HandoffFile: "handoff.md",
		}
		data, _ := json.Marshal(marker)
		path := filepath.Join(dir, "test.json")
		os.WriteFile(path, data, 0644) //nolint:errcheck

		_, _, ok := matchPendingHandoff(path, dir, "target-session")
		if ok {
			t.Error("expected no match for wrong session")
		}
	})

	t.Run("no match for consumed marker", func(t *testing.T) {
		dir := t.TempDir()
		marker := handoffMarker{
			SessionID: "target-session",
			Consumed:  true,
		}
		data, _ := json.Marshal(marker)
		path := filepath.Join(dir, "test.json")
		os.WriteFile(path, data, 0644) //nolint:errcheck

		_, _, ok := matchPendingHandoff(path, dir, "target-session")
		if ok {
			t.Error("expected no match for consumed marker")
		}
	})

	t.Run("no match for nonexistent file", func(t *testing.T) {
		_, _, ok := matchPendingHandoff("/nonexistent/file.json", "/cwd", "any")
		if ok {
			t.Error("expected no match for nonexistent file")
		}
	})

	t.Run("no match for invalid JSON", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "bad.json")
		os.WriteFile(path, []byte("{bad json}"), 0644) //nolint:errcheck

		_, _, ok := matchPendingHandoff(path, dir, "any")
		if ok {
			t.Error("expected no match for invalid JSON")
		}
	})
}

// --- renderHandoffMarkdown ---

func TestContext_RenderHandoffMarkdown(t *testing.T) {
	now := time.Date(2026, 2, 20, 15, 30, 0, 0, time.UTC)
	status := contextSessionStatus{
		SessionID:        "render-test-001",
		Status:           string(contextbudget.StatusCritical),
		UsagePercent:     0.92,
		RemainingPercent: 0.08,
		Readiness:        contextReadinessCritical,
		Action:           "handoff_now",
		LastTask:         "orchestrate workers",
		EstimatedUsage:   184000,
		MaxTokens:        contextbudget.DefaultMaxTokens,
		Model:            "claude-opus",
		Recommendation:   "CRITICAL: Context nearly full.",
		AgentName:        "worker-01",
		AgentRole:        "worker",
		TeamName:         "team-alpha",
		IssueID:          "ag-999",
		TmuxTarget:       "convoy:0",
		IsStale:          false,
	}
	usage := transcriptUsage{
		InputTokens:             1000,
		CacheCreationInputToken: 83000,
		CacheReadInputToken:     100000,
		Model:                   "claude-opus",
	}

	md := renderHandoffMarkdown(now, status, usage, "ag-999", []string{"file1.go", "file2.go"})

	checks := []struct {
		label    string
		contains string
	}{
		{"title", "# Auto-Handoff (Context Guard)"},
		{"timestamp", "2026-02-20T15:30:00Z"},
		{"session id", "render-test-001"},
		{"status", "CRITICAL (92.0%)"},
		{"hull", "CRITICAL"},
		{"action", "handoff_now"},
		{"last task", "orchestrate workers"},
		{"active bead", "ag-999"},
		{"agent name", "worker-01"},
		{"agent role", "worker"},
		{"team name", "team-alpha"},
		{"issue id", "ag-999"},
		{"tmux target", "convoy:0"},
		{"modified files", "file1.go"},
		{"modified files 2", "file2.go"},
		{"model", "claude-opus"},
		{"input tokens", "1000"},
		{"cache creation", "83000"},
		{"cache read", "100000"},
		{"estimated usage", "184000/200000"},
		{"recommendation", "CRITICAL: Context nearly full."},
		{"next action", "Start a fresh session"},
		{"blockers none", "none detected"},
	}

	for _, c := range checks {
		if !strings.Contains(md, c.contains) {
			t.Errorf("[%s] expected markdown to contain %q", c.label, c.contains)
		}
	}
}

func TestContext_RenderHandoffMarkdown_StaleBlocker(t *testing.T) {
	now := time.Now().UTC()
	status := contextSessionStatus{
		SessionID:        "stale-render",
		Status:           string(contextbudget.StatusCritical),
		UsagePercent:     0.90,
		RemainingPercent: 0.10,
		Readiness:        contextReadinessCritical,
		Action:           "recover_dead_session",
		LastTask:         "some task",
		IsStale:          true,
	}
	usage := transcriptUsage{}

	md := renderHandoffMarkdown(now, status, usage, "none", nil)

	if !strings.Contains(md, "stale") {
		t.Error("expected stale blocker in markdown")
	}
	if !strings.Contains(md, "watchdog recovery") {
		t.Error("expected watchdog recovery in blockers section")
	}
}

func TestContext_RenderHandoffMarkdown_NoChangedFiles(t *testing.T) {
	now := time.Now().UTC()
	status := contextSessionStatus{
		SessionID: "no-files",
		LastTask:  "task",
	}
	usage := transcriptUsage{}

	md := renderHandoffMarkdown(now, status, usage, "none", nil)

	// Should contain "none" under Modified Files
	if !strings.Contains(md, "## Modified Files\nnone") {
		t.Error("expected 'none' under Modified Files when no files changed")
	}
}

func TestContext_RenderHandoffMarkdown_FallbackReadiness(t *testing.T) {
	now := time.Now().UTC()
	status := contextSessionStatus{
		SessionID:    "fallback-readiness",
		UsagePercent: 0.50,
		Readiness:    "", // empty readiness should fall back
		LastTask:     "task",
	}
	usage := transcriptUsage{}

	md := renderHandoffMarkdown(now, status, usage, "none", nil)

	// With 0.50 usage -> remaining 0.50 -> RED readiness (>= 0.40 remaining)
	if !strings.Contains(md, "RED") {
		t.Error("expected RED readiness as fallback for 50% usage (remaining 50%)")
	}
}

func TestContext_RenderHandoffMarkdown_FallbackRemainingPercent(t *testing.T) {
	now := time.Now().UTC()
	status := contextSessionStatus{
		SessionID:        "fallback-remaining",
		UsagePercent:     0.70,
		RemainingPercent: 0, // zero remaining should be recomputed
		Readiness:        contextReadinessCritical,
		LastTask:         "task",
	}
	usage := transcriptUsage{}

	md := renderHandoffMarkdown(now, status, usage, "none", nil)

	// With 0.70 usage and remaining=0 -> should recompute to 30%
	if !strings.Contains(md, "30.0% remaining") {
		t.Errorf("expected recomputed 30.0%% remaining in markdown, got:\n%s", md)
	}
}

// --- collectTrackedSessionStatuses ---

func TestContext_CollectTrackedSessionStatuses(t *testing.T) {
	t.Run("no budget files returns nil", func(t *testing.T) {
		dir := t.TempDir()
		statuses, err := collectTrackedSessionStatuses(dir, 20*time.Minute)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if statuses != nil {
			t.Errorf("expected nil, got %d statuses", len(statuses))
		}
	})

	t.Run("reads budget files and produces statuses", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)
		dir := t.TempDir()

		// Create a budget file
		contextDir := filepath.Join(dir, ".agents", "ao", "context")
		if err := os.MkdirAll(contextDir, 0755); err != nil {
			t.Fatal(err)
		}

		sessionID := "tracked-session-001"
		tracker := contextbudget.NewBudgetTracker(sessionID)
		tracker.MaxTokens = contextbudget.DefaultMaxTokens
		tracker.UpdateUsage(50000)
		data, _ := json.MarshalIndent(tracker, "", "  ")
		if err := os.WriteFile(filepath.Join(contextDir, "budget-"+sessionID+".json"), data, 0644); err != nil {
			t.Fatal(err)
		}

		// Create a transcript for it
		transcriptDir := filepath.Join(tmpHome, ".claude", "projects", "proj", "conversations")
		if err := os.MkdirAll(transcriptDir, 0755); err != nil {
			t.Fatal(err)
		}
		lines := []map[string]any{
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
					"model": "claude-opus",
					"usage": map[string]any{
						"input_tokens":                1000,
						"cache_creation_input_tokens": 2000,
						"cache_read_input_tokens":     3000,
					},
				},
			},
		}
		writeTranscriptLines(t, filepath.Join(transcriptDir, sessionID+".jsonl"), lines)

		statuses, err := collectTrackedSessionStatuses(dir, 20*time.Minute)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(statuses) != 1 {
			t.Fatalf("expected 1 status, got %d", len(statuses))
		}
		if statuses[0].SessionID != sessionID {
			t.Errorf("SessionID = %q, want %q", statuses[0].SessionID, sessionID)
		}
	})

	t.Run("skips invalid budget files", func(t *testing.T) {
		dir := t.TempDir()
		contextDir := filepath.Join(dir, ".agents", "ao", "context")
		if err := os.MkdirAll(contextDir, 0755); err != nil {
			t.Fatal(err)
		}
		// Write invalid JSON as a budget file
		if err := os.WriteFile(filepath.Join(contextDir, "budget-invalid.json"), []byte("{bad}"), 0644); err != nil {
			t.Fatal(err)
		}

		statuses, err := collectTrackedSessionStatuses(dir, 20*time.Minute)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(statuses) != 0 {
			t.Errorf("expected 0 statuses for invalid budget, got %d", len(statuses))
		}
	})
}

// --- collectOneTrackedStatus ---

func TestContext_CollectOneTrackedStatus(t *testing.T) {
	t.Run("returns false for unreadable file", func(t *testing.T) {
		_, ok := collectOneTrackedStatus("/tmp", "/nonexistent/budget.json", 20*time.Minute)
		if ok {
			t.Error("expected false for unreadable file")
		}
	})

	t.Run("returns false for invalid JSON", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "budget-bad.json")
		os.WriteFile(path, []byte("not json"), 0644) //nolint:errcheck

		_, ok := collectOneTrackedStatus(dir, path, 20*time.Minute)
		if ok {
			t.Error("expected false for invalid JSON")
		}
	})

	t.Run("returns false for empty session ID", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "budget-empty.json")
		tracker := contextbudget.BudgetTracker{SessionID: ""}
		data, _ := json.Marshal(tracker)
		os.WriteFile(path, data, 0644) //nolint:errcheck

		_, ok := collectOneTrackedStatus(dir, path, 20*time.Minute)
		if ok {
			t.Error("expected false for empty session ID")
		}
	})

	t.Run("falls back to staleBudgetFallbackStatus on transcript error", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)
		dir := t.TempDir()

		path := filepath.Join(dir, "budget-fallback.json")
		tracker := contextbudget.BudgetTracker{
			SessionID:      "fallback-session",
			MaxTokens:      contextbudget.DefaultMaxTokens,
			EstimatedUsage: 100000,
			LastUpdated:    time.Now().Add(-2 * time.Hour),
		}
		data, _ := json.Marshal(tracker)
		os.WriteFile(path, data, 0644) //nolint:errcheck

		// No transcript exists, so collectSessionStatus will fail and fallback kicks in
		status, ok := collectOneTrackedStatus(dir, path, 20*time.Minute)
		if !ok {
			t.Fatal("expected true for fallback status")
		}
		if status.SessionID != "fallback-session" {
			t.Errorf("SessionID = %q, want %q", status.SessionID, "fallback-session")
		}
		// Should be marked stale since LastUpdated is 2 hours ago and watchdog is 20 min
		if !status.IsStale {
			t.Error("expected stale status for old budget with no transcript")
		}
	})
}

// --- maybeAutoRestartStaleSession edge cases ---

func TestContext_MaybeAutoRestartStaleSession_NonRecoverAction(t *testing.T) {
	status := contextSessionStatus{
		Action:     "continue",
		TmuxTarget: "some-target:0",
	}
	updated := maybeAutoRestartStaleSession(status)
	// Should return unchanged
	if updated.RestartAttempt {
		t.Error("should not attempt restart for non-recover action")
	}
}

func TestContext_MaybeAutoRestartStaleSession_MissingTarget(t *testing.T) {
	status := contextSessionStatus{
		Action:     "recover_dead_session",
		TmuxTarget: "",
	}
	updated := maybeAutoRestartStaleSession(status)
	if updated.RestartMessage != "missing tmux target mapping" {
		t.Errorf("RestartMessage = %q, want %q", updated.RestartMessage, "missing tmux target mapping")
	}
}

func TestContext_MaybeAutoRestartStaleSession_WhitespaceTarget(t *testing.T) {
	status := contextSessionStatus{
		Action:     "recover_dead_session",
		TmuxTarget: "   ",
	}
	updated := maybeAutoRestartStaleSession(status)
	if updated.RestartMessage != "missing tmux target mapping" {
		t.Errorf("RestartMessage = %q, want %q", updated.RestartMessage, "missing tmux target mapping")
	}
}
