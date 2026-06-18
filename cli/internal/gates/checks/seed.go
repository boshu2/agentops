// Package checks holds the gate registry: each check registers itself into
// gates.Default via init(), so adding a check is one new registration in this
// package — there is no central orchestrator switch or barrel file to edit
// (the anti-monolith property; ag-qidx G2).
//
// Phase A seeds a representative subset that shells to existing scripts/*.sh via
// the ScriptRunner. Phase B ports individual checks to native Go Run funcs
// opportunistically (see go_build.go for the native pattern).
package checks

import "github.com/boshu2/agentops/cli/internal/gates"

// Change-class path globs (ported from the bash gate's HAS_<CLASS> regexes).
var (
	goPaths          = []string{"cli/**", "go.mod", "go.sum"}
	skillPaths       = []string{"skills/**", "skills-codex/**", "tests/skills/**"}
	frontDoorPaths   = []string{"skills/**", ".claude/workflows/**"}
	contractPaths    = []string{"docs/contracts/**", "schemas/**"}
	ciPolicyPaths    = []string{".github/workflows/validate.yml", "docs/CI-CD.md", "AGENTS.md"}
	evalPaths        = []string{"evals/**", "schemas/eval-*", "cli/internal/eval/**"}
	contextMapPaths  = []string{"skills/**", "docs/contracts/context-map.md"}
	swarmPaths       = []string{".agents/swarm/**", "schemas/swarm-*"}
	docsPaths        = []string{"docs/**", "README.md", "CHANGELOG.md", "PRODUCT.md", "SKILL-TIERS.md"}
	agentsDocPaths   = []string{"AGENTS.md", "AGENTS-WORKFLOW.md", "AGENTS-CI.md", "AGENTS-CODEX.md", "AGENTS-RUNTIME.md", ".github/workflows/validate.yml"}
	corpusPaths      = []string{".agents/**", "docs/canon/**", "canon/**"}
	goalsPaths       = []string{"GOALS.md", "spec/scenarios/**", "docs/adr/ADR-0003*"}
	registryPaths    = []string{"skills/**", "hooks/**", "evals/**", "cli/cmd/ao/**", "cli/internal/**", "registry.json"}
	docSkillRefPaths = []string{
		"AGENTS.md",
		"CLAUDE.md",
		"docs/ARCHITECTURE.md",
		"docs/SKILLS.md",
		"docs/architecture/operating-loop.md",
		"skills/SKILL-TIERS.md",
		"skills/**",
		"scripts/check-doc-skill-refs.sh",
		"tests/scripts/check-doc-skill-refs.bats",
	}
	archDocDriftPaths = []string{
		"docs/architecture/ports-and-adapters.md",
		"docs/contracts/bounded-contexts.yaml",
		"docs/reference/agentops-skill-domain-map.md",
		"docs/reference/agentops-hexagonal-architecture-map.md",
		"docs/ARCHITECTURE.md",
		"docs/CI-CD.md",
		"scripts/check-architecture-doc-drift.sh",
		"tests/scripts/check-architecture-doc-drift.bats",
	}
	cliAgentsTrackerPaths = []string{
		"cli/AGENTS.md",
		"scripts/check-cli-agents-tracker-drift.sh",
		"tests/scripts/check-cli-agents-tracker-drift.bats",
	}
	controlPlaneTaxonomyPaths = []string{
		"docs/architecture/the-agent-factory.md",
		"docs/architecture/control-loop-model.md",
		"docs/architecture/ports-and-adapters.md",
		"docs/architecture/primitive-chains.md",
		"docs/architecture/canonical-loop-model.md",
		"docs/architecture/loop-map.md",
		"scripts/check-control-plane-taxonomy.sh",
		"tests/scripts/check-control-plane-taxonomy.bats",
	}
	regenScopePaths = []string{
		"skills/**",
		"skills-codex/**",
		"skills-codex-overrides/**",
		"docs/contracts/**",
		"docs/reference/agentops-skill-domain-map.md",
		"docs/reference/agentops-hexagonal-architecture-map.md",
		"registry.json",
		"cli/cmd/ao/**",
		"cli/docs/COMMANDS.md",
		"docs/cli-surface.json",
		"docs/cli-surface.md",
		"evals/agentops-core/cli-command-surface-matrix.json",
		"evals/agentops-core/fixtures/cli-command-surface-smoke.sh",
	}
)

func init() {
	seed := []gates.Check{
		// always-run (no Match): structural invariants that hold regardless of
		// what changed.
		{ID: "always.mutation-route", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-mutation-route-coverage.sh"},
		{ID: "always.agents-write-surfaces", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-agents-write-surfaces.sh"},
		{ID: "always.door9-no-claude-p", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-door9-no-claude-p.sh"},
		{ID: "always.no-tracked-agents", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-no-tracked-agents.sh"},
		// always-run + fail-closed PATH guard: a private artifact (corpus,
		// tracker, untraceable wiki) must never reach the PUBLIC repo. No Match
		// glob — a force-added private path might not match any corpus glob, so
		// changed-file scoping must never be able to skip this (ag-ao0eo).
		{ID: "corpus.path-guard", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-corpus-path-guard.sh"},
		// local-only br ledger policy: _beads is gitignored/private, so this
		// must be always-run and gracefully skip when the ledger is absent.
		{ID: "always.ledger-prefix-policy", Tiers: gates.Fast | gates.Full, Blocking: false, Backing: "check-ledger-prefix-policy.sh"},
		{ID: "always.embedded-sync", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "validate-embedded-sync.sh"},

		// routed by change class
		{ID: "go.command-test-pair", Tiers: gates.Fast | gates.Full, Match: goPaths, Blocking: true, Backing: "check-go-command-test-pair.sh"},
		{ID: "skill.schema", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-skill-schema.sh"},
		{ID: "skill.triggers", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-skill-triggers.sh"},
		{ID: "contract.registry-drift", Tiers: gates.Fast | gates.Full, Match: contractPaths, Blocking: true, Backing: "check-registry-drift.sh", RepairHint: "bash scripts/generate-registry.sh"},
		{ID: "contract.bounded-contexts-drift", Tiers: gates.Fast | gates.Full, Match: contractPaths, Blocking: true, Backing: "check-bounded-contexts-drift.sh"},
		{ID: "contract.disposition-schema", Tiers: gates.Fast | gates.Full, Match: contractPaths, Blocking: true, Backing: "validate-skill-disposition-schema.sh"},
		{ID: "contract.finding-registry", Tiers: gates.Fast | gates.Full, Match: contractPaths, Blocking: true, Backing: "check-finding-registry.sh"},
		{ID: "ci.policy-parity", Tiers: gates.Fast | gates.Full, Match: ciPolicyPaths, Blocking: true, Backing: "validate-ci-policy-parity.sh"},
		{ID: "eval.corpus-freshness", Tiers: gates.Fast | gates.Full, Match: evalPaths, Blocking: true, Backing: "check-corpus-freshness.sh"},

		// skill class (PB1 parity batch — all shell-backed via ScriptRunner)
		{ID: "skill.cli-skills-map", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-cli-skills-map.sh"},
		{ID: "skill.runtime-formats", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-skill-runtime-formats.sh"},
		{ID: "skill.runtime-parity", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-skill-runtime-parity.sh"},
		{ID: "skill.cli-snippets", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-skill-cli-snippets.sh"},
		{ID: "skill.manifests", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-manifests.sh", Args: []string{"--repo-root", "."}},
		{ID: "skill.next-work-contract", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-next-work-contract-parity.sh"},
		{ID: "skill.codex-parity-drift", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "check-codex-parity-drift.sh"},
		{ID: "skill.codex-runtime-sections", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-codex-runtime-sections.sh"},
		{ID: "skill.codex-override-coverage", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-codex-override-coverage.sh"},
		{ID: "skill.codex-backbone-prompts", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-codex-backbone-prompts.sh"},
		{ID: "skill.codex-rpi-contract", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-codex-rpi-contract.sh"},
		{ID: "skill.codex-lifecycle-guards", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-codex-lifecycle-guards.sh"},
		{ID: "skill.codex-generated-artifacts", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-codex-generated-artifacts.sh"},
		{ID: "skill.isolation", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: false, Backing: "check-skill-isolation.sh"},
		{ID: "skill.heal-strict", Tiers: gates.Full, Match: skillPaths, Blocking: true, Backing: "skills/heal-skill/scripts/heal.sh", Args: []string{"--strict"}},
		{ID: "skill.frontmatter-v2", Tiers: gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-skill-frontmatter.sh"},
		{ID: "skill.body-refs", Tiers: gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-skill-body-refs.sh"},
		{ID: "skill.flow", Tiers: gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-skill-flow.sh"},
		{ID: "skill.domain-map-golden", Tiers: gates.Full, Match: skillPaths, Blocking: true, Backing: "generate-skill-domain-map.sh", Args: []string{"--check"}},
		{ID: "skill.scenario-test-linkage", Tiers: gates.Full, Match: skillPaths, Blocking: true, Backing: "check-scenario-test-linkage.sh"},

		// governance front-door admission (M5): a newly-ADDED skill/workflow/loop
		// cannot merge without bounded-context + role + a runnable acceptance.
		{ID: "governance.frontdoor-admission", Tiers: gates.Fast | gates.Full, Match: frontDoorPaths, Blocking: true, Backing: "check-frontdoor-admission.sh"},

		// go class
		{ID: "go.home-isolation", Tiers: gates.Fast | gates.Full, Match: goPaths, Blocking: true, Backing: "check-home-isolation.sh"},
		{ID: "go.test-home-isolation", Tiers: gates.Fast | gates.Full, Match: goPaths, Blocking: true, Backing: "check-test-home-isolation.sh"},
		{ID: "go.complexity", Tiers: gates.Full, Match: goPaths, Blocking: true, Backing: "check-go-complexity.sh"},
		{ID: "go.cli-reference", Tiers: gates.Full, Match: goPaths, Blocking: true, Backing: "generate-cli-reference.sh", Args: []string{"--check"}},
		{ID: "go.cli-surface-counts", Tiers: gates.Full, Match: goPaths, Blocking: true, Backing: "update-cli-surface-counts.sh"},
		{ID: "go.test-count-regression", Tiers: gates.Full, Match: goPaths, Blocking: true, Backing: "check-test-count-regression.sh"},
		{ID: "go.test-isolation", Tiers: gates.Fast | gates.Full, Match: goPaths, Blocking: true, Backing: "check-test-isolation.sh"},
		{ID: "go.test-staleness", Tiers: gates.Full, Match: goPaths, Blocking: false, Backing: "check-test-staleness.sh"},

		// contract / context-map / swarm classes
		{ID: "contract.compatibility", Tiers: gates.Fast | gates.Full, Match: contractPaths, Blocking: true, Backing: "check-contract-compatibility.sh"},
		{ID: "contract.context-map-drift", Tiers: gates.Fast | gates.Full, Match: contextMapPaths, Blocking: true, Backing: "validate-context-map-drift.sh", RepairHint: "bash scripts/generate-context-map.sh"},
		{ID: "contract.registry-json", Tiers: gates.Full, Match: registryPaths, Blocking: true, Backing: "generate-registry.sh", Args: []string{"--check"}},
		{ID: "contract.sku-catalog-drift", Tiers: gates.Full, Match: registryPaths, Blocking: true, Backing: "validate-sku-catalog-drift.sh"},
		{ID: "docs.agents-split", Tiers: gates.Full, Match: agentsDocPaths, Blocking: true, Backing: "validate-agents-split.sh"},
		{ID: "swarm.evidence", Tiers: gates.Fast | gates.Full, Match: swarmPaths, Blocking: true, Backing: "validate-swarm-evidence.sh"},

		// always-run structural invariants (no Match)
		{ID: "always.author-judge-convergence", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-author-judge-convergence.sh"},
		{ID: "always.contracts-structural-floor", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-contracts-structural-floor.sh"},
		{ID: "always.docs-learning-references", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-docs-learning-references.sh"},
		{ID: "docs.skill-refs", Tiers: gates.Fast | gates.Full, Match: docSkillRefPaths, Blocking: true,
			Backing: "check-doc-skill-refs.sh", Args: []string{"--strict"}},
		{ID: "cli.agents-tracker", Tiers: gates.Fast | gates.Full, Match: cliAgentsTrackerPaths, Blocking: true,
			Backing: "check-cli-agents-tracker-drift.sh"},
		{ID: "docs.architecture-drift", Tiers: gates.Fast | gates.Full, Match: archDocDriftPaths, Blocking: true,
			Backing: "check-architecture-doc-drift.sh"},
		{ID: "docs.control-plane-taxonomy", Tiers: gates.Fast | gates.Full, Match: controlPlaneTaxonomyPaths, Blocking: true,
			Backing: "check-control-plane-taxonomy.sh",
			RepairHint: "keep the etcd-analog bound to br + the proof/verdict ledger (not bd/Dolt); keep the agent two-altitude note in the-agent-factory.md + ports-and-adapters.md; keep the taxonomy cross-links bidirectional; see scripts/check-control-plane-taxonomy.sh"},
		{ID: "eval.skill-probe-i0", Tiers: gates.Full, Match: skillPaths, Blocking: false,
			Backing: "skill-probe-i0.sh", Args: []string{"skills", ".agents/ao/skill-eval"}},
		{ID: "provenance.orphans", Tiers: gates.Full, Match: contractPaths, Blocking: true,
			Backing: "check-provenance-orphans.sh"},
		{ID: "always.docs-hookless", Tiers: gates.Full, Blocking: true, Backing: "check-doc-hooks-drift.sh"},
		{ID: "always.flywheel-compounding-snapshot", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-flywheel-compounding-snapshot.sh"},
		{ID: "always.retrieval-manifest-paths", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-retrieval-manifest-paths.sh",
			Args: []string{"cli/cmd/ao/testdata/retrieval-bench/search-eval-manifest.json"}},
		{ID: "always.wiring-closure", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-wiring-closure.sh"},
		{ID: "always.bd-closeout-contract", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "validate-bd-closeout-contract.sh"},
		{ID: "always.domain-evolution-plan", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-agentops-domain-evolution-plan.sh"},
		{ID: "always.file-manifest-overlap", Tiers: gates.Full, Blocking: true, Backing: "check-file-manifest-overlap.sh"},
		{ID: "derived.changed-scope", Tiers: gates.Fast, Blocking: true, Backing: "regen-changed-scope.sh", Args: []string{"--check", "--scope", "head"},
			Match: regenScopePaths, RepairHint: "bash scripts/regen-changed-scope.sh --scope head"},
		{ID: "always.regen-all", Tiers: gates.Full, Blocking: true, Backing: "regen-all.sh", Args: []string{"--check"}, RepairHint: "bash scripts/regen-all.sh"},
		{ID: "always.three-gap-supergate", Tiers: gates.Full, Match: goalsPaths, Blocking: true, Backing: "check-three-gap-supergate.sh"},
		{ID: "always.sovereignty-proof-citations", Tiers: gates.Full, Match: docsPaths, Blocking: true, Backing: "validate-sovereignty-proof-citations.sh"},
		{ID: "docs.no-retired-tech", Tiers: gates.Fast | gates.Full, Match: docsPaths, Blocking: true, Backing: "check-docs-no-retired-tech.sh", RepairHint: "convert to current truth, or add a RETIRED/HISTORICAL banner in the first 15 lines; see scripts/check-docs-no-retired-tech.sh"},
		{ID: "corpus.secret-scan", Tiers: gates.Full, Match: corpusPaths, Blocking: true, Backing: "check-corpus-secret-scan.sh"},
		{ID: "corpus.witness-dolt-jsonl-crosscheck", Tiers: gates.Full, Match: corpusPaths, Blocking: true, Backing: "witness-dolt-jsonl-crosscheck.sh"},
		{ID: "doctrine.memrl-health", Tiers: gates.Full, Blocking: true, Backing: "check-memrl-health.sh"},
		{ID: "doctrine.flywheel-proof", Tiers: gates.Full, Blocking: true, Backing: "proof-run.sh"},
		{ID: "eval.retrieval-quality-smoke", Tiers: gates.Full, Blocking: true, Backing: "retrieval-quality-smoke.sh"},

		// full-mode-only / advisory (mirror the bash gate: these skip in fast or warn)
		{ID: "full.worktree-disposition", Tiers: gates.Full, Blocking: true, Backing: "check-worktree-disposition.sh"},
		{ID: "full.retrieval-quality-ratchet", Tiers: gates.Full, Blocking: false, Backing: "check-retrieval-quality-ratchet.sh"},
		{ID: "always.loop-shape", Tiers: gates.Fast | gates.Full, Blocking: false, Backing: "check-loop-shape.sh"},
		{ID: "skill.catalog-drift", Tiers: gates.Full, Blocking: false, Backing: "check-skill-catalog-drift.sh"},

		// final backing-script batch (PB1)
		{ID: "always.quarantine-empty", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-quarantine-empty.sh"},
		{ID: "always.test-fixture-parity", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-test-fixture-parity.sh"},
		{ID: "go.race-fast", Tiers: gates.Fast | gates.Full, Match: goPaths, Blocking: true, Backing: "validate-go-fast.sh"},
		{ID: "full.headless-runtime-skills", Tiers: gates.Full, Blocking: false, Backing: "validate-headless-runtime-skills.sh"},
		// release-audit: narrow Match (only release files) + --mode changed, mirroring
		// the bash gate's needs_release_audit_artifact_check (PB1a Args support).
		{ID: "release.audit-artifacts", Tiers: gates.Fast | gates.Full, Blocking: true,
			Backing: "validate-release-audit-artifacts.sh", Args: []string{"--mode", "changed"},
			Match: []string{"docs/releases/**", "scripts/ci-local-release.sh", "scripts/resolve-release-artifacts.sh", "scripts/validate-release-audit-artifacts.sh", "tests/scripts/release-artifacts.bats"}},
		// DEFERRED: check-agents-hash-snapshot.sh is a stateful capture/diff pair —
		// needs a native Go port, not a single wrapper.
	}
	for _, c := range seed {
		gates.Register(c)
	}
}
