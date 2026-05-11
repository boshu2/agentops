// practices: [design-by-contract, gitops]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHooksInstall_Integration_DryRun(t *testing.T) {
	dir := initTestRepo(t)
	chdirTo(t, dir)
	setupAgentsDir(t, dir)

	// Set HOME to temp dir so install writes to temp .claude/settings.json
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create .claude dir for settings
	claudeDir := filepath.Join(tmpHome, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"hooks", "install", "--dry-run"})
		return rootCmd.Execute()
	})

	if err != nil {
		t.Fatalf("hooks install --dry-run failed: %v", err)
	}

	// Dry-run should produce output describing what would be installed
	if out == "" {
		t.Fatal("expected dry-run output, got empty string")
	}

	// Should mention settings or hooks in the output
	hasRelevant := strings.Contains(out, "settings") ||
		strings.Contains(out, "hook") ||
		strings.Contains(out, "SessionStart") ||
		strings.Contains(out, "install") ||
		strings.Contains(out, "Would")
	if !hasRelevant {
		t.Errorf("expected output to mention settings/hooks/install, got:\n%s", out)
	}
}

func TestHooksInstall_Integration_AlreadyInstalled(t *testing.T) {
	dir := initTestRepo(t)
	chdirTo(t, dir)

	// Set HOME to temp dir
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	claudeDir := filepath.Join(tmpHome, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Pre-populate settings.json with ao hooks marker
	settingsPath := filepath.Join(claudeDir, "settings.json")
	existingSettings := `{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "ao inject --auto"}]}],
    "Stop": [{"hooks": [{"type": "command", "command": "ao stop-hook"}]}]
  },
  "_ao_managed": true
}`
	if err := os.WriteFile(settingsPath, []byte(existingSettings), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"hooks", "install"})
		return rootCmd.Execute()
	})

	if err != nil {
		t.Fatalf("hooks install (already installed) failed: %v", err)
	}

	// Should indicate hooks are already installed
	if !strings.Contains(out, "already installed") {
		t.Errorf("expected 'already installed' message, got:\n%s", out)
	}
}

func TestHooksTest_Integration_DryRun(t *testing.T) {
	dir := initTestRepo(t)
	chdirTo(t, dir)
	setupAgentsDir(t, dir)

	// Set HOME to temp dir
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	claudeDir := filepath.Join(tmpHome, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"hooks", "test", "--dry-run"})
		return rootCmd.Execute()
	})

	if err != nil {
		t.Fatalf("hooks test --dry-run failed: %v", err)
	}

	if out == "" {
		t.Fatal("expected hooks test output, got empty string")
	}

	// Should mention test steps or ao verification
	hasContent := strings.Contains(out, "ao") ||
		strings.Contains(out, "hook") ||
		strings.Contains(out, "test") ||
		strings.Contains(out, "PATH") ||
		strings.Contains(out, "dry-run")
	if !hasContent {
		t.Errorf("expected hooks test output to reference ao/hook/test, got:\n%s", out)
	}
}

func TestHooksInit_Integration_JSONFormat(t *testing.T) {
	dir := initTestRepo(t)
	chdirTo(t, dir)
	setupAgentsDir(t, dir)

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"hooks", "init", "--format", "json"})
		return rootCmd.Execute()
	})

	if err != nil {
		t.Fatalf("hooks init --format=json failed: %v", err)
	}

	if out == "" {
		t.Fatal("expected hooks init JSON output, got empty string")
	}

	// Output should contain hook event names
	if !strings.Contains(out, "SessionStart") {
		t.Errorf("expected JSON output to contain SessionStart event, got:\n%s", out)
	}
}
