// practices: [design-by-contract, code-complete]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedConstraintIndex writes a constraint index with the given entries.
func seedConstraintIndex(t *testing.T, dir string, entries []constraintEntry) {
	t.Helper()
	idx := constraintIndex{
		SchemaVersion: 1,
		Constraints:   entries,
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		t.Fatalf("marshal index: %v", err)
	}
	indexPath := filepath.Join(dir, ".agents", "constraints", "index.json")
	writeFile(t, indexPath, string(data)+"\n")
}

func TestConstraint_Integration_FullLifecycle(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	// Seed index with a draft constraint
	seedConstraintIndex(t, dir, []constraintEntry{
		{
			ID:         "C001-no-eval",
			Title:      "No eval() in production code",
			Source:     "retro-2026-01",
			Status:     "draft",
			CompiledAt: "2026-01-15T10:00:00Z",
			File:       ".agents/constraints/C001-no-eval.md",
		},
	})

	// Step 1: list shows the draft constraint
	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"constraint", "list"})
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("constraint list failed: %v", err)
	}
	if !strings.Contains(out, "C001-no-eval") {
		t.Errorf("expected C001-no-eval in list output, got: %s", out)
	}
	if !strings.Contains(out, "draft") {
		t.Errorf("expected 'draft' status in list output, got: %s", out)
	}

	// Step 2: activate transitions draft -> active
	out, err = captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"constraint", "activate", "C001-no-eval"})
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("constraint activate failed: %v", err)
	}
	if !strings.Contains(out, "activated") {
		t.Errorf("expected 'activated' in output, got: %s", out)
	}

	// Verify persisted status is now active
	idxData, err := os.ReadFile(filepath.Join(dir, ".agents", "constraints", "index.json"))
	if err != nil {
		t.Fatalf("read index after activate: %v", err)
	}
	if !strings.Contains(string(idxData), `"active"`) {
		t.Errorf("expected 'active' status in index.json, got: %s", idxData)
	}

	// Step 3: retire transitions active -> retired
	out, err = captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"constraint", "retire", "C001-no-eval"})
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("constraint retire failed: %v", err)
	}
	if !strings.Contains(out, "retired") {
		t.Errorf("expected 'retired' in output, got: %s", out)
	}

	// Verify persisted status is now retired
	idxData, err = os.ReadFile(filepath.Join(dir, ".agents", "constraints", "index.json"))
	if err != nil {
		t.Fatalf("read index after retire: %v", err)
	}
	if !strings.Contains(string(idxData), `"retired"`) {
		t.Errorf("expected 'retired' status in index.json, got: %s", idxData)
	}
}

func TestConstraint_Integration_ActivateNonexistent(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	// Seed empty index
	seedConstraintIndex(t, dir, []constraintEntry{})

	_, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"constraint", "activate", "C999-missing"})
		return rootCmd.Execute()
	})
	if err == nil {
		t.Fatal("expected error activating nonexistent constraint, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestConstraint_Integration_RetireNonexistent(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	seedConstraintIndex(t, dir, []constraintEntry{})

	_, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"constraint", "retire", "C999-missing"})
		return rootCmd.Execute()
	})
	if err == nil {
		t.Fatal("expected error retiring nonexistent constraint, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestConstraint_Integration_ActivateNotDraft(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	// Seed with an already-active constraint
	seedConstraintIndex(t, dir, []constraintEntry{
		{
			ID:         "C001-active",
			Title:      "Already active",
			Source:     "test",
			Status:     "active",
			CompiledAt: "2026-01-15T10:00:00Z",
			File:       ".agents/constraints/C001-active.md",
		},
	})

	_, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"constraint", "activate", "C001-active"})
		return rootCmd.Execute()
	})
	if err == nil {
		t.Fatal("expected error activating already-active constraint, got nil")
	}
	if !strings.Contains(err.Error(), "can only activate from draft") {
		t.Errorf("expected 'can only activate from draft' error, got: %v", err)
	}
}

func TestConstraint_Integration_RetireNotActive(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	// Seed with a draft constraint (not active)
	seedConstraintIndex(t, dir, []constraintEntry{
		{
			ID:         "C001-draft",
			Title:      "Still draft",
			Source:     "test",
			Status:     "draft",
			CompiledAt: "2026-01-15T10:00:00Z",
			File:       ".agents/constraints/C001-draft.md",
		},
	})

	_, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"constraint", "retire", "C001-draft"})
		return rootCmd.Execute()
	})
	if err == nil {
		t.Fatal("expected error retiring draft constraint, got nil")
	}
	if !strings.Contains(err.Error(), "can only retire from active") {
		t.Errorf("expected 'can only retire from active' error, got: %v", err)
	}
}

func TestConstraint_Integration_ListNoIndex(t *testing.T) {
	_ = chdirTemp(t)

	// No .agents/constraints/index.json exists
	_, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"constraint", "list"})
		return rootCmd.Execute()
	})
	if err == nil {
		t.Fatal("expected error listing with no index file, got nil")
	}
	if !strings.Contains(err.Error(), "no constraints found") {
		t.Errorf("expected 'no constraints found' error, got: %v", err)
	}
}

func TestConstraint_Integration_ListEmpty(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	seedConstraintIndex(t, dir, []constraintEntry{})

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"constraint", "list"})
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("constraint list with empty index failed: %v", err)
	}
	if !strings.Contains(out, "No constraints found") {
		t.Errorf("expected 'No constraints found' message, got: %s", out)
	}
}

func TestConstraint_Integration_ReviewStale(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	// Seed with an old active constraint (>90 days)
	seedConstraintIndex(t, dir, []constraintEntry{
		{
			ID:         "C001-old",
			Title:      "Very old constraint",
			Source:     "retro-ancient",
			Status:     "active",
			CompiledAt: "2025-01-01T00:00:00Z",
			File:       ".agents/constraints/C001-old.md",
		},
		{
			ID:         "C002-new",
			Title:      "Recent constraint",
			Source:     "retro-recent",
			Status:     "active",
			CompiledAt: "2026-04-01T00:00:00Z",
			File:       ".agents/constraints/C002-new.md",
		},
	})

	out, err := captureStdout(t, func() error {
		rootCmd.SetArgs([]string{"constraint", "review"})
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("constraint review failed: %v", err)
	}
	// The old constraint should appear in review, the new one should not
	if !strings.Contains(out, "C001-old") {
		t.Errorf("expected stale C001-old in review output, got: %s", out)
	}
	if strings.Contains(out, "C002-new") {
		t.Errorf("recent C002-new should NOT appear in review output, got: %s", out)
	}
	if !strings.Contains(out, "1 constraint(s) need review") {
		t.Errorf("expected '1 constraint(s) need review' in output, got: %s", out)
	}
}
