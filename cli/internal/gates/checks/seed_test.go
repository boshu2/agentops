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
		"docs.no-retired-tech",
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

func TestRetrievalManifestPathGateHasFixtureManifestArgs(t *testing.T) {
	check, ok := gates.Default.Get("always.retrieval-manifest-paths")
	if !ok {
		t.Fatal("always.retrieval-manifest-paths gate is not registered")
	}
	if check.Backing != "check-retrieval-manifest-paths.sh" {
		t.Fatalf("always.retrieval-manifest-paths backing = %q, want check-retrieval-manifest-paths.sh", check.Backing)
	}

	want := []string{"cli/cmd/ao/testdata/retrieval-bench/search-eval-manifest.json"}
	if len(check.Args) != len(want) {
		t.Fatalf("always.retrieval-manifest-paths args = %v, want %v", check.Args, want)
	}
	for i := range want {
		if check.Args[i] != want[i] {
			t.Fatalf("always.retrieval-manifest-paths args = %v, want %v", check.Args, want)
		}
	}
}

func TestSkillIsolationGateIsWarnFirst(t *testing.T) {
	check, ok := gates.Default.Get("skill.isolation")
	if !ok {
		t.Fatal("skill.isolation gate is not registered")
	}
	if check.Backing != "check-skill-isolation.sh" {
		t.Fatalf("skill.isolation backing = %q, want check-skill-isolation.sh", check.Backing)
	}
	if check.Blocking {
		t.Fatal("skill.isolation must be warn-first / non-blocking")
	}
	if !check.Tiers.Has(gates.Fast) || !check.Tiers.Has(gates.Full) {
		t.Fatalf("skill.isolation tiers = %v, want Fast|Full", check.Tiers)
	}
	if len(check.Match) == 0 {
		t.Fatal("skill.isolation should be routed by skill paths, not always-run")
	}
}

func TestLedgerPrefixPolicyGateIsWarnFirstLocalOnly(t *testing.T) {
	check, ok := gates.Default.Get("always.ledger-prefix-policy")
	if !ok {
		t.Fatal("always.ledger-prefix-policy gate is not registered")
	}
	if check.Backing != "check-ledger-prefix-policy.sh" {
		t.Fatalf("always.ledger-prefix-policy backing = %q, want check-ledger-prefix-policy.sh", check.Backing)
	}
	if check.Blocking {
		t.Fatal("always.ledger-prefix-policy must be warn-first / non-blocking")
	}
	if !check.Tiers.Has(gates.Fast) || !check.Tiers.Has(gates.Full) {
		t.Fatalf("always.ledger-prefix-policy tiers = %v, want Fast|Full", check.Tiers)
	}
	if len(check.Match) != 0 {
		t.Fatalf("always.ledger-prefix-policy should be always-run with graceful local-only skip; got Match=%v", check.Match)
	}
}

func TestDocSkillRefsGateIsBlockingAndStrict(t *testing.T) {
	check, ok := gates.Default.Get("docs.skill-refs")
	if !ok {
		t.Fatal("docs.skill-refs gate is not registered")
	}
	if check.Backing != "check-doc-skill-refs.sh" {
		t.Fatalf("docs.skill-refs backing = %q, want check-doc-skill-refs.sh", check.Backing)
	}
	if !check.Blocking {
		t.Fatal("docs.skill-refs must be blocking")
	}
	if !check.Tiers.Has(gates.Fast) || !check.Tiers.Has(gates.Full) {
		t.Fatalf("docs.skill-refs tiers = %v, want Fast|Full", check.Tiers)
	}
	if len(check.Args) != 2 || check.Args[0] != "--all-docs" || check.Args[1] != "--strict" {
		t.Fatalf("docs.skill-refs args = %v, want [--all-docs --strict]", check.Args)
	}
	if len(check.Match) == 0 {
		t.Fatal("docs.skill-refs should be routed by live docs + skill paths")
	}
	// The gate now runs in --all-docs mode (scans every LIVE docs/** file), so
	// the Match is widened to docs/** plus the pinned doctrine + the script,
	// baseline, and lib self-references.
	for _, want := range []string{"AGENTS.md", "CLAUDE.md", "docs/**", "skills/SKILL-TIERS.md", "scripts/.docs-skill-refs-baseline", "scripts/lib/docs-scope.sh"} {
		found := false
		for _, got := range check.Match {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("docs.skill-refs match paths missing %q in %v", want, check.Match)
		}
	}
}

func TestCliAgentsTrackerGateIsBlockingAndStrict(t *testing.T) {
	check, ok := gates.Default.Get("cli.agents-tracker")
	if !ok {
		t.Fatal("cli.agents-tracker gate is not registered")
	}
	if check.Backing != "check-cli-agents-tracker-drift.sh" {
		t.Fatalf("cli.agents-tracker backing = %q, want check-cli-agents-tracker-drift.sh", check.Backing)
	}
	if !check.Blocking {
		t.Fatal("cli.agents-tracker must be blocking")
	}
	if !check.Tiers.Has(gates.Fast) || !check.Tiers.Has(gates.Full) {
		t.Fatalf("cli.agents-tracker tiers = %v, want Fast|Full", check.Tiers)
	}
	if len(check.Match) == 0 {
		t.Fatal("cli.agents-tracker should be routed by cli/AGENTS.md + checker paths")
	}
	for _, want := range []string{"cli/AGENTS.md", "scripts/check-cli-agents-tracker-drift.sh", "tests/scripts/check-cli-agents-tracker-drift.bats"} {
		found := false
		for _, got := range check.Match {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("cli.agents-tracker match paths missing %q in %v", want, check.Match)
		}
	}
}

func TestCodexContractGatesAreAlwaysRun(t *testing.T) {
	// age-2s5k: the two codex-contract validators assert whole-twin invariants,
	// so they must run on EVERY push (empty Match => AlwaysRun), not be
	// scope-gated to skills-codex changes — otherwise latent twin drift lies
	// invisible on green main and ambushes a later unrelated skill push.
	for _, id := range []string{"skill.codex-rpi-contract", "skill.codex-lifecycle-guards"} {
		check, ok := gates.Default.Get(id)
		if !ok {
			t.Fatalf("%s gate is not registered", id)
		}
		if !check.AlwaysRun() {
			t.Errorf("%s must be always-run (empty Match) so twin drift fails the next push regardless of scope; got Match=%v", id, check.Match)
		}
		if !check.Blocking {
			t.Errorf("%s must be blocking", id)
		}
		if !check.Tiers.Has(gates.Fast) || !check.Tiers.Has(gates.Full) {
			t.Errorf("%s tiers = %v, want Fast|Full so routine fast pushes are covered", id, check.Tiers)
		}
	}
}

func TestArchitectureDriftGateIsBlocking(t *testing.T) {
	check, ok := gates.Default.Get("docs.architecture-drift")
	if !ok {
		t.Fatal("docs.architecture-drift gate is not registered")
	}
	if check.Backing != "check-architecture-doc-drift.sh" {
		t.Fatalf("docs.architecture-drift backing = %q, want check-architecture-doc-drift.sh", check.Backing)
	}
	if !check.Blocking {
		t.Fatal("docs.architecture-drift must be blocking")
	}
}
