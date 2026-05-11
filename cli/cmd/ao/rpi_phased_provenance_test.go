// practices: [agile-manifesto, dora-metrics]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWritePromptAuditTrail_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	runID := "test-run-abc"
	phaseNum := 2
	prompt := "Run /crank ag-123 --test-first"

	if err := writePromptAuditTrail(dir, runID, phaseNum, prompt); err != nil {
		t.Fatalf("writePromptAuditTrail: %v", err)
	}

	expectedPath := filepath.Join(dir, ".agents", "rpi", "runs", runID, "phase-2-prompt.md")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("expected file at %s: %v", expectedPath, err)
	}
}

func TestWritePromptAuditTrail_ContentMatches(t *testing.T) {
	dir := t.TempDir()
	runID := "content-run"
	phaseNum := 1
	prompt := "CONTEXT DISCIPLINE: phase 1\n\nRun /research \"add auth\"\n"

	if err := writePromptAuditTrail(dir, runID, phaseNum, prompt); err != nil {
		t.Fatalf("writePromptAuditTrail: %v", err)
	}

	path := filepath.Join(dir, ".agents", "rpi", "runs", runID, "phase-1-prompt.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != prompt {
		t.Errorf("content mismatch:\ngot:  %q\nwant: %q", string(data), prompt)
	}
}

func TestWritePromptAuditTrail_EmptyRunID(t *testing.T) {
	dir := t.TempDir()

	if err := writePromptAuditTrail(dir, "", 1, "some prompt"); err != nil {
		t.Fatalf("expected no error for empty runID, got: %v", err)
	}

	// Verify no files were created
	runsDir := filepath.Join(dir, ".agents", "rpi", "runs")
	if _, err := os.Stat(runsDir); err == nil {
		t.Error("runs directory should not be created for empty runID")
	}
}

func TestBuildHandoffContext_SourceAttribution(t *testing.T) {
	handoffs := []*phaseHandoff{
		{
			Phase:     1,
			PhaseName: "discovery",
			Status:    "completed",
			Goal:      "add auth",
			Narrative: "Discovery completed successfully.",
		},
		{
			Phase:     2,
			PhaseName: "implementation",
			Status:    "time_boxed",
			Goal:      "add auth",
			Narrative: "Implementation ran out of time.",
		},
	}

	ctx := buildHandoffContext(handoffs, phaseManifest{NarrativeCap: 1000})

	// Verify source attribution on phase headers
	if !strings.Contains(ctx, "(source: phase-1-handoff.json)") {
		t.Errorf("missing source attribution for phase 1 header\ngot:\n%s", ctx)
	}
	if !strings.Contains(ctx, "(source: phase-2-handoff.json)") {
		t.Errorf("missing source attribution for phase 2 header\ngot:\n%s", ctx)
	}

	// Verify source attribution on narratives
	if !strings.Contains(ctx, "Narrative (from phase-1-summary):") {
		t.Errorf("missing source attribution for phase 1 narrative\ngot:\n%s", ctx)
	}
	if !strings.Contains(ctx, "Narrative (from phase-2-summary):") {
		t.Errorf("missing source attribution for phase 2 narrative\ngot:\n%s", ctx)
	}
}
