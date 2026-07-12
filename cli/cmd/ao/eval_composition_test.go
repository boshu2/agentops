package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvalCompositionOwnsCompleteCommandTree(t *testing.T) {
	command := newEvalCommand()
	want := []string{
		"ao eval baseline",
		"ao eval baseline-audit",
		"ao eval bench",
		"ao eval chaos",
		"ao eval cleanup",
		"ao eval compare",
		"ao eval coverage",
		"ao eval outcomes compile",
		"ao eval outcomes ingest",
		"ao eval run",
		"ao eval scenario add",
		"ao eval scenario evaluate",
		"ao eval scenario init",
		"ao eval scenario list",
		"ao eval scenario validate",
		"ao eval scenario-ab",
		"ao eval scenario-moat",
		"ao eval scorecard",
		"ao eval session-outcome",
		"ao eval suite n-required",
		"ao eval suite verdict",
		"ao eval task add",
		"ao eval task list",
		"ao eval task run",
		"ao eval task show",
	}
	for _, path := range want {
		args := strings.Fields(strings.TrimPrefix(path, "ao eval "))
		child, remaining, err := command.Find(args)
		if err != nil || child == command || len(remaining) != 0 {
			t.Fatalf("missing eval command %q: child=%v remaining=%v err=%v", path, child, remaining, err)
		}
	}
}

func TestEvalCompositionResolvesGoalsPathAndProjectRoot(t *testing.T) {
	dir := chdirTemp(t)
	resetCommandState(t)
	if err := os.WriteFile(filepath.Join(dir, "GOALS.yaml"), []byte("version: 3\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	host := newEvalHostOptions()
	if got := host.ProjectRoot(); got != dir {
		t.Fatalf("ProjectRoot() = %q, want cwd %q", got, dir)
	}
	if got := host.GoalsPath(); got != "GOALS.yaml" {
		t.Fatalf("GoalsPath() = %q, want GOALS.yaml fallback", got)
	}
}
