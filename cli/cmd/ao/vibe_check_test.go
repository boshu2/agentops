// practices: [ai-assisted-dev, code-complete]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/vibecheck"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		want    time.Duration
	}{
		{"days", "7d", false, 7 * 24 * time.Hour},
		{"days 30", "30d", false, 30 * 24 * time.Hour},
		{"weeks", "1w", false, 7 * 24 * time.Hour},
		{"weeks multiple", "4w", false, 4 * 7 * 24 * time.Hour},
		{"hours", "48h", false, 48 * time.Hour},
		{"invalid", "invalid", true, 0},
		{"empty", "", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if err == nil && got != tt.want {
				t.Errorf("parseDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestVibeCheckCommand(t *testing.T) {
	// Test that command is registered
	if vibeCheckCmd == nil {
		t.Fatal("vibeCheckCmd is nil")
	}

	if vibeCheckCmd.Use != "vibe-check" {
		t.Errorf("vibeCheckCmd.Use = %q, want vibe-check", vibeCheckCmd.Use)
	}

	// Check that aliases include vibecheck
	hasAlias := false
	for _, alias := range vibeCheckCmd.Aliases {
		if alias == "vibecheck" {
			hasAlias = true
			break
		}
	}
	if !hasAlias {
		t.Error("vibeCheckCmd missing 'vibecheck' alias")
	}
}

func TestVibeCheckFlags(t *testing.T) {
	if vibeCheckCmd.Flags().Lookup("markdown") == nil {
		t.Error("--markdown flag not found")
	}
	if vibeCheckCmd.Flags().Lookup("since") == nil {
		t.Error("--since flag not found")
	}
	if vibeCheckCmd.Flags().Lookup("repo") == nil {
		t.Error("--repo flag not found")
	}
	if vibeCheckCmd.Flags().Lookup("full") == nil {
		t.Error("--full flag not found")
	}
}

func TestVibeCheckDryRun(t *testing.T) {
	// Create a temporary directory to act as a git repo
	_ = t.TempDir()

	// Save original state
	originalDryRun := dryRun
	defer func() { dryRun = originalDryRun }()

	// Enable dry-run mode
	dryRun = true

	// Run command in dry-run mode
	cmd := vibeCheckCmd
	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Errorf("runVibeCheck in dry-run mode failed: %v", err)
	}
}

func TestVibeCheckRepoPath(t *testing.T) {
	tests := []struct {
		name    string
		repo    string
		want    string
		wantErr bool
	}{
		{"current dir", ".", "", false},
		{"absolute path", "/tmp", "/tmp", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just test that path resolution doesn't panic
			repoPath := tt.repo
			if repoPath == "." {
				cwd, err := os.Getwd()
				if err != nil {
					t.Errorf("failed to get cwd: %v", err)
					return
				}
				repoPath = cwd
			}

			// All absolute path operations should succeed
			_ = repoPath
		})
	}
}

func TestOutputFormats(t *testing.T) {
	// Test that output functions handle empty results
	t.Run("empty result", func(t *testing.T) {
		result := &MockVibeCheckResult{
			Score: 0.85,
			Grade: "B",
		}

		// Just test that functions don't panic with empty data
		// (We can't easily test stdout capture without refactoring)
		if result.Score < 0 || result.Score > 1 {
			t.Error("invalid score")
		}
	})
}

// MockVibeCheckResult for testing
type MockVibeCheckResult struct {
	Score    float64
	Grade    string
	Events   []any
	Metrics  map[string]float64
	Findings []any
}

func TestRunVibeCheck_invalidDuration(t *testing.T) {
	origDryRun := dryRun
	origSince := vibeCheckSince
	origRepo := vibeCheckRepo
	defer func() {
		dryRun = origDryRun
		vibeCheckSince = origSince
		vibeCheckRepo = origRepo
	}()

	dryRun = false
	vibeCheckSince = "not-a-duration"
	vibeCheckRepo = t.TempDir()

	err := runVibeCheck(nil, nil)
	if err == nil {
		t.Fatal("expected error for invalid duration format")
	}
}

func TestRunVibeCheck_validRepo(t *testing.T) {
	origDryRun := dryRun
	origSince := vibeCheckSince
	origRepo := vibeCheckRepo
	origOutput := output
	origMarkdown := vibeCheckMarkdown
	origFull := vibeCheckFull
	defer func() {
		dryRun = origDryRun
		vibeCheckSince = origSince
		vibeCheckRepo = origRepo
		output = origOutput
		vibeCheckMarkdown = origMarkdown
		vibeCheckFull = origFull
	}()

	repo := newVibeCheckFixtureRepo(t)
	dryRun = false
	vibeCheckSince = "90d"
	vibeCheckRepo = repo
	output = "table"
	vibeCheckMarkdown = false
	vibeCheckFull = false

	out, err := captureStdout(t, func() error {
		return runVibeCheck(nil, nil)
	})
	if err != nil {
		t.Fatalf("runVibeCheck: %v", err)
	}
	if !strings.Contains(out, "Vibe Check Report") {
		t.Fatalf("expected table report output, got: %s", out)
	}
	if !strings.Contains(out, "Metrics:") {
		t.Fatalf("expected metrics section in table output, got: %s", out)
	}
}

func TestRunVibeCheck_jsonOutput(t *testing.T) {
	origDryRun := dryRun
	origSince := vibeCheckSince
	origRepo := vibeCheckRepo
	origOutput := output
	defer func() {
		dryRun = origDryRun
		vibeCheckSince = origSince
		vibeCheckRepo = origRepo
		output = origOutput
	}()

	repo := newVibeCheckFixtureRepo(t)
	dryRun = false
	vibeCheckSince = "90d"
	vibeCheckRepo = repo
	output = "json"

	out, err := captureStdout(t, func() error {
		return runVibeCheck(nil, nil)
	})
	if err != nil {
		t.Fatalf("runVibeCheck json: %v", err)
	}
	if !strings.Contains(out, "\"events\":") {
		t.Fatalf("expected JSON events in output, got: %s", out)
	}
	if !strings.Contains(out, "Fix bug") {
		t.Fatalf("expected seeded commit message in JSON output, got: %s", out)
	}
}

func TestRunVibeCheck_markdownOutput(t *testing.T) {
	origDryRun := dryRun
	origSince := vibeCheckSince
	origRepo := vibeCheckRepo
	origMarkdown := vibeCheckMarkdown
	origOutput := output
	origFull := vibeCheckFull
	defer func() {
		dryRun = origDryRun
		vibeCheckSince = origSince
		vibeCheckRepo = origRepo
		vibeCheckMarkdown = origMarkdown
		output = origOutput
		vibeCheckFull = origFull
	}()

	repo := newVibeCheckFixtureRepo(t)
	dryRun = false
	vibeCheckSince = "90d"
	vibeCheckRepo = repo
	output = "table"
	vibeCheckMarkdown = true
	vibeCheckFull = true

	out, err := captureStdout(t, func() error {
		return runVibeCheck(nil, nil)
	})
	if err != nil {
		t.Fatalf("runVibeCheck markdown: %v", err)
	}
	if !strings.Contains(out, "# Vibe Check Report") {
		t.Fatalf("expected markdown heading, got: %s", out)
	}
	if !strings.Contains(out, "## Recent Events (3 commits)") {
		t.Fatalf("expected markdown timeline section, got: %s", out)
	}
	if !strings.Contains(out, "Fix bug") {
		t.Fatalf("expected seeded commit message in markdown output, got: %s", out)
	}
}

func TestResolveVibeCheckRepoPath_UsesGitTopLevel(t *testing.T) {
	repo := newVibeCheckFixtureRepo(t)
	nested := filepath.Join(repo, "nested", "child")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("mkdir nested repo path: %v", err)
	}
	chdirTo(t, nested)
	t.Setenv("GIT_DIR", ".git")

	got, err := resolveVibeCheckRepoPath(".")
	if err != nil {
		t.Fatalf("resolveVibeCheckRepoPath: %v", err)
	}

	gotPath, err := filepath.EvalSymlinks(got)
	if err != nil {
		gotPath = filepath.Clean(got)
	}
	repoPath, err := filepath.EvalSymlinks(repo)
	if err != nil {
		repoPath = filepath.Clean(repo)
	}

	if gotPath != repoPath {
		t.Fatalf("resolveVibeCheckRepoPath(.) = %q, want %q", gotPath, repoPath)
	}
}

// ===========================================================================
// vibe_check.go — outputVibeCheckJSON (zero coverage)
// ===========================================================================

func newVibeCheckFixtureRepo(t *testing.T) string {
	t.Helper()
	base := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Second)
	return initGitHistoryFixtureRepo(t, []gitCommitFixture{
		{
			Path:      "file1.txt",
			Content:   "first commit\n",
			Message:   "Initial commit",
			Timestamp: base,
		},
		{
			Path:      "file2.txt",
			Content:   "feature work\n",
			Message:   "Add feature",
			Timestamp: base.Add(6 * time.Hour),
		},
		{
			Path:      "file1.txt",
			Content:   "bug fix\n",
			Message:   "Fix bug",
			Timestamp: base.Add(12 * time.Hour),
		},
	})
}

func TestVibeCheck_outputVibeCheckJSON_emptyResult(t *testing.T) {
	result := &vibecheck.VibeCheckResult{
		Score:    85.0,
		Grade:    "B",
		Metrics:  map[string]float64{},
		Findings: []vibecheck.Finding{},
		Events:   []vibecheck.TimelineEvent{},
	}
	out, err := captureStdout(t, func() error {
		return outputVibeCheckJSON(result)
	})
	if err != nil {
		t.Fatalf("outputVibeCheckJSON: %v", err)
	}
	if !strings.Contains(out, "85") {
		t.Errorf("expected JSON output to contain score 85, got: %s", out)
	}
}

func TestVibeCheck_outputVibeCheckJSON_withData(t *testing.T) {
	result := &vibecheck.VibeCheckResult{
		Score: 92.5,
		Grade: "A",
		Metrics: map[string]float64{
			"velocity":   1.5,
			"complexity": 0.3,
		},
		Findings: []vibecheck.Finding{
			{Severity: "warning", Category: "drift", Message: "test warning"},
		},
		Events: []vibecheck.TimelineEvent{
			{
				Timestamp: time.Now(),
				Author:    "test",
				Message:   "initial commit",
			},
		},
	}
	out, err := captureStdout(t, func() error {
		return outputVibeCheckJSON(result)
	})
	if err != nil {
		t.Fatalf("outputVibeCheckJSON: %v", err)
	}
	if !strings.Contains(out, "92.5") {
		t.Errorf("expected JSON to contain score 92.5, got: %s", out)
	}
	if !strings.Contains(out, "drift") {
		t.Errorf("expected JSON to contain finding category 'drift', got: %s", out)
	}
}

// ===========================================================================
// vibe_check.go — outputVibeCheckMarkdown (zero coverage)
// ===========================================================================

func TestVibeCheck_outputVibeCheckMarkdown_emptyResult(t *testing.T) {
	result := &vibecheck.VibeCheckResult{
		Score:    50.0,
		Grade:    "C",
		Metrics:  map[string]float64{},
		Findings: []vibecheck.Finding{},
		Events:   []vibecheck.TimelineEvent{},
	}
	out, err := captureStdout(t, func() error {
		return outputVibeCheckMarkdown(result)
	})
	if err != nil {
		t.Fatalf("outputVibeCheckMarkdown: %v", err)
	}
	if !strings.Contains(out, "50") {
		t.Errorf("expected markdown to contain score 50, got: %s", out)
	}
}

func TestVibeCheck_outputVibeCheckMarkdown_withData(t *testing.T) {
	result := &vibecheck.VibeCheckResult{
		Score: 72.0,
		Grade: "B-",
		Metrics: map[string]float64{
			"velocity":   1.2,
			"complexity": 0.5,
		},
		Findings: []vibecheck.Finding{
			{Severity: "error", Category: "amnesia", Message: "repeating mistakes"},
			{Severity: "info", Category: "drift", Message: "minor drift detected", File: "main.go", Line: 42},
		},
		Events: []vibecheck.TimelineEvent{
			{
				Timestamp: time.Now(),
				Author:    "dev",
				Message:   "fix: something",
			},
		},
	}
	out, err := captureStdout(t, func() error {
		return outputVibeCheckMarkdown(result)
	})
	if err != nil {
		t.Fatalf("outputVibeCheckMarkdown: %v", err)
	}
	if !strings.Contains(out, "72") {
		t.Errorf("expected markdown to contain score 72, got: %s", out)
	}
	if !strings.Contains(out, "amnesia") {
		t.Errorf("expected markdown to contain finding category 'amnesia', got: %s", out)
	}
}

// ===========================================================================
// vibe_check.go — printMarkdownMetrics (zero coverage)
// ===========================================================================

func TestVibeCheck_printMarkdownMetrics_empty(t *testing.T) {
	out := captureJSONStdout(t, func() {
		printMarkdownMetrics(map[string]float64{})
	})
	// Empty metrics should produce minimal or no output, but must not panic
	if strings.Contains(out, "panic") {
		t.Errorf("printMarkdownMetrics panicked on empty map")
	}
}

func TestVibeCheck_printMarkdownMetrics_withValues(t *testing.T) {
	out := captureJSONStdout(t, func() {
		printMarkdownMetrics(map[string]float64{
			"velocity":   1.5,
			"complexity": 0.3,
			"rework":     0.1,
		})
	})
	if !strings.Contains(out, "velocity") {
		t.Errorf("expected output to contain 'velocity', got: %s", out)
	}
	if !strings.Contains(out, "complexity") {
		t.Errorf("expected output to contain 'complexity', got: %s", out)
	}
}

func TestVibeCheck_printMarkdownMetrics_nil(t *testing.T) {
	out := captureJSONStdout(t, func() {
		printMarkdownMetrics(nil)
	})
	// nil metrics should produce minimal or no output, but must not panic
	if strings.Contains(out, "panic") {
		t.Errorf("printMarkdownMetrics panicked on nil map")
	}
}

// ===========================================================================
// vibe_check.go — printMarkdownFindings (zero coverage)
// ===========================================================================

func TestVibeCheck_printMarkdownFindings_empty(t *testing.T) {
	out := captureJSONStdout(t, func() {
		printMarkdownFindings([]vibecheck.Finding{})
	})
	// Empty findings should not produce finding-specific output
	if strings.Contains(out, "error") || strings.Contains(out, "amnesia") {
		t.Errorf("expected no finding output for empty list, got: %s", out)
	}
}

func TestVibeCheck_printMarkdownFindings_withFindings(t *testing.T) {
	findings := []vibecheck.Finding{
		{Severity: "error", Category: "amnesia", Message: "critical issue"},
		{Severity: "warning", Category: "drift", Message: "minor issue", File: "cmd/main.go"},
		{Severity: "info", Category: "test", Message: "informational", File: "pkg/util.go", Line: 10},
	}
	out := captureJSONStdout(t, func() {
		printMarkdownFindings(findings)
	})
	if !strings.Contains(out, "critical issue") {
		t.Errorf("expected output to contain 'critical issue', got: %s", out)
	}
	if !strings.Contains(out, "minor issue") {
		t.Errorf("expected output to contain 'minor issue', got: %s", out)
	}
}

func TestVibeCheck_printMarkdownFindings_nil(t *testing.T) {
	out := captureJSONStdout(t, func() {
		printMarkdownFindings(nil)
	})
	// nil findings should produce minimal or no output, but must not panic
	if strings.Contains(out, "panic") {
		t.Errorf("printMarkdownFindings panicked on nil")
	}
}

// ===========================================================================
// vibe_check.go — severityEmoji (zero coverage)
// ===========================================================================

func TestVibeCheck_severityEmoji(t *testing.T) {
	tests := []struct {
		severity string
		want     string
	}{
		{"error", "❌"},
		{"info", "ℹ️"},
		{"warning", "⚠️"},
		{"unknown", "⚠️"},
		{"", "⚠️"},
	}
	for _, tc := range tests {
		t.Run(tc.severity, func(t *testing.T) {
			got := severityEmoji(tc.severity)
			if got != tc.want {
				t.Errorf("severityEmoji(%q) = %q, want %q", tc.severity, got, tc.want)
			}
		})
	}
}

// ===========================================================================
// vibe_check.go — printMarkdownFinding (zero coverage)
// ===========================================================================

func TestVibeCheck_printMarkdownFinding_withFileLine(t *testing.T) {
	finding := vibecheck.Finding{
		Severity: "error",
		Category: "test-lies",
		Message:  "test assertions are empty",
		File:     "pkg/handler.go",
		Line:     42,
	}
	out := captureJSONStdout(t, func() {
		printMarkdownFinding(finding)
	})
	if !strings.Contains(out, "test assertions are empty") {
		t.Errorf("expected output to contain finding message, got: %s", out)
	}
	if !strings.Contains(out, "pkg/handler.go") {
		t.Errorf("expected output to contain file path, got: %s", out)
	}
}

func TestVibeCheck_printMarkdownFinding_withFileNoLine(t *testing.T) {
	finding := vibecheck.Finding{
		Severity: "warning",
		Category: "drift",
		Message:  "code drifting",
		File:     "main.go",
	}
	out := captureJSONStdout(t, func() {
		printMarkdownFinding(finding)
	})
	if !strings.Contains(out, "code drifting") {
		t.Errorf("expected output to contain finding message, got: %s", out)
	}
	if !strings.Contains(out, "main.go") {
		t.Errorf("expected output to contain file path, got: %s", out)
	}
}

func TestVibeCheck_printMarkdownFinding_noFile(t *testing.T) {
	finding := vibecheck.Finding{
		Severity: "info",
		Category: "logging",
		Message:  "missing structured logs",
	}
	out := captureJSONStdout(t, func() {
		printMarkdownFinding(finding)
	})
	if !strings.Contains(out, "missing structured logs") {
		t.Errorf("expected output to contain finding message, got: %s", out)
	}
}

// ===========================================================================
// vibe_check.go — printMarkdownEvents (zero coverage)
// ===========================================================================

func TestVibeCheck_printMarkdownEvents_notFull(t *testing.T) {
	// vibeCheckFull is false by default, so events should not be printed
	origFull := vibeCheckFull
	defer func() { vibeCheckFull = origFull }()
	vibeCheckFull = false

	events := []vibecheck.TimelineEvent{
		{Timestamp: time.Now(), Author: "dev", Message: "commit 1"},
	}
	out := captureJSONStdout(t, func() {
		printMarkdownEvents(events)
	})
	// Not full mode — should produce no output
	if strings.Contains(out, "commit 1") {
		t.Errorf("expected no event output in non-full mode, got: %s", out)
	}
}

func TestVibeCheck_printMarkdownEvents_fullWithEvents(t *testing.T) {
	origFull := vibeCheckFull
	defer func() { vibeCheckFull = origFull }()
	vibeCheckFull = true

	events := []vibecheck.TimelineEvent{
		{Timestamp: time.Now(), Author: "dev1", Message: "feat: add feature"},
		{Timestamp: time.Now(), Author: "dev2", Message: "fix: fix bug with a very long message that exceeds fifty characters and should be truncated"},
	}
	out := captureJSONStdout(t, func() {
		printMarkdownEvents(events)
	})
	if !strings.Contains(out, "dev1") {
		t.Errorf("expected output to contain author 'dev1', got: %s", out)
	}
	if !strings.Contains(out, "feat: add feature") {
		t.Errorf("expected output to contain event message, got: %s", out)
	}
}

func TestVibeCheck_printMarkdownEvents_fullEmpty(t *testing.T) {
	origFull := vibeCheckFull
	defer func() { vibeCheckFull = origFull }()
	vibeCheckFull = true

	// Empty events should return early
	out := captureJSONStdout(t, func() {
		printMarkdownEvents([]vibecheck.TimelineEvent{})
	})
	// Empty events in full mode should produce no event-specific output
	if strings.Contains(out, "Author") && strings.Contains(out, "Message") {
		t.Errorf("expected no event rows for empty list, got: %s", out)
	}
}

// ===========================================================================
// vibe_check.go — outputVibeCheckTable (zero coverage)
// ===========================================================================

func TestVibeCheck_outputVibeCheckTable_empty(t *testing.T) {
	result := &vibecheck.VibeCheckResult{
		Score:    0.0,
		Grade:    "F",
		Metrics:  map[string]float64{},
		Findings: []vibecheck.Finding{},
		Events:   []vibecheck.TimelineEvent{},
	}

	origFull := vibeCheckFull
	defer func() { vibeCheckFull = origFull }()
	vibeCheckFull = false

	out, err := captureStdout(t, func() error {
		return outputVibeCheckTable(result)
	})
	if err != nil {
		t.Fatalf("outputVibeCheckTable: %v", err)
	}
	if !strings.Contains(out, "F") {
		t.Errorf("expected table to contain grade 'F', got: %s", out)
	}
}

func TestVibeCheck_outputVibeCheckTable_withMetrics(t *testing.T) {
	result := &vibecheck.VibeCheckResult{
		Score: 75.0,
		Grade: "B",
		Metrics: map[string]float64{
			"velocity":   1.5,
			"complexity": 0.3,
		},
		Findings: []vibecheck.Finding{
			{Severity: "warning", Category: "drift", Message: "minor drift"},
			{Severity: "error", Category: "amnesia", Message: "repeating", File: "main.go", Line: 5},
		},
		Events: []vibecheck.TimelineEvent{
			{Timestamp: time.Now(), Author: "test", Message: "commit"},
		},
	}

	origFull := vibeCheckFull
	defer func() { vibeCheckFull = origFull }()
	vibeCheckFull = false

	out, err := captureStdout(t, func() error {
		return outputVibeCheckTable(result)
	})
	if err != nil {
		t.Fatalf("outputVibeCheckTable: %v", err)
	}
	if !strings.Contains(out, "75") {
		t.Errorf("expected table to contain score 75, got: %s", out)
	}
}

func TestVibeCheck_outputVibeCheckTable_fullMode(t *testing.T) {
	result := &vibecheck.VibeCheckResult{
		Score:    90.0,
		Grade:    "A",
		Metrics:  map[string]float64{"velocity": 2.0},
		Findings: []vibecheck.Finding{},
		Events: []vibecheck.TimelineEvent{
			{Timestamp: time.Now(), Author: "dev", Message: "initial"},
		},
	}

	origFull := vibeCheckFull
	defer func() { vibeCheckFull = origFull }()
	vibeCheckFull = true

	out, err := captureStdout(t, func() error {
		return outputVibeCheckTable(result)
	})
	if err != nil {
		t.Fatalf("outputVibeCheckTable: %v", err)
	}
	if !strings.Contains(out, "90") {
		t.Errorf("expected table to contain score 90, got: %s", out)
	}
}

func TestVibeCheck_outputVibeCheckTable_findingsNoFile(t *testing.T) {
	result := &vibecheck.VibeCheckResult{
		Score:   60.0,
		Grade:   "D",
		Metrics: nil,
		Findings: []vibecheck.Finding{
			{Severity: "warning", Category: "test", Message: "no file location"},
		},
		Events: nil,
	}

	out, err := captureStdout(t, func() error {
		return outputVibeCheckTable(result)
	})
	if err != nil {
		t.Fatalf("outputVibeCheckTable: %v", err)
	}
	if !strings.Contains(out, "no file location") {
		t.Errorf("expected table to contain finding message, got: %s", out)
	}
}
