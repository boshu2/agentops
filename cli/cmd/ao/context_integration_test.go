// practices: [ai-assisted-dev, distributed-tracing]
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContextAssemble_Integration_FullAgents(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	// Populate .agents/ with sample content
	writeFile(t, filepath.Join(dir, ".agents", "learnings", "retry-backoff.md"),
		"---\ntitle: Retry Backoff\n---\nExponential backoff prevents thundering herd.\n")
	writeFile(t, filepath.Join(dir, ".agents", "patterns", "circuit-breaker.md"),
		"---\nname: Circuit Breaker\n---\nTrip on 5 consecutive failures.\n")
	writeFile(t, filepath.Join(dir, ".agents", "research", "auth-research.md"),
		"# Auth Research\nOAuth2 PKCE flow recommended for SPAs.\n")
	writeFile(t, filepath.Join(dir, ".agents", "plans", "sprint-1.md"),
		"# Sprint 1\n- Implement auth middleware\n- Add rate limiter\n")
	writeFile(t, filepath.Join(dir, "GOALS.md"),
		"# Goals\n- Ship auth by Friday\n")

	oldTask := assembleTask
	oldPhase := assemblePhase
	oldMaxChars := assembleMaxChars
	oldOutput := assembleOutput
	t.Cleanup(func() {
		assembleTask = oldTask
		assemblePhase = oldPhase
		assembleMaxChars = oldMaxChars
		assembleOutput = oldOutput
	})

	outFile := filepath.Join(dir, "briefing.md")
	assembleTask = "Implement auth middleware"
	assemblePhase = "task"
	assembleMaxChars = 50000
	assembleOutput = outFile

	var buf bytes.Buffer
	contextCmd.SetOut(&buf)
	t.Cleanup(func() { contextCmd.SetOut(nil) })
	err := runContextAssemble(contextCmd, []string{})
	out := buf.String()
	if err != nil {
		t.Fatalf("context assemble returned error: %v", err)
	}

	if !strings.Contains(out, "Briefing written to") {
		t.Errorf("expected 'Briefing written to' confirmation, got: %s", out)
	}

	data, readErr := os.ReadFile(outFile)
	if readErr != nil {
		t.Fatalf("failed to read briefing output: %v", readErr)
	}
	briefing := string(data)
	if !strings.Contains(briefing, "TASK") {
		t.Errorf("expected briefing to contain TASK section, got: %s", briefing)
	}
}

func TestContextAssemble_Integration_MinimalAgents(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)
	// No extra files -- minimal .agents/

	oldTask := assembleTask
	oldPhase := assemblePhase
	oldMaxChars := assembleMaxChars
	oldOutput := assembleOutput
	t.Cleanup(func() {
		assembleTask = oldTask
		assemblePhase = oldPhase
		assembleMaxChars = oldMaxChars
		assembleOutput = oldOutput
	})

	outFile := filepath.Join(dir, "briefing-minimal.md")
	assembleTask = "Add unit tests"
	assemblePhase = "task"
	assembleMaxChars = 50000
	assembleOutput = outFile

	var buf bytes.Buffer
	contextCmd.SetOut(&buf)
	t.Cleanup(func() { contextCmd.SetOut(nil) })
	err := runContextAssemble(contextCmd, []string{})
	out := buf.String()
	if err != nil {
		t.Fatalf("context assemble returned error: %v", err)
	}

	if !strings.Contains(out, "Briefing written to") {
		t.Errorf("expected 'Briefing written to' confirmation, got: %s", out)
	}

	data, readErr := os.ReadFile(outFile)
	if readErr != nil {
		t.Fatalf("failed to read briefing output: %v", readErr)
	}
	if len(data) == 0 {
		t.Error("expected non-empty briefing for minimal .agents/")
	}
}

func TestContextAssemble_Integration_Empty(t *testing.T) {
	dir := chdirTemp(t)
	// No .agents/ directory at all

	oldTask := assembleTask
	oldPhase := assemblePhase
	oldMaxChars := assembleMaxChars
	oldOutput := assembleOutput
	t.Cleanup(func() {
		assembleTask = oldTask
		assemblePhase = oldPhase
		assembleMaxChars = oldMaxChars
		assembleOutput = oldOutput
	})

	outFile := filepath.Join(dir, "briefing-empty.md")
	assembleTask = "Bootstrap project"
	assemblePhase = "task"
	assembleMaxChars = 50000
	assembleOutput = outFile

	var buf bytes.Buffer
	contextCmd.SetOut(&buf)
	t.Cleanup(func() { contextCmd.SetOut(nil) })
	err := runContextAssemble(contextCmd, []string{})
	// Should not error even with no .agents/ directory
	if err != nil {
		t.Fatalf("context assemble returned error on empty dir: %v", err)
	}

	data, readErr := os.ReadFile(outFile)
	if readErr != nil {
		t.Fatalf("failed to read briefing output: %v", readErr)
	}
	if len(data) == 0 {
		t.Error("expected non-empty briefing even with empty directory")
	}
}
