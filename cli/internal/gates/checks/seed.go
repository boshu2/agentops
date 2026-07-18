// Package checks holds the gate check definitions registered into
// gates.Default. Two registration shapes coexist deliberately:
//
//   - script-backed checks are declared together in seed.go's init() — one
//     table entry per check (ID, tiers, match globs, backing scripts/*.sh);
//   - native Go checks register from their own file's init() with a Run func
//     (go_build.go, native_inline.go, constraints.go, workflow_install.go).
//
// Adding a script-backed check is one seed.go entry plus its backing script;
// porting a check to native Go moves it into its own file. Either way there is
// no orchestrator switch to edit — the registry is the only coupling point.
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
	// previously un-routed (self-routing repair, premortem FM3,
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
	operatorLeakPaths = []string{"skills/**", "skills-codex/**", "docs/SKILLS.md", "registry.json", "tests/scripts/check-no-operator-skills.bats", "scripts/check-no-operator-skills.sh"}
	contractPaths     = []string{"docs/contracts/**", "schemas/**"}
	// honest-voice gate (age-5qjyn / FU3): routes on the user-facing surfaces it
	// scans (cli/** Go + seed/template assets), the lexicon it reads, and a
	// self-reference (the gate script + its bats) so editing the gate re-runs it.
	honestVoicePaths = []string{
		"cli/**",
		"docs/contracts/forbidden-claims.yaml",
		"scripts/check-honest-voice.sh",
		"tests/scripts/check-honest-voice.bats",
	}
	ciPolicyPaths    = []string{".github/workflows/validate.yml", "docs/CI-CD.md", "AGENTS.md"}
	agentsDocPaths   = []string{"AGENTS.md", "docs/agent-workflow-reference.md", "docs/CI-CD.md", "docs/contracts/codex-skill-api.md", ".github/workflows/validate.yml"}
	corpusPaths      = []string{".agents/**", "docs/canon/**", "canon/**"}
	cliContractPaths = []string{"cli/**", "docs/cli-surface.*", "scripts/check-cli-contract.sh", "scripts/check-docs-cli-snippets.sh", "scripts/generate-cli-reference.sh", "tests/cli_contract_gate.bats", "tests/cli_quality_zero_debt.bats"}
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
	cathedralCutPaths = []string{
		"AGENTS.md", "PRODUCT.md", "README.md", "docs/architecture/operating-loop.md",
		"skills/**", "skills-codex/**", "schemas/**", "cli/cmd/ao/**", "cli/internal/**",
		"scripts/check-cathedral-cut-conformance.py",
	}
	gcExecutorPaths = []string{
		"packs/agentops-executor/**",
		"packs/agentops-factory/**",
		"deploy/gc/**",
		"docs/contracts/gas-city-execution-adapter.md",
		"docs/architecture/gas-city-factory.md",
		"docs/adr/ADR-0015-gas-city-fenced-steward.md",
		"docs/audits/gas-city-factory-live-bead-canary.md",
		"docs/plans/2026-07-17-gas-city-factory-operationalization.md",
		"skills-codex/implement/**",
		"skills-codex/validate/**",
		"skills-codex/using-gc/**",
		"scripts/sync-gc-pack.py",
		"scripts/check-gc-executor.sh",
		"scripts/regen-all.sh",
		"tests/python/test_gc_packet.py",
		"tests/python/test_gc_factory.py",
		"tests/python/test_sync_gc_pack.py",
		"tests/scripts/check-gc-executor.bats",
		"tests/scripts/gc-agentops-bootstrap.bats",
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
		// honest-voice: user-facing CLI strings + seed/template assets must not
		// claim proven/automatic knowledge compounding (unproven — ADR-0004,
		// ADR-0011) or hookless-3.0-violating "session hooks" (docs/3.0.md, honest-voice:allow
		// ADR-0009). The claims regrew because nothing gated them (#907, FU4);
		// this is the gate. Lexicon: docs/contracts/forbidden-claims.yaml (age-5qjyn).
		{ID: "contract.honest-voice", Tiers: gates.Fast | gates.Full, Match: honestVoicePaths, Blocking: true, Backing: "check-honest-voice.sh", RepairHint: "rewrite to honest phrasing (context accrues in .agents/ — compounding still being measured; 3.0 is hookless), or add a reviewed `honest-voice:allow`; lexicon docs/contracts/forbidden-claims.yaml"},

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
		{ID: "contract.cathedral-cut", Tiers: gates.Fast | gates.Full, Match: cathedralCutPaths, Blocking: true, Backing: "check-cathedral-cut-conformance.py"},
		{ID: "adapter.gc-executor", Tiers: gates.Fast | gates.Full, Match: gcExecutorPaths, Blocking: true, Backing: "check-gc-executor.sh", RepairHint: "run scripts/sync-gc-pack.py, then scripts/check-gc-executor.sh"},
		{ID: "contract.skill-mesh", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "check-skill-mesh.py"},
		{ID: "contract.finding-registry", Tiers: gates.Fast | gates.Full, Match: contractPaths, Blocking: true, Backing: "check-finding-registry.sh"},
		{ID: "ci.policy-parity", Tiers: gates.Fast | gates.Full, Match: ciPolicyPaths, Blocking: true, Backing: "validate-ci-policy-parity.sh"},
		// skill class (PB1 parity batch — all shell-backed via ScriptRunner)
		{ID: "skill.runtime-formats", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-skill-runtime-formats.sh"},
		{ID: "skill.runtime-parity", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-skill-runtime-parity.sh"},
		{ID: "skill.cli-snippets", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-skill-cli-snippets.sh"},
		{ID: "skill.manifests", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-manifests.sh", Args: []string{"--repo-root", "."}},
		{ID: "skill.codex-parity-drift", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "check-codex-parity-drift.sh"},
		{ID: "skill.codex-runtime-sections", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-codex-runtime-sections.sh"},
		{ID: "skill.codex-override-coverage", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-codex-override-coverage.sh"},
		// age-2s5k: always-run (no Match) — these validators assert whole-twin
		// contract invariants over hardcoded file lists, so latent drift in a twin
		// must fail the NEXT push regardless of scope, not lie invisible on green
		// main until an unrelated skills-codex touch triggers a scope-gated run and
		// ambushes it (the age-huim / age-3pdt failure mode). Cheap (string greps),
		// so the per-push cost is negligible against the anti-ambush guarantee.
		{ID: "skill.codex-generated-artifacts", Tiers: gates.Fast | gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-codex-generated-artifacts.sh"},
		// skill.probe-coverage (ADVISORY): a product-/judgment-tier skill whose
		// tier badge carries no BEHAVIORAL-probe result is unmeasured — the badge
		// is editorial, not proven. This NAMES the unmeasured ones. Advisory-first
		// (Blocking:false, warn never fail) exactly like skill.isolation and the
		// egwt gates: the spine is probed first, the ratchet drives the rest, and
		// the Blocking:false->true flip is made deliberately once covered. age-e508.1.
		{ID: "skill.probe-coverage", Tiers: gates.Fast | gates.Full, Match: skillProbePaths, Blocking: false, Backing: "check-skill-probe-coverage.sh", RepairHint: "bash scripts/probe-skill.sh --probe <skill> then record it in the MEASURED ledger of skills/SKILL-TIERS.md; advisory — probe the spine, ratchet the rest"},
		{ID: "skill.no-operator-leakage", Tiers: gates.Fast | gates.Full, Match: operatorLeakPaths, Blocking: true, Backing: "check-no-operator-skills.sh"},
		{ID: "skill.heal-strict", Tiers: gates.Full, Match: skillPaths, Blocking: true, Backing: "skills/skill-builder/scripts/heal.sh", Args: []string{"--check", "--strict"}},
		{ID: "skill.frontmatter-v2", Tiers: gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-skill-frontmatter.sh"},
		{ID: "skill.body-refs", Tiers: gates.Full, Match: skillPaths, Blocking: true, Backing: "validate-skill-body-refs.sh"},
		{ID: "skill.scenario-test-linkage", Tiers: gates.Full, Match: scenarioLinkagePaths, Blocking: true, Backing: "check-scenario-test-linkage.sh"},

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
		{ID: "contract.verdict-corpus", Tiers: gates.Fast | gates.Full, Match: []string{
			"schemas/verdict.v2.schema.json",
			"skills/validate/scripts/**",
			"cli/internal/verdictcheck/**",
			"cli/cmd/ao/status.go",
			"tests/fixtures/verdict-contract/**",
			"scripts/check-verdict-contract-corpus.sh",
		}, Blocking: true, Backing: "check-verdict-contract-corpus.sh",
			RepairHint: "the Go reader, Python validator, and JSON schema disagree over tests/fixtures/verdict-contract — change contract behavior only together with the corpus"},
		{ID: "docs.agents-split", Tiers: gates.Full, Match: agentsDocPaths, Blocking: true, Backing: "validate-agents-split.sh"},

		// always-run structural invariants (no Match)
		{ID: "docs.skill-refs", Tiers: gates.Full, Match: docSkillRefPaths, Blocking: true,
			Backing: "check-doc-skill-refs.sh", Args: []string{"--all-docs", "--strict"}},
		{ID: "provenance.orphans", Tiers: gates.Full, Match: contractPaths, Blocking: true,
			Backing: "check-provenance-orphans.sh"},
		{ID: "provenance.chain", Tiers: gates.Fast | gates.Full, Match: provenanceChainPaths, Blocking: true,
			Backing:    "check-provenance-chain.sh",
			RepairHint: "docs/provenance/ledger.jsonl hash chain broken — find the first bad entry with 'ao provenance verify'; repair is a deliberate re-seal, never hand-edit (age-gate-the-ungated-egwt.9)"},
		{ID: "always.docs-hookless", Tiers: gates.Full, Blocking: true, Backing: "check-doc-hooks-drift.sh"},
		{ID: "always.retrieval-manifest-paths", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-retrieval-manifest-paths.sh",
			Args: []string{"cli/cmd/ao/testdata/retrieval-bench/search-eval-manifest.json"}},
		{ID: "always.file-manifest-overlap", Tiers: gates.Full, Blocking: true, Backing: "check-file-manifest-overlap.sh"},
		{ID: "derived.changed-scope", Tiers: gates.Fast, Blocking: true, Backing: "regen-changed-scope.sh", Args: []string{"--check", "--scope", "head"},
			Match: regenScopePaths, RepairHint: "bash scripts/regen-changed-scope.sh --scope head; for a reported skill run: bash skills/skill-builder/scripts/heal.sh --check --strict skills/<skill>"},
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

		// final backing-script batch (PB1)
		{ID: "always.quarantine-empty", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-quarantine-empty.sh"},
		{ID: "always.test-fixture-parity", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "check-test-fixture-parity.sh"},
		{ID: "go.race-fast", Tiers: gates.Fast | gates.Full, Match: goPaths, Blocking: true, Backing: "validate-go-fast.sh"},
		// release-audit: narrow Match (only release files) + --mode changed, mirroring
		// the bash gate's needs_release_audit_artifact_check (PB1a Args support).
		{ID: "release.audit-artifacts", Tiers: gates.Fast | gates.Full, Blocking: true,
			Backing: "validate-release-audit-artifacts.sh", Args: []string{"--mode", "changed"},
			Match: []string{"docs/releases/**", "scripts/ci-local-release.sh", "scripts/resolve-release-artifacts.sh", "scripts/validate-release-audit-artifacts.sh", "tests/scripts/release-artifacts.bats"}},
	}
	for _, c := range seed {
		gates.Register(c)
	}
}
