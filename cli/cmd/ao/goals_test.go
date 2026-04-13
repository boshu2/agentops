package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoalsCmd_Exists(t *testing.T) {
	if goalsCmd == nil {
		t.Fatal("goalsCmd is nil")
	}
	if goalsCmd.Use != "goals" {
		t.Errorf("Use = %q, want %q", goalsCmd.Use, "goals")
	}
}

func TestGoalsCmd_HasExpectedSubcommands(t *testing.T) {
	subNames := map[string]bool{}
	for _, sub := range goalsCmd.Commands() {
		subNames[sub.Name()] = true
	}

	expected := []string{
		"measure", "validate", "drift", "history", "export",
		"init", "add", "steer", "prune", "migrate", "meta",
	}
	for _, name := range expected {
		if !subNames[name] {
			t.Errorf("missing expected subcommand %q", name)
		}
	}
}

func TestGoalsCmd_HasGroups(t *testing.T) {
	groups := goalsCmd.Groups()
	if len(groups) == 0 {
		t.Fatal("goalsCmd has no groups")
	}

	ids := map[string]bool{}
	for _, g := range groups {
		ids[g.ID] = true
	}

	for _, want := range []string{"measurement", "analysis", "management"} {
		if !ids[want] {
			t.Errorf("missing group %q", want)
		}
	}
}

func TestGoalsCmd_PersistentFlags(t *testing.T) {
	flags := goalsCmd.PersistentFlags()

	tests := []struct {
		name string
	}{
		{"file"},
		{"json"},
		{"timeout"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := flags.Lookup(tt.name)
			if f == nil {
				t.Errorf("missing persistent flag %q", tt.name)
			}
		})
	}
}

func TestGoalsCmd_DefaultTimeoutCoversRepoRaceGate(t *testing.T) {
	flag := goalsCmd.PersistentFlags().Lookup("timeout")
	if flag == nil {
		t.Fatal("missing persistent flag \"timeout\"")
	}
	if flag.DefValue != "240" {
		t.Fatalf("timeout default = %q, want 240", flag.DefValue)
	}
	if defaultGoalsTimeoutSeconds != 240 {
		t.Fatalf("defaultGoalsTimeoutSeconds = %d, want 240", defaultGoalsTimeoutSeconds)
	}
}

func TestResolveGoalsFile_ExplicitPath(t *testing.T) {
	// When goalsFile is set explicitly, it should be returned as-is.
	old := goalsFile
	defer func() { goalsFile = old }()

	goalsFile = "/tmp/custom-goals.yaml"
	got := resolveGoalsFile()
	if got != "/tmp/custom-goals.yaml" {
		t.Errorf("resolveGoalsFile() = %q, want /tmp/custom-goals.yaml", got)
	}
}

func TestResolveGoalsFile_PrefersGOALSmd(t *testing.T) {
	dir := t.TempDir()

	// Create both GOALS.md and GOALS.yaml
	if err := os.WriteFile(filepath.Join(dir, "GOALS.md"), []byte("# Goals\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "GOALS.yaml"), []byte("version: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	old := goalsFile
	defer func() { goalsFile = old }()
	goalsFile = ""

	// Save and restore cwd
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	got := resolveGoalsFile()
	if got != "GOALS.md" {
		t.Errorf("resolveGoalsFile() = %q, want GOALS.md (preferred over GOALS.yaml)", got)
	}
}

func TestResolveGoalsFile_FallsBackToYAML(t *testing.T) {
	dir := t.TempDir()

	// Create only GOALS.yaml
	if err := os.WriteFile(filepath.Join(dir, "GOALS.yaml"), []byte("version: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	old := goalsFile
	defer func() { goalsFile = old }()
	goalsFile = ""

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	got := resolveGoalsFile()
	if got != "GOALS.yaml" {
		t.Errorf("resolveGoalsFile() = %q, want GOALS.yaml (fallback)", got)
	}
}

func TestResolveGoalsFile_DefaultsToGOALSmd(t *testing.T) {
	dir := t.TempDir()

	// No goals files exist
	old := goalsFile
	defer func() { goalsFile = old }()
	goalsFile = ""

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	got := resolveGoalsFile()
	if got != "GOALS.md" {
		t.Errorf("resolveGoalsFile() = %q, want GOALS.md (default for new projects)", got)
	}
}
