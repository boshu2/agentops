// practices: [dora-metrics, lean-startup]
package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/goals"
)

func TestFindMissingPath_NoPathReferences(t *testing.T) {
	tests := []struct {
		name  string
		check string
	}{
		{"simple command", "echo hello"},
		{"exit code", "exit 0"},
		{"command with flags", "go test -v --count=1"},
		{"make target", "make build"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findMissingPath(tt.check)
			if got != "" {
				t.Errorf("findMissingPath(%q) = %q, want empty", tt.check, got)
			}
		})
	}
}

func TestFindMissingPath_DetectsMissingScripts(t *testing.T) {
	tests := []struct {
		name  string
		check string
		want  string
	}{
		{"scripts/ prefix", "scripts/check-build.sh", "scripts/check-build.sh"},
		{"./scripts/ prefix", "./scripts/check-build.sh", "./scripts/check-build.sh"},
		{"tests/ prefix", "tests/validate.sh", "tests/validate.sh"},
		{"./tests/ prefix", "./tests/validate.sh", "./tests/validate.sh"},
		{"hooks/ prefix", "hooks/pre-commit.sh", "hooks/pre-commit.sh"},
		{"./hooks/ prefix", "./hooks/pre-commit.sh", "./hooks/pre-commit.sh"},
		{"with args", "scripts/check-build.sh --strict", "scripts/check-build.sh"},
		{"path with extension", "lib/helpers.sh arg1", "lib/helpers.sh"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findMissingPath(tt.check)
			if got != tt.want {
				t.Errorf("findMissingPath(%q) = %q, want %q", tt.check, got, tt.want)
			}
		})
	}
}

func TestFindMissingPath_ExistingFileReturnsEmpty(t *testing.T) {
	dir := t.TempDir()

	// Create a script file
	scriptsDir := filepath.Join(dir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, "check-build.sh"), []byte("#!/bin/bash\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Chdir(dir)

	got := findMissingPath("scripts/check-build.sh --strict")
	if got != "" {
		t.Errorf("findMissingPath for existing file = %q, want empty", got)
	}
}

func TestFindMissingPath_TrailingShellOperators(t *testing.T) {
	tests := []struct {
		name  string
		check string
		want  string
	}{
		{"semicolon", "scripts/run.sh;", "scripts/run.sh"},
		{"pipe", "scripts/run.sh|", "scripts/run.sh"},
		{"ampersand", "scripts/run.sh&", "scripts/run.sh"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findMissingPath(tt.check)
			if got != tt.want {
				t.Errorf("findMissingPath(%q) = %q, want %q", tt.check, got, tt.want)
			}
		})
	}
}

func TestGoalsPrune_NoStaleGoals(t *testing.T) {
	dir := t.TempDir()

	// Create a goals file with only non-path-referencing checks
	md := `# Goals

Mission.

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| simple-gate | ` + "`echo ok`" + ` | 5 | Simple |
| exit-gate | ` + "`exit 0`" + ` | 5 | Exit |
`
	goalsPath := filepath.Join(dir, "GOALS.md")
	if err := os.WriteFile(goalsPath, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Chdir(dir)

	oldFile := goalsFile
	oldDryRun := dryRun
	oldJSON := goalsJSON
	defer func() {
		goalsFile = oldFile
		dryRun = oldDryRun
		goalsJSON = oldJSON
	}()
	goalsFile = goalsPath
	dryRun = false
	goalsJSON = false

	err := goalsPruneCmd.RunE(goalsPruneCmd, nil)
	if err != nil {
		t.Fatalf("prune returned error: %v", err)
	}
}

func TestGoalsPrune_DryRun_DetectsStale(t *testing.T) {
	dir := t.TempDir()

	md := `# Goals

Mission.

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| good-gate | ` + "`echo ok`" + ` | 5 | Good |
| stale-gate | ` + "`scripts/nonexistent.sh`" + ` | 5 | Stale |
`
	goalsPath := filepath.Join(dir, "GOALS.md")
	if err := os.WriteFile(goalsPath, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Chdir(dir)

	oldFile := goalsFile
	oldDryRun := dryRun
	oldJSON := goalsJSON
	defer func() {
		goalsFile = oldFile
		dryRun = oldDryRun
		goalsJSON = oldJSON
	}()
	goalsFile = goalsPath
	dryRun = true
	goalsJSON = false

	err := goalsPruneCmd.RunE(goalsPruneCmd, nil)
	if err != nil {
		t.Fatalf("prune dry-run returned error: %v", err)
	}

	// Verify the file was NOT modified (dry-run)
	gf, err := goals.LoadGoals(goalsPath)
	if err != nil {
		t.Fatalf("LoadGoals: %v", err)
	}
	if len(gf.Goals) != 2 {
		t.Errorf("goals count = %d, want 2 (dry-run should not modify)", len(gf.Goals))
	}
}

func TestGoalsPrune_RemovesStaleGoals(t *testing.T) {
	dir := t.TempDir()

	md := `# Goals

Mission.

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| good-gate | ` + "`echo ok`" + ` | 5 | Good |
| stale-gate | ` + "`scripts/nonexistent.sh`" + ` | 5 | Stale |
`
	goalsPath := filepath.Join(dir, "GOALS.md")
	if err := os.WriteFile(goalsPath, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Chdir(dir)

	oldFile := goalsFile
	oldDryRun := dryRun
	oldJSON := goalsJSON
	defer func() {
		goalsFile = oldFile
		dryRun = oldDryRun
		goalsJSON = oldJSON
	}()
	goalsFile = goalsPath
	dryRun = false
	goalsJSON = false

	err := goalsPruneCmd.RunE(goalsPruneCmd, nil)
	if err != nil {
		t.Fatalf("prune returned error: %v", err)
	}

	// Verify the stale goal was removed
	gf, err := goals.LoadGoals(goalsPath)
	if err != nil {
		t.Fatalf("LoadGoals: %v", err)
	}
	if len(gf.Goals) != 1 {
		t.Fatalf("goals count = %d, want 1 (stale should be removed)", len(gf.Goals))
	}
	if gf.Goals[0].ID != "good-gate" {
		t.Errorf("remaining goal = %q, want good-gate", gf.Goals[0].ID)
	}
}

func TestGoalsPrune_MissingGoalsFile(t *testing.T) {
	oldFile := goalsFile
	defer func() { goalsFile = oldFile }()

	goalsFile = "/nonexistent/GOALS.md"

	err := goalsPruneCmd.RunE(goalsPruneCmd, nil)
	if err == nil {
		t.Fatal("expected error for missing goals file")
	}
}

func TestGoalsPrune_CmdAttributes(t *testing.T) {
	if goalsPruneCmd.Use != "prune" {
		t.Errorf("Use = %q, want prune", goalsPruneCmd.Use)
	}
	if goalsPruneCmd.GroupID != "management" {
		t.Errorf("GroupID = %q, want management", goalsPruneCmd.GroupID)
	}
	found := false
	for _, a := range goalsPruneCmd.Aliases {
		if a == "p" {
			found = true
		}
	}
	if !found {
		t.Error("expected alias 'p' for prune command")
	}
}
