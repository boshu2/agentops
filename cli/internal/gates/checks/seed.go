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
	goPaths = []string{"cli/**", "go.mod", "go.sum"}
	// go.lint runs the repo-pinned golangci-lint over any Go change, plus a
	// self-reference (the gate script + its bats) so editing the gate re-runs it.
	goLintPaths = []string{
		"cli/**",
		"go.mod",
		"go.sum",
		"scripts/check-go-lint.sh",
		"tests/scripts/check-go-lint.bats",
	}
	skillPaths = []string{"skills/**", "skills-codex/**", "tests/skills/**"}
	// skill.scenario-test-linkage routes on the scenario corpus PLUS its own
	// surfaces — the script, allowlist, bats twin, and shared ratchet lib were
	// previously un-routed (self-routing repair, pre-mortem FM3,
	// age-ratchet-lib-extraction-bv7d.7).
	scenarioLinkagePaths = []string{
		"skills/**", "skills-codex/**", "tests/skills/**",
		"scripts/check-scenario-test-linkage.sh",
		"scripts/.scenario-linkage-allow",
		"tests/scripts/check-scenario-test-linkage.bats",
		"scripts/lib/ratchet.sh",
	}
	// skill.probe-coverage (advisory): routes when any skill changes (a new
	// product/judgment skill needs a probe), when the MEASURED probe ledger
	// changes, when a probe scenario changes, plus self-reference so editing the
	// gate/its test re-runs it.
	skillProbePaths = []string{
		"skills/**",
		"skills/SKILL-TIERS.md",
		"evals/skill-probes/**",
		"scripts/probe-skill.sh",
		"scripts/check-skill-probe-coverage.sh",
		"tests/scripts/check-skill-probe-coverage.bats",
	}
	// nextWorkContractPaths routes skill.next-work-contract on every surface its
	// validator actually reads — most critically the subject file itself
	// (.agents/rpi/next-work.jsonl): a next-work-only commit previously SKIPped
	// its own contract gate and the defect had to be caught by a pawl round
	// (age-77g6, age-e70a).
	nextWorkContractPaths = []string{
		"skills/**", "skills-codex/**", "tests/skills/**",
		".agents/rpi/next-work.jsonl",
		"docs/contracts/next-work.schema.md",
		"cli/internal/rpi/**",
		"cli/cmd/ao/rpi_loop.go",
		"scripts/validate-next-work-contract-parity.sh",
		"scripts/validate-next-work.sh",
	}
	operatorLeakPaths = []string{"skills/**", "skills-codex/**", "docs/SKILLS.md", "registry.json", "tests/scripts/check-no-operator-skills.bats", "scripts/check-no-operator-skills.sh"}
	frontDoorPaths    = []string{"skills/**", ".claude/workflows/**"}
	contractPaths     = []string{"docs/contracts/**", "schemas/**"}
	ciPolicyPaths     = []string{".github/workflows/validate.yml", "docs/CI-CD.md", "AGENTS.md"}
	evalPaths         = []string{"evals/**", "schemas/eval-*", "cli/internal/eval/**"}
	contextMapPaths   = []string{"skills/**", "docs/contracts/context-map.md"}
	swarmPaths        = []string{".agents/swarm/**", "schemas/swarm-*"}
	agentsDocPaths    = []string{"AGENTS.md", "AGENTS-WORKFLOW.md", "AGENTS-CI.md", "AGENTS-CODEX.md", "AGENTS-RUNTIME.md", ".github/workflows/validate.yml"}
	corpusPaths       = []string{".agents/**", "docs/canon/**", "canon/**"}
	cliContractPaths  = []string{"cli/**", "docs/cli-surface.*", "scripts/check-cli-contract.sh", "scripts/check-docs-cli-snippets.sh", "scripts/generate-cli-reference.sh", "tests/cli_contract_gate.bats", "tests/cli_quality_zero_debt.bats"}
	registryPaths     = []string{"skills/**", "hooks/**", "evals/**", "cli/cmd/ao/**", "cli/internal/**", "registry.json"}
	// Widened to docs/** (--all-docs mode): the checker no longer scans a fixed
	// 6-file set — it scans every LIVE docs/** file (plus the pinned doctrine
	// files) and ratchets against scripts/.docs-skill-refs-baseline, so any live
	// doc that acquires a dead `/skill` ref, or any baselined file that no longer
	// offends, re-runs the gate. skills/** stays a trigger (a rename/retire under
	// skills/ can turn a live ref dead); the script + baseline + bats self-ref so
	// editing the gate re-runs it.
	docSkillRefPaths = []string{
		"AGENTS.md",
		"CLAUDE.md",
		"docs/**",
		"skills/SKILL-TIERS.md",
		"skills/**",
		"scripts/check-doc-skill-refs.sh",
		"scripts/.docs-skill-refs-baseline",
		"scripts/lib/docs-scope.sh",
		// shared ratchet mechanics (age-ratchet-lib-extraction-bv7d.8, FM3)
		"scripts/lib/ratchet.sh",
		"tests/scripts/check-doc-skill-refs.bats",
		"tests/scripts/check-doc-skill-refs-all-docs.bats",
	}
	// A folded skill (state: merged-into) is a redirect; its target must stay a
	// live skill. A rename/prune of a fold target (under skills/) silently
	// breaks the redirect, so this runs when either the ledger or the skill set
	// changes (plus self-reference so editing the gate re-runs it).
	skillRedirectPaths = []string{
		"docs/contracts/skill-dispositions.yaml",
		"skills/**",
		"skills-codex/**",
		"scripts/check-skill-redirects.sh",
		"tests/scripts/check-skill-redirects.bats",
	}
	// docs.cli-snippets resolves every `ao …` command cited in a live doc against
	// the cobra tree; a rename/removal of a command silently strands a golden-path
	// snippet. Runs on any docs change plus self-reference (the script, its baseline
	// allowlist, the bats twin, and the shared resolution lib) so editing the gate
	// re-runs it. (age-gate-the-ungated-egwt.4)
	docsCliSnippetsPaths = []string{
		"docs/**",
		"scripts/check-docs-cli-snippets.sh",
		"scripts/.docs-cli-snippets-baseline",
		"scripts/lib/ao-snippet-resolve.sh",
		"scripts/lib/ao_snippet_resolve.py",
		"tests/scripts/check-docs-cli-snippets.bats",
		// shared ratchet mechanics: a lib edit must re-run every consumer
		// (age-ratchet-lib-extraction-bv7d.6, FM3)
		"scripts/lib/ratchet.sh",
	}
	// scripts.ao-invocations resolves every LITERAL first-token `ao <sub>`
	// invocation in an executable script/test against the cobra tree; a
	// command removal silently strands a caller (the stale-retired-surface
	// class — age-sydq, age-zei7). Sibling of docs.cli-snippets / skill.cli-snippets
	// but over EXECUTABLE callers. Runs on any scripts/** or tests/** change plus
	// a cli/cmd/ao/** change (a removal is the trigger), and self-references the
	// script, its baseline, the bats twin, and the shared resolution lib so
	// editing the gate re-runs it. (age-owcs)
	scriptsAoInvocationsPaths = []string{
		"scripts/**",
		"tests/**",
		"cli/cmd/ao/**",
		"scripts/lib/ao-snippet-resolve.sh",
		"scripts/lib/ao_snippet_resolve.py",
		"scripts/.scripts-ao-invocations-baseline",
		"tests/scripts/check-scripts-ao-invocations.bats",
		// covered by scripts/** already; explicit for the ratchet-lib routing
		// closure test (age-ratchet-lib-extraction-bv7d.5, FM3)
		"scripts/lib/ratchet.sh",
	}
	// Claude workflows must use `br` (bd/Dolt is retired). operating-loop.js —
	// the most-viewed content artifact on the public repo — shipped a prompt
	// telling agents to run `bd ready` with no gate to catch it.
	workflowTrackerPaths = []string{
		".claude/workflows/**",
		"scripts/check-workflow-no-retired-tracker.sh",
		"tests/scripts/check-workflow-no-retired-tracker.bats",
	}
	cliAgentsTrackerPaths = []string{
		"cli/AGENTS.md",
		"scripts/check-cli-agents-tracker-drift.sh",
		"tests/scripts/check-cli-agents-tracker-drift.bats",
	}
	// provenance.chain: verify the committed ledger's hash chain at the pre-push
	// authority boundary (age-gate-the-ungated-egwt.9). Runs on any ledger change
	// plus self-reference (script + bats) so editing the gate re-runs it.
	provenanceChainPaths = []string{
		"docs/provenance/**",
		"scripts/check-provenance-chain.sh",
		"tests/scripts/check-provenance-chain.bats",
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
		"scripts/regen-changed-scope.sh",
		"tests/scripts/skill-standards-convergence.bats",
	}
	// Preamble-adoption ratchet: run whenever any script changes (a new/modified
	// scripts/*.sh is what the ratchet governs) plus the gate's own self-refs so
	// editing the check, the grandfather snapshot, or its bats re-runs it. The
	// matcher has no `**.sh` form, so `scripts/**` (dir-prefix) is the routing
	// glob; the backing script itself narrows governance to top-level scripts/*.sh.
	preambleRatchetPaths = []string{
		"scripts/**",
		"tests/scripts/check-new-scripts-use-preamble.bats",
		"tests/scripts/preamble.bats",
		// covered by scripts/** already; explicit for the ratchet-lib routing
		// closure test (age-ratchet-lib-extraction-bv7d.4, FM3)
		"scripts/lib/ratchet.sh",
	}
	// ADR registry integrity: unique NNNN across files, filename number ==
	// in-file title number, every ADR carries a Status: line. A duplicate
	// number (two ADR-0004s — resolved in age-gate-the-ungated-egwt.11) or a
	// filename/title mismatch must fail the next push, plus self-refs so
	// go.jsonl-scanner-ratchet: ADVISORY heuristic — flags a NEW raw
	// bufio.NewScanner over JSONL outside cli/internal/storage. A raw scanner
	// silently truncates at its 64KB default buffer; the blessed replacement is
	// storage.ScanJSONL / storage.ScanJSONLFile (loud ErrLineTooLong policy).
	// Match = cli/** plus self-refs (script, grandfather list, bats) so editing
	// the gate re-runs it. Advisory PERMANENTLY by design (grep heuristic —
	// file-level, no AST; false pos/neg possible), see age-storage-hardening-roxg.3.
	jsonlScannerRatchetPaths = []string{
		"cli/**",
		"scripts/check-jsonl-scanner-ratchet.sh",
		"scripts/.jsonl-scanner-grandfather",
		"tests/scripts/check-jsonl-scanner-ratchet.bats",
		// shared ratchet mechanics (age-ratchet-lib-extraction-bv7d.3, FM3)
		"scripts/lib/ratchet.sh",
	}
	// go.atomic-write-ratchet: hand-rolled tmp+rename outside storage/ (the
	// adoption-convergence gap three recon sweeps flagged). Routes on cli Go
	// changes + self-references (age-ratchet-lib-extraction-bv7d.9).
	atomicWriteRatchetPaths = []string{
		"cli/**",
		"scripts/check-atomic-write-ratchet.sh",
		"scripts/.atomic-write-grandfather",
		"tests/scripts/check-atomic-write-ratchet.bats",
		"scripts/lib/ratchet.sh",
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
		// static portability guard for the never-safe `find -printf` (GNU-only)
		// class — four real instances shipped because no gate caught it and Linux
		// CI succeeds at runtime; only BSD/macOS find errors. Always-run: it scans
		// ALL tracked first-party shell (incl. extensionless hooks no *.sh Match
		// glob would catch), and the fast grep makes a path-class trigger needless.
		{ID: "always.shell-portability", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-shell-portability.sh"},
		// preamble-adoption ratchet: a new/changed top-level scripts/*.sh must
		// source scripts/lib/preamble.sh (the hardened strict-mode + REPO_ROOT +
		// portable helpers) or carry a `# preamble-exempt: <reason>` line. The
		// preamble had ZERO adopters and all 13 scripts added after the
		// opportunistic-adoption decision re-hand-rolled it — a doc instruction was
		// measured-inert, only a gate changes behavior. Grandfathered tree is
		// exempt and the allowlist only shrinks. Advisory for one clean cycle, then
		// flips Blocking (age-gate-the-ungated-egwt.10).
		{ID: "shell.preamble-ratchet", Tiers: gates.Full, Match: preambleRatchetPaths, Blocking: false, Backing: "check-new-scripts-use-preamble.sh", RepairHint: "source scripts/lib/preamble.sh (or add '# preamble-exempt: <reason>'); advisory one clean cycle then flips Blocking (age-gate-the-ungated-egwt.10)"},
		// always-run + fail-closed PATH guard: a private artifact (corpus,
		// tracker, untraceable wiki) must never reach the PUBLIC repo. No Match
		// glob — a force-added private path might not match any corpus glob, so
		// changed-file scoping must never be able to skip this (ag-ao0eo).
		{ID: "corpus.path-guard", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-corpus-path-guard.sh"},
		{ID: "always.embedded-sync", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "validate-embedded-sync.sh"},

		// routed by change class
		gates.GoCLIArchitectureCheck(),
		{ID: "go.command-test-pair", Tiers: gates.Fast | gates.Full, Match: goPaths, Blocking: true, Backing: "check-go-command-test-pair.sh"},
		// go.lint: enforce the documented lint budgets (.claude/rules/go.md —
		// gocyclo fail at 25, errcheck, staticcheck, copyloopvar) via the
		// repo-pinned golangci-lint. Full-tier only for now: the full-tree run is
		// too slow to sit in Fast; promote to Fast once measured <60s on changed
		// scope (age-gate-the-ungated-egwt.7). Blocking is safe — the tree lands
		// at 0 findings.
		{ID: "go.lint", Tiers: gates.Full, Match: goLintPaths, Blocking: true, Backing: "check-go-lint.sh", RepairHint: "cd cli && make lint; fix or split — budgets in .claude/rules/go.md; promote to Fast tier only after measured <60s on changed scope (age-gate-the-ungated-egwt.7)"},
		{ID: "skill.schema", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-skill-schema.sh"},
		{ID: "skill.triggers", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-skill-triggers.sh"},
		{ID: "contract.registry-drift", Tiers: gates.Fast | gates.Full, Match: contractPaths, Blocking: true, Backing: "check-registry-drift.sh", RepairHint: "bash scripts/generate-registry.sh"},
		{ID: "contract.bounded-contexts-drift", Tiers: gates.Fast | gates.Full, Match: contractPaths, Blocking: true, Backing: "check-bounded-contexts-drift.sh"},
		{ID: "contract.disposition-schema", Tiers: gates.Fast | gates.Full, Match: contractPaths, Blocking: true, Backing: "validate-skill-disposition-schema.sh"},
		{ID: "contract.skill-redirects", Tiers: gates.Fast | gates.Full, Match: skillRedirectPaths, Blocking: true, Backing: "check-skill-redirects.sh"},
		{ID: "workflow.no-retired-tracker", Tiers: gates.Fast | gates.Full, Match: workflowTrackerPaths, Blocking: true, Backing: "check-workflow-no-retired-tracker.sh"},
		{ID: "contract.finding-registry", Tiers: gates.Fast | gates.Full, Match: contractPaths, Blocking: true, Backing: "check-finding-registry.sh"},
		{ID: "ci.policy-parity", Tiers: gates.Fast | gates.Full, Match: ciPolicyPaths, Blocking: true, Backing: "validate-ci-policy-parity.sh"},
		{ID: "eval.corpus-freshness", Tiers: gates.Fast | gates.Full, Match: evalPaths, Blocking: true, Backing: "check-corpus-freshness.sh"},

		// skill class (PB1 parity batch — all shell-backed via ScriptRunner)
		{ID: "skill.cli-skills-map", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-cli-skills-map.sh"},
		{ID: "skill.runtime-formats", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-skill-runtime-formats.sh"},
		{ID: "skill.runtime-parity", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-skill-runtime-parity.sh"},
		{ID: "skill.cli-snippets", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-skill-cli-snippets.sh"},
		{ID: "skill.manifests", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-manifests.sh", Args: []string{"--repo-root", "."}},
		{ID: "skill.next-work-contract", Tiers: gates.Fast | gates.Full, Match: nextWorkContractPaths, Blocking: true, Backing: "validate-next-work-contract-parity.sh"},
		{ID: "skill.codex-parity-drift", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "check-codex-parity-drift.sh"},
		{ID: "skill.codex-runtime-sections", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-codex-runtime-sections.sh"},
		{ID: "skill.codex-override-coverage", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-codex-override-coverage.sh"},
		{ID: "skill.codex-backbone-prompts", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-codex-backbone-prompts.sh"},
		// age-2s5k: always-run (no Match) — these validators assert whole-twin
		// contract invariants over hardcoded file lists, so latent drift in a twin
		// must fail the NEXT push regardless of scope, not lie invisible on green
		// main until an unrelated skills-codex touch triggers a scope-gated run and
		// ambushes it (the age-huim / age-3pdt failure mode). Cheap (string greps),
		// so the per-push cost is negligible against the anti-ambush guarantee.
		{ID: "skill.codex-rpi-contract", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "validate-codex-rpi-contract.sh"},
		{ID: "skill.codex-lifecycle-guards", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "validate-codex-lifecycle-guards.sh"},
		{ID: "skill.four-umbrella-examples", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-four-umbrella-examples.sh"},
		{ID: "skill.mortem-name-migration", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-mortem-name-migration.sh"},
		{ID: "skill.validation-learning-boundary", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-validation-learning-boundary.sh"},
		{ID: "skill.validation-delivery-boundary", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-validation-delivery-boundary.sh"},
		{ID: "skill.codex-generated-artifacts", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-codex-generated-artifacts.sh"},
		{ID: "skill.isolation", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: false, Backing: "check-skill-isolation.sh"},
		// skill.probe-coverage (ADVISORY): a product-/judgment-tier skill whose
		// tier badge carries no BEHAVIORAL-probe result is unmeasured — the badge
		// is editorial, not proven. This NAMES the unmeasured ones. Advisory-first
		// (Blocking:false, warn never fail) exactly like skill.isolation and the
		// egwt gates: the spine is probed first, the ratchet drives the rest, and
		// the Blocking:false->true flip is made deliberately once covered. age-e508.1.
		{ID: "skill.probe-coverage", Tiers: gates.Fast | gates.Full, Match: skillProbePaths, Blocking: false, Backing: "check-skill-probe-coverage.sh", RepairHint: "bash scripts/probe-skill.sh --probe <skill> then record it in the MEASURED ledger of skills/SKILL-TIERS.md; advisory — probe the spine, ratchet the rest"},
		{ID: "skill.no-operator-leakage", Tiers: gates.Fast | gates.Full, Match: operatorLeakPaths, Blocking: true, Backing: "check-no-operator-skills.sh"},
		{ID: "skill.heal-strict", Tiers: gates.Full, Match: skillPaths, Blocking: true, Backing: "skills/heal-skill/scripts/heal.sh", Args: []string{"--strict"}},
		{ID: "skill.frontmatter-v2", Tiers: gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-skill-frontmatter.sh"},
		{ID: "skill.body-refs", Tiers: gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-skill-body-refs.sh"},
		{ID: "skill.flow", Tiers: gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-skill-flow.sh"},
		{ID: "skill.domain-map-golden", Tiers: gates.Full, Match: skillPaths, Blocking: true, Backing: "generate-skill-domain-map.sh", Args: []string{"--check"}},
		{ID: "skill.scenario-test-linkage", Tiers: gates.Full, Match: scenarioLinkagePaths, Blocking: true, Backing: "check-scenario-test-linkage.sh"},

		// governance front-door admission (M5): a newly-ADDED skill/workflow/loop
		// cannot merge without bounded-context + role + a runnable acceptance.
		{ID: "governance.frontdoor-admission", Tiers: gates.Fast | gates.Full, Match: frontDoorPaths, Blocking: true, Backing: "check-frontdoor-admission.sh"},

		// go class
		{ID: "go.home-isolation", Tiers: gates.Fast | gates.Full, Match: goPaths, Blocking: true, Backing: "check-home-isolation.sh"},
		{ID: "go.test-home-isolation", Tiers: gates.Fast | gates.Full, Match: goPaths, Blocking: true, Backing: "check-test-home-isolation.sh"},
		{ID: "go.complexity", Tiers: gates.Full, Match: goPaths, Blocking: true, Backing: "check-go-complexity.sh"},
		{ID: "go.cli-reference", Tiers: gates.Full, Match: goPaths, Blocking: true, Backing: "generate-cli-reference.sh", Args: []string{"--check"}},
		{ID: "go.cli-contract", Tiers: gates.Fast | gates.Full, Match: cliContractPaths, Blocking: true, Backing: "check-cli-contract.sh"},
		{ID: "go.cli-surface-counts", Tiers: gates.Full, Match: goPaths, Blocking: true, Backing: "update-cli-surface-counts.sh"},
		{ID: "go.test-count-regression", Tiers: gates.Full, Match: goPaths, Blocking: true, Backing: "check-test-count-regression.sh"},
		{ID: "go.test-isolation", Tiers: gates.Fast | gates.Full, Match: goPaths, Blocking: true, Backing: "check-test-isolation.sh"},

		// contract / context-map / swarm classes
		{ID: "contract.compatibility", Tiers: gates.Fast | gates.Full, Match: contractPaths, Blocking: true, Backing: "check-contract-compatibility.sh"},
		{ID: "contract.context-map-drift", Tiers: gates.Fast | gates.Full, Match: contextMapPaths, Blocking: true, Backing: "validate-context-map-drift.sh", RepairHint: "bash scripts/generate-context-map.sh"},
		{ID: "contract.registry-json", Tiers: gates.Full, Match: registryPaths, Blocking: true, Backing: "generate-registry.sh", Args: []string{"--check"}},
		{ID: "contract.sku-catalog-drift", Tiers: gates.Full, Match: registryPaths, Blocking: true, Backing: "validate-sku-catalog-drift.sh"},
		{ID: "docs.agents-split", Tiers: gates.Full, Match: agentsDocPaths, Blocking: true, Backing: "validate-agents-split.sh"},
		{ID: "swarm.evidence", Tiers: gates.Fast | gates.Full, Match: swarmPaths, Blocking: true, Backing: "validate-swarm-evidence.sh"},

		// always-run structural invariants (no Match)
		{ID: "docs.skill-refs", Tiers: gates.Full, Match: docSkillRefPaths, Blocking: true,
			Backing: "check-doc-skill-refs.sh", Args: []string{"--all-docs", "--strict"}},
		{ID: "cli.agents-tracker", Tiers: gates.Fast | gates.Full, Match: cliAgentsTrackerPaths, Blocking: true,
			Backing: "check-cli-agents-tracker-drift.sh"},
		{ID: "provenance.orphans", Tiers: gates.Full, Match: contractPaths, Blocking: true,
			Backing: "check-provenance-orphans.sh"},
		{ID: "provenance.chain", Tiers: gates.Fast | gates.Full, Match: provenanceChainPaths, Blocking: true,
			Backing:    "check-provenance-chain.sh",
			RepairHint: "docs/provenance/ledger.jsonl hash chain broken — find the first bad entry with 'ao provenance verify'; repair is a deliberate re-seal, never hand-edit (age-gate-the-ungated-egwt.9)"},
		{ID: "always.docs-hookless", Tiers: gates.Full, Blocking: true, Backing: "check-doc-hooks-drift.sh"},
		{ID: "always.retrieval-manifest-paths", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-retrieval-manifest-paths.sh",
			Args: []string{"cli/cmd/ao/testdata/retrieval-bench/search-eval-manifest.json"}},
		{ID: "always.bd-closeout-contract", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "validate-bd-closeout-contract.sh"},
		{ID: "always.file-manifest-overlap", Tiers: gates.Full, Blocking: true, Backing: "check-file-manifest-overlap.sh"},
		{ID: "derived.changed-scope", Tiers: gates.Fast, Blocking: true, Backing: "regen-changed-scope.sh", Args: []string{"--check", "--scope", "head"},
			Match: regenScopePaths, RepairHint: "bash scripts/regen-changed-scope.sh --scope head; for a reported skill run: bash skills/heal-skill/scripts/audit.sh --strict skills/<skill>"},
		{ID: "always.regen-all", Tiers: gates.Full, Blocking: true, Backing: "regen-all.sh", Args: []string{"--check"}, RepairHint: "bash scripts/regen-all.sh"},
		{ID: "docs.cli-snippets", Tiers: gates.Full, Match: docsCliSnippetsPaths, Blocking: false, Backing: "check-docs-cli-snippets.sh", RepairHint: "fix the dead ao reference or prune the stale baseline entry; flips Blocking after one clean advisory cycle (age-gate-the-ungated-egwt.4)"},
		{ID: "scripts.ao-invocations", Tiers: gates.Fast | gates.Full, Match: scriptsAoInvocationsPaths, Blocking: false, Backing: "check-scripts-ao-invocations.sh", RepairHint: "fix the dead ao invocation (use the live subcommand or add `# ao-resolve: ignore`), or prune the stale baseline entry; advisory-first, flips Blocking after one clean cycle (age-owcs)"},
		// go.jsonl-scanner-ratchet: ADVISORY grep-ratchet — a NEW raw
		// bufio.NewScanner over JSONL outside cli/internal/storage silently
		// truncates at the 64KB default buffer. Stays advisory PERMANENTLY (unless
		// drift recurs) — it is a file-level grep heuristic (no AST; false pos/neg
		// possible), so a false positive must never block a push. age-storage-hardening-roxg.3.
		{ID: "go.jsonl-scanner-ratchet", Tiers: gates.Full, Match: jsonlScannerRatchetPaths, Blocking: false, Backing: "check-jsonl-scanner-ratchet.sh", Args: []string{"--scope", "head"}, RepairHint: "use storage.ScanJSONL/ScanJSONLFile (loud ErrLineTooLong policy) instead of a raw bufio.NewScanner over JSONL; advisory — see age-storage-hardening-roxg.3"},
		// go.atomic-write-ratchet: ADVISORY PERMANENTLY at file-level fidelity
		// (grep heuristic, no AST — same rationale as the jsonl row; graduation
		// to blocking requires a precision detector earned in its own arc).
		{ID: "go.atomic-write-ratchet", Tiers: gates.Full, Match: atomicWriteRatchetPaths, Blocking: false, Backing: "check-atomic-write-ratchet.sh", Args: []string{"--scope", "head"}, RepairHint: "use storage.AtomicWriteFile (temp+fsync+rename+fsync-dir) or storage.FsyncDir instead of a hand-rolled tmp+rename; advisory — see age-ratchet-lib-extraction-bv7d.9"},
		{ID: "corpus.secret-scan", Tiers: gates.Full, Match: corpusPaths, Blocking: true, Backing: "check-corpus-secret-scan.sh"},
		{ID: "corpus.witness-dolt-jsonl-crosscheck", Tiers: gates.Full, Match: corpusPaths, Blocking: true, Backing: "witness-dolt-jsonl-crosscheck.sh"},

		// full-mode-only / advisory (mirror the bash gate: these skip in fast or warn)
		{ID: "full.worktree-disposition", Tiers: gates.Full, Blocking: true, Backing: "check-worktree-disposition.sh"},
		{ID: "skill.catalog-drift", Tiers: gates.Full, Blocking: false, Backing: "check-skill-catalog-drift.sh"},

		// final backing-script batch (PB1)
		{ID: "always.quarantine-empty", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-quarantine-empty.sh"},
		{ID: "always.test-fixture-parity", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-test-fixture-parity.sh"},
		{ID: "go.race-fast", Tiers: gates.Fast | gates.Full, Match: goPaths, Blocking: true, Backing: "validate-go-fast.sh"},
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
