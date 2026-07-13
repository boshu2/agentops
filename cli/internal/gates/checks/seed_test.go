package checks

import (
	"strings"
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

func TestCLIContractGateRoutesRelevantChangesOnly(t *testing.T) {
	check, ok := gates.Default.Get("go.cli-contract")
	if !ok {
		t.Fatal("go.cli-contract gate is not registered")
	}
	if check.Backing != "check-cli-contract.sh" || !check.Blocking {
		t.Fatalf("go.cli-contract = %+v, want blocking check-cli-contract.sh", check)
	}
	if !check.Tiers.Has(gates.Fast) || !check.Tiers.Has(gates.Full) {
		t.Fatalf("go.cli-contract tiers = %v, want Fast|Full", check.Tiers)
	}
	for _, path := range []string{
		"cli/cmd/ao/root.go",
		"docs/cli-surface.md",
		"scripts/generate-cli-reference.sh",
		"tests/cli_contract_gate.bats",
	} {
		if !gates.PathMatchesAny(check.Match, path) {
			t.Errorf("go.cli-contract does not route relevant path %q", path)
		}
	}
	for _, path := range []string{"docs/adr/ADR-0001-example.md", "skills/test/SKILL.md"} {
		if gates.PathMatchesAny(check.Match, path) {
			t.Errorf("go.cli-contract incorrectly routes irrelevant path %q", path)
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
	if !strings.Contains(changed.RepairHint, "skills/heal-skill/scripts/audit.sh --strict") {
		t.Fatalf("derived.changed-scope repair hint = %q, want canonical deep-audit command", changed.RepairHint)
	}
	for _, path := range []string{
		"scripts/regen-changed-scope.sh",
		"tests/scripts/skill-standards-convergence.bats",
	} {
		if !gates.PathMatchesAny(changed.Match, path) {
			t.Errorf("derived.changed-scope does not self-route %q", path)
		}
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

func TestSkillProbeCoverageGateIsWarnFirstAdvisory(t *testing.T) {
	check, ok := gates.Default.Get("skill.probe-coverage")
	if !ok {
		t.Fatal("skill.probe-coverage gate is not registered")
	}
	if check.Backing != "check-skill-probe-coverage.sh" {
		t.Fatalf("skill.probe-coverage backing = %q, want check-skill-probe-coverage.sh", check.Backing)
	}
	// The whole point (age-e508.1): advisory-first. A product-tier badge that is
	// merely unmeasured must WARN, never block a release — the flip to blocking is
	// made deliberately once the spine is probed.
	if check.Blocking {
		t.Fatal("skill.probe-coverage must be warn-first / non-blocking (advisory)")
	}
	if !check.Tiers.Has(gates.Fast) || !check.Tiers.Has(gates.Full) {
		t.Fatalf("skill.probe-coverage tiers = %v, want Fast|Full", check.Tiers)
	}
	// Routed on skill changes + the MEASURED ledger + the probe scenarios so a new
	// product skill or a ledger edit re-runs it (not always-run — the probe corpus
	// is the scope).
	for _, want := range []string{"skills/**", "skills/SKILL-TIERS.md", "evals/skill-probes/**", "scripts/check-skill-probe-coverage.sh"} {
		if !gates.PathMatchesAny(check.Match, want) {
			t.Fatalf("skill.probe-coverage must route on %q; match globs = %v", want, check.Match)
		}
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

func TestSkillContractGatesAreAlwaysRun(t *testing.T) {
	// age-2s5k, age-tpeel: these validators assert whole-skill invariants,
	// so they must run on EVERY push (empty Match => AlwaysRun), not be
	// scope-gated to skills-codex changes — otherwise latent twin drift lies
	// invisible on green main and ambushes a later unrelated skill push.
	for _, id := range []string{
		"skill.codex-rpi-contract",
		"skill.codex-lifecycle-guards",
		"skill.validation-learning-boundary",
		"skill.validation-delivery-boundary",
	} {
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

func TestNextWorkContractGateRoutesOnItsSubjectFile(t *testing.T) {
	check, ok := gates.Default.Get("skill.next-work-contract")
	if !ok {
		t.Fatal("skill.next-work-contract gate is not registered")
	}
	if check.Backing != "validate-next-work-contract-parity.sh" {
		t.Fatalf("skill.next-work-contract backing = %q, want validate-next-work-contract-parity.sh", check.Backing)
	}
	if !check.Blocking {
		t.Fatal("skill.next-work-contract must be blocking")
	}
	if !check.Tiers.Has(gates.Fast) || !check.Tiers.Has(gates.Full) {
		t.Fatalf("skill.next-work-contract tiers = %v, want Fast|Full", check.Tiers)
	}
	// The validator asserts the live queue's aggregate lifecycle plus parity
	// across the schema doc, the rpi runtime, and the validator scripts. A
	// change to any of those surfaces must route the gate — most critically
	// the subject file itself: a next-work.jsonl-only commit previously
	// SKIPped its own contract gate and had to be caught by a pawl round
	// (age-77g6).
	for _, path := range []string{
		".agents/rpi/next-work.jsonl",
		"docs/contracts/next-work.schema.md",
		"cli/internal/rpi/types.go",
		"cli/cmd/ao/rpi_loop.go",
		"scripts/validate-next-work-contract-parity.sh",
		"scripts/validate-next-work.sh",
		"skills/post-mortem/SKILL.md",
	} {
		if !gates.PathMatchesAny(check.Match, path) {
			t.Fatalf("skill.next-work-contract must route on %q; match globs = %v", path, check.Match)
		}
	}
}
