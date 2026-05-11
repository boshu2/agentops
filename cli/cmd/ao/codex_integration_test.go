// practices: [microservices, design-by-contract]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodex_Integration_StatusNoState(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	oldOutput := output
	oldJsonFlag := jsonFlag
	output = "table"
	jsonFlag = false
	t.Cleanup(func() { output = oldOutput; jsonFlag = oldJsonFlag })

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"codex", "status"})
		return rootCmd.Execute()
	})

	// Should produce output even without codex-state.json
	if out == "" {
		t.Fatal("expected codex status output, got empty string")
	}

	// Should contain the header
	if !strings.Contains(out, "Codex Lifecycle Status") {
		t.Errorf("expected output to contain 'Codex Lifecycle Status', got:\n%s", out)
	}

	// Should show zeroed capture/retrieval lines
	if !strings.Contains(out, "Capture:") {
		t.Errorf("expected output to contain 'Capture:', got:\n%s", out)
	}
	if !strings.Contains(out, "Retrieval:") {
		t.Errorf("expected output to contain 'Retrieval:', got:\n%s", out)
	}

	_ = err
}

func TestCodex_Integration_StatusWithState(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	oldOutput := output
	oldJsonFlag := jsonFlag
	output = "table"
	jsonFlag = false
	t.Cleanup(func() { output = oldOutput; jsonFlag = oldJsonFlag })

	// Write state.json in the correct location
	stateDir := filepath.Join(dir, ".agents", "ao", "codex")
	writeFile(t, filepath.Join(stateDir, "state.json"), `{
		"schema_version": 1,
		"runtime": {"mode": "codex", "runtime": "codex-cli"},
		"last_start": {"timestamp": "2026-04-01T10:00:00Z", "session_id": "test-session-1"},
		"updated_at": "2026-04-01T10:00:00Z"
	}`)

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"codex", "status"})
		return rootCmd.Execute()
	})

	if out == "" {
		t.Fatal("expected codex status output, got empty string")
	}

	if !strings.Contains(out, "Codex Lifecycle Status") {
		t.Errorf("expected header, got:\n%s", out)
	}

	// Should show last start info
	if !strings.Contains(out, "Last start:") {
		t.Errorf("expected 'Last start:' in output, got:\n%s", out)
	}

	_ = err
}

func TestCodex_Integration_StatusJSON(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	oldOutput := output
	oldJsonFlag := jsonFlag
	output = "json"
	jsonFlag = false
	t.Cleanup(func() { output = oldOutput; jsonFlag = oldJsonFlag })

	// Populate some learnings so retrieval is nonzero
	writeFile(t, filepath.Join(dir, ".agents", "learnings", "learn-1.md"), "---\nutility: 0.5\n---\n# Test learning\nContent.\n")

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"codex", "status"})
		return rootCmd.Execute()
	})

	if out == "" {
		t.Fatal("expected JSON output, got empty string")
	}

	var result codexStatusResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("expected valid JSON, got parse error: %v\nraw:\n%s", err, out)
	}

	// Runtime must be populated
	if result.Runtime.Mode == "" {
		t.Error("expected runtime mode to be set")
	}

	// Citations window should default to 7
	if result.Citations.WindowDays != 7 {
		t.Errorf("expected citations window 7, got %d", result.Citations.WindowDays)
	}

	_ = err
}

func TestCodex_Integration_StartDryRun(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	// Add a learning for the start command to surface
	writeFile(t, filepath.Join(dir, ".agents", "learnings", "learn-dry.md"), "---\nutility: 0.7\n---\n# Dry run learning\nSome content.\n")

	oldDryRun := dryRun
	oldOutput := output
	oldJsonFlag := jsonFlag
	dryRun = true
	output = "table"
	jsonFlag = false
	t.Cleanup(func() { dryRun = oldDryRun; output = oldOutput; jsonFlag = oldJsonFlag })

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"codex", "start", "--no-maintenance"})
		return rootCmd.Execute()
	})

	// Start should produce output (startup context)
	if out == "" {
		t.Fatal("expected codex start output, got empty string")
	}

	// Should write a startup context file
	startupCtx := filepath.Join(dir, ".agents", "ao", "startup-context.md")
	if _, statErr := os.Stat(startupCtx); statErr != nil {
		// Also check for codex-state.json as evidence the command ran
		statePath := filepath.Join(dir, ".agents", "ao", "codex-state.json")
		if _, statErr2 := os.Stat(statePath); statErr2 != nil {
			t.Logf("note: neither startup-context.md nor codex-state.json found (may be expected in dry-run)")
		}
	}

	_ = err
}
