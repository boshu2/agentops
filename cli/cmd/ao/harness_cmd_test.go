// practices: [tdd]
package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

func TestHarnessStatus_EmitsAllEntries(t *testing.T) {
	stub := func(_ context.Context, _ harnessStatusOptions) ([]ports.HarnessSkillSync, error) {
		return []ports.HarnessSkillSync{
			{Harness: ports.HarnessClaude, Skill: "evolve", Path: "skills/evolve/SKILL.md", ContentHash: "h1"},
			{Harness: ports.HarnessCodex, Skill: "evolve", Path: "skills-codex/evolve/SKILL.md", ContentHash: "h2", OutOfSync: true},
		}, nil
	}
	var buf bytes.Buffer
	err := harnessStatusRun(context.Background(), harnessStatusOptions{
		writer:   &buf,
		statusFn: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("len = %d, want 2", len(lines))
	}
	if !strings.Contains(lines[0], `"Harness":"claude"`) {
		t.Fatalf("missing Harness claude: %s", lines[0])
	}
	if !strings.Contains(lines[1], `"OutOfSync":true`) {
		t.Fatalf("missing OutOfSync:true on codex: %s", lines[1])
	}
}

func TestHarnessStatus_OutOfSyncOnlyFilters(t *testing.T) {
	stub := func(_ context.Context, _ harnessStatusOptions) ([]ports.HarnessSkillSync, error) {
		return []ports.HarnessSkillSync{
			{Harness: ports.HarnessClaude, Skill: "a", OutOfSync: false},
			{Harness: ports.HarnessCodex, Skill: "a", OutOfSync: true},
			{Harness: ports.HarnessClaude, Skill: "b", OutOfSync: false},
		}, nil
	}
	var buf bytes.Buffer
	err := harnessStatusRun(context.Background(), harnessStatusOptions{
		outOfSyncOnly: true,
		writer:        &buf,
		statusFn:      stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("--out-of-sync-only should filter to 1 entry, got %d", len(lines))
	}
}

func TestHarnessStatus_EmptyResultsEmitsZeroBytes(t *testing.T) {
	stub := func(_ context.Context, _ harnessStatusOptions) ([]ports.HarnessSkillSync, error) {
		return []ports.HarnessSkillSync{}, nil
	}
	var buf bytes.Buffer
	err := harnessStatusRun(context.Background(), harnessStatusOptions{
		writer:   &buf,
		statusFn: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Fatalf("empty result should emit 0 bytes, got %q", buf.String())
	}
}

func TestHarnessStatus_AllInSyncWithFilterEmitsZero(t *testing.T) {
	stub := func(_ context.Context, _ harnessStatusOptions) ([]ports.HarnessSkillSync, error) {
		return []ports.HarnessSkillSync{
			{Skill: "a", OutOfSync: false},
			{Skill: "b", OutOfSync: false},
		}, nil
	}
	var buf bytes.Buffer
	err := harnessStatusRun(context.Background(), harnessStatusOptions{
		outOfSyncOnly: true,
		writer:        &buf,
		statusFn:      stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Fatalf("all-in-sync should produce 0 bytes when --out-of-sync-only, got %q", buf.String())
	}
}

func TestHarnessStatus_ErrorWrapped(t *testing.T) {
	stub := func(_ context.Context, _ harnessStatusOptions) ([]ports.HarnessSkillSync, error) {
		return nil, errors.New("scan failed")
	}
	err := harnessStatusRun(context.Background(), harnessStatusOptions{
		statusFn: stub,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "harness status:") {
		t.Fatalf("error not wrapped: %v", err)
	}
}
