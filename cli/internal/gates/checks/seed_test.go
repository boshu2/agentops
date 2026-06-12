package checks

import (
	"testing"

	"github.com/boshu2/agentops/cli/internal/gates"
)

func TestSeedChecksRegistered(t *testing.T) {
	want := []string{
		"go.build",
		"always.mutation-route",
		"always.embedded-sync",
		"skill.schema",
		"contract.registry-drift",
		"ci.policy-parity",
		"eval.corpus-freshness",
	}
	for _, id := range want {
		if _, ok := gates.Default.Get(id); !ok {
			t.Errorf("seed check %q not registered in gates.Default", id)
		}
	}
}

func TestSeedRegistryNonTrivial(t *testing.T) {
	if got := gates.Default.Len(); got < 10 {
		t.Fatalf("gates.Default.Len() = %d, want >= 10 seed checks", got)
	}
}

func TestSeedChecksHaveValidShape(t *testing.T) {
	for _, c := range gates.Default.All() {
		// exactly one of Backing/Run, non-zero tiers — Add() enforces this, so a
		// registered check that violates it would have panicked at init().
		if c.Backing == "" && c.Run == nil {
			t.Errorf("check %q has neither Backing nor Run", c.ID)
		}
		if c.Backing != "" && c.Run != nil {
			t.Errorf("check %q has both Backing and Run", c.ID)
		}
		if c.Tiers == 0 {
			t.Errorf("check %q has no tiers", c.ID)
		}
	}
}

func TestChangedScopeRegenIsSplitFromReleaseWideRegenAll(t *testing.T) {
	changed, ok := gates.Default.Get("derived.changed-scope")
	if !ok {
		t.Fatal("derived.changed-scope gate is not registered")
	}
	if changed.Tiers&gates.Fast == 0 {
		t.Fatalf("derived.changed-scope tiers = %v, want fast", changed.Tiers)
	}
	if changed.Tiers&gates.Full != 0 {
		t.Fatalf("derived.changed-scope tiers = %v, want changed-scope only in fast; release-wide regen-all owns full", changed.Tiers)
	}
	if changed.Backing != "regen-changed-scope.sh" {
		t.Fatalf("derived.changed-scope backing = %q, want regen-changed-scope.sh", changed.Backing)
	}

	releaseWide, ok := gates.Default.Get("always.regen-all")
	if !ok {
		t.Fatal("always.regen-all gate is not registered")
	}
	if releaseWide.Tiers&gates.Fast != 0 {
		t.Fatalf("always.regen-all tiers = %v, want release-wide regen-all out of fast changed-scope", releaseWide.Tiers)
	}
	if releaseWide.Tiers&gates.Full == 0 {
		t.Fatalf("always.regen-all tiers = %v, want full", releaseWide.Tiers)
	}
	if releaseWide.Backing != "regen-all.sh" {
		t.Fatalf("always.regen-all backing = %q, want regen-all.sh", releaseWide.Backing)
	}
}
