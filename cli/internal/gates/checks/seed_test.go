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
		"skill.schema",
		"contract.skill-mesh",
		"ci.policy-parity",
		"corpus.path-guard",
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
	if !strings.Contains(changed.RepairHint, "skills/skill-builder/scripts/heal.sh --check --strict") {
		t.Fatalf("derived.changed-scope repair hint = %q, want one-pass structural audit command", changed.RepairHint)
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
	if !strings.Contains(check.RepairHint, "v3 scorecard") || strings.Contains(check.RepairHint, "v2 scorecard") {
		t.Fatalf("skill.probe-coverage repair hint = %q, want current v3 scorecard guidance", check.RepairHint)
	}
	// Routed on skill changes + the measurement-status ledger + the probe scenarios so a new
	// product skill or a ledger edit re-runs it (not always-run — the probe corpus
	// is the scope).
	for _, want := range []string{
		"skills/**",
		"skills/SKILL-TIERS.md",
		"evals/skill-probes/**",
		"scripts/probe-skill.sh",
		"scripts/lib/probe-fixture-metadata.py",
		"scripts/lib/codex-exec.sh",
		"scripts/lib/preamble.sh",
		"scripts/check-skill-probe-coverage.sh",
		"scripts/.skill-probe-denominator-exclusions",
		"tests/scripts/probe-skill.bats",
		"tests/scripts/check-skill-probe-coverage.bats",
	} {
		if !gates.PathMatchesAny(check.Match, want) {
			t.Fatalf("skill.probe-coverage must route on %q; match globs = %v", want, check.Match)
		}
	}
}

func TestSkillProbeHeadroomGateIsWarnFirstAdvisory(t *testing.T) {
	check, ok := gates.Default.Get("skill.probe-headroom")
	if !ok {
		t.Fatal("skill.probe-headroom gate is not registered")
	}
	if check.Backing != "check-skill-probe-headroom.sh" {
		t.Fatalf("skill.probe-headroom backing = %q, want check-skill-probe-headroom.sh", check.Backing)
	}
	// Advisory-first, exactly like skill.probe-coverage: a saturated historical
	// probe group is a true finding about the ledger, not a regression the
	// change under test introduced. The flip to blocking is made deliberately,
	// on measured evidence.
	if check.Blocking {
		t.Fatal("skill.probe-headroom must be warn-first / non-blocking (advisory)")
	}
	if !check.Tiers.Has(gates.Fast) || !check.Tiers.Has(gates.Full) {
		t.Fatalf("skill.probe-headroom tiers = %v, want Fast|Full", check.Tiers)
	}
	if !strings.Contains(check.RepairHint, "SATURATED") {
		t.Fatalf("skill.probe-headroom repair hint = %q, want the retire-don't-rerun guidance", check.RepairHint)
	}
	// Routed on the probe corpus, the scorecard evidence it sweeps, and the
	// detector's own surfaces (rule, helper, fixtures, script, bats).
	for _, want := range []string{
		"evals/skill-probes/LEDGER.md",
		"docs/evals/scorecards/2026-08-05/validate-not-proven-v2-low.json",
		"tests/fixtures/skill-probes/saturated/fixture-saturated-quiz-low.json",
		"cli/internal/probeheadroom/probeheadroom.go",
		"cli/cmd/probe-headroom/main.go",
		"scripts/check-skill-probe-headroom.sh",
		"tests/scripts/check-skill-probe-headroom.bats",
	} {
		if !gates.PathMatchesAny(check.Match, want) {
			t.Fatalf("skill.probe-headroom must route on %q; match globs = %v", want, check.Match)
		}
	}
	// Headroom is a property of the SCENARIO and its captures, not of the skill
	// under test: a SKILL.md edit must not re-run this sweep.
	if gates.PathMatchesAny(check.Match, "skills/validate/SKILL.md") {
		t.Fatalf("skill.probe-headroom must not route on skills/**; match globs = %v", check.Match)
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
	if check.Tiers.Has(gates.Fast) || !check.Tiers.Has(gates.Full) {
		t.Fatalf("docs.skill-refs tiers = %v, want Full only (not on --fast path)", check.Tiers)
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

func TestGateTighteningRatchetIsAdvisoryAndRoutedOnTheGateSurface(t *testing.T) {
	check, ok := gates.Default.Get("gate.tightening-ratchet")
	if !ok {
		t.Fatal("gate.tightening-ratchet is not registered")
	}
	if check.Backing != "check-gate-tightening-ratchet.sh" {
		t.Fatalf("gate.tightening-ratchet backing = %q, want check-gate-tightening-ratchet.sh", check.Backing)
	}
	// Advisory PERMANENTLY at this fidelity: the detector is a text-diff
	// heuristic over the gate surface, so a false positive must never block a
	// push. A flip to blocking needs a precision detector earned on evidence.
	if check.Blocking {
		t.Fatal("gate.tightening-ratchet must be warn-first / non-blocking (advisory)")
	}
	if check.Tiers.Has(gates.Fast) || !check.Tiers.Has(gates.Full) {
		t.Fatalf("gate.tightening-ratchet tiers = %v, want Full only (it diffs a whole range)", check.Tiers)
	}
	if !strings.Contains(check.RepairHint, "Gate-Loosen-Reason") {
		t.Fatalf("gate.tightening-ratchet repair hint = %q, want the trailer escape hatch named", check.RepairHint)
	}
	// The two governed classes plus self-reference.
	for _, want := range []string{
		"cli/internal/gates/checks/seed.go",
		"scripts/check-go-lint.sh",
		"tests/scripts/check-gate-tightening-ratchet.bats",
	} {
		if !gates.PathMatchesAny(check.Match, want) {
			t.Fatalf("gate.tightening-ratchet must route on %q; match globs = %v", want, check.Match)
		}
	}
	// A skill or doc edit cannot loosen a gate threshold; it must not re-run.
	for _, unwanted := range []string{"skills/validate/SKILL.md", "docs/CI-CD.md"} {
		if gates.PathMatchesAny(check.Match, unwanted) {
			t.Fatalf("gate.tightening-ratchet must not route on %q; match globs = %v", unwanted, check.Match)
		}
	}
}

func TestEvidenceGroundingIsAdvisoryAndRoutedOnEvidenceRoots(t *testing.T) {
	check, ok := gates.Default.Get("evidence.grounding")
	if !ok {
		t.Fatal("evidence.grounding is not registered")
	}
	if check.Backing != "check-evidence-grounding.sh" {
		t.Fatalf("evidence.grounding backing = %q, want check-evidence-grounding.sh", check.Backing)
	}
	if check.Blocking {
		t.Fatal("evidence.grounding must be warn-first / non-blocking (advisory)")
	}
	if check.Tiers.Has(gates.Fast) || !check.Tiers.Has(gates.Full) {
		t.Fatalf("evidence.grounding tiers = %v, want Full only (it scans the whole evidence corpus)", check.Tiers)
	}
	if !strings.Contains(check.RepairHint, ".evidence-grounding-baseline") {
		t.Fatalf("evidence.grounding repair hint = %q, want the baseline named", check.RepairHint)
	}
	// The three evidence roots, the declared baseline, and the detector's own
	// surfaces (script, shared libs, bats).
	for _, want := range []string{
		"docs/audits/2026-07-15-skills-go-cli-audit.md",
		"docs/evidence/membrane-receipts.md",
		"docs/handoffs/2026-06-07-ag-qidx-postmortem.md",
		"scripts/.evidence-grounding-baseline",
		"scripts/check-evidence-grounding.sh",
		"scripts/lib/docs-scope.sh",
		"tests/scripts/check-evidence-grounding.bats",
	} {
		if !gates.PathMatchesAny(check.Match, want) {
			t.Fatalf("evidence.grounding must route on %q; match globs = %v", want, check.Match)
		}
	}
	// Deleting the file an audit CITES is the defect this gate reports, not a
	// reason to re-run it on the deleting change.
	if gates.PathMatchesAny(check.Match, "cli/internal/gates/checks/seed.go") {
		t.Fatalf("evidence.grounding must not route on cited source paths; match globs = %v", check.Match)
	}
}

func TestShellExecBitsGateIsAdvisoryAndRoutesShellSurfaces(t *testing.T) {
	check, ok := gates.Default.Get("shell.exec-bits")
	if !ok {
		t.Fatal("shell.exec-bits gate is not registered")
	}
	if check.Backing != "check-shell-exec-bits.sh" {
		t.Fatalf("shell.exec-bits backing = %q, want check-shell-exec-bits.sh", check.Backing)
	}
	// Advisory by design: a mode bit is repository hygiene, not a correctness
	// floor, and the repo convention is advisory-first for a new gate.
	if check.Blocking {
		t.Fatal("shell.exec-bits must be non-blocking (advisory)")
	}
	if !check.Tiers.Has(gates.Fast) || !check.Tiers.Has(gates.Full) {
		t.Fatalf("shell.exec-bits tiers = %v, want Fast|Full", check.Tiers)
	}
	if !strings.Contains(check.RepairHint, "git update-index --chmod=+x") {
		t.Fatalf("shell.exec-bits repair hint = %q, want the update-index repair command", check.RepairHint)
	}
	for _, want := range []string{
		"scripts/regen-all.sh",
		"scripts/lib/preamble.sh",
		"tests/run-all.sh",
		"scripts/check-shell-exec-bits.sh",
		"tests/scripts/legible-l2-exec-bits.bats",
	} {
		if !gates.PathMatchesAny(check.Match, want) {
			t.Fatalf("shell.exec-bits must route on %q; match globs = %v", want, check.Match)
		}
	}
	for _, unwanted := range []string{"cli/cmd/ao/root.go", "skills/validate/SKILL.md", "README.md"} {
		if gates.PathMatchesAny(check.Match, unwanted) {
			t.Errorf("shell.exec-bits incorrectly routes irrelevant path %q", unwanted)
		}
	}
}
