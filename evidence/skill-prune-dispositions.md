# Skill prune — disposition ledger (Lane D, ag-if7p)

Phase 1 ledger ONLY — zero deletions performed. Razor: AgentOps = inner loop.
Scope: zero/low-usage skills not classified INNER / OLYMPUS / JEFF-ADAPTER / RUNTIME-ADAPTER
in evidence/skill-prune-recon.md. Inbound refs = literal `skills/<name>` path/link references
across skills/**, docs/**, cli/**, excluding the skill's own dir and the using-agentops catalog
(an index listing is not demand). Usage = explicit invocations mined from 880MB Claude transcripts.

**Candidates: 69** — KEEP: 11, MERGE-INTO: 13, RETIRE: 45

| Skill | Usage | Inbound refs | Disposition | Why |
|---|---:|---:|---|---|
| agent-native | 0 | 4 | KEEP | 4 inbound ref file(s) |
| autodev | 0 | 2 | KEEP | 2 inbound ref file(s) |
| automation-shape-routing | 0 | 2 | KEEP | 2 inbound ref file(s) |
| casr | 0 | 1 | KEEP | 1 inbound ref file(s) |
| codebase-audit | 4 | 0 | KEEP | recorded usage=4 |
| converter | 0 | 6 | KEEP | 6 inbound ref file(s) |
| reverse-engineer-rpi | 2 | 1 | KEEP | recorded usage=2 |
| swarm | 1 | 13 | KEEP | recorded usage=1 |
| system-tuning | 0 | 2 | KEEP | 2 inbound ref file(s) |
| using-agentops | 0 | 4 | KEEP | 4 inbound ref file(s) |
| workflow-builder | 2 | 0 | KEEP | recorded usage=2 |
| codebase-archaeology | 0 | 0 | MERGE-INTO:codebase-audit | zero usage, zero inbound refs; cluster sibling exists |
| codebase-briefing-report | 0 | 0 | MERGE-INTO:codebase-audit | zero usage, zero inbound refs; cluster sibling exists |
| codebase-pattern-extraction | 0 | 0 | MERGE-INTO:codebase-audit | zero usage, zero inbound refs; cluster sibling exists |
| codebase-report | 0 | 0 | MERGE-INTO:codebase-audit | zero usage, zero inbound refs; cluster sibling exists |
| codebase-risk-audit | 0 | 0 | MERGE-INTO:codebase-audit | zero usage, zero inbound refs; cluster sibling exists |
| measured-performance-optimization | 0 | 0 | MERGE-INTO:perf | zero usage, zero inbound refs; cluster sibling exists |
| performance-profile-triage | 0 | 0 | MERGE-INTO:perf | zero usage, zero inbound refs; cluster sibling exists |
| rust-ub-risk-audit | 0 | 0 | MERGE-INTO:rust-ub-risk-audit | zero usage, zero inbound refs; cluster sibling exists |
| rust-unsafe-boundary-audit | 0 | 0 | MERGE-INTO:rust-ub-risk-audit | zero usage, zero inbound refs; cluster sibling exists |
| contract-conformance-testing | 0 | 0 | MERGE-INTO:test | zero usage, zero inbound refs; cluster sibling exists |
| fuzz-test-design | 0 | 0 | MERGE-INTO:test | zero usage, zero inbound refs; cluster sibling exists |
| golden-artifact-testing | 0 | 0 | MERGE-INTO:test | zero usage, zero inbound refs; cluster sibling exists |
| metamorphic-test-design | 0 | 0 | MERGE-INTO:test | zero usage, zero inbound refs; cluster sibling exists |
| artifact-clarity-pass | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| automation-loop-hardening | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| bd-first-memory-migration | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| behavior-preserving-simplification | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| changelog-quality-pass | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| cli-agent-ux-audit | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| cli-doctoring-workflow | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| concurrency-deadlock-remediation | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| dependency-update-safety | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| expertise-to-procedure | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| external-search-triage | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| filesystem-path-rationalization | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| gcloud | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| gh-actions | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| gh-cli | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| gh-triage-ru | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| idea-option-forge | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| implementation-pattern-mining | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| installer-quality-audit | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| layered-defect-hunt | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| legacy-codebase-recon | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| live-service-e2e-testing | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| mcp-interface-design | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| native-debugger-triage | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| process-triage | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| production-placeholder-audit | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| project-readme-craft | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| project-reasoning-lens-analysis | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| release-readiness-gate | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| repeatedly-apply-skill | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| repository-hygiene-sweep | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| research-software | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| ripgrep-search-discipline | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| ru-multi-repo-workflow | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| rust-crate-release-readiness | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| rust-port-validation-gauntlet | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| rust-search-integration | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| rust-sqlite-cli-architecture | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| spec-reliability-implementation | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| ssh | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| stash-hygiene-sweep | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| storage-watchdog-ops | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| system-performance-remediation | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| work-contract-portability | 0 | 0 | RETIRE | zero usage, zero inbound refs |
| worktree-branch-rationalization | 0 | 0 | RETIRE | zero usage, zero inbound refs |

## Inbound-ref detail (KEEP-by-reference evidence)

- **agent-native**: `cli/embedded/skills/using-agentops/SKILL.md`, `docs/contracts/agent-native-mechanism.md`, `skills/shared/SKILL.md`, `skills/using-atm/SKILL.md`
- **autodev**: `docs/architecture/autonomy-ladder.md`, `skills/burndown/SKILL.md`
- **automation-shape-routing**: `docs/contracts/orchestration-ports.md`, `skills/using-atm/SKILL.md`
- **casr**: `skills/cass/references/RESUME.md`
- **converter**: `docs/contracts/ubiquitous-language.md`, `docs/plans/2026-05-15-skill-catalog-strangler-fig.md`, `skills/agent-native/SKILL.md`, `skills/agent-native/references/codex-ntm-runtime.md`, `skills/skill-builder/SKILL.md`, `skills/skill-builder/scripts/init.sh`
- **reverse-engineer-rpi**: `docs/comparisons/competition-rpi-memory-pipelines.md`
- **swarm**: `cli/cmd/ao/beads_test.go`, `docs/GLOSSARY.md`, `docs/agent-footguns.md`, `docs/rfcs/0001-finding-generator-parallelism.md`, `docs/runbooks/pr-creation-from-linked-worktrees.md`, `skills/SKILL-TIERS.md`, `skills/agent-native/SKILL.md`, `skills/council/SKILL.md`…
- **system-tuning**: `cli/embedded/skills/standards/references/external-source-attribution.md`, `skills/standards/references/external-source-attribution.md`
- **using-agentops**: `cli/Makefile`, `docs/agent-footguns.md`, `docs/agents-operator-guide.md`, `docs/plans/2026-04-09-operating-review.md`

## Phase 2 gates (NOT executed)

- RETIRE rows → archive branch, pending Bo review of this ledger.
- MERGE-INTO rows → fold unique content into the named sibling, then archive.
- Lane A (Olympus extraction) and Lane E (admission gate + usage-GC) remain Bo-gated.
