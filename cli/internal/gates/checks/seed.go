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
	contractPaths = []string{"docs/contracts/**", "schemas/**"}
	ciPolicyPaths = []string{".github/workflows/validate.yml", "docs/CI-CD.md", "AGENTS.md"}
	evalPaths     = []string{"evals/**", "schemas/eval-*", "cli/internal/eval/**"}
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
	}
	for _, c := range seed {
		gates.Register(c)
	}
}
