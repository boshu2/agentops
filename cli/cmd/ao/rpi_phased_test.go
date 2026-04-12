package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

func TestExtractCouncilVerdict(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
		wantErr  bool
	}{
		{
			name:     "PASS verdict",
			content:  "# Pre-Mortem\n\n## Council Verdict: PASS\n\nDetails here.",
			expected: "PASS",
		},
		{
			name:     "WARN verdict",
			content:  "## Council Verdict: WARN\n\nSome concerns.",
			expected: "WARN",
		},
		{
			name:     "FAIL verdict",
			content:  "## Council Verdict: FAIL\n\nCritical issues.",
			expected: "FAIL",
		},
		{
			name:    "no verdict",
			content: "# Report\n\nNo verdict line here.",
			wantErr: true,
		},
		{
			name:    "empty file",
			content: "",
			wantErr: true,
		},
		{
			name:     "verdict with extra whitespace",
			content:  "## Council Verdict:  PASS \n",
			expected: "PASS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "report.md")
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			verdict, err := extractCouncilVerdict(path)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got verdict %q", verdict)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if verdict != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, verdict)
			}
		})
	}
}

func TestStartPhasedDashboardIfNeededHonorsNoDashboard(t *testing.T) {
	var out strings.Builder
	opts := phasedEngineOptions{NoDashboard: true, StdoutWriter: &out}
	state := &phasedState{RunID: "run-test"}

	if srv := startPhasedDashboardIfNeeded(t.TempDir(), state, &opts); srv != nil {
		t.Fatalf("startPhasedDashboardIfNeeded returned server with NoDashboard=true")
	}
	if opts.StdoutWriter != &out {
		t.Fatalf("StdoutWriter changed with NoDashboard=true")
	}
}

func TestExtractCouncilFindings(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		max      int
		expected int
	}{
		{
			name:     "structured findings",
			content:  "FINDING: Missing auth | FIX: Add middleware | REF: auth.go:10\nFINDING: No tests | FIX: Add tests | REF: auth_test.go",
			max:      5,
			expected: 2,
		},
		{
			name:     "max limit applied",
			content:  "FINDING: A | FIX: B | REF: C\nFINDING: D | FIX: E | REF: F\nFINDING: G | FIX: H | REF: I",
			max:      2,
			expected: 2,
		},
		{
			name:     "fallback to markdown findings",
			content:  "## Shared Findings\n\n1. **Missing auth** — No middleware\n2. **No tests** — Zero coverage",
			max:      5,
			expected: 2,
		},
		{
			name:     "no findings",
			content:  "# Empty report",
			max:      5,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "report.md")
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			findings, err := extractCouncilFindings(path, tt.max)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(findings) != tt.expected {
				t.Errorf("expected %d findings, got %d", tt.expected, len(findings))
			}
		})
	}
}

func TestBuildPromptForPhase(t *testing.T) {
	tests := []struct {
		name     string
		phase    int
		state    *phasedState
		contains string
	}{
		{
			name:     "discovery phase contains research",
			phase:    1,
			state:    &phasedState{Goal: "add auth"},
			contains: `/research "add auth" --auto`,
		},
		{
			name:     "discovery phase contains plan",
			phase:    1,
			state:    &phasedState{Goal: "add auth"},
			contains: `/plan "add auth" --auto`,
		},
		{
			name:     "discovery phase contains pre-mortem",
			phase:    1,
			state:    &phasedState{Goal: "add auth"},
			contains: "/pre-mortem",
		},
		{
			name:     "discovery fast path",
			phase:    1,
			state:    &phasedState{Goal: "add auth", FastPath: true},
			contains: "--quick",
		},
		{
			name:     "implementation with epic",
			phase:    2,
			state:    &phasedState{EpicID: "ag-5k2"},
			contains: "/crank ag-5k2",
		},
		{
			name:     "implementation with test-first",
			phase:    2,
			state:    &phasedState{EpicID: "ag-5k2", TestFirst: true},
			contains: "--test-first",
		},
		{
			name:     "validation contains vibe",
			phase:    3,
			state:    &phasedState{EpicID: "ag-5k2"},
			contains: "/vibe",
		},
		{
			name:     "validation contains post-mortem",
			phase:    3,
			state:    &phasedState{EpicID: "ag-5k2"},
			contains: "/post-mortem",
		},
		{
			name:     "validation fast path vibe",
			phase:    3,
			state:    &phasedState{EpicID: "ag-5k2", FastPath: true},
			contains: "/vibe --quick recent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt, err := buildPromptForPhase("", tt.phase, tt.state, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !containsStr(prompt, tt.contains) {
				t.Errorf("prompt %q does not contain %q", prompt, tt.contains)
			}
		})
	}
}

func TestBuildPromptForPhase_TestFirstOptOut(t *testing.T) {
	state := &phasedState{EpicID: "ag-5k2", TestFirst: false}
	prompt, err := buildPromptForPhase("", 2, state, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if containsStr(prompt, "--test-first") {
		t.Errorf("implementation prompt should omit --test-first when opted out, got: %q", prompt)
	}
}

func TestBuildPromptForPhase_Retry(t *testing.T) {
	state := &phasedState{Goal: "add auth", EpicID: "ag-5k2", TestFirst: true}
	retryCtx := &retryContext{
		Attempt: 2,
		Findings: []finding{
			{Description: "Missing error handling", Fix: "Add try-catch", Ref: "auth.go:42"},
		},
		Verdict: "FAIL",
	}

	// Vibe retry (phase 3) → re-crank
	prompt, err := buildRetryPrompt("", 3, state, retryCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsStr(prompt, "/crank") {
		t.Errorf("vibe retry should invoke /crank, got: %q", prompt)
	}
	if !containsStr(prompt, "--test-first") {
		t.Errorf("vibe retry should preserve strict-quality default via --test-first, got: %q", prompt)
	}
	if !containsStr(prompt, "Missing error handling") {
		t.Errorf("retry prompt should contain finding description, got: %q", prompt)
	}

	// Phase 1 has no retry template (retries happen within the session)
	// buildRetryPrompt should fall back to normal prompt
	prompt, err = buildRetryPrompt("", 1, state, retryCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsStr(prompt, "/research") {
		t.Errorf("phase 1 retry should fall back to normal prompt, got: %q", prompt)
	}
}

func TestBuildRetryPrompt_TestFirstOptOut(t *testing.T) {
	state := &phasedState{Goal: "add auth", EpicID: "ag-5k2", TestFirst: false}
	retryCtx := &retryContext{
		Attempt: 1,
		Findings: []finding{
			{Description: "Missing error handling", Fix: "Add try-catch", Ref: "auth.go:42"},
		},
		Verdict: "FAIL",
	}

	prompt, err := buildRetryPrompt("", 3, state, retryCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if containsStr(prompt, "--test-first") {
		t.Errorf("retry prompt should omit --test-first when opted out, got: %q", prompt)
	}
}

func TestDefaultPhasedEngineOptions_StrictQualityDefaultOn(t *testing.T) {
	opts := defaultPhasedEngineOptions()
	if !opts.TestFirst {
		t.Fatal("default phased engine options should enable test-first strict-quality mode")
	}
}

func TestBuildRetryPrompt_ContextDiscipline(t *testing.T) {
	state := &phasedState{Goal: "add auth", EpicID: "ag-5k2"}
	retryCtx := &retryContext{
		Attempt: 1,
		Findings: []finding{
			{Description: "Missing error handling", Fix: "Add try-catch", Ref: "auth.go:42"},
		},
		Verdict: "FAIL",
	}

	// Phase 3 has a retry template (/crank re-invocation)
	prompt, err := buildRetryPrompt("", 3, state, retryCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsStr(prompt, "CONTEXT DISCIPLINE") {
		t.Errorf("retry prompt should contain CONTEXT DISCIPLINE, got: %q", prompt)
	}
	if !containsStr(prompt, "PHASE SUMMARY CONTRACT") {
		t.Errorf("retry prompt should contain PHASE SUMMARY CONTRACT, got: %q", prompt)
	}
	if !containsStr(prompt, "phase 3 of 3") {
		t.Errorf("retry prompt should contain 'phase 3 of 3' (PhaseNum rendered), got: %q", prompt)
	}
	if !containsStr(prompt, "/crank") {
		t.Errorf("retry prompt should contain /crank (retry template rendered), got: %q", prompt)
	}
}

func TestPhasedState_SaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	original := &phasedState{
		Goal:      "test goal",
		EpicID:    "ag-test",
		Phase:     3,
		Cycle:     1,
		FastPath:  true,
		TestFirst: false,
		Verdicts:  map[string]string{"pre_mortem": "PASS"},
		Attempts:  map[string]int{"phase_3": 1},
		StartedAt: "2026-02-14T12:00:00Z",
	}

	if err := savePhasedState(tmpDir, original); err != nil {
		t.Fatalf("save error: %v", err)
	}

	loaded, err := loadPhasedState(tmpDir)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if loaded.Goal != original.Goal {
		t.Errorf("goal: got %q, want %q", loaded.Goal, original.Goal)
	}
	if loaded.EpicID != original.EpicID {
		t.Errorf("epic_id: got %q, want %q", loaded.EpicID, original.EpicID)
	}
	if loaded.Phase != original.Phase {
		t.Errorf("phase: got %d, want %d", loaded.Phase, original.Phase)
	}
	if loaded.FastPath != original.FastPath {
		t.Errorf("fast_path: got %v, want %v", loaded.FastPath, original.FastPath)
	}
	if loaded.Verdicts["pre_mortem"] != "PASS" {
		t.Errorf("verdicts: got %v, want pre_mortem=PASS", loaded.Verdicts)
	}

	// Verify JSON round-trip
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal original: %v", err)
	}
	var roundTrip phasedState
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("round-trip unmarshal: %v", err)
	}
	if roundTrip.Goal != original.Goal {
		t.Errorf("round-trip goal mismatch")
	}
}

func TestPhaseNameToNum(t *testing.T) {
	tests := []struct {
		name     string
		expected int
	}{
		// Canonical 3-phase names
		{"discovery", 1},
		{"implementation", 2},
		{"validation", 3},
		// Backward-compatible aliases
		{"research", 1},
		{"plan", 1},
		{"pre-mortem", 1},
		{"premortem", 1},
		{"pre_mortem", 1},
		{"crank", 2},
		{"implement", 2},
		{"vibe", 3},
		{"validate", 3},
		{"post-mortem", 3},
		{"postmortem", 3},
		{"post_mortem", 3},
		// Unknown
		{"unknown", 0},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := phaseNameToNum(tt.name)
			if got != tt.expected {
				t.Errorf("phaseNameToNum(%q) = %d, want %d", tt.name, got, tt.expected)
			}
		})
	}
}

func TestFindLatestCouncilReport(t *testing.T) {
	tmpDir := t.TempDir()
	councilDir := filepath.Join(tmpDir, ".agents", "council")
	if err := os.MkdirAll(councilDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create two reports with different timestamps
	if err := os.WriteFile(filepath.Join(councilDir, "2026-02-13-pre-mortem-auth.md"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(councilDir, "2026-02-14-pre-mortem-auth.md"), []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	// Unrelated report
	if err := os.WriteFile(filepath.Join(councilDir, "2026-02-14-vibe-recent.md"), []byte("vibe"), 0644); err != nil {
		t.Fatal(err)
	}

	// Should find the latest pre-mortem report
	report, err := findLatestCouncilReport(tmpDir, "pre-mortem", time.Time{}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsStr(report, "2026-02-14-pre-mortem") {
		t.Errorf("expected latest report, got: %s", report)
	}

	// Should find vibe report
	report, err = findLatestCouncilReport(tmpDir, "vibe", time.Time{}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsStr(report, "vibe-recent") {
		t.Errorf("expected vibe report, got: %s", report)
	}

	// Should error on missing pattern
	_, err = findLatestCouncilReport(tmpDir, "nonexistent", time.Time{}, "")
	if err == nil {
		t.Error("expected error for missing pattern")
	}

	// notBefore filter: only return files modified after the cutoff
	t.Run("notBefore filters older files", func(t *testing.T) {
		dir := t.TempDir()
		cDir := filepath.Join(dir, ".agents", "council")
		if err := os.MkdirAll(cDir, 0755); err != nil {
			t.Fatal(err)
		}

		oldFile := filepath.Join(cDir, "2026-02-10-pre-mortem-old.md")
		newFile := filepath.Join(cDir, "2026-02-14-pre-mortem-new.md")
		if err := os.WriteFile(oldFile, []byte("old"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(newFile, []byte("new"), 0644); err != nil {
			t.Fatal(err)
		}

		// Set old file mtime to the past
		oldTime := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)
		if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}

		// Set new file mtime to recent
		newTime := time.Date(2026, 2, 14, 12, 0, 0, 0, time.UTC)
		if err := os.Chtimes(newFile, newTime, newTime); err != nil {
			t.Fatal(err)
		}

		// notBefore between old and new
		cutoff := time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC)
		report, err := findLatestCouncilReport(dir, "pre-mortem", cutoff, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !containsStr(report, "2026-02-14-pre-mortem-new") {
			t.Errorf("expected new report, got: %s", report)
		}

		// notBefore after both files should return error
		future := time.Date(2026, 2, 20, 0, 0, 0, 0, time.UTC)
		_, err = findLatestCouncilReport(dir, "pre-mortem", future, "")
		if err == nil {
			t.Error("expected error when all files are before notBefore")
		}
	})
}

func TestFindLatestCouncilReport_EpicScoped(t *testing.T) {
	tmpDir := t.TempDir()
	councilDir := filepath.Join(tmpDir, ".agents", "council")
	if err := os.MkdirAll(councilDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create two reports: one with epicID in the name (older timestamp),
	// one without (newer timestamp).
	epicReport := filepath.Join(councilDir, "2026-02-13-pre-mortem-ag-abc1.md")
	genericReport := filepath.Join(councilDir, "2026-02-14-pre-mortem-other.md")

	if err := os.WriteFile(epicReport, []byte("## Council Verdict: FAIL\nepic-scoped"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(genericReport, []byte("## Council Verdict: PASS\ngeneric"), 0644); err != nil {
		t.Fatal(err)
	}

	// With epicID provided, should select the epic-scoped report even though
	// the generic one sorts later (newer date in filename).
	report, err := findLatestCouncilReport(tmpDir, "pre-mortem", time.Time{}, "ag-abc1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsStr(report, "ag-abc1") {
		t.Errorf("expected epic-scoped report, got: %s", report)
	}

	// With empty epicID, should fall back to the latest overall (generic).
	report, err = findLatestCouncilReport(tmpDir, "pre-mortem", time.Time{}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsStr(report, "2026-02-14-pre-mortem-other") {
		t.Errorf("expected latest generic report, got: %s", report)
	}

	// With a non-matching epicID, should fall back to all matches.
	report, err = findLatestCouncilReport(tmpDir, "pre-mortem", time.Time{}, "ag-zzz9")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsStr(report, "2026-02-14-pre-mortem-other") {
		t.Errorf("expected fallback to latest report, got: %s", report)
	}
}

func TestParseFastPath(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected bool
	}{
		{"empty output (no issues)", "", true},
		{"one issue no blockers", "ag-001  open  Fix login bug", true},
		{"two issues no blockers", "ag-001  open  Fix login bug\nag-002  open  Add tests", true},
		{"three issues", "ag-001  open  Fix login\nag-002  open  Add tests\nag-003  open  Refactor", false},
		{"one blocked issue", "ag-001  blocked  Fix login bug", false},
		{"two issues one blocked", "ag-001  open  Fix login\nag-002  blocked  Add tests", false},
		{"whitespace only lines", "  \n  \n", true},
		{"mixed with empty lines", "ag-001  open  Fix login\n\nag-002  open  Add tests\n", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFastPath(tt.output)
			if got != tt.expected {
				t.Errorf("parseFastPath(%q) = %v, want %v", tt.output, got, tt.expected)
			}
		})
	}
}

func TestParseLatestEpicIDFromJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "ag prefix",
			input: `[{"id":"ag-a1"},{"id":"ag-b2"}]`,
			want:  "ag-b2",
		},
		{
			name:  "custom prefix",
			input: `[{"id":"bd-10"},{"id":"bd-11"}]`,
			want:  "bd-11",
		},
		{
			name:  "single task (non-epic fallback scenario)",
			input: `[{"id":"jo-qqq"}]`,
			want:  "jo-qqq",
		},
		{
			name:  "mixed types returns last",
			input: `[{"id":"ag-a1"},{"id":"jo-qqq"},{"id":"test-85b"}]`,
			want:  "test-85b",
		},
		{
			name:    "empty list",
			input:   `[]`,
			wantErr: true,
		},
		{
			name:    "malformed json",
			input:   `{`,
			wantErr: true,
		},
		{
			name:    "missing id",
			input:   `[{"title":"x"}]`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLatestEpicIDFromJSON([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got id=%q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestParseLatestEpicIDFromText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name: "unicode bullets",
			input: strings.Join([]string{
				"○ ag-4p9 [● P2] [epic] - Existing epic",
				"○ bd-17 [● P2] [epic] - Runtime-focused follow-up",
			}, "\n"),
			want: "bd-17",
		},
		{
			name:  "plain output",
			input: "ag-1 open first\nag-2 open second",
			want:  "ag-2",
		},
		{
			name:    "no ids",
			input:   "no epic rows here",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLatestEpicIDFromText(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got id=%q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestParseCrankCompletion(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{"empty output", "", "DONE"},
		{"all closed", "ag-001  closed  Fix login\nag-002  ✓  Add tests", "DONE"},
		{"one blocked", "ag-001  closed  Fix login\nag-002  blocked  Add tests", "BLOCKED"},
		{"partial", "ag-001  closed  Fix login\nag-002  open  Add tests", "PARTIAL"},
		{"all open", "ag-001  open  Fix login\nag-002  open  Add tests", "PARTIAL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCrankCompletion(tt.output)
			if got != tt.expected {
				t.Errorf("parseCrankCompletion(%q) = %q, want %q", tt.output, got, tt.expected)
			}
		})
	}
}

func TestPhasedState_SchemaVersion(t *testing.T) {
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	state := newTestPhasedState()

	if err := savePhasedState(tmpDir, state); err != nil {
		t.Fatalf("save error: %v", err)
	}

	// Verify JSON contains schema_version
	data, err := os.ReadFile(filepath.Join(stateDir, phasedStateFile))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if v, ok := raw["schema_version"]; !ok {
		t.Error("schema_version missing from JSON")
	} else if v.(float64) != 1 {
		t.Errorf("schema_version = %v, want 1", v)
	}

	loaded, err := loadPhasedState(tmpDir)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if loaded.SchemaVersion != 1 {
		t.Errorf("loaded SchemaVersion = %d, want 1", loaded.SchemaVersion)
	}
}

func TestBuildPromptForPhase_Interactive(t *testing.T) {
	// Default (non-interactive) — should have --auto
	state := newTestPhasedState().WithGoal("add auth").WithOpts(phasedEngineOptions{Interactive: false})
	prompt, err := buildPromptForPhase("", 1, state, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !containsStr(prompt, "--auto") {
		t.Errorf("non-interactive discovery prompt should contain --auto, got: %q", prompt)
	}

	// Interactive — should NOT have --auto
	state.Opts.Interactive = true
	prompt, err = buildPromptForPhase("", 1, state, nil)
	if err != nil {
		t.Fatal(err)
	}
	if containsStr(prompt, "--auto") {
		t.Errorf("interactive discovery prompt should not contain --auto, got: %q", prompt)
	}
}

func TestBuildPhaseContext(t *testing.T) {
	// With goal and verdicts
	state := newTestPhasedState().WithGoal("add user authentication").WithEpicID("ag-5k2")
	state.Verdicts["pre_mortem"] = "WARN"

	ctx := buildPhaseContext("", state, 2)
	if !containsStr(ctx, "Goal: add user authentication") {
		t.Errorf("context should contain goal, got: %q", ctx)
	}
	if !containsStr(ctx, "pre-mortem verdict: WARN") {
		t.Errorf("context should contain verdict, got: %q", ctx)
	}
	if !containsStr(ctx, "RPI Context") {
		t.Errorf("context should have header, got: %q", ctx)
	}

	// Phase 1 with empty state — no context needed
	emptyState := &phasedState{Verdicts: make(map[string]string)}
	ctx = buildPhaseContext("", emptyState, 1)
	if ctx != "" {
		t.Errorf("empty state should produce empty context, got: %q", ctx)
	}
}

func TestBuildPromptForPhase_WithContext(t *testing.T) {
	state := newTestPhasedState().WithGoal("add auth").WithEpicID("ag-5k2")
	state.Verdicts["pre_mortem"] = "PASS"

	// Phase 2 (implementation) should include context and summary instruction
	prompt, err := buildPromptForPhase("", 2, state, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !containsStr(prompt, "/crank ag-5k2") {
		t.Errorf("implementation prompt missing command, got: %q", prompt)
	}
	if !containsStr(prompt, "Goal: add auth") {
		t.Errorf("implementation prompt missing goal context, got: %q", prompt)
	}
	if !containsStr(prompt, "pre-mortem verdict: PASS") {
		t.Errorf("implementation prompt missing verdict context, got: %q", prompt)
	}
	if !containsStr(prompt, "phase-2-summary.md") {
		t.Errorf("implementation prompt missing summary instruction, got: %q", prompt)
	}

	// Phase 1 (discovery) should NOT include cross-phase context but SHOULD have summary instruction
	prompt, err = buildPromptForPhase("", 1, state, nil)
	if err != nil {
		t.Fatal(err)
	}
	if containsStr(prompt, "RPI Context") {
		t.Errorf("discovery prompt should not have context block, got: %q", prompt)
	}
	if !containsStr(prompt, "phase-1-summary.md") {
		t.Errorf("discovery prompt should have summary instruction, got: %q", prompt)
	}
}

func TestGeneratePhaseSummary(t *testing.T) {
	state := newTestPhasedState().WithGoal("add auth").WithEpicID("ag-5k2").WithFastPath(true)
	state.Verdicts["pre_mortem"] = "WARN"
	state.Verdicts["vibe"] = "PASS"

	// Phase 1: discovery (research + plan + pre-mortem)
	s := generatePhaseSummary(state, 1)
	if !containsStr(s, "add auth") {
		t.Errorf("discovery summary missing goal, got: %q", s)
	}
	if !containsStr(s, "ag-5k2") {
		t.Errorf("discovery summary missing epic, got: %q", s)
	}
	if !containsStr(s, "WARN") {
		t.Errorf("discovery summary missing pre-mortem verdict, got: %q", s)
	}
	if !containsStr(s, "fast path") {
		t.Errorf("discovery summary missing fast path, got: %q", s)
	}

	// Phase 2: implementation (crank)
	s = generatePhaseSummary(state, 2)
	if !containsStr(s, "ag-5k2") {
		t.Errorf("implementation summary missing epic, got: %q", s)
	}

	// Phase 3: validation (vibe + post-mortem)
	s = generatePhaseSummary(state, 3)
	if !containsStr(s, "PASS") {
		t.Errorf("validation summary missing vibe verdict, got: %q", s)
	}
}

func TestReadPhaseSummaries(t *testing.T) {
	tmpDir := t.TempDir()
	rpiDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write summaries for phases 1 and 2
	if err := os.WriteFile(filepath.Join(rpiDir, "phase-1-summary.md"), []byte("Discovery found X and Y"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rpiDir, "phase-2-summary.md"), []byte("Crank completed epic ag-test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Reading for phase 3 should get both
	result := readPhaseSummaries(tmpDir, 3)
	if !containsStr(result, "Discovery found X and Y") {
		t.Errorf("should include phase 1 summary, got: %q", result)
	}
	if !containsStr(result, "Crank completed epic ag-test") {
		t.Errorf("should include phase 2 summary, got: %q", result)
	}

	// Reading for phase 1 should get nothing (no prior phases)
	result = readPhaseSummaries(tmpDir, 1)
	if result != "" {
		t.Errorf("phase 1 should have no prior summaries, got: %q", result)
	}

	// Reading for phase 2 should get only phase 1
	result = readPhaseSummaries(tmpDir, 2)
	if !containsStr(result, "Discovery found X and Y") {
		t.Errorf("should include phase 1 summary, got: %q", result)
	}
	if containsStr(result, "Crank completed") {
		t.Errorf("should NOT include phase 2 summary, got: %q", result)
	}
}

func TestWritePhaseSummary(t *testing.T) {
	tmpDir := t.TempDir()
	state := newTestPhasedState().WithGoal("add auth").WithEpicID("ag-5k2")
	state.Verdicts["pre_mortem"] = "PASS"

	// Fallback: no existing summary → writes mechanical one
	writePhaseSummary(tmpDir, state, 1)

	path := filepath.Join(tmpDir, ".agents", "rpi", "phase-1-summary.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("summary file not written: %v", err)
	}
	if !containsStr(string(data), "PASS") {
		t.Errorf("summary should contain verdict, got: %q", string(data))
	}

	// Claude-written summary exists → don't overwrite
	richSummary := "Discovery found JWT is best approach because stateless and fits API."
	if err := os.WriteFile(path, []byte(richSummary), 0644); err != nil {
		t.Fatal(err)
	}
	writePhaseSummary(tmpDir, state, 1) // should not overwrite
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != richSummary {
		t.Errorf("should not overwrite Claude summary, got: %q", string(data))
	}
}

func TestCleanPhaseSummaries(t *testing.T) {
	tmpDir := t.TempDir()
	rpiDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create summaries
	for i := 1; i <= 3; i++ {
		path := filepath.Join(rpiDir, fmt.Sprintf("phase-%d-summary.md", i))
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cleanPhaseSummaries(rpiDir)

	for i := 1; i <= 6; i++ {
		path := filepath.Join(rpiDir, fmt.Sprintf("phase-%d-summary.md", i))
		if _, err := os.Stat(path); err == nil {
			t.Errorf("phase-%d-summary.md should be deleted", i)
		}
	}
}

func TestContextDisciplineInPrompt(t *testing.T) {
	state := newTestPhasedState().WithEpicID("ag-test")

	// Every phase (1-3) should contain context discipline
	for phaseNum := 1; phaseNum <= 3; phaseNum++ {
		prompt, err := buildPromptForPhase("", phaseNum, state, nil)
		if err != nil {
			t.Fatalf("phase %d: unexpected error: %v", phaseNum, err)
		}
		if !containsStr(prompt, "CONTEXT DISCIPLINE") {
			t.Errorf("phase %d: prompt should contain CONTEXT DISCIPLINE", phaseNum)
		}
		if !containsStr(prompt, "PHASE SUMMARY CONTRACT") {
			t.Errorf("phase %d: prompt should contain PHASE SUMMARY CONTRACT", phaseNum)
		}
		if !containsStr(prompt, "handoff") {
			t.Errorf("phase %d: prompt should mention handoff", phaseNum)
		}
		if !containsStr(prompt, "BUDGET") {
			t.Errorf("phase %d: prompt should contain BUDGET guidance", phaseNum)
		}
	}
}

func TestContextDiscipline_PhaseSpecificBudgets(t *testing.T) {
	// Verify each phase has a specific budget
	for phaseNum := 1; phaseNum <= 3; phaseNum++ {
		budget, ok := phaseContextBudgets[phaseNum]
		if !ok {
			t.Errorf("phase %d: no context budget defined", phaseNum)
		}
		if budget == "" {
			t.Errorf("phase %d: context budget is empty", phaseNum)
		}
	}

	// Phase 2 (implementation/crank) should have CRITICAL warning
	if !containsStr(phaseContextBudgets[2], "CRITICAL") {
		t.Error("phase 2 budget should contain CRITICAL warning")
	}
}

func TestContextDiscipline_PromptOrdering(t *testing.T) {
	state := newTestPhasedState().WithEpicID("ag-test")
	state.Verdicts["pre_mortem"] = "PASS"

	// Phase 2: check that discipline comes before skill invocation
	prompt, err := buildPromptForPhase("", 2, state, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	disciplineIdx := strings.Index(prompt, "CONTEXT DISCIPLINE")
	summaryIdx := strings.Index(prompt, "PHASE SUMMARY CONTRACT")
	// Use LastIndex for /crank since budget text also mentions it
	crankIdx := strings.LastIndex(prompt, "/crank")

	if disciplineIdx < 0 {
		t.Fatal("CONTEXT DISCIPLINE not found in prompt")
	}
	if summaryIdx < 0 {
		t.Fatal("PHASE SUMMARY CONTRACT not found in prompt")
	}
	if crankIdx < 0 {
		t.Fatal("/crank not found in prompt")
	}

	// Discipline should come first, then summary, then skill invocation (last /crank)
	if disciplineIdx >= summaryIdx {
		t.Errorf("discipline (%d) should come before summary (%d)", disciplineIdx, summaryIdx)
	}
	if summaryIdx >= crankIdx {
		t.Errorf("summary (%d) should come before skill invocation (%d)", summaryIdx, crankIdx)
	}
}

func TestHandoffDetection(t *testing.T) {
	tmpDir := t.TempDir()
	rpiDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0755); err != nil {
		t.Fatal(err)
	}

	// No handoff file → not detected
	if handoffDetected(tmpDir, 2) {
		t.Error("should not detect handoff when file doesn't exist")
	}

	// Write handoff file → detected
	handoffPath := filepath.Join(rpiDir, "phase-2-handoff.json")
	if err := os.WriteFile(handoffPath, []byte("# Handoff\nContext degraded."), 0644); err != nil {
		t.Fatal(err)
	}

	if !handoffDetected(tmpDir, 2) {
		t.Error("should detect handoff when file exists")
	}

	// Different phase → not detected
	if handoffDetected(tmpDir, 1) {
		t.Error("should not detect handoff for different phase")
	}
}

func TestCleanPhaseSummaries_AlsoRemovesHandoffs(t *testing.T) {
	tmpDir := t.TempDir()
	rpiDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create summaries, .md handoffs, and .json handoffs
	for i := 1; i <= 3; i++ {
		summaryPath := filepath.Join(rpiDir, fmt.Sprintf("phase-%d-summary.md", i))
		if err := os.WriteFile(summaryPath, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
		handoffPath := filepath.Join(rpiDir, fmt.Sprintf("phase-%d-handoff.md", i))
		if err := os.WriteFile(handoffPath, []byte("handoff"), 0644); err != nil {
			t.Fatal(err)
		}
		jsonHandoffPath := filepath.Join(rpiDir, fmt.Sprintf("phase-%d-handoff.json", i))
		if err := os.WriteFile(jsonHandoffPath, []byte(`{"phase":`+fmt.Sprintf("%d", i)+`}`), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cleanPhaseSummaries(rpiDir)

	for i := 1; i <= 6; i++ {
		summaryPath := filepath.Join(rpiDir, fmt.Sprintf("phase-%d-summary.md", i))
		if _, err := os.Stat(summaryPath); err == nil {
			t.Errorf("phase-%d-summary.md should be deleted", i)
		}
		handoffPath := filepath.Join(rpiDir, fmt.Sprintf("phase-%d-handoff.md", i))
		if _, err := os.Stat(handoffPath); err == nil {
			t.Errorf("phase-%d-handoff.md should be deleted", i)
		}
		jsonHandoffPath := filepath.Join(rpiDir, fmt.Sprintf("phase-%d-handoff.json", i))
		if _, err := os.Stat(jsonHandoffPath); err == nil {
			t.Errorf("phase-%d-handoff.json should be deleted", i)
		}
	}
}

func TestPromptBudgetEstimate(t *testing.T) {
	state := newTestPhasedState().WithGoal("test goal with a reasonable description of what needs to happen").WithEpicID("ag-test")
	state.Verdicts["pre_mortem"] = "PASS"
	state.Verdicts["vibe"] = "WARN"

	// Every phase prompt should stay under 5000 chars (without summaries on disk)
	for phaseNum := 1; phaseNum <= 3; phaseNum++ {
		prompt, err := buildPromptForPhase("", phaseNum, state, nil)
		if err != nil {
			t.Fatalf("phase %d: unexpected error: %v", phaseNum, err)
		}
		if len(prompt) > 5000 {
			t.Errorf("phase %d: prompt is %d chars (max 5000 without summaries)", phaseNum, len(prompt))
		}
	}
}

func TestParsePhaseBudgetSpec(t *testing.T) {
	budgets, err := parsePhaseBudgetSpec("research:180,plan:120,validation:60")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := budgets[1]; got != 300*time.Second {
		t.Fatalf("discovery budget = %s, want %s", got, 300*time.Second)
	}
	if got := budgets[3]; got != 60*time.Second {
		t.Fatalf("validation budget = %s, want %s", got, 60*time.Second)
	}
}

func TestParsePhaseBudgetSpec_Invalid(t *testing.T) {
	_, err := parsePhaseBudgetSpec("unknown:30")
	if err == nil {
		t.Fatal("expected unknown phase parse error")
	}

	_, err = parsePhaseBudgetSpec("discovery:0")
	if err == nil {
		t.Fatal("expected non-positive seconds parse error")
	}
}

func TestResolvePhaseBudget_DefaultsAndOverrides(t *testing.T) {
	state := newTestPhasedState().WithGoal("add auth")
	state.Complexity = ComplexityStandard

	discoveryBudget, hasBudget, err := resolvePhaseBudget(state, 1)
	if err != nil {
		t.Fatalf("resolve discovery budget: %v", err)
	}
	if !hasBudget {
		t.Fatal("expected default discovery budget to be enabled")
	}
	if discoveryBudget != 13*time.Minute {
		t.Fatalf("discovery budget = %s, want %s", discoveryBudget, 13*time.Minute)
	}

	implementationBudget, hasBudget, err := resolvePhaseBudget(state, 2)
	if err != nil {
		t.Fatalf("resolve implementation budget: %v", err)
	}
	if hasBudget {
		t.Fatalf("implementation should be unbounded, got budget %s", implementationBudget)
	}

	validationBudget, hasBudget, err := resolvePhaseBudget(state, 3)
	if err != nil {
		t.Fatalf("resolve validation budget: %v", err)
	}
	if !hasBudget || validationBudget != 5*time.Minute {
		t.Fatalf("validation budget = %s (enabled=%v), want %s enabled", validationBudget, hasBudget, 5*time.Minute)
	}

	state.Opts.NoBudget = true
	_, hasBudget, err = resolvePhaseBudget(state, 1)
	if err != nil {
		t.Fatalf("resolve no-budget override: %v", err)
	}
	if hasBudget {
		t.Fatal("--no-budget should disable all phase budgets")
	}

	state.Opts.NoBudget = false
	state.Opts.BudgetSpec = "discovery:42"
	overrideBudget, hasBudget, err := resolvePhaseBudget(state, 1)
	if err != nil {
		t.Fatalf("resolve custom budget override: %v", err)
	}
	if !hasBudget || overrideBudget != 42*time.Second {
		t.Fatalf("override budget = %s (enabled=%v), want %s enabled", overrideBudget, hasBudget, 42*time.Second)
	}
}

func TestRunSinglePhaseBudgetTimeout_WritesTimeBoxedMarker(t *testing.T) {
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(stateDir, "phased-orchestration.log")

	state := newTestPhasedState().WithRunID("budget-timeout-run")
	p := phase{Num: 1, Name: "discovery"}
	budget := 30 * time.Second

	phaseStart := time.Now().Add(-budget) // simulate a phase that ran for the budget duration
	if !handleBudgetTimeout(tmpDir, state, p, budget, logPath, phaseStart) {
		t.Fatal("expected timeout handler to continue after writing time-box marker")
	}

	summaryPath := filepath.Join(stateDir, "phase-1-summary.md")
	summaryData, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("read phase summary marker: %v", err)
	}
	summary := string(summaryData)
	if !containsStr(summary, "[TIME-BOXED]") {
		t.Fatalf("summary missing [TIME-BOXED] marker: %q", summary)
	}
	if !containsStr(summary, "budget: 30s") {
		t.Fatalf("summary missing budget metadata: %q", summary)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read orchestration log: %v", err)
	}
	logLine := string(logData)
	if !containsStr(logLine, "[TIME-BOXED]") || !containsStr(logLine, "time-boxed") {
		t.Fatalf("log missing time-box transition marker: %q", logLine)
	}
}

// TestRatchetPhasedStepMapping verifies that each phase's Step field maps to a
// valid, queryable ratchet step name.  This catches the bug where "discovery"
// or "validation" (not recognised by ParseStep) were used as ratchet steps,
// causing recordRatchetCheckpoint to silently fail.
func TestRatchetPhasedStepMapping(t *testing.T) {
	// Canonical mapping: phase name → expected canonical ratchet step
	want := map[string]ratchet.Step{
		"discovery":      ratchet.StepResearch,
		"implementation": ratchet.StepImplement,
		"validation":     ratchet.StepVibe,
	}

	for _, p := range phases {
		t.Run(p.Name, func(t *testing.T) {
			// The Step field must be a valid ratchet step name.
			parsed := ratchet.ParseStep(p.Step)
			if parsed == "" {
				t.Errorf("phase %q has Step=%q which is not a valid ratchet step name", p.Name, p.Step)
			}
			// Verify it maps to the expected canonical step.
			if expected, ok := want[p.Name]; ok {
				if parsed != expected {
					t.Errorf("phase %q: Step=%q parsed to %q, want %q", p.Name, p.Step, parsed, expected)
				}
			}
		})
	}
}

// TestRatchetPhasedAliases verifies that the phase-canonical names
// ("discovery", "validation") are accepted as ratchet step aliases so that
// ao ratchet record / skip commands can use them directly.
func TestRatchetPhasedAliases(t *testing.T) {
	tests := []struct {
		alias    string
		wantStep ratchet.Step
	}{
		{"discovery", ratchet.StepResearch},
		{"validation", ratchet.StepVibe},
		// Ensure existing canonical names still work
		{"research", ratchet.StepResearch},
		{"implement", ratchet.StepImplement},
		{"validate", ratchet.StepVibe},
	}

	for _, tt := range tests {
		t.Run(tt.alias, func(t *testing.T) {
			got := ratchet.ParseStep(tt.alias)
			if got == "" {
				t.Errorf("ParseStep(%q) returned empty — alias not registered", tt.alias)
			}
			if got != tt.wantStep {
				t.Errorf("ParseStep(%q) = %q, want %q", tt.alias, got, tt.wantStep)
			}
		})
	}
}

func TestPostPhaseProcessing_Discovery(t *testing.T) {
	tmpDir := t.TempDir()
	councilDir := filepath.Join(tmpDir, ".agents", "council")
	if err := os.MkdirAll(councilDir, 0755); err != nil {
		t.Fatal(err)
	}

	reportPath := filepath.Join(councilDir, "2026-02-19-ag-new-pre-mortem.md")
	report := "# Pre-mortem\n\n## Council Verdict: PASS\n"
	if err := os.WriteFile(reportPath, []byte(report), 0644); err != nil {
		t.Fatal(err)
	}

	fakeBin := t.TempDir()
	writeFakeBDScript(t, fakeBin)
	t.Setenv("PATH", fakeBin+":"+os.Getenv("PATH"))

	state := newTestPhasedState().WithGoal("add auth").WithRunID("run-discovery")

	if err := postPhaseProcessing(tmpDir, state, 1, filepath.Join(tmpDir, "orchestration.log")); err != nil {
		t.Fatalf("postPhaseProcessing(discovery): %v", err)
	}
	if state.EpicID != "ag-new" {
		t.Fatalf("expected extracted epic ag-new, got %q", state.EpicID)
	}
	if got := state.Verdicts["pre_mortem"]; got != "PASS" {
		t.Fatalf("expected pre_mortem verdict PASS, got %q", got)
	}
}

func TestPostPhaseProcessing_Implementation(t *testing.T) {
	tmpDir := t.TempDir()
	rpiDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rpiDir, "phase-1-result.json"), []byte(`{"status":"completed"}`), 0644); err != nil {
		t.Fatal(err)
	}

	fakeBin := t.TempDir()
	writeFakeBDScript(t, fakeBin)
	t.Setenv("PATH", fakeBin+":"+os.Getenv("PATH"))

	state := newTestPhasedState().WithEpicID("ag-new").WithRunID("run-implementation")

	if err := postPhaseProcessing(tmpDir, state, 2, filepath.Join(tmpDir, "orchestration.log")); err != nil {
		t.Fatalf("postPhaseProcessing(implementation): %v", err)
	}
}

func TestPostPhaseProcessing_Validation(t *testing.T) {
	tmpDir := t.TempDir()
	rpiDir := filepath.Join(tmpDir, ".agents", "rpi")
	councilDir := filepath.Join(tmpDir, ".agents", "council")
	for _, dir := range []string{rpiDir, councilDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(rpiDir, "phase-2-result.json"), []byte(`{"status":"completed"}`), 0644); err != nil {
		t.Fatal(err)
	}

	vibeReport := "# Vibe\n\n## Council Verdict: PASS\n"
	postMortemReport := "# Post-mortem\n\n## Council Verdict: WARN\n"
	if err := os.WriteFile(filepath.Join(councilDir, "2026-02-19-ag-new-vibe.md"), []byte(vibeReport), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(councilDir, "2026-02-19-ag-new-post-mortem.md"), []byte(postMortemReport), 0644); err != nil {
		t.Fatal(err)
	}

	state := newTestPhasedState().WithEpicID("ag-new").WithRunID("run-validation")

	if err := postPhaseProcessing(tmpDir, state, 3, filepath.Join(tmpDir, "orchestration.log")); err != nil {
		t.Fatalf("postPhaseProcessing(validation): %v", err)
	}
	if got := state.Verdicts["vibe"]; got != "PASS" {
		t.Fatalf("expected vibe verdict PASS, got %q", got)
	}
	if got := state.Verdicts["post_mortem"]; got != "WARN" {
		t.Fatalf("expected post_mortem verdict WARN, got %q", got)
	}
}

func TestNoWorktreeRunIDGeneration(t *testing.T) {
	tmpDir := t.TempDir()
	prevDryRun := dryRun
	dryRun = true
	defer func() { dryRun = prevDryRun }()

	opts := defaultPhasedEngineOptions()
	opts.NoWorktree = true
	opts.SwarmFirst = false

	if err := runPhasedEngine(context.Background(), tmpDir, "test goal", opts); err != nil {
		t.Fatalf("runPhasedEngine --no-worktree --dry-run: %v", err)
	}

	logPath := filepath.Join(tmpDir, ".agents", "rpi", "phased-orchestration.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read orchestration log: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `start: goal="test goal" from=discovery`) {
		t.Fatalf("expected start entry in orchestration log, got: %s", content)
	}
	runIDPattern := regexp.MustCompile(`\[[0-9a-f]{8}\] start:`)
	if !runIDPattern.MatchString(content) {
		t.Fatalf("expected generated runID in start entry, got: %s", content)
	}

	statePath := filepath.Join(tmpDir, ".agents", "rpi", phasedStateFile)
	stateData, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read phased state: %v", err)
	}
	var st phasedState
	if err := json.Unmarshal(stateData, &st); err != nil {
		t.Fatalf("unmarshal phased state: %v", err)
	}
	if st.RunID == "" {
		t.Fatal("expected run_id to be persisted")
	}
	if st.Backend == "" {
		t.Fatal("expected backend to be persisted")
	}
	registryStatePath := filepath.Join(tmpDir, ".agents", "rpi", "runs", st.RunID, phasedStateFile)
	if _, err := os.Stat(registryStatePath); err != nil {
		t.Fatalf("expected run registry state at %s: %v", registryStatePath, err)
	}
}

func TestRunPhasedEngine_AutoCleanupStale_DryRunDoesNotMutate(t *testing.T) {
	tmpDir := t.TempDir()

	runID := "stale-old-run"
	runDir := filepath.Join(tmpDir, ".agents", "rpi", "runs", runID)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}
	state := map[string]any{
		"schema_version": 1,
		"run_id":         runID,
		"goal":           "stale",
		"phase":          2,
		"started_at":     time.Now().Add(-3 * time.Hour).UTC().Format(time.RFC3339),
	}
	data, marshalErr := json.Marshal(state)
	if marshalErr != nil {
		t.Fatalf("marshal state: %v", marshalErr)
	}
	if err := os.WriteFile(filepath.Join(runDir, phasedStateFile), data, 0644); err != nil {
		t.Fatal(err)
	}

	prevDryRun := dryRun
	dryRun = true
	defer func() { dryRun = prevDryRun }()

	opts := defaultPhasedEngineOptions()
	opts.NoWorktree = true
	opts.SwarmFirst = false
	opts.AutoCleanStale = true
	opts.AutoCleanStaleAfter = 1 * time.Hour

	if err := runPhasedEngine(context.Background(), tmpDir, "test auto cleanup", opts); err != nil {
		t.Fatalf("runPhasedEngine auto-clean --dry-run: %v", err)
	}

	updatedData, err := os.ReadFile(filepath.Join(runDir, phasedStateFile))
	if err != nil {
		t.Fatalf("read updated state: %v", err)
	}
	var updated map[string]any
	if err := json.Unmarshal(updatedData, &updated); err != nil {
		t.Fatalf("unmarshal updated state: %v", err)
	}
	if _, ok := updated["terminal_status"]; ok {
		t.Fatalf("expected no terminal_status mutation in dry-run, got %v", updated["terminal_status"])
	}
}

func TestRunPhasedEngine_DoesNotMutateProcessCWD(t *testing.T) {
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd(): %v", err)
	}

	callerDir := t.TempDir()
	targetDir := t.TempDir()
	if err := os.Chdir(callerDir); err != nil {
		t.Fatalf("Chdir(%s): %v", callerDir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	prevDryRun := dryRun
	dryRun = true
	t.Cleanup(func() { dryRun = prevDryRun })

	opts := defaultPhasedEngineOptions()
	opts.NoWorktree = true
	opts.SwarmFirst = false

	if err := runPhasedEngine(context.Background(), targetDir, "cwd isolation", opts); err != nil {
		t.Fatalf("runPhasedEngine(): %v", err)
	}

	gotWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() after run: %v", err)
	}
	gotResolved, err := filepath.EvalSymlinks(gotWD)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", gotWD, err)
	}
	callerResolved, err := filepath.EvalSymlinks(callerDir)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", callerDir, err)
	}
	if gotResolved != callerResolved {
		t.Fatalf("process cwd changed to %q (resolved %q), want %q (resolved %q)", gotWD, gotResolved, callerDir, callerResolved)
	}
}

func writeFakeBDScript(t *testing.T, dir string) {
	t.Helper()
	script := filepath.Join(dir, "bd")
	content := `#!/usr/bin/env bash
set -euo pipefail

if [ "${1:-}" = "ready" ] && [ "${2:-}" = "--json" ]; then
  echo '[]'
  exit 0
fi

if [ "${1:-}" = "list" ]; then
  if printf '%s\n' "$*" | grep -q -- '--json'; then
    if printf '%s\n' "$*" | grep -q -- '--type epic'; then
      echo '[{"id":"ag-old"},{"id":"ag-new"}]'
      exit 0
    fi
    echo '[{"id":"ag-new"}]'
    exit 0
  fi
  echo "ag-old [EPIC]"
  echo "ag-new [EPIC]"
  exit 0
fi

if [ "${1:-}" = "children" ]; then
  echo "ag-new.1  closed  done"
  echo "ag-new.2  closed  done"
  exit 0
fi

echo "unsupported bd invocation: $*" >&2
exit 1
`
	if err := os.WriteFile(script, []byte(content), 0755); err != nil {
		t.Fatalf("write fake bd script: %v", err)
	}
}

// containsStr is a helper to check substring presence.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, sub string) bool {
	for i := range len(s) - len(sub) + 1 {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestLogPhaseTransition(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Test with runID
	logPhaseTransition(logPath, "abc123", "research", "started")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "[abc123] research: started") {
		t.Errorf("expected runID in log, got: %s", string(data))
	}

	// Test without runID (empty string)
	logPath2 := filepath.Join(tmpDir, "test2.log")
	logPhaseTransition(logPath2, "", "plan", "completed")
	data2, err := os.ReadFile(logPath2)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data2)
	if !strings.Contains(content, "plan: completed") {
		t.Errorf("expected phase in log, got: %s", content)
	}
	if strings.Contains(content, "[]") {
		t.Errorf("empty runID should not produce brackets, got: %s", content)
	}
}

func TestLogPhaseTransition_MirrorsToLedgerAndCache(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, ".agents", "rpi", "phased-orchestration.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		t.Fatal(err)
	}

	logPhaseTransition(logPath, "run-ledger", "implementation", "started")

	records, err := LoadRPILedgerRecords(tmpDir)
	if err != nil {
		t.Fatalf("load ledger records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 ledger record, got %d", len(records))
	}
	if records[0].RunID != "run-ledger" {
		t.Fatalf("unexpected run_id: %q", records[0].RunID)
	}
	if records[0].Phase != "implementation" {
		t.Fatalf("unexpected phase: %q", records[0].Phase)
	}
	if records[0].Action != "started" {
		t.Fatalf("unexpected action: %q", records[0].Action)
	}

	cachePath := filepath.Join(tmpDir, ".agents", "rpi", "runs", "run-ledger.json")
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("expected run cache at %s: %v", cachePath, err)
	}
}

// --- writePhaseResult + validatePriorPhaseResult ---

func TestWritePhaseResult(t *testing.T) {
	tmpDir := t.TempDir()

	result := &phaseResult{
		SchemaVersion: 1,
		RunID:         "test-run",
		Phase:         1,
		PhaseName:     "discovery",
		Status:        "completed",
		Artifacts: map[string]string{
			"plan": ".agents/plans/test-plan.md",
		},
		Verdicts: map[string]string{
			"pre_mortem": "PASS",
		},
		StartedAt:       "2026-02-24T10:00:00Z",
		CompletedAt:     "2026-02-24T10:05:00Z",
		DurationSeconds: 300,
	}

	if err := writePhaseResult(tmpDir, result); err != nil {
		t.Fatalf("writePhaseResult: %v", err)
	}

	// Verify file exists at expected path
	path := filepath.Join(tmpDir, ".agents", "rpi", fmt.Sprintf(phaseResultFileFmt, 1))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read phase result: %v", err)
	}

	// Verify JSON round-trips correctly
	var loaded phaseResult
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal phase result: %v", err)
	}
	if loaded.Phase != 1 {
		t.Errorf("phase = %d, want 1", loaded.Phase)
	}
	if loaded.Status != "completed" {
		t.Errorf("status = %q, want %q", loaded.Status, "completed")
	}
	if loaded.PhaseName != "discovery" {
		t.Errorf("phase_name = %q, want %q", loaded.PhaseName, "discovery")
	}
	if loaded.Verdicts["pre_mortem"] != "PASS" {
		t.Errorf("verdicts[pre_mortem] = %q, want %q", loaded.Verdicts["pre_mortem"], "PASS")
	}
}

func TestValidatePriorPhaseResult(t *testing.T) {
	tests := []struct {
		name    string
		phase   int
		result  *phaseResult
		wantErr bool
	}{
		{
			name:  "valid completed phase",
			phase: 1,
			result: &phaseResult{
				Phase:  1,
				Status: "completed",
			},
		},
		{
			name:    "failed status rejects",
			phase:   1,
			result:  &phaseResult{Phase: 1, Status: "failed"},
			wantErr: true,
		},
		{
			name:    "empty status rejects",
			phase:   1,
			result:  &phaseResult{Phase: 1, Status: ""},
			wantErr: true,
		},
		{
			name:    "missing file rejects",
			phase:   2,
			result:  nil, // don't write anything
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			if tt.result != nil {
				if err := writePhaseResult(tmpDir, tt.result); err != nil {
					t.Fatalf("writePhaseResult: %v", err)
				}
			}

			err := validatePriorPhaseResult(tmpDir, tt.phase)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// --- updateRunHeartbeat + readRunHeartbeat ---

func TestRunHeartbeat(t *testing.T) {
	tmpDir := t.TempDir()
	runID := "test-heartbeat-run"

	// No heartbeat yet — should return zero time
	ts := readRunHeartbeat(tmpDir, runID)
	if !ts.IsZero() {
		t.Fatalf("expected zero time for missing heartbeat, got %v", ts)
	}

	// Write heartbeat
	updateRunHeartbeat(tmpDir, runID)

	// Read it back
	ts = readRunHeartbeat(tmpDir, runID)
	if ts.IsZero() {
		t.Fatal("expected non-zero time after heartbeat update")
	}
	if time.Since(ts) > 10*time.Second {
		t.Fatalf("heartbeat timestamp too old: %v", ts)
	}
}

func TestRunHeartbeat_EmptyRunID(t *testing.T) {
	tmpDir := t.TempDir()

	// Should be a no-op, not panic
	updateRunHeartbeat(tmpDir, "")
	ts := readRunHeartbeat(tmpDir, "")
	if !ts.IsZero() {
		t.Fatalf("expected zero time for empty runID, got %v", ts)
	}
}

// --- deriveRepoRootFromRPIOrchestrationLog ---

func TestDeriveRepoRootFromRPIOrchestrationLog(t *testing.T) {
	tests := []struct {
		name    string
		logPath string
		want    string
		wantOK  bool
	}{
		{
			name:    "valid rpi log path",
			logPath: "/home/user/project/.agents/rpi/orchestration.log",
			want:    "/home/user/project",
			wantOK:  true,
		},
		{
			name:    "nested project path",
			logPath: "/tmp/worktree/.agents/rpi/run-log.jsonl",
			want:    "/tmp/worktree",
			wantOK:  true,
		},
		{
			name:    "not in rpi dir",
			logPath: "/home/user/project/.agents/council/report.md",
			want:    "",
			wantOK:  false,
		},
		{
			name:    "not in .agents dir",
			logPath: "/home/user/project/rpi/log.txt",
			want:    "",
			wantOK:  false,
		},
		{
			name:    "bare filename",
			logPath: "orchestration.log",
			want:    "",
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := deriveRepoRootFromRPIOrchestrationLog(tt.logPath)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("got = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- rpiRunRegistryDir ---

func TestRpiRunRegistryDir(t *testing.T) {
	got := rpiRunRegistryDir("/home/user/project", "run-abc123")
	want := filepath.Join("/home/user/project", ".agents", "rpi", "runs", "run-abc123")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// Empty runID returns empty
	got = rpiRunRegistryDir("/home/user/project", "")
	if got != "" {
		t.Errorf("expected empty for empty runID, got %q", got)
	}
}

func TestWritePhaseResult_TimeBoxed(t *testing.T) {
	tmpDir := t.TempDir()

	pr := &phaseResult{
		SchemaVersion:   1,
		RunID:           "test-run-123",
		Phase:           1,
		PhaseName:       "discovery",
		Status:          "time_boxed",
		StartedAt:       time.Now().Add(-30 * time.Second).Format(time.RFC3339),
		CompletedAt:     time.Now().Format(time.RFC3339),
		DurationSeconds: 30.0,
	}

	err := writePhaseResult(tmpDir, pr)
	if err != nil {
		t.Fatalf("writePhaseResult failed: %v", err)
	}

	resultPath := filepath.Join(tmpDir, ".agents", "rpi", fmt.Sprintf(phaseResultFileFmt, 1))
	data, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatalf("failed to read result: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if result["status"] != "time_boxed" {
		t.Errorf("expected status time_boxed, got %v", result["status"])
	}
	if result["phase_name"] != "discovery" {
		t.Errorf("expected phase_name discovery, got %v", result["phase_name"])
	}
	if result["run_id"] != "test-run-123" {
		t.Errorf("expected run_id test-run-123, got %v", result["run_id"])
	}
	if result["schema_version"] != float64(1) {
		t.Errorf("expected schema_version 1, got %v", result["schema_version"])
	}
	if result["duration_seconds"] != float64(30) {
		t.Errorf("expected duration_seconds 30, got %v", result["duration_seconds"])
	}
}

func TestWritePhaseResult_TimeBoxed_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	pr := &phaseResult{
		SchemaVersion: 1,
		RunID:         "run-dir-test",
		Phase:         2,
		PhaseName:     "implementation",
		Status:        "time_boxed",
		StartedAt:     time.Now().Format(time.RFC3339),
	}

	err := writePhaseResult(tmpDir, pr)
	if err != nil {
		t.Fatalf("writePhaseResult failed: %v", err)
	}

	resultPath := filepath.Join(tmpDir, ".agents", "rpi", fmt.Sprintf(phaseResultFileFmt, 2))
	if _, err := os.Stat(resultPath); os.IsNotExist(err) {
		t.Fatal("expected result file to exist after writePhaseResult")
	}
}

func TestValidatePriorPhaseResult_TimeBoxed(t *testing.T) {
	tmpDir := t.TempDir()

	// Write a time_boxed result for phase 1
	pr := &phaseResult{
		SchemaVersion: 1,
		RunID:         "test-run",
		Phase:         1,
		PhaseName:     "discovery",
		Status:        "time_boxed",
		StartedAt:     time.Now().Format(time.RFC3339),
	}
	if err := writePhaseResult(tmpDir, pr); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// time_boxed should be accepted as a valid continuation status
	err := validatePriorPhaseResult(tmpDir, 1)
	if err != nil {
		t.Fatalf("validatePriorPhaseResult should accept time_boxed, got error: %v", err)
	}
}

func TestValidatePriorPhaseResult_NoFile(t *testing.T) {
	tmpDir := t.TempDir()

	err := validatePriorPhaseResult(tmpDir, 1)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestValidatePriorPhaseResult_CompletedAccepted(t *testing.T) {
	tmpDir := t.TempDir()

	pr := &phaseResult{
		SchemaVersion: 1,
		RunID:         "test-run",
		Phase:         1,
		PhaseName:     "discovery",
		Status:        "completed",
		StartedAt:     time.Now().Format(time.RFC3339),
	}
	if err := writePhaseResult(tmpDir, pr); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	err := validatePriorPhaseResult(tmpDir, 1)
	if err != nil {
		t.Fatalf("validatePriorPhaseResult should accept completed, got error: %v", err)
	}
}

func TestValidatePriorPhaseResult_FailedStatus(t *testing.T) {
	tmpDir := t.TempDir()

	pr := &phaseResult{
		SchemaVersion: 1,
		RunID:         "test-run",
		Phase:         1,
		PhaseName:     "discovery",
		Status:        "failed",
		StartedAt:     time.Now().Format(time.RFC3339),
	}
	if err := writePhaseResult(tmpDir, pr); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	err := validatePriorPhaseResult(tmpDir, 1)
	if err == nil {
		t.Fatal("expected error for failed status, got nil")
	}
	if !strings.Contains(err.Error(), "failed") {
		t.Errorf("expected error mentioning 'failed', got: %v", err)
	}
}

func TestBudgetTimeout_WritesPhaseResultArtifact(t *testing.T) {
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(stateDir, "phased-orchestration.log")

	state := newTestPhasedState().WithRunID("budget-result-run")
	p := phase{Num: 1, Name: "discovery"}
	budget := 45 * time.Second

	phaseStart := time.Now().Add(-budget) // simulate a phase that ran for the budget duration
	if !handleBudgetTimeout(tmpDir, state, p, budget, logPath, phaseStart) {
		t.Fatal("expected timeout handler to return true")
	}

	// Verify the structured JSON result artifact was written
	resultPath := filepath.Join(stateDir, fmt.Sprintf(phaseResultFileFmt, 1))
	data, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatalf("expected phase result artifact: %v", err)
	}

	var result phaseResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if result.Status != "time_boxed" {
		t.Errorf("expected status time_boxed, got %v", result.Status)
	}
	if result.RunID != "budget-result-run" {
		t.Errorf("expected run_id budget-result-run, got %v", result.RunID)
	}
	if result.PhaseName != "discovery" {
		t.Errorf("expected phase_name discovery, got %v", result.PhaseName)
	}
	if result.SchemaVersion != 1 {
		t.Errorf("expected schema_version 1, got %v", result.SchemaVersion)
	}
	if result.DurationSeconds < 44 {
		t.Errorf("expected duration_seconds >= 44 (budget was 45s), got %v", result.DurationSeconds)
	}
}

func TestDefaultPhasedEngineOptions_AllFields(t *testing.T) {
	opts := defaultPhasedEngineOptions()

	if opts.From != "discovery" {
		t.Errorf("From = %q, want %q", opts.From, "discovery")
	}
	if !opts.TestFirst {
		t.Error("TestFirst should default to true")
	}
	if opts.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", opts.MaxRetries)
	}
	if opts.PhaseTimeout != 90*time.Minute {
		t.Errorf("PhaseTimeout = %v, want 90m", opts.PhaseTimeout)
	}
	if opts.StallTimeout != 10*time.Minute {
		t.Errorf("StallTimeout = %v, want 10m", opts.StallTimeout)
	}
	if opts.StreamStartupTimeout != 45*time.Second {
		t.Errorf("StreamStartupTimeout = %v, want 45s", opts.StreamStartupTimeout)
	}
	if !opts.SwarmFirst {
		t.Error("SwarmFirst should default to true")
	}
	if opts.AutoCleanStaleAfter != 24*time.Hour {
		t.Errorf("AutoCleanStaleAfter = %v, want 24h", opts.AutoCleanStaleAfter)
	}
	if opts.RuntimeMode != "auto" {
		t.Errorf("RuntimeMode = %q, want %q", opts.RuntimeMode, "auto")
	}
	if opts.RuntimeCommand != "claude" {
		t.Errorf("RuntimeCommand = %q, want %q", opts.RuntimeCommand, "claude")
	}
	if opts.AOCommand != "ao" {
		t.Errorf("AOCommand = %q, want %q", opts.AOCommand, "ao")
	}
	if opts.BDCommand != "bd" {
		t.Errorf("BDCommand = %q, want %q", opts.BDCommand, "bd")
	}
	if opts.TmuxCommand != "tmux" {
		t.Errorf("TmuxCommand = %q, want %q", opts.TmuxCommand, "tmux")
	}
	if opts.TmuxWorkers != 1 {
		t.Errorf("TmuxWorkers = %d, want 1", opts.TmuxWorkers)
	}
	if opts.NoBudget {
		t.Error("NoBudget should default to false")
	}
	if opts.BudgetSpec != "" {
		t.Errorf("BudgetSpec = %q, want empty", opts.BudgetSpec)
	}
	if opts.StallCheckInterval != 30*time.Second {
		t.Errorf("StallCheckInterval = %v, want 30s", opts.StallCheckInterval)
	}
}

func TestGenerateRunID_Format(t *testing.T) {
	id := generateRunID()
	if len(id) != 12 {
		t.Errorf("generateRunID length = %d, want 12", len(id))
	}
	// Should be lowercase hex
	for _, c := range id {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Errorf("generateRunID contains non-hex char %q in %q", string(c), id)
			break
		}
	}
	// Should match the rpiRunIDPattern (12-hex variant)
	if !rpiRunIDPattern.MatchString(id) {
		t.Errorf("generateRunID %q does not match rpiRunIDPattern", id)
	}
}

func TestGenerateRunID_Unique(t *testing.T) {
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		id := generateRunID()
		if seen[id] {
			t.Fatalf("duplicate run ID %q after %d calls", id, i)
		}
		seen[id] = true
	}
}

// ===========================================================================
// Coverage tests for rpi_phased_processing.go — targeting zero-coverage functions
// ===========================================================================

// --- gateFailError ---

func TestGateFailError_Error(t *testing.T) {
	err := &gateFailError{
		Phase:   2,
		Verdict: "BLOCKED",
		Report:  "bd children ag-xyz",
	}
	got := err.Error()
	if !strings.Contains(got, "phase 2") {
		t.Errorf("expected 'phase 2' in error, got %q", got)
	}
	if !strings.Contains(got, "BLOCKED") {
		t.Errorf("expected 'BLOCKED' in error, got %q", got)
	}
}

// isPlanFileEpic / planFileFromEpic tests are in rpi_phased_planfile_test.go

// --- discoverPlanFile ---

func TestDiscoverPlanFile_Found(t *testing.T) {
	cwd := t.TempDir()
	plansDir := filepath.Join(cwd, ".agents", "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write two plan files with different mod times
	path1 := filepath.Join(plansDir, "plan-a.md")
	if err := os.WriteFile(path1, []byte("plan A"), 0644); err != nil {
		t.Fatal(err)
	}
	// Ensure second file has a later mod time
	time.Sleep(10 * time.Millisecond)
	path2 := filepath.Join(plansDir, "plan-b.md")
	if err := os.WriteFile(path2, []byte("plan B"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := discoverPlanFile(cwd)
	if err != nil {
		t.Fatalf("discoverPlanFile: %v", err)
	}
	if !strings.HasSuffix(got, "plan-b.md") {
		t.Errorf("expected most recent plan file, got %q", got)
	}
}

func TestDiscoverPlanFile_NoPlans(t *testing.T) {
	cwd := t.TempDir()
	plansDir := filepath.Join(cwd, ".agents", "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}

	_, err := discoverPlanFile(cwd)
	if err == nil {
		t.Fatal("expected error for empty plans directory")
	}
}

func TestDiscoverPlanFile_SkipNonMD(t *testing.T) {
	cwd := t.TempDir()
	plansDir := filepath.Join(cwd, ".agents", "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Only non-md files
	if err := os.WriteFile(filepath.Join(plansDir, "notes.txt"), []byte("notes"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := discoverPlanFile(cwd)
	if err == nil {
		t.Fatal("expected error when no .md files exist")
	}
}

func TestFindLatestCouncilReport_NoMatches(t *testing.T) {
	cwd := t.TempDir()
	councilDir := filepath.Join(cwd, ".agents", "council")
	if err := os.MkdirAll(councilDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(councilDir, "unrelated.md"), []byte("nope"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := findLatestCouncilReport(cwd, "vibe", time.Time{}, "")
	if err == nil {
		t.Fatal("expected error when no matching reports")
	}
}

func TestFindLatestCouncilReport_WithEpicID(t *testing.T) {
	cwd := t.TempDir()
	councilDir := filepath.Join(cwd, ".agents", "council")
	if err := os.MkdirAll(councilDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Generic vibe report
	if err := os.WriteFile(filepath.Join(councilDir, "2026-01-01-vibe-generic.md"), []byte("## Council Verdict: PASS"), 0644); err != nil {
		t.Fatal(err)
	}
	// Epic-scoped vibe report
	if err := os.WriteFile(filepath.Join(councilDir, "2026-01-02-vibe-ag-xyz.md"), []byte("## Council Verdict: WARN"), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := findLatestCouncilReport(cwd, "vibe", time.Time{}, "ag-xyz")
	if err != nil {
		t.Fatalf("findLatestCouncilReport: %v", err)
	}
	if !strings.Contains(report, "ag-xyz") {
		t.Errorf("expected epic-scoped report, got %q", report)
	}
}

func TestFindLatestCouncilReport_NotBeforeFilter(t *testing.T) {
	cwd := t.TempDir()
	councilDir := filepath.Join(cwd, ".agents", "council")
	if err := os.MkdirAll(councilDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a file with a specific mod time
	reportPath := filepath.Join(councilDir, "2026-02-01-vibe-test.md")
	if err := os.WriteFile(reportPath, []byte("## Council Verdict: PASS"), 0644); err != nil {
		t.Fatal(err)
	}

	// notBefore in the future should exclude it
	_, err := findLatestCouncilReport(cwd, "vibe", time.Now().Add(1*time.Hour), "")
	if err == nil {
		t.Fatal("expected error when notBefore excludes all reports")
	}
}

func TestFindLatestCouncilReport_SkipDirs(t *testing.T) {
	cwd := t.TempDir()
	councilDir := filepath.Join(cwd, ".agents", "council")
	if err := os.MkdirAll(councilDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create a directory that matches the pattern name
	if err := os.MkdirAll(filepath.Join(councilDir, "2026-01-01-vibe-subdir.md"), 0755); err != nil {
		t.Fatal(err)
	}

	_, err := findLatestCouncilReport(cwd, "vibe", time.Time{}, "")
	if err == nil {
		t.Fatal("expected error when only matching entry is a directory")
	}
}

// --- matchCouncilEntry ---

func TestMatchCouncilEntry(t *testing.T) {
	cwd := t.TempDir()
	councilDir := filepath.Join(cwd, ".agents", "council")
	if err := os.MkdirAll(councilDir, 0755); err != nil {
		t.Fatal(err)
	}

	matchingFile := filepath.Join(councilDir, "2026-01-01-vibe-test.md")
	if err := os.WriteFile(matchingFile, []byte("report"), 0644); err != nil {
		t.Fatal(err)
	}
	nonMatchingFile := filepath.Join(councilDir, "2026-01-01-other.md")
	if err := os.WriteFile(nonMatchingFile, []byte("other"), 0644); err != nil {
		t.Fatal(err)
	}
	nonMDFile := filepath.Join(councilDir, "2026-01-01-vibe-test.txt")
	if err := os.WriteFile(nonMDFile, []byte("txt"), 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(councilDir)
	if err != nil {
		t.Fatal(err)
	}

	matched := 0
	for _, entry := range entries {
		if _, ok := matchCouncilEntry(entry, councilDir, "vibe", time.Time{}); ok {
			matched++
		}
	}
	if matched != 1 {
		t.Errorf("expected 1 match, got %d", matched)
	}
}

// --- extractCouncilFindings ---

func TestExtractCouncilFindings_StructuredFormat(t *testing.T) {
	cwd := t.TempDir()
	reportPath := filepath.Join(cwd, "report.md")
	content := `# Report
FINDING: Missing error handling | FIX: Add nil check | REF: cmd/ao/main.go:42
FINDING: Race condition | FIX: Add mutex | REF: cmd/ao/server.go:88
FINDING: Extra 1 | FIX: fix 1 | REF: ref1
FINDING: Extra 2 | FIX: fix 2 | REF: ref2
`
	if err := os.WriteFile(reportPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	findings, err := extractCouncilFindings(reportPath, 3)
	if err != nil {
		t.Fatalf("extractCouncilFindings: %v", err)
	}
	if len(findings) != 3 {
		t.Errorf("expected 3 findings (max), got %d", len(findings))
	}
	if findings[0].Description != "Missing error handling" {
		t.Errorf("first finding = %q", findings[0].Description)
	}
}

func TestExtractCouncilFindings_FallbackFormat(t *testing.T) {
	cwd := t.TempDir()
	reportPath := filepath.Join(cwd, "report.md")
	content := `# Report

## Shared Findings

1. **Missing tests** — No unit tests for new function
2. **Duplicate code** — Same logic in two files
`
	if err := os.WriteFile(reportPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	findings, err := extractCouncilFindings(reportPath, 5)
	if err != nil {
		t.Fatalf("extractCouncilFindings: %v", err)
	}
	if len(findings) != 2 {
		t.Errorf("expected 2 fallback findings, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Description, "Missing tests") {
		t.Errorf("first finding = %q", findings[0].Description)
	}
}

func TestExtractCouncilFindings_NoFindings(t *testing.T) {
	reportPath := filepath.Join(t.TempDir(), "empty-report.md")
	if err := os.WriteFile(reportPath, []byte("# Report\nNo findings here."), 0644); err != nil {
		t.Fatal(err)
	}

	findings, err := extractCouncilFindings(reportPath, 5)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

// parseFastPath, parseCrankCompletion tests declared earlier in this file. Duplicates removed.

// NOTE: TestRunRPIPhased_BasicFlow is intentionally omitted. The runRPIPhased cobra RunE
// handler reads from package-level flag variables, wires signal.NotifyContext, and calls
// resolveRPIToolchain — all tightly coupled to cobra command registration. Testing through
// the cobra entry point would require reconstructing the full flag set and command tree.
// The engine logic is already tested via runRPIPhasedWithOpts in rpi_phased_swarm_test.go
// (TestRunRPIPhased_DryRunBackendSelection, TestRunRPIPhased_SwarmFirstBackendLogged,
// TestRunRPIPhased_BackendStoredInState). Config resolution is tested via
// TestResolveWorktreeModeFromConfig_* in rpi_phased_context_test.go.
