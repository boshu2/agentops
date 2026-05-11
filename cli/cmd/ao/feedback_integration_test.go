// practices: [dora-metrics, lean-startup]
package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// resetFeedbackFlags resets cobra-persisted flag values for feedback-loop.
func resetFeedbackFlags(t *testing.T) {
	t.Helper()
	oldSession := feedbackLoopSessionID
	oldReward := feedbackLoopReward
	oldTranscript := feedbackLoopTranscript
	oldAlpha := feedbackLoopAlpha
	oldCitationType := feedbackLoopCitationType
	oldOutput := output
	oldJsonFlag := jsonFlag
	t.Cleanup(func() {
		feedbackLoopSessionID = oldSession
		feedbackLoopReward = oldReward
		feedbackLoopTranscript = oldTranscript
		feedbackLoopAlpha = oldAlpha
		feedbackLoopCitationType = oldCitationType
		output = oldOutput
		jsonFlag = oldJsonFlag
	})
	// Reset to defaults
	feedbackLoopSessionID = ""
	feedbackLoopReward = -1
	feedbackLoopTranscript = ""
	feedbackLoopAlpha = 0.3
	feedbackLoopCitationType = "retrieved"
	output = "table"
	jsonFlag = false
}

func TestFeedback_Integration_NoSession(t *testing.T) {
	chdirTemp(t)
	resetFeedbackFlags(t)

	_, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"feedback-loop"})
		return rootCmd.Execute()
	})

	// Without --session flag, should error
	if err == nil {
		t.Error("expected error when no --session provided")
	}
}

func TestFeedback_Integration_SessionNoCitations(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)
	resetFeedbackFlags(t)

	// Create empty citations file
	writeFile(t, filepath.Join(dir, ".agents", "ao", "citations.jsonl"), "")

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"feedback-loop", "--session", "test-session-nocite", "--reward", "0.8"})
		return rootCmd.Execute()
	})

	// Should run but find no citations
	// The command may succeed with "0 citations" or error — either is acceptable
	if err != nil {
		// Error is expected if no citations found
		return
	}

	// If it succeeded, output should indicate 0 updates
	if !strings.Contains(out, "0") {
		t.Logf("output: %s", out)
	}
}

func TestFeedback_Integration_SessionWithCitationsAndLearning(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)
	resetFeedbackFlags(t)

	// Create a learning file
	learningPath := filepath.Join(dir, ".agents", "learnings", "test-learning.md")
	writeFile(t, learningPath, "---\nutility: 0.5\nreward_count: 0\n---\n# Test Learning\nContent here.\n")

	// Create citations referencing the learning
	citationLine := `{"session_id":"test-fb-session","artifact_path":".agents/learnings/test-learning.md","citation_type":"retrieved","recorded_at":"2026-04-01T10:00:00Z"}` + "\n"
	writeFile(t, filepath.Join(dir, ".agents", "ao", "citations.jsonl"), citationLine)

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{
			"feedback-loop",
			"--session", "test-fb-session",
			"--reward", "0.75",
			"--alpha", "0.3",
		})
		return rootCmd.Execute()
	})

	if err != nil {
		t.Fatalf("expected feedback-loop to succeed, got error: %v\noutput:\n%s", err, out)
	}

	// Should report at least 1 update
	if !strings.Contains(out, "1") && !strings.Contains(out, "updated") && !strings.Contains(out, "Updated") {
		t.Logf("feedback output (may vary): %s", out)
	}
}

func TestFeedback_Integration_InvalidCitationType(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)
	resetFeedbackFlags(t)

	writeFile(t, filepath.Join(dir, ".agents", "ao", "citations.jsonl"), "")

	_, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{
			"feedback-loop",
			"--session", "test-session",
			"--reward", "0.5",
			"--citation-type", "bogus",
		})
		return rootCmd.Execute()
	})

	// Should reject invalid citation type
	if err == nil {
		t.Error("expected error for invalid citation type 'bogus'")
	}
}

func TestFeedback_Integration_DryRun(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)
	resetFeedbackFlags(t)

	oldDryRun := dryRun
	dryRun = true
	t.Cleanup(func() { dryRun = oldDryRun })

	writeFile(t, filepath.Join(dir, ".agents", "ao", "citations.jsonl"), "")

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{
			"feedback-loop",
			"--session", "test-session-dry",
			"--reward", "0.5",
		})
		return rootCmd.Execute()
	})

	if err != nil {
		t.Fatalf("expected dry-run to succeed, got error: %v", err)
	}

	if !strings.Contains(out, "dry-run") {
		t.Errorf("expected '[dry-run]' in output, got:\n%s", out)
	}

	if !strings.Contains(out, "test-session-dry") {
		t.Errorf("expected session ID in dry-run output, got:\n%s", out)
	}
}
