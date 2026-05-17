// practices: [dora-metrics, lean-startup]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoals_Integration_FullLifecycle(t *testing.T) {
	resetCommandState(t)
	dir := chdirTemp(t)

	// Step 1: init creates GOALS.md with --non-interactive
	out, err := captureStdout(t, func() error {
		goalsInitNonInteractive = true
		goalsInitTemplate = ""
		defer func() { goalsInitNonInteractive = false }()
		rootCmd.SetArgs([]string{"goals", "init", "--non-interactive"})
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("goals init failed: %v", err)
	}

	goalsPath := filepath.Join(dir, "GOALS.md")
	if _, statErr := os.Stat(goalsPath); statErr != nil {
		t.Fatalf("GOALS.md not created: %v", statErr)
	}
	if !strings.Contains(out, "Created") {
		t.Errorf("expected 'Created' in output, got: %s", out)
	}

	// Verify file contains expected content
	content, err := os.ReadFile(goalsPath)
	if err != nil {
		t.Fatalf("read GOALS.md: %v", err)
	}
	if !strings.Contains(string(content), "Establish baseline") {
		t.Errorf("GOALS.md missing default directive, content:\n%s", content)
	}

	// Step 2: steer add appends a directive
	out, err = captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"goals", "steer", "add", "Improve coverage", "--description", "Raise test coverage above 80%", "--steer", "increase"})
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("goals steer add failed: %v", err)
	}
	if !strings.Contains(out, "Added directive #2") {
		t.Errorf("expected 'Added directive #2' in output, got: %s", out)
	}

	// Verify the directive was persisted
	content, err = os.ReadFile(goalsPath)
	if err != nil {
		t.Fatalf("read GOALS.md after steer add: %v", err)
	}
	if !strings.Contains(string(content), "Improve coverage") {
		t.Errorf("GOALS.md missing added directive, content:\n%s", content)
	}

	// Step 3: measure runs checks (all gates should be skipped/pass with no real checks)
	// Create snapshot dir so measure can save
	if err := os.MkdirAll(filepath.Join(dir, ".agents", "ao", "goals", "baselines"), 0o755); err != nil {
		t.Fatalf("create baselines dir: %v", err)
	}

	out, err = captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"goals", "measure"})
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("goals measure failed: %v", err)
	}
	if !strings.Contains(out, "Score:") {
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

func TestGoals_Integration_SteerAddInvalidSteer(t *testing.T) {
	dir := chdirTemp(t)

	// Create a valid GOALS.md first
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

	// Try adding directive with invalid steer value
	_, err = captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"goals", "steer", "add", "Bad directive", "--description", "test", "--steer", "bogus"})
		return rootCmd.Execute()
	})
	if err == nil {
		t.Fatal("expected error for invalid steer value, got nil")
	}
	if !strings.Contains(err.Error(), "invalid steer") {
		t.Errorf("expected 'invalid steer' error, got: %v", err)
	}
}

func TestGoals_Integration_MeasureDirectivesJSON(t *testing.T) {
	dir := chdirTemp(t)

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
