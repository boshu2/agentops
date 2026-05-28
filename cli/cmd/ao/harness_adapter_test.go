// practices: [hexagonal-architecture, tdd]
package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// Sibling pattern: cycle 110 operator_adapter_test.go.

func mustWriteSkill(t *testing.T, root, harness, skill, body string) {
	t.Helper()
	dir := filepath.Join(root, harness, skill)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProductionHarness_StatusReportsBothTrees(t *testing.T) {
	root := t.TempDir()
	mustWriteSkill(t, root, "skills", "evolve", "claude evolve")
	mustWriteSkill(t, root, "skills-codex", "evolve", "codex evolve")
	h := newProductionHarness(root)
	all, err := h.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("len = %d, want 2", len(all))
	}
	// Sorted by (skill, harness): claude < codex
	if all[0].Harness != ports.HarnessClaude || all[1].Harness != ports.HarnessCodex {
		t.Fatalf("sort order wrong: got %v, %v", all[0].Harness, all[1].Harness)
	}
}

func TestProductionHarness_OutOfSyncDetected(t *testing.T) {
	root := t.TempDir()
	mustWriteSkill(t, root, "skills", "evolve", "canonical body v1")
	mustWriteSkill(t, root, "skills-codex", "evolve", "codex body DIFFERENT")
	h := newProductionHarness(root)
	codex, err := h.StatusForSkill(context.Background(), "evolve")
	if err != nil {
		t.Fatal(err)
	}
	if len(codex) != 2 {
		t.Fatalf("len = %d, want 2", len(codex))
	}
	var codexEntry, claudeEntry ports.HarnessSkillSync
	for _, e := range codex {
		switch e.Harness {
		case ports.HarnessCodex:
			codexEntry = e
		case ports.HarnessClaude:
			claudeEntry = e
		}
	}
	if !codexEntry.OutOfSync {
		t.Fatal("codex OutOfSync should be true (hashes differ)")
	}
	if claudeEntry.OutOfSync {
		t.Fatal("canonical Claude entry should never report OutOfSync=true")
	}
}

func TestProductionHarness_InSyncWhenHashesMatch(t *testing.T) {
	root := t.TempDir()
	body := "same content for both"
	mustWriteSkill(t, root, "skills", "evolve", body)
	mustWriteSkill(t, root, "skills-codex", "evolve", body)
	h := newProductionHarness(root)
	codex, _ := h.StatusForSkill(context.Background(), "evolve")
	for _, e := range codex {
		if e.OutOfSync {
			t.Fatalf("expected in-sync for %s, got OutOfSync=true", e.Harness)
		}
	}
}

func TestProductionHarness_MissingTreeIsNotError(t *testing.T) {
	root := t.TempDir()
	mustWriteSkill(t, root, "skills", "evolve", "claude only")
	// no skills-codex/ at all
	h := newProductionHarness(root)
	all, err := h.Status(context.Background())
	if err != nil {
		t.Fatalf("missing skills-codex/ should be tolerated, got: %v", err)
	}
	if len(all) != 1 || all[0].Harness != ports.HarnessClaude {
		t.Fatalf("got %+v, want single Claude entry", all)
	}
}

func TestProductionHarness_SkillWithoutSKILLMDIsSkipped(t *testing.T) {
	root := t.TempDir()
	// Make skills/empty/ directory but NO SKILL.md
	if err := os.MkdirAll(filepath.Join(root, "skills", "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteSkill(t, root, "skills", "real", "body")
	h := newProductionHarness(root)
	all, _ := h.Status(context.Background())
	if len(all) != 1 || all[0].Skill != "real" {
		t.Fatalf("got %+v, want only 'real' skill", all)
	}
}

func TestProductionHarness_StatusForSkillEmptyNameErrors(t *testing.T) {
	h := newProductionHarness(t.TempDir())
	if _, err := h.StatusForSkill(context.Background(), ""); err == nil {
		t.Fatal("expected error on empty skill name, got nil")
	}
}

func TestProductionHarness_EmptyRootErrors(t *testing.T) {
	h := newProductionHarness("")
	if _, err := h.Status(context.Background()); err == nil {
		t.Fatal("expected error on empty rootDir, got nil")
	}
}

func TestProductionHarness_HonorsContextCancellation(t *testing.T) {
	h := newProductionHarness(t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := h.Status(ctx); err == nil {
		t.Fatal("expected cancellation error, got nil")
	} else if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
