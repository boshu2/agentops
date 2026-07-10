---
name: crank
description: 'Execute epics through waves. Triggers: "crank an epic", "execute epics through waves", "drive the bead wave plan".'
practices:
- continuous-delivery
- xp
- agile-manifesto
hexagonal_role: domain
consumes:
- beads-br
- implement
- post-mortem
- swarm
- validate
produces:
- .agents/swarm/results/*.json
- git-changes
context_rel:
- kind: shared-kernel
  with: standards
skill_api_version: 1
user-invocable: true
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude:
    - HISTORY
  intel_scope: full
metadata:
  graph_root: true
  tier: execution
  dependencies:
  - swarm
  - validate
  - implement
  - beads-br
  - post-mortem
  - agent-native
  - automation-shape-routing
  - dcg
  - pawl-review
output_contract: code changes across wave execution, .agents/swarm/results/*.json
---
# Crank Skill

> **Quick Ref:** Autonomous epic execution. `/swarm` for each wave with runtime-native spawning. Output: closed issues + phase-2 handoff for `/validate`.

**YOU MUST EXECUTE THIS WORKFLOW. Do not just describe it.**

## Loop position

Move **5 (wave execution)** of the [operating loop](../../docs/architecture/operating-loop.md). Consumes the [slice validation plan](../../docs/templates/slice-validation.md); produces wave-by-wave slice completion via `/swarm` + `/implement`. Each slice runs the canonical [narrow-waist micro-cycle](../../docs/architecture/operating-loop.md#the-narrow-waist-micro-cycle-canonical--every-loop-skill-cites-this): its acceptance test authored RED before code is the slice contract, and **refactor-under-green is its own wave, never optional** (`references/wave-patterns.md`) — a refactor wave must change no test. Hard gate at wave start: every row of the wave-validity check must pass (distinct write scopes, no shared migration/contract/CLI surface, declared integration order, owner per slice, discard path per slice). Any failed row → run those slices sequential, not parallel. **Coupled-chain rule:** two slices that both regenerate a shared *derived* surface (`cli-command-surface` / `registry.json` / `context-map` / codex manifest) collide even with disjoint source files — run them as a sequential chain, each link branched off the freshly-MERGED prior link. Parallelism is explicit ownership, not swarm chaos.

Autonomous execution: implement all issues until the epic is DONE.

**Feed the orchestrator's re-plan loop — don't swallow findings into a silent retry.** When run under `/rpi`, surface what a wave proved or broke UP to the orchestrator. A failed or surprising wave is *re-plan input*, not just a retry target: per the [`/rpi` Agile Re-Plan Loop](../rpi/SKILL.md#agile-re-plan-loop-the-anti-waterfall-rule), the *remaining* waves may be refactored, inserted, dropped, or reordered before the next one runs. Re-cranking the same objective forever instead of letting the remaining plan change is the waterfall anti-pattern.

**CLI dependencies:** br (issue tracking, via `BEADS_DIR="$(ao beads dir)" br`), ao (knowledge flywheel). Both optional — see `skills/shared/SKILL.md` for fallback table. If br is unavailable, use TaskList for issue tracking and skip beads sync. If ao is unavailable, skip knowledge injection/extraction.

For Claude runtime feature coverage (agents/hooks/worktree/settings), the shared source of truth is `skills/shared/references/claude-code-latest-features.md`, mirrored locally at `references/claude-code-latest-features.md`.

## Architecture: Crank + Swarm

Crank owns orchestration, epic/task lifecycle, and knowledge-flywheel steps. Swarm owns runtime-native worker spawning, fresh-context isolation, per-wave execution, and cleanup. In beads mode Crank gets each wave from `ao beads exec ready` (tracker- and ledger-agnostic — it resolves bd or br and its ledger automatically), bridges issues into worker tasks, verifies results, and syncs status back to beads. In TaskList mode the same loop runs over pending unblocked tasks instead of beads issues.

Read `references/team-coordination.md` for the full per-wave execution model, `references/ralph-loop-contract.md` for the fresh-context worker contract, and [references/worker-specs.md](references/worker-specs.md) for per-worker model/tool/prompt specs.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--test-first` | off | Enable spec-first TDD: SPEC WAVE generates contracts, TEST WAVE generates failing tests, IMPL WAVES make tests pass |
| `--per-task-commits` | off | Opt-in per-task commit strategy. Falls back to wave-batch when file boundaries overlap. See `references/commit-strategies.md`. |
| `--tier=<name>` | (auto) | Force a specific cost tier (quality/balanced/budget) for all council calls. Overrides effort-to-tier auto-mapping. |
| `--no-lifecycle` | off | Skip ALL lifecycle skill auto-invocations (test delegation in TEST WAVE, pre-validation deps/test checks) |
| `--lifecycle=<tier>` | matches complexity | Controls which lifecycle skills fire: `minimal` (test only), `standard` (+deps vuln), `full` (all) |
| `--no-scope-check` | off | Skip scope-completion check before DONE marker (Step 8.7) |
| `--skip-audit` | off | Skip bd-audit pre-flight gate (Step 3a.2) |

## Global Limits

**MAX_EPIC_WAVES = 50** (hard limit across entire epic)

This prevents infinite loops on circular dependencies or cascading failures. Typical epics use 5–10 waves max.

## Completion Enforcement (The Sisyphus Rule)

Not done until you emit an explicit completion marker after each wave:
- `<promise>DONE</promise>` when the epic is truly complete
- `<promise>BLOCKED</promise>` when progress cannot continue
- `<promise>PARTIAL</promise>` when work remains

Never claim completion without one of these markers.

## Node Repair Operator

When a task fails during wave execution, classify as **RETRY** (transient — re-add with adjustment, max 2), **DECOMPOSE** (too complex — split into sub-issues, terminal), or **PRUNE** (blocked — escalate immediately). Budget: 2 per task. Read `references/failure-recovery.md` for classification signals and recovery commands.

**Mutation logging on failure classification:**
- **DECOMPOSE:** Log `task_removed` for the original task, then `task_added` for each new sub-task.
- **PRUNE:** Log `task_removed` with the block reason.
- **RETRY:** No mutation (task identity unchanged).

## Execution Steps

Given `/crank [epic-id | .agents/rpi/execution-packet.json | plan-file.md | "description"]`:

### Preflight (Recovery hooks → Step 3a.3)

Read [references/execution-preflight.md](references/execution-preflight.md) when you need recovery-hook setup, effort/tier mapping, knowledge-context loading (Step 0), tracking-mode detection (0.5), gc-pool detection (0.6), epic identification (Step 1), branch isolation (1.5), wave-counter / mutation-trail / shared-task-notes initialization (1a–1a.2), test-first classification (1b), epic details (Step 2), ready-issue listing (Step 3), and the four pre-flight checks (3a, 3a.1 pre-mortem, 3a.2 bd-audit, 3a.3 changed-string grep).

The Branch Isolation Gate (Step 1.5) has its own dedicated contract — see [references/branch-isolation.md](references/branch-isolation.md) for when crank must create or refuse an isolation branch.

### Wave dispatch (Step 3b → Step 4)

Read [references/wave-dispatch.md](references/wave-dispatch.md) when you need SPEC WAVE / TEST WAVE / RED Gate flow (Steps 3b–3c), context-briefing assembly (3b.1), shared-notes injection (3b.2), parallel-wave isolation (3b.3), or Step 4 wave execution detail — GREEN mode, issue-typing + file manifests, grep-for-existing-functions, validation metadata policy, acceptance-criteria injection, language-standards injection, file-ownership table, wave-counter / 50-cap gate, spec-consistency gate, cross-cutting constraint injection, gc-pool dispatch, and cross-cutting validation.

### Wave completion (Step 5 → Step 8.7)

Read [references/wave-completion.md](references/wave-completion.md) when you need verify-and-sync (Step 5, external-gate protocol), wave acceptance check + CI-policy parity gate (5.5), wave checkpoint + per-criterion verdicts + back-compat fallback (5.7), validation-context checkpoint (5.7b), shared-task-notes harvest (5.7c), plan-mutation logging (5.7d), wave status report (5.8), worktree base-SHA refresh (5.9), check-for-more-work loop (Step 6), de-sloppify pass (6.5), pre-validation lifecycle checks (6.9), final batched validation (Step 7), phase-2 summary (Step 8), learnings extraction (8.5), shared-notes archive (8.6), and the scope-completion pre-close gate (8.7).

Step 5.5 includes the **CI-Policy Parity Gate**: if a wave diff touches `.github/workflows/*.yml`, run `bash scripts/validate-ci-policy-parity.sh`; any non-zero exit fails wave acceptance and surfaces the generated drift report. See [references/wave-patterns.md](references/wave-patterns.md) "CI-Policy Parity Gate" for the worked example and trigger pattern.

### Step 9: Report Completion

Tell the user:
1. Epic ID and title
2. Number of issues completed
3. Total iterations used (of 50 max)
4. Final validation (/validate --mode=post-impl, absorbs vibe) results
5. Flywheel status (if ao available)
6. Suggest running `/validate` to complete closeout and promote learnings

**Output completion marker:**
```
<promise>DONE</promise>
Epic: <epic-id>
Issues completed: N
Iterations: M/50
Flywheel: <status from ao metrics flywheel status>
```

If stopped early:
```
<promise>BLOCKED</promise>
Reason: <global limit reached | unresolvable blockers>
Issues remaining: N
Iterations: M/50
```

## Land Loop (per-bead, direct-main)

Crank lands each bead's slice to `main` **directly** — PR-per-bead is retired for THIS repo (external-repo variant at the end of this section). Land each bead from **its own worktree**, one at a time:

1. **Isolate.** The bead's slice is committed on HEAD in the bead's own `git worktree` (never the shared checkout), the HEAD message citing the bead id (the gate + pawl resolve the bead from it).
2. **Gate.** `ao gate check --fast --scope head` — the local cockpit gate (also the pre-push hook; run it manually to fail fast). Fix-forward stale/transient reds, never revert green work; regenerate drifted derived surfaces (`registry.json` / `contracts-sync` → `make regen-all`, scoped via `--skills` when only some skills changed) and commit them WITH the change.
3. **Review — the mutate-shared-trunk pawl.** `pawl-review` runs immutable fresh reviewer lanes and hands evidence to `ao pawl`; `ao pawl` alone applies diversity, commit binding, and admission ([docs/contracts/pawls.md](../../docs/contracts/pawls.md); [`/pawl-review`](../pawl-review/SKILL.md)). CONFIRMED requires all refuters confirmed, the selected diversity floor, real nonempty evidence, and a verdict bound to the live head. No CONFIRMED verdict means no land. REFUTED routes back to re-work; circuit-breaker exhaustion HOLDs.
4. **Land.** `bash scripts/pawl-land.sh <bead>` — fetch + rebase onto current `origin/main`, restamp the CONFIRMED verdict onto the post-rebase feat, single-shot `push origin HEAD:main`. Aborts without pushing on a rebase conflict (resolve locally, re-run pawl-review if the tree changed, re-land).
5. **Close on landed-only.** `ao beads exec close` a child bead ONLY after its commit is confirmed on trunk — `git fetch origin main && git merge-base --is-ancestor <feat-sha> origin/main` — never on a log line or a batch `br --json` query (those flake to null/0).
6. **Epic-close gate.** **NEVER close a parent epic before EVERY child bead's commit is confirmed an ancestor of `origin/main`** (re-check `git merge-base --is-ancestor` per child; each child `br` CLOSED). One unlanded child aborts the close. (Post-mortem governance checkpoint: hard gate, not advisory.)

### Close checkpoint — a closed bead is a sensor reading (age-cysr)

Not a checkbox ([the flywheel](../../docs/architecture/the-flywheel.md)): the close is the loop's highest-signal, membrane-verified evidence. On EVERY close (step 5) answer two questions before moving on:
1. **What did completing this bead teach?** (one line — usually "nothing new", and that's fine)
2. **Does it CONTRADICT an assumption the remaining plan depends on?**

If **no** → proceed to the next bead. If **yes** (a falsified plan assumption) → re-plan the remaining slices NOW, not at the wave boundary: invoke `/discovery` as the re-plan engine over the remaining DAG (split / re-order / add / drop beads) and record the trigger in the close reason (`replan: <falsified assumption>`). **Anti-thrash guard:** the trigger is a falsified plan assumption ONLY — most closes teach nothing; never re-plan on mere surprise, difficulty, or a new idea (park those for `/post-mortem`). **Andon bound:** a re-plan that would rework the same remaining DAG a 3rd time escalates to the human instead of re-planning again.

**Multi-lane serialization + by-hand land.** When several lanes land onto a hot `main` at once, or when you land by hand via the `ao pawl review` CLI (which sets `PAWL_UNTRUSTED_REPO=1` and SKIPS auto-bind, so the sealed bind is manual), follow the serialized land-token discipline and the exact `[feat, #trivial-bind]` command sequence in [references/land-protocol.md](references/land-protocol.md) — one land at a time across lanes, `ao provenance emit-verdict` for the sealed bind (never a hand-appended ledger edge), and `git merge-base --is-ancestor` before every `ao beads exec close`.

> Enforce steps 3–4 with the committed scripts, not by hand: `scripts/pawl-review.sh <bead> --scope head --author-family <family>` (runs the refuter; on CONFIRMED writes + verifies the commit-bound verdict via `scripts/pawl-verdict.sh`, REFUTED exit 3 prints the defects) then `scripts/pawl-land.sh <bead>` (rebase → restamp → single-shot push). The epic-close gate is `scripts/check-epic-children-closed.sh <epic>` (no-epic-close-with-open-child). All are hermetic-tested under `tests/scripts/`.

> **External-repo variant (PR flow).** When crank targets an **external repo** (an upstream fork where you cannot push `main`), the land half becomes a PR: prepare it with [`/pr-prep`](../pr-prep/SKILL.md), then reconcile mechanically with `scripts/reconcile-pr.sh <pr> <bead> [--epic <epic>]` (polls `gh pr checks`, reruns the lone correctness-ubuntu flake once, verifies the CONFIRMED pawl verdict via `scripts/pawl-verdict.sh check <bead> <pr> --head <live-sha>` — exit 5/HOLD if absent/REFUTED/ESCALATE/stale-head/no-evidence, merges `gh pr merge --squash --admin`, closes the bead only on confirmed `MERGED`). This path is for **external targets only** — never for landing AgentOps' own beads.

## The FIRE Loop

Crank repeats FIRE (Find → Ignite → Reap → Vibe → Escalate) for each wave until all issues are CLOSED (beads) or all tasks are completed (TaskList). Read `references/wave-patterns.md` for the loop model, parallel wave rules, and acceptance check details.

## Key Rules

- Auto-detect tracking (`br` first, TaskList fallback) and use the provided epic or plan input directly.
- Use `/swarm` for every wave, preserve fresh per-issue context, and refuse to continue past unresolved conflicts or the 50-wave cap.
- Per-wave validation is **chaos**, not a pawl ([docs/contracts/pawls.md](../../docs/contracts/pawls.md)): use lightweight inline checks between waves. Reserve `/validate`, council, and `/pawl-review` evidence for the bead-acceptance / merge-to-main pawl, not every intermediate wave.
- Load learnings at the start, extract learnings at the end, and always emit `DONE`, `BLOCKED`, or `PARTIAL`.

### Folded triggers (ag-s43tg wave 1): `burndown` + `ship-loop` route here

- **`burndown` → bounded epic mode.** Use when you need to drive a finite epic set to all-merged,
  then stop — finishing a specific list of tasks, burning down a backlog epic, or executing a
  bounded set of beads until done. Crank's per-wave loop with a fixed input set (epic-id or bead
  list) and the epic-close gate IS the burndown: no new-work discovery, terminate on all-closed.
- **`ship-loop` → single-bead fast lane.** Use when running the fast-lane internal ship cycle for
  one closable bead or small slice: claim, test, implement, gate, pawl-review, pawl-land, close. That
  is a one-issue, one-wave crank — the Land Loop above (CONFIRMED pawl verdict + landed-on-trunk
  before close) owns the land/close half.

### Verb Disambiguation for Worker Prompts

Read `references/worker-verb-disambiguation.md` for the verb clarification table. Ambiguous verbs (extract, remove, update, consolidate) cause workers to implement wrong operations — always use explicit instructions with `wc -l` assertions.

## Examples

**User says:** `/crank ag-m0r` — Beads epic: loads learnings, swarm per wave, loops until all closed, final validation.
**User says:** `/crank .agents/plans/auth-refactor.md` — Plan file: decomposes into tasks, swarm per wave, final validation.
**User says:** `/crank --test-first ag-xj9` — SPEC → TEST → RED Gate → GREEN IMPL. See `references/test-first-mode.md`.

---

## Output Specification

**Format:** committed code plus a markdown progress/closeout summary to stdout; per-slice [slice-validation](../../docs/templates/slice-validation.md) roll-ups.
**Files:** reads `.agents/rpi/execution-packet.json`; writes wave/slice results under `.agents/swarm/results/`; closes beads via `ao beads exec close` in the resolved bead ledger.
**Exit signal:** `<promise>DONE</promise>` (all slices accepted) · `<promise>PARTIAL</promise>` (retry the same objective) · `<promise>BLOCKED</promise>` (manual intervention).

## Troubleshooting

Common failure modes: no ready issues, repeated wave gate failures, missing files from workers, bad RED-gate output, or TaskList/beads mismatches. See `references/troubleshooting.md` for fixes and command-level recovery steps.

---

## Inline Work Policy

Most `/crank` steps delegate worker execution via `/swarm` or `Skill()`. A small number of steps are **orchestrator-owned** by design — these are inline gates, scans, and bookkeeping that must stay in the orchestrator's context to make a downstream decision. Orchestrator-owned steps are marked with a `*(orchestrator-owned: …)*` admonition in the body (see STEP 3a.3, STEP 6.5 slop-scan, STEP 8.7).

**Do NOT convert orchestrator-owned steps into `Skill()` or `/swarm` delegations** — they are intentionally inline. Every other step (SPEC wave, TEST wave, IMPL wave, validation, lifecycle checks) should delegate via the documented `Skill(...)` call or `/swarm` invocation.

If unsure whether a step is orchestrator-owned or delegatable, the default is **delegate**. Only steps marked with the admonition above are exempt.

Crank runs as an isolated phase-2 execution context — discovery and validation are sealed off from this skill. See [references/isolation-contract.md](references/isolation-contract.md) for the four-lever enforcement model and the compression patterns `scripts/check-skill-isolation.sh` flags. See [references/best-practices.md](references/best-practices.md) for the lifecycle principle + anti-pattern citation table (cite by number; do not duplicate body content).

## Related skills

- [`/agent-native`](../agent-native/SKILL.md) — portable persistent-worker lifecycle; use [`/ntm`](../ntm/SKILL.md) for NTM pane mechanics.

## Reference Documents

- [references/crank.feature](references/crank.feature) — Executable spec: wave-validity hard gate, FIRE loop, mandatory completion marker, 50-wave cap (soc-qk4b.2)
- [references/de-sloppify.md](references/de-sloppify.md)
- [references/execution-preflight.md](references/execution-preflight.md)
- [references/parallel-wave-isolation.md](references/parallel-wave-isolation.md)
- [references/plan-mutations.md](references/plan-mutations.md)
- [references/shared-task-notes.md](references/shared-task-notes.md)
- [references/claude-code-latest-features.md](references/claude-code-latest-features.md)
- [references/commit-strategies.md](references/commit-strategies.md)
- [references/worktree-per-worker.md](references/worktree-per-worker.md)
- [references/contract-template.md](references/contract-template.md)
- [references/failure-recovery.md](references/failure-recovery.md)
- [references/land-protocol.md](references/land-protocol.md) — serialized multi-lane land protocol: land-token, the `[feat, #trivial-bind]` sequence, stale-bind drop, failure playbook (age-e508.3)
- [references/failure-taxonomy.md](references/failure-taxonomy.md)
- [references/fire.md](references/fire.md)
- [references/gc-pool-dispatch.md](references/gc-pool-dispatch.md)
- [references/ralph-loop-contract.md](references/ralph-loop-contract.md)
- [references/taskcreate-examples.md](references/taskcreate-examples.md)
- [references/team-coordination.md](references/team-coordination.md)
- [references/test-first-mode.md](references/test-first-mode.md)
- [references/troubleshooting.md](references/troubleshooting.md)
- [references/phase-data-contracts.md](references/phase-data-contracts.md) — phase artifact data contracts (cited from references/isolation-contract.md)
- [references/uat-integration-wave.md](references/uat-integration-wave.md)
- [references/wave-completion.md](references/wave-completion.md)
- [references/wave-dispatch.md](references/wave-dispatch.md)
- [references/wave1-spec-consistency-checklist.md](references/wave1-spec-consistency-checklist.md)
- [references/wave-patterns.md](references/wave-patterns.md)
- [references/worker-verb-disambiguation.md](references/worker-verb-disambiguation.md)
- [references/external-gate-protocol.md](references/external-gate-protocol.md)

- [references/ship-loop-anti-patterns.md](references/ship-loop-anti-patterns.md) — absorbed ship-loop anti-pattern catalog (ag-s43tg)
