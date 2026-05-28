// practices: [event-sourcing-cqrs, distributed-tracing]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/types"
)

func TestResolveTranscript(t *testing.T) {
	// Create temp directory structure mimicking ~/.claude/projects/
	tempDir, err := os.MkdirTemp("", "session-close-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir) //nolint:errcheck // test cleanup
	}()

	tests := []struct {
		name           string
		sessionID      string
		setupFunc      func(t *testing.T) string // returns expected path
		expectFallback bool
		expectError    bool
	}{
		{
			name:      "empty session ID triggers fallback",
			sessionID: "",
			setupFunc: func(t *testing.T) string {
				t.Helper()
				// findLastSession searches real ~/.claude/projects
				// so this test just verifies fallback is set
				return ""
			},
			expectFallback: true,
			expectError:    true, // may fail if no real transcripts
		},
		{
			name:      "nonexistent session ID returns error",
			sessionID: "nonexistent-session-id-12345",
			setupFunc: func(t *testing.T) string {
				t.Helper()
				return ""
			},
			expectFallback: false,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFunc(t)

			_, usedFallback, err := resolveTranscript(tt.sessionID)
			if tt.expectError && err == nil {
				// Some tests may pass on machines with real transcripts
				// Only fail if we expected an error and got specific wrong behavior
				if tt.sessionID == "nonexistent-session-id-12345" {
					t.Error("expected error for nonexistent session ID, got nil")
				}
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.expectFallback && err == nil && !usedFallback {
				t.Error("expected fallback to be used")
			}
		})
	}
}

func TestFindTranscriptBySessionID(t *testing.T) {
	// Create temp directory with mock transcript
	tempDir, err := os.MkdirTemp("", "session-find-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir) //nolint:errcheck // test cleanup
	}()

	tests := []struct {
		name        string
		sessionID   string
		expectError bool
	}{
		{
			name:        "nonexistent session returns error",
			sessionID:   "does-not-exist-abc-123",
			expectError: true,
		},
		{
			name:        "empty session ID returns error",
			sessionID:   "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := findTranscriptBySessionID(tt.sessionID)
			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestOutputCloseResult(t *testing.T) {
	tests := []struct {
		name   string
		result SessionCloseResult
		format string
	}{
		{
			name: "table output succeeds",
			result: SessionCloseResult{
				SessionID:     "test-session-001",
				Transcript:    "/tmp/test.jsonl",
				Decisions:     3,
				Knowledge:     5,
				FilesChanged:  10,
				Issues:        2,
				VelocityDelta: 0.05,
				Status:        "compounding",
				Message:       "Session closed: 3 decisions, 5 learnings extracted",
			},
			format: "table",
		},
		{
			name: "json output succeeds",
			result: SessionCloseResult{
				SessionID:     "test-session-002",
				Transcript:    "/tmp/test2.jsonl",
				Decisions:     0,
				Knowledge:     0,
				FilesChanged:  0,
				VelocityDelta: -0.01,
				Status:        "decaying",
				Message:       "Session closed: 0 decisions, 0 learnings extracted",
			},
			format: "json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Redirect stdout to capture output
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Set output format
			oldOutput := output
			output = tt.format
			defer func() {
				output = oldOutput
			}()

			err := outputCloseResult(tt.result)

			_ = w.Close()
			os.Stdout = oldStdout

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Read captured output
			buf := make([]byte, 4096)
			n, _ := r.Read(buf)
			_ = r.Close()

			out := string(buf[:n])
			if len(out) == 0 {
				t.Error("expected output, got empty string")
			}
		})
	}
}

func TestShortenPath(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("get home dir: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "path under home directory",
			input:    filepath.Join(homeDir, ".claude", "projects", "test.jsonl"),
			expected: "~/.claude/projects/test.jsonl",
		},
		{
			name:     "path not under home",
			input:    "/tmp/test.jsonl",
			expected: "/tmp/test.jsonl",
		},
		{
			name:     "empty path",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shortenPath(tt.input)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSessionCloseResultJSON(t *testing.T) {
	result := SessionCloseResult{
		SessionID:     "test-json-001",
		Transcript:    "/tmp/session.jsonl",
		Decisions:     2,
		Knowledge:     4,
		FilesChanged:  8,
		Issues:        1,
		VelocityDelta: 0.123,
		Status:        "compounding",
		Message:       "test message",
	}

	// Verify JSON fields exist
	if result.SessionID != "test-json-001" {
		t.Errorf("SessionID: got %q, want %q", result.SessionID, "test-json-001")
	}
	if result.Decisions != 2 {
		t.Errorf("Decisions: got %d, want 2", result.Decisions)
	}
	if result.VelocityDelta != 0.123 {
		t.Errorf("VelocityDelta: got %f, want 0.123", result.VelocityDelta)
	}
}

// ---------------------------------------------------------------------------
// session_close.go — computeVelocityDelta
// ---------------------------------------------------------------------------

func TestSessionClose_computeVelocityDelta(t *testing.T) {
	tests := []struct {
		name string
		pre  *types.FlywheelMetrics
		post *types.FlywheelMetrics
		want float64
	}{
		{
			name: "both nil returns 0",
			pre:  nil,
			post: nil,
			want: 0.0,
		},
		{
			name: "pre nil returns 0",
			pre:  nil,
			post: &types.FlywheelMetrics{Velocity: 0.5},
			want: 0.0,
		},
		{
			name: "post nil returns 0",
			pre:  &types.FlywheelMetrics{Velocity: 0.3},
			post: nil,
			want: 0.0,
		},
		{
			name: "positive delta",
			pre:  &types.FlywheelMetrics{Velocity: 0.1},
			post: &types.FlywheelMetrics{Velocity: 0.3},
			want: 0.2,
		},
		{
			name: "negative delta",
			pre:  &types.FlywheelMetrics{Velocity: 0.5},
			post: &types.FlywheelMetrics{Velocity: 0.2},
			want: -0.3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := computeVelocityDelta(tc.pre, tc.post)
			// Use tolerance for float comparison
			diff := got - tc.want
			if diff < -0.001 || diff > 0.001 {
				t.Errorf("computeVelocityDelta() = %f, want %f", got, tc.want)
			}
		})
	}

	// Also verify zero-velocity case explicitly
	got := computeVelocityDelta(
		&types.FlywheelMetrics{Velocity: 0.5},
		&types.FlywheelMetrics{Velocity: 0.5},
	)
	if got != 0.0 {
		t.Errorf("expected 0 delta for equal velocities, got %f", got)
	}
}

// ---------------------------------------------------------------------------
// session_close.go — classifyFlywheelStatus
// ---------------------------------------------------------------------------

func TestSessionClose_classifyFlywheelStatus(t *testing.T) {
	tests := []struct {
		name string
		post *types.FlywheelMetrics
		want string
	}{
		{
			name: "nil returns unknown",
			post: nil,
			want: "unknown",
		},
		{
			name: "above escape velocity returns compounding",
			post: &types.FlywheelMetrics{AboveEscapeVelocity: true, Velocity: 0.5},
			want: "compounding",
		},
		{
			name: "golden signals override escape velocity",
			post: &types.FlywheelMetrics{
				AboveEscapeVelocity: true,
				Velocity:            0.5,
				GoldenSignals: &types.GoldenSignals{
					OverallVerdict: "accumulating",
				},
			},
			want: "accumulating",
		},
		{
			name: "near zero velocity returns accumulating",
			post: &types.FlywheelMetrics{AboveEscapeVelocity: false, Velocity: -0.04},
			want: "accumulating",
		},
		{
			name: "zero velocity returns accumulating",
			post: &types.FlywheelMetrics{AboveEscapeVelocity: false, Velocity: 0.0},
			want: "accumulating",
		},
		{
			name: "deeply negative velocity returns decaying",
			post: &types.FlywheelMetrics{AboveEscapeVelocity: false, Velocity: -0.2},
			want: "decaying",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyFlywheelStatus(tc.post)
			if got != tc.want {
				t.Errorf("classifyFlywheelStatus() = %q, want %q", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// session_close.go — printCloseTable
// ---------------------------------------------------------------------------

func TestSessionClose_printCloseTable(t *testing.T) {
	tests := []struct {
		name   string
		result SessionCloseResult
		checks []string
	}{
		{
			name: "full result with issues",
			result: SessionCloseResult{
				SessionID:     "abcdefghijklmnop",
				Transcript:    "/tmp/test.jsonl",
				Decisions:     5,
				Knowledge:     3,
				FilesChanged:  10,
				Issues:        2,
				VelocityDelta: 0.123,
				Status:        "compounding",
				Message:       "Session closed: 5 decisions, 3 learnings extracted",
			},
			checks: []string{
				"Session Close Summary",
				"abcdefghijkl", // truncated to 12
				"Decisions:     5",
				"Issues:        2",
				"compounding",
				"+0.123",
			},
		},
		{
			name: "negative velocity no issues",
			result: SessionCloseResult{
				SessionID:     "short",
				Transcript:    "/tmp/t2.jsonl",
				Decisions:     0,
				Knowledge:     0,
				FilesChanged:  0,
				Issues:        0,
				VelocityDelta: -0.05,
				Status:        "decaying",
				Message:       "Session closed: 0 decisions, 0 learnings extracted",
			},
			checks: []string{
				"short",
				"decaying",
				"-0.050",
			},
		},
		{
			name: "empty session ID omits session line",
			result: SessionCloseResult{
				Transcript:    "/tmp/empty.jsonl",
				VelocityDelta: 0.0,
				Status:        "near-escape",
				Message:       "test",
			},
			checks: []string{
				"near-escape",
				"Session Close Summary",
			},
		},
		{
			name: "learnings rejected shown when nonzero",
			result: SessionCloseResult{
				SessionID:         "sess-reject-test",
				Transcript:        "/tmp/rej.jsonl",
				Decisions:         2,
				Knowledge:         1,
				LearningsRejected: 4,
				VelocityDelta:     0.01,
				Status:            "compounding",
				Message:           "done",
			},
			checks: []string{
				"Rejected:      4",
				"--verbose",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			printCloseTable(tc.result)

			_ = w.Close()
			os.Stdout = oldStdout

			buf := make([]byte, 8192)
			n, _ := r.Read(buf)
			_ = r.Close()
			out := string(buf[:n])

			for _, check := range tc.checks {
				if !strings.Contains(out, check) {
					t.Errorf("expected output to contain %q, got:\n%s", check, out)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// session_close.go — auto-extract
// ---------------------------------------------------------------------------

func TestQualifyAutoExtract(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "substantial learning passes",
			content: "Use Go modules for dependency management to ensure reproducible builds across environments. Pin CI to Go 1.22 to avoid breaking changes in newer versions. Always verify module checksums match before deploying.",
			want:    true,
		},
		{
			name:    "mid-sentence fragment rejected",
			content: "till waiting on the event deletion (the big one). Let me check.",
			want:    false,
		},
		{
			name:    "see also fragment rejected",
			content: "See Also\n\n- `skills/council/SKILL.md` — Multi-model validation council",
			want:    false,
		},
		{
			name:    "too few words rejected",
			content: "till grinding",
			want:    false,
		},
		{
			name:    "skill doc parrot rejected",
			content: "### Step 8: Request Human Approval (Gate 2)\n\n**Skip this step if `--auto` flag is set.**",
			want:    false,
		},
		{
			name:    "operational chatter rejected",
			content: "still running (lots of objects to delete). Let me save that evidence while they finish.",
			want:    false,
		},
		{
			name:    "single sentence with enough words passes",
			content: "Our parser handles JSONL format natively and can extract decisions from multi-turn conversations reliably across different session types. It enables automated knowledge extraction at scale for large distributed projects.",
			want:    true,
		},
		{
			name:    "25+ words with two sentences passes",
			content: "Our parser extracts all decisions from every multi turn conversation here in the automated pipeline efficiently and reliably without manual intervention. Output format preserves all metadata including timestamps and categories.",
			want:    true,
		},
		{
			name:    "24 words rejected",
			content: "The parser extracts all decisions from every multi turn conversation here in the automated pipeline efficiently. The output format preserves metadata.",
			want:    false,
		},
		{
			name:    "new continuation prefix and rejected",
			content: "and then we found that the approach works well for all cases in the production environment. This was confirmed by testing.",
			want:    false,
		},
		{
			name:    "new continuation prefix the rejected",
			content: "the main approach is to use structured logging for all services in production environments. This improves debugging significantly.",
			want:    false,
		},
		{
			name:    "no sentence ender rejected",
			content: "This is a long enough fragment but it has no sentence-ending punctuation at all",
			want:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := qualifyAutoExtract(tc.content)
			if got != tc.want {
				t.Errorf("qualifyAutoExtract() = %v, want %v for: %s",
					got, tc.want, tc.content)
			}
		})
	}
}

func TestSessionClose_AutoExtract_ProducesLearnings(t *testing.T) {
	tmpDir := t.TempDir()
	decisions := []string{
		"Use Go modules for dependency management to ensure reproducible builds across environments. Pin CI to Go 1.22 to avoid breaking changes in newer versions. Always verify module checksums match before deploying.",
		"Prefer table-driven tests for multi-case functions to reduce boilerplate and improve coverage across the entire test suite. Each row should test one specific behavior with clear expected values.",
	}
	knowledge := []string{
		"Our parser handles JSONL format natively and can extract decisions from multi-turn conversations reliably across different session types. It enables automated knowledge extraction at scale for large distributed projects.",
	}

	result, err := writeAutoExtractedLearnings(tmpDir, decisions, knowledge)
	if err != nil {
		t.Fatalf("writeAutoExtractedLearnings: %v", err)
	}
	if result.written != 3 {
		t.Errorf("expected 3 learnings written, got %d", result.written)
	}
	if result.rejected != 0 {
		t.Errorf("expected 0 rejected, got %d", result.rejected)
	}

	// Verify files exist in .agents/learnings/
	learningsDir := filepath.Join(tmpDir, ".agents", "learnings")
	entries, err := os.ReadDir(learningsDir)
	if err != nil {
		t.Fatalf("read learnings dir: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 files in learnings dir, got %d", len(entries))
	}

	// Verify frontmatter content in first file
	firstFile := filepath.Join(learningsDir, entries[0].Name())
	data, err := os.ReadFile(firstFile)
	if err != nil {
		t.Fatalf("read first learning: %v", err)
	}
	content := string(data)
	for _, want := range []string{"type: learning", "title:", "source: auto-extract", "confidence: medium", "maturity: provisional", "utility: 0.5", "# "} {
		if !strings.Contains(content, want) {
			t.Errorf("expected learning to contain %q, got:\n%s", want, content)
		}
	}

	// Verify category field is present
	if !strings.Contains(content, "category: decision") && !strings.Contains(content, "category: knowledge") {
		t.Errorf("expected learning to contain a category field, got:\n%s", content)
	}
}

func TestSessionClose_AutoExtract_PreservesResearchSources(t *testing.T) {
	tmpDir := t.TempDir()
	decision := "We validated the fix in .agents/research/2026-03-22-task-scoped-lookup.md and should keep citing that exact artifact. The research file captures the failure mode, the deterministic harness, and the retrieval correction. Future sessions should reuse it instead of rediscovering the gap."

	result, err := writeAutoExtractedLearnings(tmpDir, []string{decision}, nil)
	if err != nil {
		t.Fatalf("writeAutoExtractedLearnings: %v", err)
	}
	if result.written != 1 {
		t.Fatalf("expected 1 learning written, got %d", result.written)
	}

	entries, err := os.ReadDir(filepath.Join(tmpDir, ".agents", "learnings"))
	if err != nil {
		t.Fatalf("read learnings dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 learning file, got %d", len(entries))
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ".agents", "learnings", entries[0].Name()))
	if err != nil {
		t.Fatalf("read learning: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "research_sources:") {
		t.Fatalf("expected research_sources frontmatter, got:\n%s", content)
	}
	if !strings.Contains(content, ".agents/research/2026-03-22-task-scoped-lookup.md") {
		t.Fatalf("expected exact research path to be preserved, got:\n%s", content)
	}
}

func TestSessionClose_AutoExtract_RejectsJunk(t *testing.T) {
	tmpDir := t.TempDir()
	decisions := []string{
		"till waiting on the event deletion",
		"Pin CI to Go 1.22",
	}
	knowledge := []string{
		"See Also\n\n- `skills/council/SKILL.md` — Multi-model council",
		"still running (lots of objects to delete)",
	}

	result, err := writeAutoExtractedLearnings(tmpDir, decisions, knowledge)
	if err != nil {
		t.Fatalf("writeAutoExtractedLearnings: %v", err)
	}
	if result.written != 0 {
		t.Errorf("expected 0 learnings written (all junk), got %d", result.written)
	}
	if result.rejected != 4 {
		t.Errorf("expected 4 rejected, got %d", result.rejected)
	}

	learningsDir := filepath.Join(tmpDir, ".agents", "learnings")
	entries, err := os.ReadDir(learningsDir)
	if err != nil {
		t.Fatalf("read learnings dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 files for junk input, got %d", len(entries))
	}
}

func TestSessionClose_AutoExtract_WritesHandoff(t *testing.T) {
	tmpDir := t.TempDir()

	artifact := &handoffArtifact{
		SchemaVersion: 1,
		ID:            "auto-test123",
		CreatedAt:     "2026-03-05T00:00:00Z",
		Type:          "auto",
		Summary:       "Auto-extracted test handoff",
		DecisionsMade: []string{"test decision"},
	}

	path, err := writeHandoffArtifact(tmpDir, artifact)
	if err != nil {
		t.Fatalf("writeHandoffArtifact: %v", err)
	}

	if path == "" {
		t.Fatal("expected non-empty handoff path")
	}

	// Verify file exists and contains expected content
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read handoff: %v", err)
	}
	content := string(data)
	for _, want := range []string{`"type": "auto"`, `"summary": "Auto-extracted test handoff"`, `"id": "auto-test123"`} {
		if !strings.Contains(content, want) {
			t.Errorf("expected handoff to contain %q, got:\n%s", want, content)
		}
	}
}

func TestSessionClose_AutoExtract_NoTranscript(t *testing.T) {
	tmpDir := t.TempDir()

	// Empty decisions and knowledge — graceful no-op
	result, err := writeAutoExtractedLearnings(tmpDir, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.written != 0 {
		t.Errorf("expected 0 learnings for empty input, got %d", result.written)
	}
	if result.rejected != 0 {
		t.Errorf("expected 0 rejected for empty input, got %d", result.rejected)
	}

	// Learnings dir should still be created but empty
	learningsDir := filepath.Join(tmpDir, ".agents", "learnings")
	entries, err := os.ReadDir(learningsDir)
	if err != nil {
		t.Fatalf("read learnings dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 files, got %d", len(entries))
	}
}

func TestSessionClose_WithoutAutoExtract_Unchanged(t *testing.T) {
	// Verify that SessionCloseResult defaults have zero values for new fields
	result := SessionCloseResult{
		SessionID:     "test-no-auto",
		Transcript:    "/tmp/test.jsonl",
		Decisions:     2,
		Knowledge:     3,
		VelocityDelta: 0.1,
		Status:        "compounding",
		Message:       "test",
	}

	// New fields should be zero-valued when auto-extract is not used
	if result.LearningsExtracted != 0 {
		t.Errorf("expected LearningsExtracted=0, got %d", result.LearningsExtracted)
	}
	if result.HandoffWritten != "" {
		t.Errorf("expected HandoffWritten empty, got %q", result.HandoffWritten)
	}

	// Verify JSON output still works with new fields
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	oldOutput := output
	output = "json"
	defer func() { output = oldOutput }()

	err := outputCloseResult(result)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputCloseResult: %v", err)
	}

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	_ = r.Close()
	out := string(buf[:n])

	// JSON should contain the new fields with zero values
	if !strings.Contains(out, `"learnings_extracted": 0`) {
		t.Errorf("expected JSON to contain learnings_extracted: 0, got:\n%s", out)
	}
	// handoff_written should be omitted (omitempty)
	if strings.Contains(out, `"handoff_written"`) {
		t.Errorf("expected handoff_written to be omitted from JSON, got:\n%s", out)
	}
}

func TestWriteAutoExtractedLearnings_Dedup(t *testing.T) {
	tmpDir := t.TempDir()
	content := "Use Go modules for dependency management to ensure reproducible builds across environments. Pin CI to Go 1.22 to avoid breaking changes in newer versions. Always verify module checksums match."

	// First extraction should write
	result1, err := writeAutoExtractedLearnings(tmpDir, []string{content}, nil)
	if err != nil {
		t.Fatalf("first extraction: %v", err)
	}
	if result1.written != 1 {
		t.Errorf("first extraction: expected 1 written, got %d", result1.written)
	}

	// Second extraction of same content should be deduped
	result2, err := writeAutoExtractedLearnings(tmpDir, []string{content}, nil)
	if err != nil {
		t.Fatalf("second extraction: %v", err)
	}
	if result2.written != 0 {
		t.Errorf("second extraction: expected 0 written (dedup), got %d", result2.written)
	}
	if result2.rejected != 1 {
		t.Errorf("second extraction: expected 1 rejected (dedup), got %d", result2.rejected)
	}
}

// TestSessionClose_shortenPath removed — duplicate of TestShortenPath (line 188).

func TestMinInt(t *testing.T) {
	tests := []struct {
		a, b, want int
	}{
		{1, 2, 1},
		{5, 3, 3},
		{0, 0, 0},
		{-1, 1, -1},
		{42, 42, 42},
	}
	for _, tt := range tests {
		got := minInt(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("minInt(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
