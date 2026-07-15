// practices: [dora-metrics, lean-startup]
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoals_Integration_FullLifecycle(t *testing.T) {
	t.Parallel()
	bin := aoBinary(t)
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, ".git"), 0o750)
	os.MkdirAll(filepath.Join(tmp, ".agents", "ao", "sessions"), 0o750)
	os.MkdirAll(filepath.Join(tmp, ".agents", "ao", "goals", "baselines"), 0o750)

	// Step 1: init creates GOALS.md with --non-interactive
	cmd := exec.Command(bin, "goals", "init", "--non-interactive")
	cmd.Dir = tmp
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("goals init failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Created") {
		t.Errorf("expected 'Created' in output, got: %s", out)
	}

	goalsPath := filepath.Join(tmp, "GOALS.md")
	if _, statErr := os.Stat(goalsPath); statErr != nil {
		t.Fatalf("GOALS.md not created: %v", statErr)
	}
	content, err := os.ReadFile(goalsPath)
	if err != nil {
		t.Fatalf("read GOALS.md: %v", err)
	}
	if !strings.Contains(string(content), "Establish baseline") {
		t.Errorf("GOALS.md missing default directive, content:\n%s", content)
	}

	// Step 2: measurement remains read-only; recommendation and apply routing
	// were removed by the Cathedral Cut.
	cmd = exec.Command(bin, "goals", "measure")
	cmd.Dir = tmp
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("goals measure failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Score:") {
		t.Errorf("expected 'Score:' in measure output, got: %s", out)
	}
}

func TestGoals_Integration_InitAlreadyExists(t *testing.T) {
	dir := chdirTemp(t)

	// Create existing GOALS.md
	writeFile(t, filepath.Join(dir, "GOALS.md"), "# Existing\n")

	// init should fail when file already exists
	_, err := captureStdout(t, func() error {
		goalsInitNonInteractive = true
		goalsInitTemplate = ""
		defer func() { goalsInitNonInteractive = false }()
		rootCmd.SetArgs([]string{"goals", "init", "--non-interactive"})
		return rootCmd.Execute()
	})
	if err == nil {
		t.Fatal("expected error when GOALS.md already exists, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

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

	// Create GOALS.md
	_, err := captureStdout(t, func() error {
		goalsInitNonInteractive = true
		goalsInitTemplate = ""
		defer func() { goalsInitNonInteractive = false }()
		rootCmd.SetArgs([]string{"goals", "init", "--non-interactive"})
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("goals init failed: %v", err)
	}
	_ = dir

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
