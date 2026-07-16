// practices: [dora-metrics, lean-startup]
package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGoals_Integration_MeasureNoGoalsFile(t *testing.T) {
	_ = chdirTemp(t)

	// measure with no GOALS.md should fail
	_, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"goals", "measure"})
		return rootCmd.Execute()
	})
	if err == nil {
		t.Fatal("expected error when no goals file exists, got nil")
	}
}

func TestGoals_Integration_MeasureDirectivesJSON(t *testing.T) {
	dir := chdirTemp(t)
	// Full isolation: resetCommandState clears leaked cobra out-writers (else
	// measure's OutOrStdout() resolves to a dead buffer → empty captured output)
	// and save/restores the goals flag globals. scenariosOnly is set explicitly
	// because resetCommandState save/restores rather than cleaning inherited state.
	resetCommandState(t)
	setGoalsMeasureScenariosOnly(t, false)

	writeFile(t, filepath.Join(dir, "GOALS.md"), `# Fitness Goals

## Mission

Measure project fitness.

## North Stars

- Passing checks

## Anti-Stars

- Hidden regressions

## Directives

### 1. Establish baseline

Keep the deterministic floor green.

**Steer:** increase

## Gates
`)

	// measure --directives should output JSON with directive info
	out, err := captureStdout(t, func() error {
		// --directives binds the package-global goalsMeasureDirectives; cobra
		// does not reset it after Execute, so restore it to avoid leaking
		// directives-mode into later goals-measure tests in the same binary.
		defer func() { goalsMeasureDirectives = false }()
		rootCmd.SetArgs([]string{"goals", "measure", "--directives"})
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("goals measure --directives failed: %v", err)
	}
	if !strings.Contains(out, "Establish baseline") {
		t.Errorf("expected directive title in JSON output, got: %s", out)
	}
}
