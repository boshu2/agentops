# AgentOps Skill Domain Map

This map is the control surface for the next evolution loop. It classifies all
66 checked-in AgentOps skills before any broad rewrite, using current
`origin/main` product direction, GOALS Directive 12, the DDD/hexagonal ADR, and
the `soc-y5vh` Loop epic.

The product frame matters: AgentOps 3.0 is a context compiler and SDLC control
plane for LLM agents. Skills, CLI commands, hooks, docs, tests, beads, and
knowledge artifacts are not separate products; they are aligned adapter surfaces
around small provable changes.

## Audit Summary

> Generated from skills/ + docs/contracts/skill-dispositions.yaml.
> Do not hand-edit the table below — run `bash scripts/generate-skill-domain-map.sh`.

<!-- BEGIN:audit-summary -->
| Signal | Result |
|---|---:|
| Skills audited | 66 |
| Domains classified | 6 of 6 (BC1-BC6) |
| Dispositions assigned | 66 / 66 |
<!-- END:audit-summary -->

Observed gap: the catalog has strong operational kernels but weak productized
skill packaging. The highest-leverage first pass is not rewriting prose; it is
adding self-tests, splitting overloaded kernels into references where needed,
and aligning loop-facing skills to the bounded-context architecture.

## Domain Taxonomy

> Generated from docs/contracts/bounded-contexts.yaml.
> Do not hand-edit; edit yaml and run `bash scripts/generate-skill-domain-map.sh`.

<!-- BEGIN:domain-taxonomy -->
| Domain | Product layer | Responsibility |
|---|---|---|
| BC1 Corpus | Bookkeeping + Context Compiler + Knowledge Flywheel | Capture, retrieve, compile, cite, and promote knowledge. |
| BC2 Validation | Validation Gates | Judge whether plans, code, docs, dependencies, and releases are fit. |
| BC3 Loop | Operating loop | Select work, execute RPI, log cycles, measure fitness, and stop at convergence. |
| BC4 Factory | Skill and claim factory | Build, audit, package, and govern reusable skills and product claims. |
| BC5 Runtime | Harness and operator adapters | Adapt the control plane to harnesses, shells, local gates, and operator machines (hookless by default). |
| BC6 Orchestration | Multi-agent orchestration substrate | Spawn, coordinate, and converge multi-agent swarms across panes, mailboxes, and renewal loops. |
<!-- END:domain-taxonomy -->

## Full Skill Map

Disposition meanings:

- `keep` means the responsibility is sound; improve later only through evidence.
- `update` means add tests, references, triggers, or validation without changing the core responsibility.
- `refactor` means the skill likely needs structural reshaping or port alignment.
- `merge-review` means compare with neighboring skills before investing.
- `cut-review` means keep only if a concrete operator workflow still justifies it.

> Generated from docs/contracts/skill-dispositions.yaml.
> Do not hand-edit the table — edit the yaml and run `bash scripts/generate-skill-domain-map.sh`.

<!-- BEGIN:full-skill-map -->
| Skill | Domain | Hex role | First disposition | Rationale |
|---|---|---|---|---|
| `account-rotation` | BC5 Runtime | supporting | update | skills-audit-2026-07-06: UPDATE - false no-caam-skill claim. |
| `agent-mail` | BC6 Orchestration | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability. [reaffirmed skills-audit-2026-07-06: highest-used substrate skill; disciplined boundary]. |
| `agent-native` | BC5 Runtime | supporting | update | skills-audit-2026-07-06: UPDATE - false not-yet-shipped caveat: ao agent bundle + ao mcp serve are live; clear allowlist entry. |
| `agy-native` | BC5 Runtime | driving-adapter | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability. [reaffirmed skills-audit-2026-07-06: current (Gemini 3.5); nit: desc scoped-worktrees vs --add-dir body]. |
| `automation-shape-routing` | BC4 Factory | supporting | keep | Front-door router: decides Workflow vs NTM swarm vs plain skill, hands off to the right builder [reaffirmed skills-audit-2026-07-06: router distinct from builders; dedupe twice-stated spike numbers]. |
| `beads-br` | BC3 Loop | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability. [reaffirmed skills-audit-2026-07-06: current post-gc carve-out]. |
| `beads-bv` | BC3 Loop | supporting | update | skills-audit-2026-07-06: UPDATE - examples use stale bd- ids + bare br without BEADS_DIR wrapper. |
| `behavior-first-planning` | BC3 Loop | domain | keep | Generative discipline behind the bdd-foundry workflow; extracted so behavior-first planning survives independent of the orchestrator (age-3va.2) [reaffirmed skills-audit-2026-07-06: single owner of the Gherkin contract (audit S11)]. |
| `bootstrap` | BC4 Factory | driving-adapter | update | First-run factory entrypoint; needs current 3.0/domain packet shape [reaffirmed skills-audit-2026-07-06: stale /rpi next-step line (command removed)]. |
| `cass` | BC1 Corpus | supporting | keep | Mines past agent sessions for prompts/decisions/patterns — a corpus reader; re-binned BC5→BC1 (ag-j3ge0, capability_class=corpus already). [reaffirmed skills-audit-2026-07-06: tight self-describing primitive]. |
| `cc-hooks` | BC5 Runtime | supporting | refactor | skills-audit-2026-07-06: REFACTOR - hookless-honest; dedupe doubled Absorbed-skills block. |
| `codex-exec` | BC5 Runtime | driving-adapter | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability. [reaffirmed skills-audit-2026-07-06: load-bearing LAW-0 headless path; dense not bloated]. |
| `compile` | BC1 Corpus | supporting | refactor | Corpus compiler is core; align read/write flows to Corpus ports. |
| `converge` | BC5 Runtime | driving-adapter | keep | Thin memo over the Go ao converge command (context-quorum loop + two-sided canary); the loop lives in cli, not the skill [reaffirmed skills-audit-2026-07-06: clean memo over live ao converge]. |
| `converter` | BC4 Factory | driven-adapter | keep | Cross-runtime packaging adapter (Codex/Cursor twins) the factory drives to emit converted-skill output; skill-builder consumes it. Re-graded generic→driven-adapter (ag-j3ge0 — it adapts the factory to runtime skill formats, a driven port, not an unclassified generic). [reaffirmed skills-audit-2026-07-06: distinct format adapter; fix stale vibe comment]. |
| `council` | BC2 Validation | domain | keep | skills-audit-2026-07-06: KEEP - pre-work decision forum the pawl does not replace. |
| `crank` | BC3 Loop | domain | update | skills-audit-2026-07-06: UPDATE - retired PR-merge loop section -> push-to-main + pawl. |
| `curate` | BC1 Corpus | supporting | keep | Designed-future canonical unified miner (m6v5.D Phase 1, epic soc-cp7pv); not redundant cruft — epic GO/REVERT is a separate decision (resolved KEEP 2026-05-24). |
| `dcg` | BC5 Runtime | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability. [reaffirmed skills-audit-2026-07-06: honest safety host-tool]. |
| `discovery` | BC3 Loop | domain | keep | skills-audit-2026-07-06: KEEP - top-used spine entry; cross-link behavior-first-planning. |
| `doc` | BC4 Factory | supporting | refactor | skills-audit-2026-07-06: REFACTOR - default mode frontier-trivial; value is readme/oss reference-led modes. |
| `domain` | BC4 Factory | domain | update | skills-audit-2026-07-06: UPDATE - bd->br in How-to-use. |
| `eval-outcomes` | BC2 Validation | supporting | cut-review | skills-audit-2026-07-06: CUT-REVIEW - stub; validate scenario target + ao eval scenario cover it. |
| `evolve` | BC3 Loop | domain | refactor | Autonomous improvement main loop with convergence STOP; promoted supporting→domain (ag-j3ge0 — the loop's core driver, not a peripheral helper). Demoted to experimental (heavy rpi chain, no measured uplift). [reaffirmed skills-audit-2026-07-06: ~40pct retired procedure (PR-merge/autodev/hooks); align compounding claims to ADR-0004/0011]. |
| `flywheel` | BC1 Corpus | domain | cut-review | skills-audit-2026-07-06: CUT-REVIEW - thin wrapper over ao flywheel status; monitor not miner. |
| `gc-membrane` | BC6 Orchestration | supporting | keep | gc adoption (age-gc-integrate-8aom.1): JIT reference for the agentops-membrane pack — the fail-closed close door; loaded by using-gc. [reaffirmed skills-audit-2026-07-06: current close-door reference]. |
| `goals` | BC3 Loop | domain | update | skills-audit-2026-07-06: UPDATE - GOALS.yaml-first lead inverted from canonical GOALS.md v4. |
| `handoff` | BC1 Corpus | supporting | update | Session continuity artifact; clarify promotion vs local-only notes [reaffirmed skills-audit-2026-07-06: frontmatter produces wrong; handoff/handoffs path split]. |
| `heal-skill` | BC4 Factory | supporting | keep | skills-audit-2026-07-06: KEEP - corpus-wide hygiene pass; not a builder mode. |
| `implement` | BC3 Loop | driving-adapter | keep | skills-audit-2026-07-06: KEEP - model references-led shape; Move-4 owner. |
| `ms` | BC1 Corpus | supporting | keep | Wraps Jeffrey Emanuel's meta_skill (ms) — the skill-search/load engine over both corpora (agentops + jsm); MCP consume, CLI writes. A corpus reader like cass. [reaffirmed skills-audit-2026-07-06: honest and current; documents its own footguns]. |
| `ntm` | BC6 Orchestration | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability. [reaffirmed skills-audit-2026-07-06: model progressive-disclosure; single owner of tending doctrine]. |
| `operationalize` | BC1 Corpus | domain | refactor | skills-audit-2026-07-06: REFACTOR - distinct route-to-enforcement lane; cut demoted-flywheel prose. |
| `perf` | BC3 Loop | domain | cut-review | skills-audit-2026-07-06: CUT-REVIEW - ~90pct frontier-generic, zero repo bindings, 0 use; jsm skills cover. |
| `plan` | BC3 Loop | domain | refactor | skills-audit-2026-07-06: REFACTOR - largest loop skill; dedupe Gherkin contract (owner: behavior-first-planning) + wave lore. |
| `post-mortem` | BC3 Loop | domain | keep | skills-audit-2026-07-06: KEEP - move-7 anchor; absorbs curate mining half at the retire wave. |
| `pr-prep` | BC5 Runtime | driving-adapter | keep | skills-audit-2026-07-06: KEEP - scoped to external repos; unaffected by in-repo PR retirement. |
| `pre-land-refuters` | BC2 Validation | driving-adapter | keep | Unbiased dual-model refuter panel before landing large multi-surface changes; codified from the ag-s43tg landing where it caught 9 real misses. [reaffirmed skills-audit-2026-07-06: THE live pawl-path memo; excise vestigial PR-binding section]. |
| `pre-mortem` | BC2 Validation | domain | update | Plan risk gate; add scenario/verdict self-test [reaffirmed skills-audit-2026-07-06: dead hook-enforcement mandates (hookless 3.0); dedupe vs validate]. |
| `product` | BC3 Loop | domain | refactor | skills-audit-2026-07-06: REFACTOR - collapse nine-framework name-drop table to a reference. |
| `push` | BC5 Runtime | driving-adapter | update | Git adapter; add branch/worktree disposition self-test [reaffirmed skills-audit-2026-07-06: wire ao gate check --fast --scope head + pawl-land as THE ship path]. |
| `rch` | BC5 Runtime | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability. [reaffirmed skills-audit-2026-07-06: best-in-class wrapper; refuses --help duplication]. |
| `reality-check` | BC2 Validation | domain | cut-review | skills-audit-2026-07-06: CUT-REVIEW - good but unreachable: user-invocable false, 0 use; promote-or-fold decision. |
| `recover` | BC1 Corpus | driving-adapter | merge-review | skills-audit-2026-07-06: MERGE-REVIEW - merge into status as post-compaction mode; keep trigger surface. |
| `red-team` | BC2 Validation | supporting | cut-review | skills-audit-2026-07-06: CUT-REVIEW - 1KB stub to mt-olympus; validate --debate absorbs. |
| `refactor` | BC3 Loop | supporting | keep | skills-audit-2026-07-06: KEEP - fix .agentscomplexity artifact-path typo. |
| `release` | BC2 Validation | supporting | keep | skills-audit-2026-07-06: KEEP - references-led exemplar. |
| `research` | BC1 Corpus | driving-adapter | refactor | skills-audit-2026-07-06: REFACTOR - workhorse; dedupe references vs shared twins. |
| `reverse-engineer` | BC1 Corpus | supporting | keep | External-system teardown -> steal-map (have/gap/steal/park/reject) -> route one-way doors to /discovery. Revived + renamed from reverse-engineer-rpi (cut ag-s43tg S24), upgraded with the steal-map discipline. [reaffirmed skills-audit-2026-07-06: 47 refs are a tested teardown engine: depth-as-tooling]. |
| `review` | BC2 Validation | driving-adapter | merge-review | skills-audit-2026-07-06: MERGE-REVIEW - merge into validate (declared absorption); bd-stale; ~0 use. |
| `rpi` | BC3 Loop | domain | refactor | Loop-spine lifecycle orchestrator (Research→Plan→Implement); promoted supporting→domain (ag-j3ge0 — the per-turn executor IS the operating loop's core logic, not a peripheral helper) [reaffirmed skills-audit-2026-07-06: demote framing to one-turn executor; dedupe pawl/replan prose vs crank+pawls.md]. |
| `sbh` | BC5 Runtime | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability. [reaffirmed skills-audit-2026-07-06: compact task-organized cheat-sheet]. |
| `scaffold` | BC4 Factory | supporting | refactor | skills-audit-2026-07-06: REFACTOR - ~85pct generic templates; keep the domain-slice binding. |
| `scope` | BC5 Runtime | driven-adapter | cut-review | skills-audit-2026-07-06: CUT-REVIEW - enforcement premise inert: promised PreToolUse hook absent; promote-or-retire decision. |
| `security` | BC2 Validation | driven-adapter | keep | Canonical security skill: continuous release-gate driver (security-gate.sh) PLUS the composable primitive library (security_suite.py / prompt_redteam.py) collapsed in from security-suite (cp-qqq, 2026-06-07) [reaffirmed skills-audit-2026-07-06: script-backed distinct product domain]. |
| `shared` | BC4 Factory | domain | update | skills-audit-2026-07-06: UPDATE - bd->br in CLI fallback rows. |
| `skill-builder` | BC4 Factory | supporting | keep | skills-audit-2026-07-06: KEEP - core meta-skill; invokes converter+heal as real gates. |
| `standards` | BC4 Factory | domain | update | skills-audit-2026-07-06: UPDATE - index routes to retired vibe (20x, dead paths); content gold, wrapper stale. |
| `status` | BC3 Loop | driving-adapter | update | Operator state surface; should show loop/domain/evidence status [reaffirmed skills-audit-2026-07-06: duplicate /validate QUICK-COMMANDS line; becomes the one session surface]. |
| `swarm` | BC6 Orchestration | supporting | refactor | skills-audit-2026-07-06: REFACTOR - dedupe wave-validity lore; excise PR-reaping debris. |
| `test` | BC2 Validation | supporting | keep | skills-audit-2026-07-06: KEEP - standalone value beyond implement (coverage/strategy/harnesses). |
| `toil-mining` | BC1 Corpus | supporting | keep | Seams epic ag-xwjlc: usage-history toil miner feeding automation-shape-routing; the flywheel's missing feeder. [reaffirmed skills-audit-2026-07-06: cleanest of cluster; LAW-0-aware feeder]. |
| `using-atm` | BC6 Orchestration | supporting | refactor | skills-audit-2026-07-06: REFACTOR - 432 lines with 0 reference files; extract meter-LIES/continuity/tmux to references/. |
| `using-gc` | BC6 Orchestration | supporting | keep | gc adoption (age-gc-integrate-8aom.1 / age-gc-adoption-u0he.3): the vibing-with-ntm analog for driving a gas city — operator-choice substrate beside NTM, routes to gc's native surface. [reaffirmed skills-audit-2026-07-06: fresh four-jobs JIT split]. |
| `validate` | BC2 Validation | driving-adapter | update | skills-audit-2026-07-06: UPDATE - strip rpi/PR-mode/dead judge-orchestration; align close path to pawl scripts; resolve Replaces claims. |
| `workflow-builder` | BC4 Factory | supporting | keep | Scaffolds Claude Workflow scripts (composite capability); counterpart to skill-builder [reaffirmed skills-audit-2026-07-06: thin distinct Workflow scaffold]. |
<!-- END:full-skill-map -->

## Priority Queue

1. **Loop spine:** `evolve`, `rpi`, `discovery`, `plan`, `crank`,
   `validation`, `post-mortem`, `ratchet`.
2. **Factory spine:** `standards`, `skill-builder`, `skill-auditor`,
   `heal-skill`, `converter`.
3. **Corpus spine:** `compile`, `inject`, `flywheel`, `forge`, `harvest`,
   `dream`.
4. **Validation spine:** `council`, `vibe`, `pre-mortem`, `test`, `review`,
   `security-suite`, `release`.
5. **Runtime spine:** `hooks-authoring`, `scope`, `push`, `swarm`,
   `codex-team`.
