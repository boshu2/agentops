package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestEvolveCommandRegisteredOnRoot(t *testing.T) {
	if evolveCmd == nil {
		t.Fatal("evolveCmd should not be nil")
	}
	if evolveCmd.Use != "evolve [goal]" {
		t.Errorf("evolveCmd.Use = %q, want %q", evolveCmd.Use, "evolve [goal]")
	}
	if evolveCmd.GroupID != "workflow" {
		t.Errorf("evolveCmd.GroupID = %q, want workflow", evolveCmd.GroupID)
	}

	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "evolve" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("evolveCmd should be registered on rootCmd")
	}
}

func TestEvolveCommandReusesRPILoopFlags(t *testing.T) {
	for _, flag := range []string{
		"max-cycles",
		"supervisor",
		"compile",
		"gate-policy",
		"landing-policy",
		"kill-switch-path",
	} {
		if evolveCmd.Flags().Lookup(flag) == nil {
			t.Fatalf("evolve command should expose --%s", flag)
		}
	}
	if got := evolveCmd.Flags().Lookup("supervisor").DefValue; got != "true" {
		t.Fatalf("evolve --supervisor help default = %q, want true", got)
	}
}

func TestEvolveHelpDescribesV2OperatorCadence(t *testing.T) {
	help := evolveCmd.Long
	for _, want := range []string{
		`The v2 name is`,
		`still "evolve"`,
		"post-mortem finished work",
		"analyze repo state",
		"planning/pre-mortem/implementation/validation",
		"harvest follow-ups",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("evolve help missing %q:\n%s", want, help)
		}
	}
}

func TestApplyEvolveDefaultsEnablesSupervisor(t *testing.T) {
	prev := snapshotLoopSupervisorGlobals()
	defer restoreLoopSupervisorGlobals(prev)

	rpiSupervisor = false
	cmd := newEvolveDefaultsTestCommand()

	applyEvolveDefaults(cmd)

	if !rpiSupervisor {
		t.Fatal("evolve should default to supervisor mode")
	}
}

func TestApplyEvolveDefaultsRespectsExplicitSupervisorFalse(t *testing.T) {
	prev := snapshotLoopSupervisorGlobals()
	defer restoreLoopSupervisorGlobals(prev)

	rpiSupervisor = false
	cmd := newEvolveDefaultsTestCommand()
	if err := cmd.ParseFlags([]string{"--supervisor=false"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	applyEvolveDefaults(cmd)

	if rpiSupervisor {
		t.Fatal("explicit --supervisor=false should not be overridden")
	}
}

func newEvolveDefaultsTestCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "evolve"}
	cmd.Flags().BoolVar(&rpiSupervisor, "supervisor", false, "")
	return cmd
}

func TestEnsureEvolveEraBaselineWritesOncePerGoalsHash(t *testing.T) {
	prevDryRun := dryRun
	prevGoalsTimeout := goalsTimeout
	dryRun = false
	goalsTimeout = 5
	t.Cleanup(func() {
		dryRun = prevDryRun
		goalsTimeout = prevGoalsTimeout
	})

	dir := chdirTemp(t)
	writeFile(t, filepath.Join(dir, "GOALS.md"), evolveBaselineTestGoals("first-gate", "first era"))

	if err := ensureEvolveEraBaseline(dir); err != nil {
		t.Fatalf("ensureEvolveEraBaseline first run: %v", err)
	}
	dirs := evolveBaselineDirs(t, dir)
	if len(dirs) != 1 {
		t.Fatalf("baseline dirs after first run = %d, want 1 (%v)", len(dirs), dirs)
	}
	files := evolveBaselineSnapshotFiles(t, dirs[0])
	if len(files) != 1 {
		t.Fatalf("baseline snapshot files after first run = %d, want 1 (%v)", len(files), files)
	}

	if err := ensureEvolveEraBaseline(dir); err != nil {
		t.Fatalf("ensureEvolveEraBaseline second run: %v", err)
	}
	files = evolveBaselineSnapshotFiles(t, dirs[0])
	if len(files) != 1 {
		t.Fatalf("baseline snapshot files after same-era rerun = %d, want 1 (%v)", len(files), files)
	}

	writeFile(t, filepath.Join(dir, "GOALS.md"), evolveBaselineTestGoals("second-gate", "second era"))
	if err := ensureEvolveEraBaseline(dir); err != nil {
		t.Fatalf("ensureEvolveEraBaseline new era: %v", err)
	}
	dirs = evolveBaselineDirs(t, dir)
	if len(dirs) != 2 {
		t.Fatalf("baseline dirs after goals change = %d, want 2 (%v)", len(dirs), dirs)
	}
}

func TestEnsureEvolveEraBaselineSkipsDryRun(t *testing.T) {
	prevDryRun := dryRun
	dryRun = true
	t.Cleanup(func() { dryRun = prevDryRun })

	dir := chdirTemp(t)
	writeFile(t, filepath.Join(dir, "GOALS.md"), evolveBaselineTestGoals("dry-run-gate", "dry run"))

	if err := ensureEvolveEraBaseline(dir); err != nil {
		t.Fatalf("ensureEvolveEraBaseline dry-run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".agents", "evolve", "fitness-baselines")); !os.IsNotExist(err) {
		t.Fatalf("dry-run baseline dir stat err = %v, want not exist", err)
	}
}

func TestRunEvolveDoesNotWriteBaselineWhenToolchainInvalid(t *testing.T) {
	prevDryRun := dryRun
	dryRun = false
	t.Cleanup(func() { dryRun = prevDryRun })
	t.Setenv("AGENTOPS_RPI_RUNTIME", "bushido")
	t.Setenv("AGENTOPS_RPI_RUNTIME_MODE", "")

	dir := chdirTemp(t)
	writeFile(t, filepath.Join(dir, "GOALS.md"), evolveBaselineTestGoals("invalid-runtime-gate", "invalid runtime"))

	err := runEvolve(&cobra.Command{Use: "evolve"}, nil)
	if err == nil {
		t.Fatal("expected invalid runtime error")
	}
	if !strings.Contains(err.Error(), `invalid runtime "bushido"`) {
		t.Fatalf("error = %q, want invalid runtime", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".agents", "evolve", "fitness-baselines")); !os.IsNotExist(statErr) {
		t.Fatalf("baseline dir stat err = %v, want not exist after invalid runtime", statErr)
	}
}

func evolveBaselineTestGoals(id, description string) string {
	return `# Goals

Mission.

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| ` + id + ` | ` + "`exit 0`" + ` | 1 | ` + description + ` |
`
}

func evolveBaselineDirs(t *testing.T, dir string) []string {
	t.Helper()
	dirs, err := filepath.Glob(filepath.Join(dir, ".agents", "evolve", "fitness-baselines", "goals-*"))
	if err != nil {
		t.Fatalf("glob baseline dirs: %v", err)
	}
	return dirs
}

func evolveBaselineSnapshotFiles(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		t.Fatalf("glob baseline snapshots: %v", err)
	}
	return files
}
