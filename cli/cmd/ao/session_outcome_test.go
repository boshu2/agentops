package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeTranscript(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "session-outcome-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir) //nolint:errcheck // test cleanup
	}()

	tests := []struct {
		name           string
		content        string
		expectedReward float64
		minReward      float64
		maxReward      float64
	}{
		{
			name: "successful session with tests and push",
			content: `{"type": "user", "sessionId": "test-001", "message": {"content": "run tests"}}
{"type": "assistant", "message": {"content": "Running tests..."}}
{"type": "tool_result", "content": "PASSED 5 tests in 1.2s"}
{"type": "assistant", "message": {"content": "All tests passed. Committing..."}}
{"type": "tool_result", "content": "[main abc1234] feat: add feature\n 3 files changed, 100 insertions(+)"}
{"type": "assistant", "message": {"content": "Pushing to remote..."}}
{"type": "tool_result", "content": "Enumerating objects: 5, done.\nWriting objects: 100% (5/5), done."}`,
			minReward: 0.6, // tests + commit + push + no errors
			maxReward: 1.0,
		},
		{
			name: "failed tests",
			content: `{"type": "user", "sessionId": "test-002", "message": {"content": "run tests"}}
{"type": "tool_result", "content": "FAILED 2 tests"}
{"type": "tool_result", "content": "exit code: 1"}`,
			minReward: 0.0, // test failure penalty
			maxReward: 0.3,
		},
		{
			name: "session with python traceback",
			content: `{"type": "user", "sessionId": "test-003", "message": {"content": "run script"}}
{"type": "tool_result", "content": "Traceback (most recent call last):\n  File \"test.py\", line 10, in <module>\n    raise ValueError()"}`,
			minReward: 0.0,
			maxReward: 0.4, // exception penalty
		},
		{
			name: "session with beads close and ratchet",
			content: `{"type": "user", "sessionId": "test-004", "message": {"content": "close issue"}}
{"type": "tool_result", "content": "bd close ol-0001 succeeded\nissue ol-0001 closed"}
{"type": "tool_result", "content": "ao ratchet record succeeded\nRatchet chain updated"}`,
			minReward: 0.2, // beads + ratchet
			maxReward: 0.5,
		},
		{
			name: "minimal session - no activity",
			content: `{"type": "user", "sessionId": "test-005", "message": {"content": "hello"}}
{"type": "assistant", "message": {"content": "Hello! How can I help?"}}`,
			minReward: 0.0, // no commit penalty
			maxReward: 0.2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write transcript
			transcriptPath := filepath.Join(tempDir, tt.name+".jsonl")
			if err := os.WriteFile(transcriptPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("write transcript: %v", err)
			}

			// Analyze
			outcome, err := analyzeTranscript(transcriptPath, "")
			if err != nil {
				t.Fatalf("analyze transcript: %v", err)
			}

			// Check reward bounds
			if outcome.Reward < tt.minReward {
				t.Errorf("reward %.2f below minimum %.2f", outcome.Reward, tt.minReward)
			}
			if outcome.Reward > tt.maxReward {
				t.Errorf("reward %.2f above maximum %.2f", outcome.Reward, tt.maxReward)
			}

			// Check session ID extraction
			if outcome.SessionID == "" {
				t.Error("session ID not extracted")
			}
		})
	}
}

func TestExtractSessionID(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{
			name:     "camelCase sessionId",
			line:     `{"type": "user", "sessionId": "abc-123"}`,
			expected: "abc-123",
		},
		{
			name:     "snake_case session_id",
			line:     `{"type": "user", "session_id": "def-456"}`,
			expected: "def-456",
		},
		{
			name:     "no session ID",
			line:     `{"type": "user", "message": "hello"}`,
			expected: "",
		},
		{
			name:     "invalid JSON",
			line:     `not json at all`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSessionID(tt.line)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFindMostRecentTranscript(t *testing.T) {
	// Create temp directory structure
	tempDir, err := os.MkdirTemp("", "transcript-find-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir) //nolint:errcheck // test cleanup
	}()

	// Create nested directories and files
	projectDir := filepath.Join(tempDir, "project-abc")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("create project dir: %v", err)
	}

	// Create older transcript
	olderPath := filepath.Join(projectDir, "old.jsonl")
	if err := os.WriteFile(olderPath, []byte(`{"type":"user"}`), 0644); err != nil {
		t.Fatalf("write older: %v", err)
	}

	// Create newer transcript
	newerPath := filepath.Join(projectDir, "new.jsonl")
	if err := os.WriteFile(newerPath, []byte(`{"type":"user"}`), 0644); err != nil {
		t.Fatalf("write newer: %v", err)
	}

	// Find most recent
	found := findMostRecentTranscript(tempDir)
	if found == "" {
		t.Fatal("no transcript found")
	}
	if filepath.Base(found) != "new.jsonl" {
		t.Errorf("got %s, want new.jsonl", filepath.Base(found))
	}
}

func TestFindTranscriptForSession(t *testing.T) {
	tempDir := t.TempDir()

	projectDir := filepath.Join(tempDir, "project-x")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("create project dir: %v", err)
	}

	otherPath := filepath.Join(projectDir, "other.jsonl")
	matchPath := filepath.Join(projectDir, "match.jsonl")
	if err := os.WriteFile(otherPath, []byte(`{"type":"user","sessionId":"other-session"}`), 0644); err != nil {
		t.Fatalf("write other transcript: %v", err)
	}
	if err := os.WriteFile(matchPath, []byte(`{"type":"user","sessionId":"target-session"}`), 0644); err != nil {
		t.Fatalf("write match transcript: %v", err)
	}

	found := findTranscriptForSession(tempDir, "target-session")
	if found == "" {
		t.Fatal("expected a matching transcript")
	}
	if filepath.Base(found) != "match.jsonl" {
		t.Errorf("got %q, want match.jsonl", filepath.Base(found))
	}
}

func TestTranscriptContainsSessionID(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "session.jsonl")
	content := `{"type":"user","sessionId":"abc-123"}
{"type":"assistant","message":{"content":"done"}}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	if !transcriptContainsSessionID(path, "abc-123") {
		t.Fatal("expected transcriptContainsSessionID to return true")
	}
	if transcriptContainsSessionID(path, "missing-session") {
		t.Fatal("expected transcriptContainsSessionID to return false")
	}
}

func TestClampReward(t *testing.T) {
	tests := []struct {
		name   string
		input  float64
		expect float64
	}{
		{"negative clamped to zero", -0.5, 0},
		{"zero stays zero", 0, 0},
		{"mid-range unchanged", 0.5, 0.5},
		{"one stays one", 1.0, 1.0},
		{"above one clamped", 1.5, 1.0},
		{"large negative clamped", -100.0, 0},
		{"large positive clamped", 100.0, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampReward(tt.input)
			if got != tt.expect {
				t.Errorf("clampReward(%v) = %v, want %v", tt.input, got, tt.expect)
			}
		})
	}
}

func TestFindTranscriptForSession_EmptySessionID(t *testing.T) {
	tmpDir := t.TempDir()
	got := findTranscriptForSession(tmpDir, "")
	if got != "" {
		t.Errorf("expected empty string for blank session ID, got %q", got)
	}
	got = findTranscriptForSession(tmpDir, "   ")
	if got != "" {
		t.Errorf("expected empty string for whitespace session ID, got %q", got)
	}
}

func TestFindTranscriptForSession_NoMatchingTranscript(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project-y")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "sess.jsonl"),
		[]byte(`{"type":"user","sessionId":"other-id"}`), 0644); err != nil {
		t.Fatal(err)
	}
	got := findTranscriptForSession(tmpDir, "nonexistent-session")
	if got != "" {
		t.Errorf("expected empty for non-matching session, got %q", got)
	}
}

func TestFindMostRecentTranscript_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	got := findMostRecentTranscript(tmpDir)
	if got != "" {
		t.Errorf("expected empty for dir with no .jsonl files, got %q", got)
	}
}

func TestFindMostRecentTranscript_NonexistentDir(t *testing.T) {
	got := findMostRecentTranscript("/nonexistent/path/xyz")
	if got != "" {
		t.Errorf("expected empty for nonexistent dir, got %q", got)
	}
}

func TestAnalyzeTranscript_EmptySessionIDGenerated(t *testing.T) {
	tmpDir := t.TempDir()
	// Transcript with no sessionId field — should generate one
	content := `{"type":"user","message":{"content":"hello"}}
{"type":"assistant","message":{"content":"hi"}}`
	path := filepath.Join(tmpDir, "no-session.jsonl")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	outcome, err := analyzeTranscript(path, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome.SessionID == "" {
		t.Error("expected generated session ID, got empty")
	}
	if outcome.TotalLines != 2 {
		t.Errorf("expected 2 lines, got %d", outcome.TotalLines)
	}
}

func TestAnalyzeTranscript_NonexistentFile(t *testing.T) {
	_, err := analyzeTranscript("/nonexistent/file.jsonl", "")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestTranscriptContainsSessionID_NonexistentFile(t *testing.T) {
	got := transcriptContainsSessionID("/nonexistent/file.jsonl", "abc")
	if got {
		t.Error("expected false for nonexistent file")
	}
}

func TestTranscriptContainsSessionID_SubstringMatch(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sub.jsonl")
	// Contains the session ID as a substring in a non-sessionId field
	content := `{"type":"user","message":{"content":"session target-sess happened"}}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	// Should match via strings.Contains fallback
	got := transcriptContainsSessionID(path, "target-sess")
	if !got {
		t.Error("expected true for substring match")
	}
}

func TestAddSignalWithEvidence_OverridesRegex(t *testing.T) {
	var signals []Signal
	var reward float64
	addSignalWithEvidence(&signals, &reward, "tests_pass", false, true, weightTestsPass)
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d: %#v", len(signals), signals)
	}
	if signals[0].Name != "tests_pass" {
		t.Fatalf("name = %q, want tests_pass", signals[0].Name)
	}
	if !signals[0].Value {
		t.Fatalf("expected Value=true (evidence override), got false: %#v", signals[0])
	}
	if signals[0].Weight != weightTestsPass {
		t.Fatalf("weight = %v, want %v", signals[0].Weight, weightTestsPass)
	}
	if reward != weightTestsPass {
		t.Fatalf("reward = %v, want %v", reward, weightTestsPass)
	}
}

func TestAddSignalWithEvidence_BothFalse(t *testing.T) {
	var signals []Signal
	var reward float64
	addSignalWithEvidence(&signals, &reward, "tests_pass", false, false, weightTestsPass)
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d: %#v", len(signals), signals)
	}
	if signals[0].Value {
		t.Fatalf("expected Value=false, got true: %#v", signals[0])
	}
	if signals[0].Weight != 0 {
		t.Fatalf("weight = %v, want 0 (not found)", signals[0].Weight)
	}
	if reward != 0 {
		t.Fatalf("reward = %v, want 0", reward)
	}
}

func TestAddSignalWithEvidence_RegexAlone(t *testing.T) {
	var signals []Signal
	var reward float64
	addSignalWithEvidence(&signals, &reward, "tests_pass", true, false, weightTestsPass)
	if len(signals) != 1 || !signals[0].Value {
		t.Fatalf("expected found via regex: %#v", signals)
	}
	if reward != weightTestsPass {
		t.Fatalf("reward = %v, want %v", reward, weightTestsPass)
	}
}

func TestSignalWeights(t *testing.T) {
	// Verify weights sum to <= 1.0 for positive signals
	positiveSum := weightTestsPass + weightGitPush + weightGitCommit +
		weightBeadsClosed + weightRatchetLock + weightNoErrors

	if positiveSum > 1.0 {
		t.Errorf("positive weights sum to %.2f (should be <= 1.0)", positiveSum)
	}

	// Verify penalty weights are positive values
	if penaltyTestFailure <= 0 || penaltyException <= 0 || penaltyNoCommit <= 0 {
		t.Error("penalty weights should be positive values")
	}
}
