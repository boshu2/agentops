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
