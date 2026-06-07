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
	goPaths       = []string{"cli/**", "go.mod", "go.sum"}
	skillPaths    = []string{"skills/**", "skills-codex/**", "tests/skills/**"}
	contractPaths   = []string{"docs/contracts/**", "schemas/**"}
	ciPolicyPaths   = []string{".github/workflows/validate.yml", "docs/CI-CD.md", "AGENTS.md"}
	evalPaths       = []string{"evals/**", "schemas/eval-*", "cli/internal/eval/**"}
	contextMapPaths = []string{"skills/**", "docs/contracts/context-map.md"}
	swarmPaths      = []string{".agents/swarm/**", "schemas/swarm-*"}
	docsPaths       = []string{"docs/**", "README.md", "CHANGELOG.md", "PRODUCT.md", "SKILL-TIERS.md"}
)

func init() {
	seed := []gates.Check{
		// always-run (no Match): structural invariants that hold regardless of
		// what changed.
		{ID: "always.mutation-route", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-mutation-route-coverage.sh"},
		{ID: "always.agents-write-surfaces", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-agents-write-surfaces.sh"},
		{ID: "always.no-tracked-agents", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-no-tracked-agents.sh"},
		{ID: "always.embedded-sync", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "validate-embedded-sync.sh"},

		// routed by change class
		{ID: "go.command-test-pair", Tiers: gates.Fast | gates.Full, Match: goPaths, Blocking: true, Backing: "check-go-command-test-pair.sh"},
		{ID: "skill.schema", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-skill-schema.sh"},
		{ID: "contract.registry-drift", Tiers: gates.Fast | gates.Full, Match: contractPaths, Blocking: true, Backing: "check-registry-drift.sh"},
		{ID: "contract.bounded-contexts-drift", Tiers: gates.Fast | gates.Full, Match: contractPaths, Blocking: true, Backing: "check-bounded-contexts-drift.sh"},
		{ID: "contract.finding-registry", Tiers: gates.Fast | gates.Full, Match: contractPaths, Blocking: true, Backing: "check-finding-registry.sh"},
		{ID: "ci.policy-parity", Tiers: gates.Fast | gates.Full, Match: ciPolicyPaths, Blocking: true, Backing: "validate-ci-policy-parity.sh"},
		{ID: "eval.corpus-freshness", Tiers: gates.Fast | gates.Full, Match: evalPaths, Blocking: true, Backing: "check-corpus-freshness.sh"},

		// skill class (PB1 parity batch — all shell-backed via ScriptRunner)
		{ID: "skill.cli-skills-map", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-cli-skills-map.sh"},
		{ID: "skill.runtime-formats", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-skill-runtime-formats.sh"},
		{ID: "skill.runtime-parity", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-skill-runtime-parity.sh"},
		{ID: "skill.cli-snippets", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-skill-cli-snippets.sh"},
		{ID: "skill.manifests", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-manifests.sh"},
		{ID: "skill.next-work-contract", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-next-work-contract-parity.sh"},
		{ID: "skill.codex-parity-drift", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "check-codex-parity-drift.sh"},
		{ID: "skill.codex-runtime-sections", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-codex-runtime-sections.sh"},
		{ID: "skill.codex-override-coverage", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-codex-override-coverage.sh"},
		{ID: "skill.codex-backbone-prompts", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-codex-backbone-prompts.sh"},
		{ID: "skill.codex-rpi-contract", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-codex-rpi-contract.sh"},
		{ID: "skill.codex-lifecycle-guards", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-codex-lifecycle-guards.sh"},
		{ID: "skill.codex-generated-artifacts", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-codex-generated-artifacts.sh"},

		// go class
		{ID: "go.home-isolation", Tiers: gates.Fast | gates.Full, Match: goPaths, Blocking: true, Backing: "check-home-isolation.sh"},
		{ID: "go.test-home-isolation", Tiers: gates.Fast | gates.Full, Match: goPaths, Blocking: true, Backing: "check-test-home-isolation.sh"},

		// contract / context-map / swarm classes
		{ID: "contract.compatibility", Tiers: gates.Fast | gates.Full, Match: contractPaths, Blocking: true, Backing: "check-contract-compatibility.sh"},
		{ID: "contract.context-map-drift", Tiers: gates.Fast | gates.Full, Match: contextMapPaths, Blocking: true, Backing: "validate-context-map-drift.sh"},
		{ID: "swarm.evidence", Tiers: gates.Fast | gates.Full, Match: swarmPaths, Blocking: true, Backing: "validate-swarm-evidence.sh"},

		// always-run structural invariants (no Match)
		{ID: "always.author-judge-convergence", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-author-judge-convergence.sh"},
		{ID: "always.contracts-structural-floor", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-contracts-structural-floor.sh"},
		{ID: "always.docs-learning-references", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-docs-learning-references.sh"},
		{ID: "always.flywheel-compounding-snapshot", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-flywheel-compounding-snapshot.sh"},
		{ID: "always.retrieval-manifest-paths", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-retrieval-manifest-paths.sh"},
		{ID: "always.wiring-closure", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-wiring-closure.sh"},
		{ID: "always.bd-closeout-contract", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "validate-bd-closeout-contract.sh"},
		{ID: "always.domain-evolution-plan", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-agentops-domain-evolution-plan.sh"},

		// full-mode-only / advisory (mirror the bash gate: these skip in fast or warn)
		{ID: "full.worktree-disposition", Tiers: gates.Full, Blocking: true, Backing: "check-worktree-disposition.sh"},
		{ID: "full.retrieval-quality-ratchet", Tiers: gates.Full, Blocking: false, Backing: "check-retrieval-quality-ratchet.sh"},
		{ID: "always.loop-shape", Tiers: gates.Fast | gates.Full, Blocking: false, Backing: "check-loop-shape.sh"},

		// final backing-script batch (PB1)
		{ID: "always.quarantine-empty", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-quarantine-empty.sh"},
		{ID: "always.test-fixture-parity", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-test-fixture-parity.sh"},
		{ID: "go.race-fast", Tiers: gates.Fast | gates.Full, Match: goPaths, Blocking: true, Backing: "validate-go-fast.sh"},
		{ID: "full.headless-runtime-skills", Tiers: gates.Full, Blocking: false, Backing: "validate-headless-runtime-skills.sh"},
		// DEFERRED (PB1 model gaps, not flaky checks):
		//  - validate-release-audit-artifacts.sh needs `--mode changed` + a narrow
		//    release-file Match + RELEASE_AUDIT_CHANGED_PATHS env. The Check model
		//    must grow Args/Env fields for checks the bash gate parameterizes
		//    (also: validate-manifests --repo-root, codex-* --scope). Until then
		//    wrapping them risks behavior drift.
		//  - check-agents-hash-snapshot.sh is a stateful capture/diff pair — needs
		//    a native Go port, not a single wrapper.
	}
	for _, c := range seed {
		gates.Register(c)
	}
}
