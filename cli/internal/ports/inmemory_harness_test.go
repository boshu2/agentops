// practices: [hexagonal-architecture, tdd]
package ports

import (
	"context"
	"errors"
	"testing"
)

// Sibling pattern: inmemory_loop_reader_test.go (cycle 102).

func sampleHarnessEntries() []HarnessSkillSync {
	return []HarnessSkillSync{
		{Harness: HarnessClaude, Skill: "evolve", Path: "skills/evolve/SKILL.md", ContentHash: "abc", OutOfSync: false},
		{Harness: HarnessCodex, Skill: "evolve", Path: "skills-codex/evolve/SKILL.md", ContentHash: "def", OutOfSync: true},
		{Harness: HarnessClaude, Skill: "rpi", Path: "skills/rpi/SKILL.md", ContentHash: "ghi", OutOfSync: false},
		{Harness: HarnessCodex, Skill: "rpi", Path: "skills-codex/rpi/SKILL.md", ContentHash: "ghi", OutOfSync: false},
	}
}

func TestInMemoryHarness_StatusReturnsAllEntries(t *testing.T) {
	h := NewInMemoryHarness(sampleHarnessEntries())
	got, err := h.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4", len(got))
	}
}

func TestInMemoryHarness_StatusReturnsFreshCopy(t *testing.T) {
	h := NewInMemoryHarness(sampleHarnessEntries())
	got, _ := h.Status(context.Background())
	// Mutate the returned slice — should not affect subsequent calls
	got[0].OutOfSync = true
	got2, _ := h.Status(context.Background())
	if got2[0].OutOfSync != false {
		t.Fatal("Status not returning fresh copy: mutation bled back into adapter")
	}
}

func TestInMemoryHarness_StatusForSkillFiltersByName(t *testing.T) {
	h := NewInMemoryHarness(sampleHarnessEntries())
	got, err := h.StatusForSkill(context.Background(), "evolve")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (evolve appears in both claude + codex)", len(got))
	}
	for _, e := range got {
		if e.Skill != "evolve" {
			t.Fatalf("got entry for %q, want only 'evolve'", e.Skill)
		}
	}
}

func TestInMemoryHarness_StatusForSkillUnknownReturnsEmpty(t *testing.T) {
	h := NewInMemoryHarness(sampleHarnessEntries())
	got, err := h.StatusForSkill(context.Background(), "does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0", len(got))
	}
}

func TestInMemoryHarness_StatusForSkillEmptyRejected(t *testing.T) {
	h := NewInMemoryHarness(nil)
	_, err := h.StatusForSkill(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty skill, got nil")
	}
}

func TestInMemoryHarness_StatusEmptyAdapterReturnsNonNilEmpty(t *testing.T) {
	h := NewInMemoryHarness(nil)
	got, err := h.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0", len(got))
	}
}

func TestInMemoryHarness_HonorsContextCancellation(t *testing.T) {
	h := NewInMemoryHarness(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := h.Status(ctx); err == nil {
		t.Fatal("Status: expected cancellation error, got nil")
	} else if !errors.Is(err, context.Canceled) {
		t.Fatalf("Status error = %v, want context.Canceled", err)
	}
	if _, err := h.StatusForSkill(ctx, "evolve"); err == nil {
		t.Fatal("StatusForSkill: expected cancellation error, got nil")
	} else if !errors.Is(err, context.Canceled) {
		t.Fatalf("StatusForSkill error = %v, want context.Canceled", err)
	}
}

func TestInMemoryHarness_OutOfSyncFlagPreserved(t *testing.T) {
	h := NewInMemoryHarness(sampleHarnessEntries())
	got, _ := h.StatusForSkill(context.Background(), "evolve")
	var outOfSyncCount int
	for _, e := range got {
		if e.OutOfSync {
			outOfSyncCount++
		}
	}
	if outOfSyncCount != 1 {
		t.Fatalf("OutOfSync count = %d, want 1 (only codex/evolve is out of sync in sample)", outOfSyncCount)
	}
}
