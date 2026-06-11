# AgentOps skill product audit — full herd, every folder judged (2026-06-11)

Frame: skills/ is the PRODUCT AgentOps ships (symlinked to local runtimes + installed by
plugin users). Razor: AgentOps = the inner loop. Judgment per skill below; usage evidence
from 880MB transcript mining + inbound-ref sweep (evidence/skill-prune-dispositions.md).

| Bucket | n | Meaning |
|---|---:|---|
| CORE | 59 | ship — the product |
| ADAPTER-JEFF | 15 | ship thin — operates a Jeff-register tool |
| ADAPTER-RUNTIME-ANCHOR | 3 | ship — one binding skill per harness (AGY/CC/Codex) |
| OLYMPUS-STUB | 6 | already extracted; stub until catalog closer |
| CONSOLIDATE | 12 | fold into its harness anchor, then archive |
| MERGE | 19 | fold unique content into named sibling, then archive |
| RETIRE | 52 | archive now — not the product |

**End state: 77 shipped skills** (from 166). CONSOLIDATE+MERGE+RETIRE = 83 removed; Olympus stubs collapse when the catalog closer lands.


## CORE (59)

- **agent-native** — inner loop / flywheel / skill-SDLC product surface
- **autodev** — inner loop / flywheel / skill-SDLC product surface
- **automation-shape-routing** — inner loop / flywheel / skill-SDLC product surface
- **bootstrap** — inner loop / flywheel / skill-SDLC product surface
- **brainstorm** — inner loop / flywheel / skill-SDLC product surface
- **bug-hunt** — inner loop / flywheel / skill-SDLC product surface
- **burndown** — inner loop / flywheel / skill-SDLC product surface
- **codebase-audit** — inner loop / flywheel / skill-SDLC product surface
- **compile** — inner loop / flywheel / skill-SDLC product surface
- **complexity** — inner loop / flywheel / skill-SDLC product surface
- **converter** — inner loop / flywheel / skill-SDLC product surface
- **crank** — inner loop / flywheel / skill-SDLC product surface
- **curate** — inner loop / flywheel / skill-SDLC product surface
- **deps** — inner loop / flywheel / skill-SDLC product surface
- **design** — inner loop / flywheel / skill-SDLC product surface
- **discovery** — inner loop / flywheel / skill-SDLC product surface
- **doc** — inner loop / flywheel / skill-SDLC product surface
- **domain** — inner loop / flywheel / skill-SDLC product surface
- **evolve** — inner loop / flywheel / skill-SDLC product surface
- **flywheel** — inner loop / flywheel / skill-SDLC product surface
- **forge** — inner loop / flywheel / skill-SDLC product surface
- **goals** — inner loop / flywheel / skill-SDLC product surface
- **handoff** — inner loop / flywheel / skill-SDLC product surface
- **heal-skill** — inner loop / flywheel / skill-SDLC product surface
- **implement** — inner loop / flywheel / skill-SDLC product surface
- **inject** — inner loop / flywheel / skill-SDLC product surface
- **operating-loop-skill** — inner loop / flywheel / skill-SDLC product surface
- **perf** — inner loop / flywheel / skill-SDLC product surface
- **plan** — inner loop / flywheel / skill-SDLC product surface
- **post-mortem** — inner loop / flywheel / skill-SDLC product surface
- **pre-mortem** — inner loop / flywheel / skill-SDLC product surface
- **product** — inner loop / flywheel / skill-SDLC product surface
- **push** — inner loop / flywheel / skill-SDLC product surface
- **quickstart** — inner loop / flywheel / skill-SDLC product surface
- **ratchet** — inner loop / flywheel / skill-SDLC product surface
- **recover** — inner loop / flywheel / skill-SDLC product surface
- **refactor** — inner loop / flywheel / skill-SDLC product surface
- **release** — inner loop / flywheel / skill-SDLC product surface
- **research** — inner loop / flywheel / skill-SDLC product surface
- **review** — inner loop / flywheel / skill-SDLC product surface
- **rpi** — inner loop / flywheel / skill-SDLC product surface
- **scaffold** — inner loop / flywheel / skill-SDLC product surface
- **scenario** — inner loop / flywheel / skill-SDLC product surface
- **scope** — inner loop / flywheel / skill-SDLC product surface
- **security** — inner loop / flywheel / skill-SDLC product surface
- **session-bootstrap** — inner loop / flywheel / skill-SDLC product surface
- **shared** — inner loop / flywheel / skill-SDLC product surface
- **ship-loop** — inner loop / flywheel / skill-SDLC product surface
- **skill-auditor** — inner loop / flywheel / skill-SDLC product surface
- **skill-builder** — inner loop / flywheel / skill-SDLC product surface
- **standards** — inner loop / flywheel / skill-SDLC product surface
- **status** — inner loop / flywheel / skill-SDLC product surface
- **swarm** — inner loop / flywheel / skill-SDLC product surface
- **test** — inner loop / flywheel / skill-SDLC product surface
- **trace** — inner loop / flywheel / skill-SDLC product surface
- **using-agentops** — inner loop / flywheel / skill-SDLC product surface
- **validate** — inner loop / flywheel / skill-SDLC product surface
- **vibe** — inner loop / flywheel / skill-SDLC product surface
- **workflow-builder** — inner loop / flywheel / skill-SDLC product surface

## ADAPTER-JEFF (15)

- **acfs** — ships thin (Phase 1 thinned); operates a register tool
- **agent-mail** — ships thin (Phase 1 thinned); operates a register tool
- **beads** — ships thin (Phase 1 thinned); operates a register tool
- **beads-workflow** — ships thin (Phase 1 thinned); operates a register tool
- **caam** — ships thin (Phase 1 thinned); operates a register tool
- **casr** — ships thin (Phase 1 thinned); operates a register tool
- **cass** — ships thin (Phase 1 thinned); operates a register tool
- **cass-memory** — ships thin (Phase 1 thinned); operates a register tool
- **dcg** — ships thin (Phase 1 thinned); operates a register tool
- **ntm** — ships thin (Phase 1 thinned); operates a register tool
- **rch** — ships thin (Phase 1 thinned); operates a register tool
- **sbh** — ships thin (Phase 1 thinned); operates a register tool
- **ubs** — ships thin (Phase 1 thinned); operates a register tool
- **using-atm** — ships thin (Phase 1 thinned); operates a register tool
- **vibing-with-ntm** — ships thin (Phase 1 thinned); operates a register tool

## ADAPTER-RUNTIME-ANCHOR (3)

- **agy-native** — runtime binding anchor (1 per harness)
- **cc-hooks** — runtime binding anchor (1 per harness)
- **codex-exec** — runtime binding anchor (1 per harness)

## OLYMPUS-STUB (6)

- **bead-completion-audit** — moved to mt-olympus (Lane A); stub stays until catalog closer
- **council** — moved to mt-olympus (Lane A); stub stays until catalog closer
- **cross-vendor-trust-gate** — moved to mt-olympus (Lane A); stub stays until catalog closer
- **eval-outcomes** — moved to mt-olympus (Lane A); stub stays until catalog closer
- **multi-model-triangulation** — moved to mt-olympus (Lane A); stub stays until catalog closer
- **red-team** — moved to mt-olympus (Lane A); stub stays until catalog closer

## CONSOLIDATE (12)

- **agy-headless-evidence** — fold into agy-native (one skill per harness)
- **agy-mcp-plugins** — fold into agy-native (one skill per harness)
- **agy-project-worktree-permissions** — fold into agy-native (one skill per harness)
- **agy-rules-workflows** — fold into agy-native (one skill per harness)
- **agy-sidecar-scheduled-tick** — fold into agy-native (one skill per harness)
- **cc-cron-ticks** — fold into cc-hooks (one skill per harness)
- **cc-loop-driver** — fold into cc-hooks (one skill per harness)
- **cc-subagents** — fold into cc-hooks (one skill per harness)
- **cc-worktree-isolation** — fold into cc-hooks (one skill per harness)
- **codex-goals** — fold into codex-exec (one skill per harness)
- **codex-mcp-plugins** — fold into codex-exec (one skill per harness)
- **codex-sandbox-evidence** — fold into codex-exec (one skill per harness)

## MERGE (19)

- **beads-br** — fold into beads, then archive
- **beads-bv** — fold into beads, then archive
- **codebase-archaeology** — fold into codebase-audit, then archive
- **codebase-briefing-report** — fold into codebase-audit, then archive
- **codebase-pattern-extraction** — fold into forge, then archive
- **codebase-report** — fold into codebase-audit, then archive
- **codebase-risk-audit** — fold into codebase-audit, then archive
- **expertise-to-procedure** — fold into forge, then archive
- **idea-option-forge** — fold into brainstorm, then archive
- **ntm-browser-test-coordination** — fold into ntm, then archive
- **ntm-review-worker-orchestration** — fold into ntm, then archive
- **operating-loop-workflow** — fold into operating-loop-skill, then archive
- **planning-workflow** — fold into plan, then archive
- **pr-implement** — fold into implement, then archive
- **pr-prep** — fold into implement, then archive
- **pr-research** — fold into implement, then archive
- **release-readiness-gate** — fold into release, then archive
- **research-software** — fold into research, then archive
- **reverse-engineer-rpi** — fold into product, then archive

## RETIRE (52)

- **artifact-clarity-pass** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **automation-loop-hardening** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **bd-first-memory-migration** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **bead-tracker-migration** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **behavior-preserving-simplification** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **changelog-quality-pass** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **cli-agent-ux-audit** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **cli-doctoring-workflow** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **concurrency-deadlock-remediation** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **contract-conformance-testing** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **dependency-update-safety** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **external-search-triage** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **filesystem-path-rationalization** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **fuzz-test-design** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **gcloud** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **gh-actions** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **gh-cli** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **gh-triage-ru** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **golden-artifact-testing** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **implementation-pattern-mining** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **installer-quality-audit** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **layered-defect-hunt** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **legacy-codebase-recon** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **live-service-e2e-testing** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **mcp-interface-design** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **measured-performance-optimization** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **metamorphic-test-design** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **native-debugger-triage** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **performance-profile-triage** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **process-triage** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **production-placeholder-audit** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **project-readme-craft** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **project-reality-check** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **project-reasoning-lens-analysis** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **repeatedly-apply-skill** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **repository-hygiene-sweep** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **ripgrep-search-discipline** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **ru-multi-repo-workflow** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **rust-crate-release-readiness** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **rust-port-validation-gauntlet** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **rust-search-integration** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **rust-sqlite-cli-architecture** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **rust-ub-risk-audit** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **rust-unsafe-boundary-audit** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **spec-reliability-implementation** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **ssh** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **stash-hygiene-sweep** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **storage-watchdog-ops** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **system-performance-remediation** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **system-tuning** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **work-contract-portability** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
- **worktree-branch-rationalization** — not the inner loop; zero recorded demand; Claude-native or one-off expertise
