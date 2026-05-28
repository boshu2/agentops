// practices: [wiki-knowledge-surface, lean-startup]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestForgeTranscript_Integration_ValidTranscript(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	// Create a sample JSONL transcript with chat messages
	transcript := `{"type":"user","role":"user","content":"How do I implement retry logic?","timestamp":"2026-04-01T10:00:00Z"}
{"type":"assistant","role":"assistant","content":"Use exponential backoff. Decision: we should use a jitter-based retry with max 3 attempts.","timestamp":"2026-04-01T10:00:05Z"}
{"type":"user","role":"user","content":"What about circuit breakers?","timestamp":"2026-04-01T10:01:00Z"}
{"type":"assistant","role":"assistant","content":"Solution: implement circuit breaker with 5-failure threshold and 30s cooldown.","timestamp":"2026-04-01T10:01:05Z"}
`
	// Filename must contain a UUID or ses_ prefix for session ID inference
	transcriptPath := filepath.Join(dir, "ses_abc123def456.jsonl")
	writeFile(t, transcriptPath, transcript)

	oldQuiet := forgeQuiet
	oldLastSession := forgeLastSession
	oldQueue := forgeQueue
	t.Cleanup(func() {
		forgeQuiet = oldQuiet
		forgeLastSession = oldLastSession
		forgeQueue = oldQueue
	})
	forgeQuiet = false
	forgeLastSession = false
	forgeQueue = false

	_, err := captureStdout(t, func() error {
		return runForgeTranscript(forgeTranscriptCmd, []string{transcriptPath})
	})
	if err != nil {
		t.Fatalf("forge transcript returned error: %v", err)
	}

	// Verify sessions directory was populated
	sessionsDir := filepath.Join(dir, ".agents", "ao", "sessions")
	entries, readErr := os.ReadDir(sessionsDir)
	if readErr != nil {
		t.Fatalf("failed to read sessions dir: %v", readErr)
	}
	if len(entries) == 0 {
		t.Error("expected at least one session file in .agents/ao/sessions/")
	}

	// Verify at least one session file contains expected content
	found := false
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(sessionsDir, entry.Name()))
		if err != nil {
			continue
		}
		content := string(data)
		if strings.Contains(content, "retry") || strings.Contains(content, "circuit") ||
			strings.Contains(content, "Decision") || strings.Contains(content, "Solution") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected forged session to contain knowledge from transcript")
	}
}

func TestForgeTranscript_Integration_EmptyTranscript(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	// Create an empty JSONL transcript (no chat messages)
	transcriptPath := filepath.Join(dir, "empty-session.jsonl")
	writeFile(t, transcriptPath, "{\"type\":\"system\",\"content\":\"init\"}\n")

	oldQuiet := forgeQuiet
	oldLastSession := forgeLastSession
	oldQueue := forgeQueue
	t.Cleanup(func() {
		forgeQuiet = oldQuiet
		forgeLastSession = oldLastSession
		forgeQueue = oldQueue
	})
	forgeQuiet = false
	forgeLastSession = false
	forgeQueue = false

	// Should not error -- just skip the transcript with no chat messages
	_, err := captureStdout(t, func() error {
		return runForgeTranscript(forgeTranscriptCmd, []string{transcriptPath})
	})
	if err != nil {
		t.Fatalf("forge transcript on empty transcript returned error: %v", err)
	}
}

func TestForgeTranscript_Integration_NoFiles(t *testing.T) {
	_ = chdirTemp(t)

	oldQuiet := forgeQuiet
	oldLastSession := forgeLastSession
	oldQueue := forgeQueue
	t.Cleanup(func() {
		forgeQuiet = oldQuiet
		forgeLastSession = oldLastSession
		forgeQueue = oldQueue
	})
	forgeQuiet = false
	forgeLastSession = false
	forgeQueue = false

	err := runForgeTranscript(forgeTranscriptCmd, []string{filepath.Join(os.TempDir(), "nonexistent-*.jsonl")})
	if err == nil {
		t.Error("expected error when no files match pattern")
	}
	if err != nil && !strings.Contains(err.Error(), "no files found") {
		t.Errorf("expected 'no files found' error, got: %v", err)
	}
}
