// practices: [agile-manifesto, dora-metrics]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// =============================================================================
// RPI Status helpers
// =============================================================================

// --- truncateGoal ---

func TestRPIStatus_TruncateGoal(t *testing.T) {
	tests := []struct {
		name     string
		goal     string
		maxLen   int
		expected string
	}{
		{"short goal unchanged", "hello", 10, "hello"},
		{"exact length unchanged", "abcde", 5, "abcde"},
		{"truncated with ellipsis", "a long goal that needs truncation", 15, "a long goal ..."},
		{"zero length goal", "", 10, ""},
		{"maxLen equals goal length", "hello", 5, "hello"},
		{"maxLen one more than goal", "hello", 6, "hello"},
		{"very short maxLen", "hello world", 4, "h..."},
		{"maxLen exactly 3", "abcdef", 3, "..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateGoal(tt.goal, tt.maxLen)
			if got != tt.expected {
				t.Errorf("truncateGoal(%q, %d) = %q, want %q", tt.goal, tt.maxLen, got, tt.expected)
			}
		})
	}
}

// --- lastPhaseName ---

func TestRPIStatus_LastPhaseName(t *testing.T) {
	tests := []struct {
		name     string
		phases   []rpiPhaseEntry
		expected string
	}{
		{"empty phases", nil, ""},
		{"single phase", []rpiPhaseEntry{{Name: "discovery"}}, "discovery"},
		{"multiple phases returns last", []rpiPhaseEntry{
			{Name: "start"},
			{Name: "discovery"},
			{Name: "implementation"},
		}, "implementation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lastPhaseName(tt.phases)
			if got != tt.expected {
				t.Errorf("lastPhaseName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// --- totalRetries ---

func TestRPIStatus_TotalRetries(t *testing.T) {
	tests := []struct {
		name     string
		retries  map[string]int
		expected int
	}{
		{"nil map", nil, 0},
		{"empty map", map[string]int{}, 0},
		{"single entry", map[string]int{"validation": 3}, 3},
		{"multiple entries", map[string]int{"validation": 2, "discovery": 1, "implementation": 4}, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := totalRetries(tt.retries)
			if got != tt.expected {
				t.Errorf("totalRetries() = %d, want %d", got, tt.expected)
			}
		})
	}
}

// --- formatLogRunDuration ---

func TestRPIStatus_FormatLogRunDuration(t *testing.T) {
	tests := []struct {
		name     string
		dur      time.Duration
		expected string
	}{
		{"zero duration", 0, ""},
		{"negative duration", -5 * time.Minute, ""},
		{"one minute", 1 * time.Minute, "1m0s"},
		{"35 minutes", 35 * time.Minute, "35m0s"},
		{"1 hour 5 minutes 30 seconds", 1*time.Hour + 5*time.Minute + 30*time.Second + 123*time.Millisecond, "1h5m30s"},
		{"sub-second truncated", 500 * time.Millisecond, "0s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatLogRunDuration(tt.dur)
			if got != tt.expected {
				t.Errorf("formatLogRunDuration(%v) = %q, want %q", tt.dur, got, tt.expected)
			}
		})
	}
}

// --- formattedLogRunStatus ---

func TestRPIStatus_FormattedLogRunStatus(t *testing.T) {
	tests := []struct {
		name     string
		run      rpiRun
		expected string
	}{
		{
			"running no verdicts",
			rpiRun{Status: "running", Verdicts: map[string]string{}},
			"running",
		},
		{
			"completed no verdicts",
			rpiRun{Status: "completed", Verdicts: map[string]string{}},
			"completed",
		},
		{
			"failed with verdicts still shows failed",
			rpiRun{Status: "failed", Verdicts: map[string]string{"vibe": "FAIL"}},
			"failed",
		},
		{
			"completed with nil verdicts",
			rpiRun{Status: "completed", Verdicts: nil},
			"completed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formattedLogRunStatus(tt.run)
			if got != tt.expected {
				t.Errorf("formattedLogRunStatus() = %q, want %q", got, tt.expected)
			}
		})
	}

	// Special case: completed with verdicts should append verdict string.
	t.Run("completed with verdicts appended", func(t *testing.T) {
		run := rpiRun{
			Status:   "completed",
			Verdicts: map[string]string{"vibe": "PASS"},
		}
		got := formattedLogRunStatus(run)
		if got != "completed [vibe=PASS]" {
			t.Errorf("formattedLogRunStatus() = %q, want %q", got, "completed [vibe=PASS]")
		}
	})
}

// --- joinVerdicts ---

func TestRPIStatus_JoinVerdicts(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]string
		empty bool
	}{
		{"nil map", nil, true},
		{"empty map", map[string]string{}, true},
		{"single verdict", map[string]string{"vibe": "PASS"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := joinVerdicts(tt.input)
			if tt.empty && got != "" {
				t.Errorf("joinVerdicts() = %q, want empty", got)
			}
			if !tt.empty && got == "" {
				t.Errorf("joinVerdicts() = empty, want non-empty")
			}
		})
	}

	t.Run("single verdict format", func(t *testing.T) {
		got := joinVerdicts(map[string]string{"vibe": "PASS"})
		if got != "vibe=PASS" {
			t.Errorf("joinVerdicts() = %q, want %q", got, "vibe=PASS")
		}
	})
}

// --- displayPhaseName ---

func TestRPIStatus_DisplayPhaseName(t *testing.T) {
	tests := []struct {
		name     string
		state    phasedState
		expected string
	}{
		{"v1 discovery", phasedState{SchemaVersion: 1, Phase: 1}, "discovery"},
		{"v1 implementation", phasedState{SchemaVersion: 1, Phase: 2}, "implementation"},
		{"v1 validation", phasedState{SchemaVersion: 1, Phase: 3}, "validation"},
		{"v1 unknown phase", phasedState{SchemaVersion: 1, Phase: 0}, "phase-0"},
		{"v1 high phase", phasedState{SchemaVersion: 2, Phase: 10}, "phase-10"},
		{"legacy research", phasedState{SchemaVersion: 0, Phase: 1}, "research"},
		{"legacy plan", phasedState{SchemaVersion: 0, Phase: 2}, "plan"},
		{"legacy pre-mortem", phasedState{SchemaVersion: 0, Phase: 3}, "pre-mortem"},
		{"legacy crank", phasedState{SchemaVersion: 0, Phase: 4}, "crank"},
		{"legacy vibe", phasedState{SchemaVersion: 0, Phase: 5}, "vibe"},
		{"legacy post-mortem", phasedState{SchemaVersion: 0, Phase: 6}, "post-mortem"},
		{"legacy unknown phase", phasedState{SchemaVersion: 0, Phase: 7}, "phase-7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := displayPhaseName(tt.state)
			if got != tt.expected {
				t.Errorf("displayPhaseName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// --- completedPhaseNumber ---

func TestRPIStatus_CompletedPhaseNumber(t *testing.T) {
	tests := []struct {
		name     string
		state    phasedState
		expected int
	}{
		{"schema v1", phasedState{SchemaVersion: 1}, 3},
		{"schema v2", phasedState{SchemaVersion: 2}, 3},
		{"legacy schema", phasedState{SchemaVersion: 0}, 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := completedPhaseNumber(tt.state)
			if got != tt.expected {
				t.Errorf("completedPhaseNumber() = %d, want %d", got, tt.expected)
			}
		})
	}
}

// --- parseOrchestrationLogLine ---

func TestRPIStatus_ParseOrchestrationLogLine(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantOK    bool
		wantRunID string
		wantPhase string
		wantDets  string
		wantTime  bool
	}{
		{
			"new format with runID",
			"[2026-02-15T10:00:00Z] [abc123] start: goal=\"test\" from=discovery",
			true, "abc123", "start", `goal="test" from=discovery`, true,
		},
		{
			"old format without runID",
			"[2026-02-15T09:00:00Z] start: goal=\"fix typo\" from=discovery",
			true, "", "start", `goal="fix typo" from=discovery`, true,
		},
		{
			"garbage line",
			"this is not a log line",
			false, "", "", "", false,
		},
		{
			"empty line",
			"",
			false, "", "", "", false,
		},
		{
			"bad timestamp still parses structure",
			"[not-a-timestamp] [run1] discovery: completed in 5m0s",
			true, "run1", "discovery", "completed in 5m0s", false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, ok := parseOrchestrationLogLine(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("parseOrchestrationLogLine() ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if entry.RunID != tt.wantRunID {
				t.Errorf("RunID = %q, want %q", entry.RunID, tt.wantRunID)
			}
			if entry.PhaseName != tt.wantPhase {
				t.Errorf("PhaseName = %q, want %q", entry.PhaseName, tt.wantPhase)
			}
			if entry.Details != tt.wantDets {
				t.Errorf("Details = %q, want %q", entry.Details, tt.wantDets)
			}
			if entry.HasTime != tt.wantTime {
				t.Errorf("HasTime = %v, want %v", entry.HasTime, tt.wantTime)
			}
		})
	}
}

// --- orchestrationLogState ---

func TestRPIStatus_ResolveRunID(t *testing.T) {
	s := newOrchestrationLogState()

	// With an explicit run ID, it returns as-is.
	if got := s.ResolveRunID("explicit", "start"); got != "explicit" {
		t.Errorf("expected 'explicit', got %q", got)
	}

	// Without a run ID and phase "start", creates anon-1.
	if got := s.ResolveRunID("", "start"); got != "anon-1" {
		t.Errorf("expected 'anon-1', got %q", got)
	}

	// Without a run ID and a non-start phase, uses current anon counter.
	if got := s.ResolveRunID("", "discovery"); got != "anon-1" {
		t.Errorf("expected 'anon-1' for non-start, got %q", got)
	}

	// Another "start" increments the counter.
	if got := s.ResolveRunID("", "start"); got != "anon-2" {
		t.Errorf("expected 'anon-2', got %q", got)
	}
}

func TestRPIStatus_ResolveRunID_NoStartFirst(t *testing.T) {
	s := newOrchestrationLogState()

	// When anonymousCounter is 0 and phase is not "start", it initializes to 1.
	if got := s.ResolveRunID("", "discovery"); got != "anon-1" {
		t.Errorf("expected 'anon-1' for first non-start line, got %q", got)
	}
}

func TestRPIStatus_GetOrCreateRun(t *testing.T) {
	s := newOrchestrationLogState()

	run1 := s.GetOrCreateRun("run-a")
	if run1.RunID != "run-a" {
		t.Errorf("expected RunID 'run-a', got %q", run1.RunID)
	}
	if run1.Status != "running" {
		t.Errorf("expected initial status 'running', got %q", run1.Status)
	}

	// Getting the same ID returns the same pointer.
	run1Again := s.GetOrCreateRun("run-a")
	if run1 != run1Again {
		t.Error("expected same pointer for same runID")
	}

	// Different ID creates a new run.
	run2 := s.GetOrCreateRun("run-b")
	if run2.RunID != "run-b" {
		t.Errorf("expected RunID 'run-b', got %q", run2.RunID)
	}
}

func TestRPIStatus_OrderedRuns(t *testing.T) {
	s := newOrchestrationLogState()

	s.GetOrCreateRun("c")
	s.GetOrCreateRun("a")
	s.GetOrCreateRun("b")

	runs := s.OrderedRuns()
	if len(runs) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(runs))
	}
	expected := []string{"c", "a", "b"}
	for i, r := range runs {
		if r.RunID != expected[i] {
			t.Errorf("run[%d] = %q, want %q", i, r.RunID, expected[i])
		}
	}
}

// --- applyOrchestrationLogEntry ---

func TestRPIStatus_ApplyOrchestrationLogEntry_Start(t *testing.T) {
	run := &rpiRun{
		RunID:    "test",
		Verdicts: make(map[string]string),
		Retries:  make(map[string]int),
		Status:   "running",
	}
	ts, _ := time.Parse(time.RFC3339, "2026-02-15T10:00:00Z")
	entry := orchestrationLogEntry{
		Timestamp: "2026-02-15T10:00:00Z",
		PhaseName: "start",
		Details:   `goal="build feature" from=discovery`,
		ParsedAt:  ts,
		HasTime:   true,
	}
	applyOrchestrationLogEntry(run, entry)

	if run.Goal != "build feature" {
		t.Errorf("expected goal 'build feature', got %q", run.Goal)
	}
	if run.StartedAt != ts {
		t.Errorf("expected StartedAt to be set")
	}
	if len(run.Phases) != 1 {
		t.Errorf("expected 1 phase entry, got %d", len(run.Phases))
	}
}

func TestRPIStatus_ApplyOrchestrationLogEntry_Complete(t *testing.T) {
	startTS, _ := time.Parse(time.RFC3339, "2026-02-15T10:00:00Z")
	completeTS, _ := time.Parse(time.RFC3339, "2026-02-15T10:35:00Z")

	run := &rpiRun{
		RunID:     "test",
		StartedAt: startTS,
		Verdicts:  make(map[string]string),
		Retries:   make(map[string]int),
		Status:    "running",
	}
	entry := orchestrationLogEntry{
		Timestamp: "2026-02-15T10:35:00Z",
		PhaseName: "complete",
		Details:   "epic=ag-test verdicts=map[vibe:PASS pre_mortem:WARN]",
		ParsedAt:  completeTS,
		HasTime:   true,
	}
	applyOrchestrationLogEntry(run, entry)

	if run.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", run.Status)
	}
	if run.EpicID != "ag-test" {
		t.Errorf("expected EpicID 'ag-test', got %q", run.EpicID)
	}
	if run.Verdicts["vibe"] != "PASS" {
		t.Errorf("expected vibe=PASS, got %q", run.Verdicts["vibe"])
	}
	if run.Verdicts["pre_mortem"] != "WARN" {
		t.Errorf("expected pre_mortem=WARN, got %q", run.Verdicts["pre_mortem"])
	}
	expectedDur := 35 * time.Minute
	if run.Duration != expectedDur {
		t.Errorf("expected duration %v, got %v", expectedDur, run.Duration)
	}
}

func TestRPIStatus_ApplyOrchestrationLogEntry_NoTimestamp(t *testing.T) {
	run := &rpiRun{
		RunID:    "test",
		Verdicts: make(map[string]string),
		Retries:  make(map[string]int),
		Status:   "running",
	}
	entry := orchestrationLogEntry{
		Timestamp: "invalid",
		PhaseName: "discovery",
		Details:   "completed in 5m0s",
		HasTime:   false,
	}
	applyOrchestrationLogEntry(run, entry)

	// StartedAt should remain zero.
	if !run.StartedAt.IsZero() {
		t.Error("expected StartedAt to remain zero when HasTime is false")
	}
	if len(run.Phases) != 1 {
		t.Errorf("expected 1 phase entry, got %d", len(run.Phases))
	}
}

// --- updateFailureStatus ---

func TestRPIStatus_UpdateFailureStatus(t *testing.T) {
	tests := []struct {
		name       string
		details    string
		wantStatus string
	}{
		{"FAILED prefix", "FAILED: some error", "failed"},
		{"FATAL prefix", "FATAL: crash", "failed"},
		{"normal details", "completed in 5m0s", "running"},
		{"FAILED in middle", "some FAILED text", "running"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := &rpiRun{Status: "running"}
			updateFailureStatus(run, tt.details)
			if run.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", run.Status, tt.wantStatus)
			}
		})
	}
}

// --- updateRetryCount ---

func TestRPIStatus_UpdateRetryCount(t *testing.T) {
	run := &rpiRun{Retries: make(map[string]int)}

	updateRetryCount(run, "validation", "RETRY attempt 2/3")
	if run.Retries["validation"] != 1 {
		t.Errorf("expected 1 retry, got %d", run.Retries["validation"])
	}

	updateRetryCount(run, "validation", "RETRY attempt 3/3")
	if run.Retries["validation"] != 2 {
		t.Errorf("expected 2 retries, got %d", run.Retries["validation"])
	}

	// Non-retry details should not increment.
	updateRetryCount(run, "validation", "completed in 5m0s")
	if run.Retries["validation"] != 2 {
		t.Errorf("expected 2 retries unchanged, got %d", run.Retries["validation"])
	}
}

// --- updateFinishedAtFromCompletedDuration ---

func TestRPIStatus_UpdateFinishedAtFromCompletedDuration(t *testing.T) {
	ts, _ := time.Parse(time.RFC3339, "2026-02-15T10:05:00Z")

	t.Run("valid completed in duration", func(t *testing.T) {
		run := &rpiRun{}
		entry := orchestrationLogEntry{
			Details:  "completed in 5m0s",
			ParsedAt: ts,
			HasTime:  true,
		}
		updateFinishedAtFromCompletedDuration(run, entry)
		if run.FinishedAt != ts {
			t.Errorf("expected FinishedAt = %v, got %v", ts, run.FinishedAt)
		}
	})

	t.Run("non-completed details", func(t *testing.T) {
		run := &rpiRun{}
		entry := orchestrationLogEntry{
			Details:  "some other details",
			ParsedAt: ts,
			HasTime:  true,
		}
		updateFinishedAtFromCompletedDuration(run, entry)
		if !run.FinishedAt.IsZero() {
			t.Errorf("expected FinishedAt to be zero, got %v", run.FinishedAt)
		}
	})

	t.Run("invalid duration string", func(t *testing.T) {
		run := &rpiRun{}
		entry := orchestrationLogEntry{
			Details:  "completed in notaduration",
			ParsedAt: ts,
			HasTime:  true,
		}
		updateFinishedAtFromCompletedDuration(run, entry)
		if !run.FinishedAt.IsZero() {
			t.Errorf("expected FinishedAt to remain zero for invalid duration")
		}
	})

	t.Run("no timestamp", func(t *testing.T) {
		run := &rpiRun{}
		entry := orchestrationLogEntry{
			Details: "completed in 5m0s",
			HasTime: false,
		}
		updateFinishedAtFromCompletedDuration(run, entry)
		if !run.FinishedAt.IsZero() {
			t.Errorf("expected FinishedAt to remain zero when no timestamp")
		}
	})
}

// --- updateInlineVerdicts ---

func TestRPIStatus_UpdateInlineVerdicts(t *testing.T) {
	tests := []struct {
		name      string
		phase     string
		details   string
		wantKey   string
		wantValue string
	}{
		{"pre-mortem phase PASS", "pre-mortem", "verdict PASS", "pre_mortem", "PASS"},
		{"vibe phase WARN", "vibe", "verdict WARN", "vibe", "WARN"},
		{"post-mortem phase FAIL", "post-mortem", "verdict FAIL", "post_mortem", "FAIL"},
		{"pre-mortem verdict in details", "validation", "pre-mortem verdict: PASS", "pre_mortem", "PASS"},
		{"vibe verdict in details", "validation", "vibe verdict: WARN", "vibe", "WARN"},
		{"post-mortem verdict in details", "review", "post-mortem verdict: FAIL", "post_mortem", "FAIL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := &rpiRun{Verdicts: make(map[string]string)}
			updateInlineVerdicts(run, tt.phase, tt.details)
			if run.Verdicts[tt.wantKey] != tt.wantValue {
				t.Errorf("Verdicts[%q] = %q, want %q", tt.wantKey, run.Verdicts[tt.wantKey], tt.wantValue)
			}
		})
	}

	t.Run("no verdict in details", func(t *testing.T) {
		run := &rpiRun{Verdicts: make(map[string]string)}
		updateInlineVerdicts(run, "discovery", "completed in 5m0s")
		if len(run.Verdicts) != 0 {
			t.Errorf("expected no verdicts, got %d", len(run.Verdicts))
		}
	})
}

// --- extractInlineVerdict ---

func TestRPIStatus_ExtractInlineVerdict(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"PASS", "PASS"},
		{"WARN", "WARN"},
		{"FAIL", "FAIL"},
		{"contains PASS and FAIL", "PASS"}, // first match wins
		{"no verdict here", ""},
		{"pass lowercase", ""},
		{"warning lowercase", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractInlineVerdict(tt.input)
			if got != tt.expected {
				t.Errorf("extractInlineVerdict(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// --- normalizeSearchRootPath ---

func TestRPIStatus_NormalizeSearchRootPath(t *testing.T) {
	tmpDir := t.TempDir()

	// A real directory should return a clean absolute path.
	got := normalizeSearchRootPath(tmpDir)
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}

	// Path with trailing slash should be cleaned.
	got2 := normalizeSearchRootPath(tmpDir + "/")
	if got2 != got {
		t.Errorf("trailing slash not cleaned: %q vs %q", got2, got)
	}
}

// --- tryAddSearchRoot ---

func TestRPIStatus_TryAddSearchRoot(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	t.Run("adds valid directory", func(t *testing.T) {
		seen := make(map[string]struct{})
		var roots []string
		tryAddSearchRoot(tmpDir, seen, &roots)
		if len(roots) != 1 {
			t.Fatalf("expected 1 root, got %d", len(roots))
		}
	})

	t.Run("skips empty path", func(t *testing.T) {
		seen := make(map[string]struct{})
		var roots []string
		tryAddSearchRoot("", seen, &roots)
		if len(roots) != 0 {
			t.Errorf("expected 0 roots for empty path, got %d", len(roots))
		}
	})

	t.Run("skips nonexistent path", func(t *testing.T) {
		seen := make(map[string]struct{})
		var roots []string
		tryAddSearchRoot("/nonexistent/path/that/does/not/exist", seen, &roots)
		if len(roots) != 0 {
			t.Errorf("expected 0 roots for nonexistent path, got %d", len(roots))
		}
	})

	t.Run("deduplicates same path", func(t *testing.T) {
		seen := make(map[string]struct{})
		var roots []string
		tryAddSearchRoot(tmpDir, seen, &roots)
		tryAddSearchRoot(tmpDir, seen, &roots)
		if len(roots) != 1 {
			t.Errorf("expected 1 root after dedup, got %d", len(roots))
		}
	})

	t.Run("adds multiple distinct directories", func(t *testing.T) {
		seen := make(map[string]struct{})
		var roots []string
		tryAddSearchRoot(tmpDir, seen, &roots)
		tryAddSearchRoot(subDir, seen, &roots)
		if len(roots) != 2 {
			t.Errorf("expected 2 roots, got %d", len(roots))
		}
	})

	t.Run("skips file path", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "afile.txt")
		if err := os.WriteFile(filePath, []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
		seen := make(map[string]struct{})
		var roots []string
		tryAddSearchRoot(filePath, seen, &roots)
		if len(roots) != 0 {
			t.Errorf("expected 0 roots for file path, got %d", len(roots))
		}
	})
}

// --- collectSearchRoots ---

func TestRPIStatus_CollectSearchRoots(t *testing.T) {
	parent := t.TempDir()
	cwd := filepath.Join(parent, "myrepo")
	sibling := filepath.Join(parent, "myrepo-rpi-abc")

	for _, dir := range []string{cwd, sibling} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	roots := collectSearchRoots(cwd)

	foundCwd := false
	for _, r := range roots {
		if r == cwd {
			foundCwd = true
		}
	}
	if !foundCwd {
		t.Error("expected cwd to be in search roots")
	}

	foundSibling := false
	for _, r := range roots {
		if r == sibling {
			foundSibling = true
		}
	}
	if !foundSibling {
		t.Error("expected sibling worktree to be in search roots")
	}
}

func TestRPIStatus_CollectSearchRoots_NoSiblings(t *testing.T) {
	parent := t.TempDir()
	cwd := filepath.Join(parent, "solo-repo")
	if err := os.MkdirAll(cwd, 0755); err != nil {
		t.Fatal(err)
	}

	roots := collectSearchRoots(cwd)
	if len(roots) == 0 {
		t.Fatal("expected at least cwd in roots")
	}
	if roots[0] != cwd {
		t.Errorf("expected first root to be cwd %q, got %q", cwd, roots[0])
	}
}

// --- discoverLiveStatuses ---

func TestRPIStatus_DiscoverLiveStatuses_Deduplication(t *testing.T) {
	tmpDir := t.TempDir()
	rpiDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0755); err != nil {
		t.Fatal(err)
	}
	statusPath := filepath.Join(rpiDir, "live-status.md")
	if err := os.WriteFile(statusPath, []byte("# Status"), 0644); err != nil {
		t.Fatal(err)
	}

	snapshots := discoverLiveStatuses(tmpDir)
	if len(snapshots) != 1 {
		t.Errorf("expected 1 snapshot, got %d", len(snapshots))
	}
}

func TestRPIStatus_DiscoverLiveStatuses_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	snapshots := discoverLiveStatuses(tmpDir)
	if len(snapshots) != 0 {
		t.Errorf("expected 0 snapshots for dir without live-status, got %d", len(snapshots))
	}
}

// --- discoverLogRuns ---

func TestRPIStatus_DiscoverLogRuns_CwdOnly(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatal(err)
	}
	logContent := "[2026-02-15T10:00:00Z] [r1] start: goal=\"test\" from=discovery\n" +
		"[2026-02-15T10:05:00Z] [r1] complete: epic=ag-test verdicts=map[]\n"
	if err := os.WriteFile(filepath.Join(logDir, "phased-orchestration.log"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	runs := discoverLogRuns(tmpDir)
	if len(runs) != 1 {
		t.Fatalf("expected 1 log run, got %d", len(runs))
	}
	if runs[0].RunID != "r1" {
		t.Errorf("expected RunID 'r1', got %q", runs[0].RunID)
	}
}

func TestRPIStatus_DiscoverLogRuns_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	runs := discoverLogRuns(tmpDir)
	if len(runs) != 0 {
		t.Errorf("expected 0 log runs for empty dir, got %d", len(runs))
	}
}

// --- classifyRunStatus (additional edge cases) ---

func TestRPIStatus_ClassifyRunStatus_ActiveOverridesCompleted(t *testing.T) {
	state := phasedState{SchemaVersion: 1, Phase: 3}
	status := classifyRunStatus(state, true)
	if status != "running" {
		t.Errorf("expected 'running' for active at terminal phase, got %q", status)
	}
}

func TestRPIStatus_ClassifyRunStatus_TerminalStatusPrecedence(t *testing.T) {
	state := phasedState{SchemaVersion: 1, Phase: 2, TerminalStatus: "interrupted"}
	status := classifyRunStatus(state, true)
	if status != "interrupted" {
		t.Errorf("expected 'interrupted', got %q", status)
	}
}

func TestRPIStatus_ClassifyRunStatus_UnknownNoWorktree(t *testing.T) {
	state := phasedState{SchemaVersion: 1, Phase: 1}
	status := classifyRunStatus(state, false)
	if status != "unknown" {
		t.Errorf("expected 'unknown', got %q", status)
	}
}

func TestRPIStatus_ClassifyRunStatus_StaleWorktreeGone(t *testing.T) {
	state := phasedState{
		SchemaVersion: 1,
		Phase:         1,
		WorktreePath:  "/nonexistent/worktree",
	}
	status := classifyRunStatus(state, false)
	if status != "stale" {
		t.Errorf("expected 'stale', got %q", status)
	}
}

func TestRPIStatus_ClassifyRunStatus_WorktreeExists(t *testing.T) {
	tmpDir := t.TempDir()
	state := phasedState{
		SchemaVersion: 1,
		Phase:         1,
		WorktreePath:  tmpDir, // exists
	}
	status := classifyRunStatus(state, false)
	if status != "unknown" {
		t.Errorf("expected 'unknown' for existing worktree without liveness, got %q", status)
	}
}

// --- classifyRunReason (additional) ---

func TestRPIStatus_ClassifyRunReason_TerminalReasonPrecedence(t *testing.T) {
	state := phasedState{
		TerminalReason: "signal: interrupt",
		WorktreePath:   "/nonexistent",
	}
	reason := classifyRunReason(state, false)
	if reason != "signal: interrupt" {
		t.Errorf("expected terminal reason, got %q", reason)
	}
}

func TestRPIStatus_ClassifyRunReason_ActiveNoReason(t *testing.T) {
	state := phasedState{WorktreePath: "/nonexistent"}
	reason := classifyRunReason(state, true)
	if reason != "" {
		t.Errorf("expected empty reason for active run, got %q", reason)
	}
}

// --- scanRegistryRuns ---

func TestRPIStatus_ScanRegistryRuns_SkipsFiles(t *testing.T) {
	tmpDir := t.TempDir()
	runsDir := filepath.Join(tmpDir, ".agents", "rpi", "runs")
	if err := os.MkdirAll(runsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runsDir, "not-a-dir"), []byte("junk"), 0644); err != nil {
		t.Fatal(err)
	}
	writeRegistryRun(t, tmpDir, registryRunSpec{
		runID:  "valid-run",
		phase:  1,
		schema: 1,
		hbAge:  0,
	})

	runs := scanRegistryRuns(tmpDir)
	if len(runs) != 1 {
		t.Fatalf("expected 1 run (skipping files), got %d", len(runs))
	}
	if runs[0].RunID != "valid-run" {
		t.Errorf("expected RunID 'valid-run', got %q", runs[0].RunID)
	}
}

func TestRPIStatus_ScanRegistryRuns_SkipsBadJSON(t *testing.T) {
	tmpDir := t.TempDir()
	runDir := filepath.Join(tmpDir, ".agents", "rpi", "runs", "bad-json")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, phasedStateFile), []byte("{{invalid"), 0644); err != nil {
		t.Fatal(err)
	}

	runs := scanRegistryRuns(tmpDir)
	if len(runs) != 0 {
		t.Errorf("expected 0 runs for bad JSON, got %d", len(runs))
	}
}

func TestRPIStatus_ScanRegistryRuns_SkipsEmptyRunID(t *testing.T) {
	tmpDir := t.TempDir()
	runDir := filepath.Join(tmpDir, ".agents", "rpi", "runs", "no-id")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}
	state := map[string]any{
		"schema_version": 1,
		"goal":           "some goal",
		"phase":          1,
	}
	data, _ := json.Marshal(state)
	if err := os.WriteFile(filepath.Join(runDir, phasedStateFile), data, 0644); err != nil {
		t.Fatal(err)
	}

	runs := scanRegistryRuns(tmpDir)
	if len(runs) != 0 {
		t.Errorf("expected 0 runs for empty run_id, got %d", len(runs))
	}
}

func TestRPIStatus_ScanRegistryRuns_MissingStateFile(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a run directory but no state file inside it.
	runDir := filepath.Join(tmpDir, ".agents", "rpi", "runs", "no-state")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}

	runs := scanRegistryRuns(tmpDir)
	if len(runs) != 0 {
		t.Errorf("expected 0 runs for missing state file, got %d", len(runs))
	}
}

// --- buildRPIStatusOutput ---

func TestRPIStatus_BuildRPIStatusOutput_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	output := buildRPIStatusOutput(tmpDir)
	if output.Count != 0 {
		t.Errorf("expected count 0, got %d", output.Count)
	}
	if len(output.Active) != 0 {
		t.Errorf("expected 0 active, got %d", len(output.Active))
	}
	if len(output.Historical) != 0 {
		t.Errorf("expected 0 historical, got %d", len(output.Historical))
	}
	if len(output.Runs) != 0 {
		t.Errorf("expected 0 combined runs, got %d", len(output.Runs))
	}
}

func TestRPIStatus_BuildRPIStatusOutput_WithRuns(t *testing.T) {
	tmpDir := t.TempDir()

	writeRegistryRun(t, tmpDir, registryRunSpec{
		runID:  "active-1",
		phase:  2,
		schema: 1,
		goal:   "active goal",
		hbAge:  1 * time.Minute,
	})
	writeRegistryRun(t, tmpDir, registryRunSpec{
		runID:  "done-1",
		phase:  3,
		schema: 1,
		goal:   "done goal",
		hbAge:  0,
	})

	output := buildRPIStatusOutput(tmpDir)

	if output.Count != 2 {
		t.Errorf("expected count 2, got %d", output.Count)
	}
	if len(output.Active) != 1 {
		t.Errorf("expected 1 active, got %d", len(output.Active))
	}
	if len(output.Historical) != 1 {
		t.Errorf("expected 1 historical, got %d", len(output.Historical))
	}
	if len(output.Runs) != 2 {
		t.Errorf("expected 2 combined runs, got %d", len(output.Runs))
	}
}

// --- applyCompletePhase (without timestamp) ---

func TestRPIStatus_ApplyCompletePhase_NoTimestamp(t *testing.T) {
	run := &rpiRun{
		RunID:    "test",
		Verdicts: make(map[string]string),
		Retries:  make(map[string]int),
		Status:   "running",
	}
	entry := orchestrationLogEntry{
		PhaseName: "complete",
		Details:   "epic=ag-notime verdicts=map[vibe:PASS]",
		HasTime:   false,
	}
	applyCompletePhase(run, entry)
	if run.Status != "completed" {
		t.Errorf("expected completed, got %q", run.Status)
	}
	if run.EpicID != "ag-notime" {
		t.Errorf("expected EpicID ag-notime, got %q", run.EpicID)
	}
	if !run.FinishedAt.IsZero() {
		t.Error("expected FinishedAt to be zero when no timestamp")
	}
}

// --- newOrchestrationLogState ---

func TestRPIStatus_NewOrchestrationLogState(t *testing.T) {
	s := newOrchestrationLogState()
	if s.RunMap == nil {
		t.Error("expected non-nil runMap")
	}
	if len(s.RunOrder) != 0 {
		t.Errorf("expected empty runOrder, got %d", len(s.RunOrder))
	}
	if s.AnonymousCounter != 0 {
		t.Errorf("expected anonymousCounter 0, got %d", s.AnonymousCounter)
	}
}

// =============================================================================
// Hooks helpers
// =============================================================================

// --- isAoManagedHookCommand ---

func TestHooks_IsAoManagedHookCommand(t *testing.T) {
	tests := []struct {
		cmd      string
		expected bool
	}{
		{"ao inject --apply-decay", true},
		{"ao forge transcript --last-session", true},
		{"/Users/test/.agentops/hooks/session-start.sh", true},
		{"~/.agentops/hooks/pre-tool-use.sh arg1 arg2", true},
		{"my-custom-hook", false},
		{"echo hello", false},
		{"aoinject", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			got := isAoManagedHookCommand(tt.cmd)
			if got != tt.expected {
				t.Errorf("isAoManagedHookCommand(%q) = %v, want %v", tt.cmd, got, tt.expected)
			}
		})
	}
}

// --- rawGroupIsAoManaged ---

func TestHooks_RawGroupIsAoManaged(t *testing.T) {
	t.Run("new format with ao command", func(t *testing.T) {
		group := map[string]any{
			"hooks": []any{
				map[string]any{"type": "command", "command": "ao inject"},
			},
		}
		if !rawGroupIsAoManaged(group) {
			t.Error("expected ao group to be detected")
		}
	})

	t.Run("new format without ao command", func(t *testing.T) {
		group := map[string]any{
			"hooks": []any{
				map[string]any{"type": "command", "command": "my-custom-tool"},
			},
		}
		if rawGroupIsAoManaged(group) {
			t.Error("expected non-ao group to not be detected")
		}
	})

	t.Run("empty hooks array", func(t *testing.T) {
		group := map[string]any{
			"hooks": []any{},
		}
		if rawGroupIsAoManaged(group) {
			t.Error("expected empty hooks to not be detected as ao")
		}
	})

	t.Run("no hooks key", func(t *testing.T) {
		group := map[string]any{
			"other": "data",
		}
		if rawGroupIsAoManaged(group) {
			t.Error("expected group without hooks key to not be detected")
		}
	})
}

// --- rawGroupHooksContainAo ---

func TestHooks_RawGroupHooksContainAo(t *testing.T) {
	t.Run("hooks with ao command", func(t *testing.T) {
		group := map[string]any{
			"hooks": []any{
				map[string]any{"type": "command", "command": "my-thing"},
				map[string]any{"type": "command", "command": "ao forge"},
			},
		}
		if !rawGroupHooksContainAo(group) {
			t.Error("expected ao command to be found")
		}
	})

	t.Run("hooks without ao command", func(t *testing.T) {
		group := map[string]any{
			"hooks": []any{
				map[string]any{"type": "command", "command": "other-tool"},
			},
		}
		if rawGroupHooksContainAo(group) {
			t.Error("expected no ao command to be found")
		}
	})

	t.Run("hooks with non-map entries", func(t *testing.T) {
		group := map[string]any{
			"hooks": []any{
				"not-a-map",
				42,
			},
		}
		if rawGroupHooksContainAo(group) {
			t.Error("expected non-map entries to not match")
		}
	})

	t.Run("hooks with non-string command", func(t *testing.T) {
		group := map[string]any{
			"hooks": []any{
				map[string]any{"type": "command", "command": 42},
			},
		}
		if rawGroupHooksContainAo(group) {
			t.Error("expected non-string command to not match")
		}
	})
}

// --- rawGroupLegacyContainsAo ---

func TestHooks_RawGroupLegacyContainsAo(t *testing.T) {
	t.Run("legacy format with ao command", func(t *testing.T) {
		group := map[string]any{
			"command": []any{"sh", "ao inject --apply-decay"},
		}
		if !rawGroupLegacyContainsAo(group) {
			t.Error("expected legacy ao command to be detected")
		}
	})

	t.Run("legacy format non-ao", func(t *testing.T) {
		group := map[string]any{
			"command": []any{"sh", "echo hello"},
		}
		if rawGroupLegacyContainsAo(group) {
			t.Error("expected non-ao legacy command to not be detected")
		}
	})

	t.Run("legacy format single element", func(t *testing.T) {
		group := map[string]any{
			"command": []any{"ao inject"},
		}
		if rawGroupLegacyContainsAo(group) {
			t.Error("expected single-element command to not match")
		}
	})

	t.Run("command is not an array", func(t *testing.T) {
		group := map[string]any{
			"command": "ao inject",
		}
		if rawGroupLegacyContainsAo(group) {
			t.Error("expected string command to not match legacy format")
		}
	})

	t.Run("second element is not a string", func(t *testing.T) {
		group := map[string]any{
			"command": []any{"sh", 42},
		}
		if rawGroupLegacyContainsAo(group) {
			t.Error("expected non-string second element to not match")
		}
	})
}

// --- hookGroupContainsAo ---

func TestHooks_HookGroupContainsAo_MissingEvent(t *testing.T) {
	hooksMap := map[string]any{}
	if hookGroupContainsAo(hooksMap, "SessionStart") {
		t.Error("expected false for missing event")
	}
}

func TestHooks_HookGroupContainsAo_NonArrayEvent(t *testing.T) {
	hooksMap := map[string]any{
		"SessionStart": "not-an-array",
	}
	if hookGroupContainsAo(hooksMap, "SessionStart") {
		t.Error("expected false for non-array event value")
	}
}

func TestHooks_HookGroupContainsAo_NonMapGroup(t *testing.T) {
	hooksMap := map[string]any{
		"SessionStart": []any{"not-a-map", 42},
	}
	if hookGroupContainsAo(hooksMap, "SessionStart") {
		t.Error("expected false for non-map group entries")
	}
}

// --- filterNonAoHookGroups ---

func TestHooks_FilterNonAoHookGroups_EmptyEvent(t *testing.T) {
	hooksMap := map[string]any{}
	filtered := filterNonAoHookGroups(hooksMap, "SessionStart")
	if len(filtered) != 0 {
		t.Errorf("expected 0 groups for missing event, got %d", len(filtered))
	}
}

func TestHooks_FilterNonAoHookGroups_MixedGroups(t *testing.T) {
	hooksMap := map[string]any{
		"Stop": []any{
			map[string]any{
				"hooks": []any{
					map[string]any{"type": "command", "command": "ao forge transcript"},
				},
			},
			map[string]any{
				"hooks": []any{
					map[string]any{"type": "command", "command": "my-custom-stop-hook"},
				},
			},
			map[string]any{
				"hooks": []any{
					map[string]any{"type": "command", "command": "/home/user/.agentops/hooks/stop.sh"},
				},
			},
		},
	}
	filtered := filterNonAoHookGroups(hooksMap, "Stop")
	if len(filtered) != 1 {
		t.Fatalf("expected 1 non-ao group, got %d", len(filtered))
	}
}

func TestHooks_FilterNonAoHookGroups_AllAo(t *testing.T) {
	hooksMap := map[string]any{
		"SessionStart": []any{
			map[string]any{
				"hooks": []any{
					map[string]any{"type": "command", "command": "ao inject --apply-decay"},
				},
			},
		},
	}
	filtered := filterNonAoHookGroups(hooksMap, "SessionStart")
	if len(filtered) != 0 {
		t.Errorf("expected 0 non-ao groups, got %d", len(filtered))
	}
}

func TestHooks_FilterNonAoHookGroups_AllCustom(t *testing.T) {
	hooksMap := map[string]any{
		"PreToolUse": []any{
			map[string]any{
				"hooks": []any{
					map[string]any{"type": "command", "command": "my-linter"},
				},
			},
			map[string]any{
				"hooks": []any{
					map[string]any{"type": "command", "command": "my-formatter"},
				},
			},
		},
	}
	filtered := filterNonAoHookGroups(hooksMap, "PreToolUse")
	if len(filtered) != 2 {
		t.Errorf("expected 2 non-ao groups, got %d", len(filtered))
	}
}

// --- countRawGroupHooks ---

func TestHooks_CountRawGroupHooks(t *testing.T) {
	tests := []struct {
		name     string
		groups   []any
		expected int
	}{
		{"nil groups", nil, 0},
		{"empty groups", []any{}, 0},
		{
			"single group one hook",
			[]any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "test"},
					},
				},
			},
			1,
		},
		{
			"single group multiple hooks",
			[]any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "test1"},
						map[string]any{"type": "command", "command": "test2"},
					},
				},
			},
			2,
		},
		{
			"multiple groups",
			[]any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "t1"},
					},
				},
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "t2"},
						map[string]any{"type": "command", "command": "t3"},
					},
				},
			},
			3,
		},
		{
			"non-map group entries skipped",
			[]any{"not-a-map", 42},
			0,
		},
		{
			"group without hooks key",
			[]any{
				map[string]any{"other": "data"},
			},
			0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countRawGroupHooks(tt.groups)
			if got != tt.expected {
				t.Errorf("countRawGroupHooks() = %d, want %d", got, tt.expected)
			}
		})
	}
}

// --- countInstalledHookEvents ---

func TestHooks_CountInstalledHookEvents(t *testing.T) {
	t.Run("empty map", func(t *testing.T) {
		hooksMap := map[string]any{}
		if got := countInstalledHookEvents(hooksMap); got != 0 {
			t.Errorf("expected 0, got %d", got)
		}
	})

	t.Run("two events installed", func(t *testing.T) {
		hooksMap := map[string]any{
			"SessionStart": []any{map[string]any{"hooks": []any{}}},
			"Stop":         []any{map[string]any{"hooks": []any{}}},
		}
		if got := countInstalledHookEvents(hooksMap); got != 2 {
			t.Errorf("expected 2, got %d", got)
		}
	})

	t.Run("all 12 events installed", func(t *testing.T) {
		hooksMap := make(map[string]any)
		for _, event := range AllEventNames() {
			hooksMap[event] = []any{map[string]any{"hooks": []any{}}}
		}
		if got := countInstalledHookEvents(hooksMap); got != 12 {
			t.Errorf("expected 12, got %d", got)
		}
	})

	t.Run("empty array not counted", func(t *testing.T) {
		hooksMap := map[string]any{
			"SessionStart": []any{},
		}
		if got := countInstalledHookEvents(hooksMap); got != 0 {
			t.Errorf("expected 0 for empty array, got %d", got)
		}
	})

	t.Run("non-array value not counted", func(t *testing.T) {
		hooksMap := map[string]any{
			"SessionStart": "not-an-array",
		}
		if got := countInstalledHookEvents(hooksMap); got != 0 {
			t.Errorf("expected 0 for non-array, got %d", got)
		}
	})

	t.Run("unknown events not counted", func(t *testing.T) {
		hooksMap := map[string]any{
			"FakeEvent":    []any{map[string]any{"hooks": []any{}}},
			"SessionStart": []any{map[string]any{"hooks": []any{}}},
		}
		if got := countInstalledHookEvents(hooksMap); got != 1 {
			t.Errorf("expected 1 (only known events), got %d", got)
		}
	})
}

// --- readSettingsHooksMap ---

func TestHooks_ReadSettingsHooksMap(t *testing.T) {
	t.Run("file not found returns nil nil", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "nonexistent.json")
		hooksMap, err := readSettingsHooksMap(path)
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
		if hooksMap != nil {
			t.Errorf("expected nil hooksMap, got %v", hooksMap)
		}
	})

	t.Run("valid settings with hooks", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "settings.json")
		settings := map[string]any{
			"hooks": map[string]any{
				"SessionStart": []any{
					map[string]any{
						"hooks": []any{
							map[string]any{"type": "command", "command": "ao inject"},
						},
					},
				},
			},
		}
		data, _ := json.Marshal(settings)
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}

		hooksMap, err := readSettingsHooksMap(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hooksMap == nil {
			t.Fatal("expected non-nil hooksMap")
		}
		if _, ok := hooksMap["SessionStart"]; !ok {
			t.Error("expected SessionStart key in hooksMap")
		}
	})

	t.Run("settings without hooks key returns nil nil", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "settings.json")
		settings := map[string]any{
			"other": "data",
		}
		data, _ := json.Marshal(settings)
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}

		hooksMap, err := readSettingsHooksMap(path)
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
		if hooksMap != nil {
			t.Errorf("expected nil hooksMap, got %v", hooksMap)
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "settings.json")
		if err := os.WriteFile(path, []byte("{{not json"), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := readSettingsHooksMap(path)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("hooks key is not a map returns nil nil", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "settings.json")
		settings := map[string]any{
			"hooks": "not-a-map",
		}
		data, _ := json.Marshal(settings)
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}

		hooksMap, err := readSettingsHooksMap(path)
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
		if hooksMap != nil {
			t.Errorf("expected nil hooksMap for non-map hooks, got %v", hooksMap)
		}
	})
}

// --- loadHooksSettings ---

func TestHooks_LoadHooksSettings(t *testing.T) {
	t.Run("existing valid file", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "settings.json")
		settings := map[string]any{
			"hooks": map[string]any{
				"SessionStart": []any{},
			},
			"other_key": "value",
		}
		data, _ := json.Marshal(settings)
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}

		rawSettings, err := loadHooksSettings(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rawSettings["other_key"] != "value" {
			t.Error("expected other_key preserved")
		}
	})

	t.Run("nonexistent file returns empty map", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "nonexistent.json")
		rawSettings, err := loadHooksSettings(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rawSettings) != 0 {
			t.Errorf("expected empty map, got %d keys", len(rawSettings))
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "settings.json")
		if err := os.WriteFile(path, []byte("{{not json"), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := loadHooksSettings(path)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}

// --- cloneHooksMap ---

func TestHooks_CloneHooksMap(t *testing.T) {
	t.Run("no existing hooks", func(t *testing.T) {
		rawSettings := map[string]any{
			"other": "data",
		}
		cloned := cloneHooksMap(rawSettings)
		if len(cloned) != 0 {
			t.Errorf("expected empty clone, got %d keys", len(cloned))
		}
	})

	t.Run("existing hooks cloned", func(t *testing.T) {
		rawSettings := map[string]any{
			"hooks": map[string]any{
				"SessionStart": []any{},
				"Stop":         []any{},
			},
		}
		cloned := cloneHooksMap(rawSettings)
		if len(cloned) != 2 {
			t.Errorf("expected 2 keys in clone, got %d", len(cloned))
		}
		if _, ok := cloned["SessionStart"]; !ok {
			t.Error("expected SessionStart in clone")
		}
	})

	t.Run("hooks is not a map", func(t *testing.T) {
		rawSettings := map[string]any{
			"hooks": "string-value",
		}
		cloned := cloneHooksMap(rawSettings)
		if len(cloned) != 0 {
			t.Errorf("expected empty clone for non-map hooks, got %d", len(cloned))
		}
	})
}

// --- mergeHookEvents ---

func TestHooks_MergeHookEvents(t *testing.T) {
	hooksMap := make(map[string]any)
	newHooks := &HooksConfig{
		SessionStart: []HookGroup{
			{Hooks: []HookEntry{{Type: "command", Command: "ao inject"}}},
		},
		Stop: []HookGroup{
			{Hooks: []HookEntry{{Type: "command", Command: "ao forge"}}},
		},
	}

	eventsToInstall := []string{"SessionStart", "Stop"}
	installed := mergeHookEvents(hooksMap, newHooks, eventsToInstall)

	if installed != 2 {
		t.Errorf("expected 2 installed events, got %d", installed)
	}
	if _, ok := hooksMap["SessionStart"]; !ok {
		t.Error("expected SessionStart in hooksMap after merge")
	}
	if _, ok := hooksMap["Stop"]; !ok {
		t.Error("expected Stop in hooksMap after merge")
	}
}

func TestHooks_MergeHookEvents_PreservesExisting(t *testing.T) {
	hooksMap := map[string]any{
		"SessionStart": []any{
			map[string]any{
				"hooks": []any{
					map[string]any{"type": "command", "command": "my-custom-start-hook"},
				},
			},
		},
	}

	newHooks := &HooksConfig{
		SessionStart: []HookGroup{
			{Hooks: []HookEntry{{Type: "command", Command: "ao inject"}}},
		},
	}

	installed := mergeHookEvents(hooksMap, newHooks, []string{"SessionStart"})
	if installed != 1 {
		t.Errorf("expected 1 installed event, got %d", installed)
	}

	groups, ok := hooksMap["SessionStart"].([]map[string]any)
	if !ok {
		t.Fatal("expected SessionStart to be []map[string]any after merge")
	}
	if len(groups) != 2 {
		t.Errorf("expected 2 groups (existing + new), got %d", len(groups))
	}
}

func TestHooks_MergeHookEvents_EmptyNewGroups(t *testing.T) {
	hooksMap := make(map[string]any)
	newHooks := &HooksConfig{}

	installed := mergeHookEvents(hooksMap, newHooks, []string{"SessionStart", "Stop"})
	if installed != 0 {
		t.Errorf("expected 0 installed events, got %d", installed)
	}
}

// --- existingAoHooksBlock ---

func TestHooks_ExistingAoHooksBlock(t *testing.T) {
	t.Run("no existing hooks", func(t *testing.T) {
		rawSettings := map[string]any{}
		origForce := hooksForce
		hooksForce = false
		defer func() { hooksForce = origForce }()

		if existingAoHooksBlock(rawSettings) {
			t.Error("expected false when no hooks exist")
		}
	})

	t.Run("existing ao hooks block", func(t *testing.T) {
		rawSettings := map[string]any{
			"hooks": map[string]any{
				"SessionStart": []any{
					map[string]any{
						"hooks": []any{
							map[string]any{"type": "command", "command": "ao inject"},
						},
					},
				},
			},
		}
		origForce := hooksForce
		hooksForce = false
		defer func() { hooksForce = origForce }()

		if !existingAoHooksBlock(rawSettings) {
			t.Error("expected true when ao hooks exist and force is false")
		}
	})

	t.Run("force overrides", func(t *testing.T) {
		rawSettings := map[string]any{
			"hooks": map[string]any{
				"SessionStart": []any{
					map[string]any{
						"hooks": []any{
							map[string]any{"type": "command", "command": "ao inject"},
						},
					},
				},
			},
		}
		origForce := hooksForce
		hooksForce = true
		defer func() { hooksForce = origForce }()

		if existingAoHooksBlock(rawSettings) {
			t.Error("expected false when force is true")
		}
	})
}

// --- hooksCopyFile ---

func TestHooks_HooksCopyFile(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.txt")
	if err := os.WriteFile(srcPath, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("copy to new location", func(t *testing.T) {
		dstPath := filepath.Join(tmpDir, "nested", "dest.txt")
		if err := hooksCopyFile(srcPath, dstPath); err != nil {
			t.Fatalf("hooksCopyFile failed: %v", err)
		}
		data, err := os.ReadFile(dstPath)
		if err != nil {
			t.Fatalf("read dest: %v", err)
		}
		if string(data) != "hello world" {
			t.Errorf("expected 'hello world', got %q", string(data))
		}
	})

	t.Run("source does not exist", func(t *testing.T) {
		dstPath := filepath.Join(tmpDir, "nonexistent_dest.txt")
		err := hooksCopyFile(filepath.Join(tmpDir, "nonexistent.txt"), dstPath)
		if err == nil {
			t.Error("expected error for nonexistent source")
		}
	})
}

// --- backupHooksSettings ---

func TestHooks_BackupHooksSettings(t *testing.T) {
	t.Run("creates backup of existing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		settingsPath := filepath.Join(tmpDir, "settings.json")
		if err := os.WriteFile(settingsPath, []byte(`{"hooks": {}}`), 0644); err != nil {
			t.Fatal(err)
		}

		if err := backupHooksSettings(settingsPath); err != nil {
			t.Fatalf("backupHooksSettings failed: %v", err)
		}

		entries, _ := os.ReadDir(tmpDir)
		backupFound := false
		for _, e := range entries {
			if e.Name() != "settings.json" {
				backupFound = true
			}
		}
		if !backupFound {
			t.Error("expected backup file to be created")
		}
	})

	t.Run("no-op for nonexistent file", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "nonexistent.json")
		if err := backupHooksSettings(path); err != nil {
			t.Errorf("expected nil error for nonexistent file, got %v", err)
		}
	})
}

// --- writeHooksSettings ---

func TestHooks_WriteHooksSettings(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, ".claude", "settings.json")

	rawSettings := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{},
		},
	}

	if err := writeHooksSettings(settingsPath, rawSettings); err != nil {
		t.Fatalf("writeHooksSettings failed: %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse written settings: %v", err)
	}
	if _, ok := parsed["hooks"]; !ok {
		t.Error("expected hooks key in written settings")
	}
}

// --- copyOptionalFile ---

func TestHooks_CopyOptionalFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("copies existing file", func(t *testing.T) {
		src := filepath.Join(tmpDir, "existing.txt")
		dst := filepath.Join(tmpDir, "out", "copied.txt")
		if err := os.WriteFile(src, []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}

		n, err := copyOptionalFile(src, dst, "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 1 {
			t.Errorf("expected 1 file copied, got %d", n)
		}
	})

	t.Run("missing source returns 0", func(t *testing.T) {
		dst := filepath.Join(tmpDir, "out2", "copied.txt")
		n, err := copyOptionalFile(filepath.Join(tmpDir, "missing.txt"), dst, "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 0 {
			t.Errorf("expected 0 files copied, got %d", n)
		}
	})
}

// --- copyShellScripts ---

func TestHooks_CopyShellScripts(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := filepath.Join(t.TempDir(), "dest")

	for _, name := range []string{"hook1.sh", "hook2.sh", "readme.txt"} {
		if err := os.WriteFile(filepath.Join(srcDir, name), []byte("#!/bin/bash"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	n, err := copyShellScripts(srcDir, dstDir)
	if err != nil {
		t.Fatalf("copyShellScripts failed: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 scripts copied, got %d", n)
	}

	for _, name := range []string{"hook1.sh", "hook2.sh"} {
		info, err := os.Stat(filepath.Join(dstDir, name))
		if err != nil {
			t.Errorf("stat %s: %v", name, err)
			continue
		}
		if info.Mode()&0111 == 0 {
			t.Errorf("%s is not executable", name)
		}
	}
}

// --- hookGroupToMap ---

func TestHooks_HookGroupToMap(t *testing.T) {
	t.Run("basic conversion", func(t *testing.T) {
		g := HookGroup{
			Hooks: []HookEntry{
				{Type: "command", Command: "test-cmd"},
			},
		}
		m := hookGroupToMap(g)
		hooks, ok := m["hooks"].([]map[string]any)
		if !ok || len(hooks) != 1 {
			t.Fatal("expected hooks array with 1 entry")
		}
		if hooks[0]["type"] != "command" || hooks[0]["command"] != "test-cmd" {
			t.Error("hook entry mismatch")
		}
		if _, exists := m["matcher"]; exists {
			t.Error("expected no matcher key for empty matcher")
		}
	})

	t.Run("with matcher", func(t *testing.T) {
		g := HookGroup{
			Matcher: "Bash",
			Hooks:   []HookEntry{{Type: "command", Command: "test"}},
		}
		m := hookGroupToMap(g)
		if m["matcher"] != "Bash" {
			t.Errorf("expected matcher 'Bash', got %v", m["matcher"])
		}
	})

	t.Run("with timeout", func(t *testing.T) {
		g := HookGroup{
			Hooks: []HookEntry{{Type: "command", Command: "test", Timeout: 30}},
		}
		m := hookGroupToMap(g)
		hooks := m["hooks"].([]map[string]any)
		if hooks[0]["timeout"] != 30 {
			t.Errorf("expected timeout 30, got %v", hooks[0]["timeout"])
		}
	})

	t.Run("zero timeout omitted", func(t *testing.T) {
		g := HookGroup{
			Hooks: []HookEntry{{Type: "command", Command: "test", Timeout: 0}},
		}
		m := hookGroupToMap(g)
		hooks := m["hooks"].([]map[string]any)
		if _, exists := hooks[0]["timeout"]; exists {
			t.Error("expected no timeout key for zero timeout")
		}
	})

	t.Run("multiple hooks", func(t *testing.T) {
		g := HookGroup{
			Hooks: []HookEntry{
				{Type: "command", Command: "cmd1"},
				{Type: "command", Command: "cmd2", Timeout: 10},
			},
		}
		m := hookGroupToMap(g)
		hooks := m["hooks"].([]map[string]any)
		if len(hooks) != 2 {
			t.Errorf("expected 2 hooks, got %d", len(hooks))
		}
	})
}

// --- replacePluginRoot ---

func TestHooks_ReplacePluginRoot(t *testing.T) {
	t.Run("replaces in all events", func(t *testing.T) {
		config := &HooksConfig{
			SessionStart: []HookGroup{
				{Hooks: []HookEntry{{Type: "command", Command: "${CLAUDE_PLUGIN_ROOT}/hooks/start.sh"}}},
			},
			PostToolUse: []HookGroup{
				{Hooks: []HookEntry{{Type: "command", Command: "${CLAUDE_PLUGIN_ROOT}/hooks/post.sh"}}},
			},
		}
		replacePluginRoot(config, "/home/user/.agentops")
		if config.SessionStart[0].Hooks[0].Command != "/home/user/.agentops/hooks/start.sh" {
			t.Errorf("SessionStart not rewritten: %s", config.SessionStart[0].Hooks[0].Command)
		}
		if config.PostToolUse[0].Hooks[0].Command != "/home/user/.agentops/hooks/post.sh" {
			t.Errorf("PostToolUse not rewritten: %s", config.PostToolUse[0].Hooks[0].Command)
		}
	})

	t.Run("empty basePath removes placeholder", func(t *testing.T) {
		config := &HooksConfig{
			Stop: []HookGroup{
				{Hooks: []HookEntry{{Type: "command", Command: "${CLAUDE_PLUGIN_ROOT}/hooks/stop.sh"}}},
			},
		}
		replacePluginRoot(config, "")
		if config.Stop[0].Hooks[0].Command != "/hooks/stop.sh" {
			t.Errorf("expected placeholder removed, got %s", config.Stop[0].Hooks[0].Command)
		}
	})
}

// --- AllEventNames ---

func TestHooks_AllEventNames_Count(t *testing.T) {
	events := AllEventNames()
	if len(events) != 12 {
		t.Errorf("expected 12 events, got %d", len(events))
	}

	seen := make(map[string]bool)
	for _, e := range events {
		if seen[e] {
			t.Errorf("duplicate event name: %s", e)
		}
		seen[e] = true
	}
}

// --- HooksConfig eventGroupPtrs ---

func TestHooks_EventGroupPtrs_AllMapped(t *testing.T) {
	config := &HooksConfig{}
	events := AllEventNames()
	for _, event := range events {
		config.SetEventGroups(event, []HookGroup{{Hooks: []HookEntry{{Type: "command", Command: "test"}}}})
		groups := config.GetEventGroups(event)
		if len(groups) != 1 {
			t.Errorf("event %s: expected 1 group after set, got %d", event, len(groups))
		}
	}
	if len(events) != 12 {
		t.Errorf("expected 12 events, got %d", len(events))
	}
}

func TestHooks_EventGroupPtr_UnknownEvent(t *testing.T) {
	config := &HooksConfig{}
	groups := config.GetEventGroups("Nonexistent")
	if groups != nil {
		t.Error("expected nil for unknown event")
	}
}

// --- generateMinimalHooksConfig ---

func TestHooks_GenerateMinimalHooksConfig_Structure(t *testing.T) {
	hooks := generateMinimalHooksConfig()

	if len(hooks.SessionStart) == 0 {
		t.Error("expected SessionStart")
	}
	if len(hooks.Stop) == 0 {
		t.Error("expected Stop")
	}
	if len(hooks.SessionEnd) == 0 {
		t.Error("expected SessionEnd")
	}

	emptyEvents := []string{
		"PreToolUse", "PostToolUse",
		"UserPromptSubmit", "TaskCompleted", "PreCompact",
		"SubagentStop", "WorktreeCreate", "WorktreeRemove", "ConfigChange",
	}
	for _, event := range emptyEvents {
		groups := hooks.GetEventGroups(event)
		if len(groups) != 0 {
			t.Errorf("expected no groups for %s in minimal config, got %d", event, len(groups))
		}
	}
}

// --- copyOptionalDir ---

func TestHooks_CopyOptionalDir(t *testing.T) {
	t.Run("copies existing dir", func(t *testing.T) {
		srcDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644); err != nil {
			t.Fatal(err)
		}
		subDir := filepath.Join(srcDir, "sub")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(subDir, "file2.txt"), []byte("content2"), 0644); err != nil {
			t.Fatal(err)
		}

		dstDir := filepath.Join(t.TempDir(), "output")
		n, err := copyOptionalDir(srcDir, dstDir, "test-dir")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 2 {
			t.Errorf("expected 2 files copied, got %d", n)
		}
	})

	t.Run("missing source returns 0", func(t *testing.T) {
		dstDir := filepath.Join(t.TempDir(), "output")
		n, err := copyOptionalDir("/nonexistent/source", dstDir, "test-dir")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 0 {
			t.Errorf("expected 0, got %d", n)
		}
	})
}
