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
  tier: execution
  dependencies:
  - swarm
  - validate
  - implement
  - beads-br
  - post-mortem
output_contract: code changes across wave execution, .agents/swarm/results/*.json
---
# Crank Skill

> **Quick Ref:** Autonomous epic execution. `/swarm` for each wave with runtime-native spawning. Output: closed issues + phase-2 handoff for `/validate`.

**YOU MUST EXECUTE THIS WORKFLOW. Do not just describe it.**

## Loop position

Move **5 (wave execution)** of the [operating loop](../../docs/architecture/operating-loop.md). Consumes the [slice validation plan](../../docs/templates/slice-validation.md); produces wave-by-wave slice completion via `/swarm` + `/implement`. Hard gate at wave start: every row of the wave-validity check must pass (distinct write scopes, no shared migration/contract/CLI surface, declared integration order, owner per slice, discard path per slice). Any failed row → run those slices sequential, not parallel. **Coupled-chain rule:** two slices that both regenerate a shared *derived* surface (`cli-command-surface` / `registry.json` / `context-map` / codex manifest) collide even with disjoint source files — run them as a sequential chain, each link branched off the freshly-MERGED prior link. Parallelism is explicit ownership, not swarm chaos.

Autonomous execution: implement all issues until the epic is DONE.

**Feed the orchestrator's re-plan loop — don't swallow findings into a silent retry.** When run under `/rpi`, surface what a wave proved or broke UP to the orchestrator. A failed or surprising wave is *re-plan input*, not just a retry target: per the [`/rpi` Agile Re-Plan Loop](../rpi/SKILL.md#agile-re-plan-loop-the-anti-waterfall-rule), the *remaining* waves may be refactored, inserted, dropped, or reordered before the next one runs. Re-cranking the same objective forever instead of letting the remaining plan change is the waterfall anti-pattern.

**CLI dependencies:** bd (issue tracking), ao (knowledge flywheel). Both optional — see `skills/shared/SKILL.md` for fallback table. If bd is unavailable, use TaskList for issue tracking and skip beads sync. If ao is unavailable, skip knowledge injection/extraction.

For Claude runtime feature coverage (agents/hooks/worktree/settings), the shared source of truth is `skills/shared/references/claude-code-latest-features.md`, mirrored locally at `references/claude-code-latest-features.md`.

## Architecture: Crank + Swarm

Crank owns orchestration, epic/task lifecycle, and knowledge-flywheel steps. Swarm owns runtime-native worker spawning, fresh-context isolation, per-wave execution, and cleanup. In beads mode Crank gets each wave from `bd ready`, bridges issues into worker tasks, verifies results, and syncs status back to beads. In TaskList mode the same loop runs over pending unblocked tasks instead of beads issues.

Read `references/team-coordination.md` for the full per-wave execution model, `references/ralph-loop-contract.md` for the fresh-context worker contract, and `references/worker-specs.md` for per-worker model/tool/prompt specs.

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

## Orchestrator-Merge + Reconcile Loop

When crank drives PRs to `main` itself (orchestrator-merge model), reconcile each PR mechanically:

1. **Poll** `gh pr checks <pr>` until all checks are terminal.
2. **Block only on substantive fails.** A failing `claude-review` on a usage-limit message is non-blocking; only substantive non-`claude-review` failures block the merge.
3. **Fix-forward stale/transient reds — never revert green work.** `correctness (ubuntu-latest)` tar-cache-restore exit-2 → `gh run rerun` **once**, then believe. `registry.json` / derived-surface or `contracts-sync` drift from another PR → `make regen-all` (scoped via `--skills` when only some skills changed), commit, push.
4. **Merge only when green AND the pawl gate CONFIRMS.** Green CI is necessary but **NOT sufficient** — merge-to-main is the **mutate-shared-trunk pawl** ([docs/contracts/pawls.md](../../docs/contracts/pawls.md)). A CONFIRMED, **evidence-bound, commit-current** pawl verdict ([`/pre-land-refuters`](../pre-land-refuters/SKILL.md): all refuters CONFIRMED; the pawl's diversity floor met — **fresh-context by default** (≥1 refuter in a context other than the author's; model-agnostic), or **multi-model opt-in** (≥2 distinct canonical model families) where the pawl is opted up; real non-empty reviewer evidence, `head_sha` == the PR's live head) tied to this bead+PR must exist, or the merge is **refused (HOLD)**. **REFUTED → AUTO-REDO**: the loop re-works the findings and re-gates on its own, no human. A human is escalated to **only when a tunable circuit breaker trips** (max-attempts / time budget / cost-quota / oscillation), at which point the disposition is `ESCALATE`/`HOLD` and the door stays closed (never auto-land on a breaker trip) — see pawls.md "Escalation — the circuit-breaker model". Then `gh pr merge --squash --admin`.
5. **Close on confirmed-MERGED only.** `bd close` a child bead ONLY after `gh pr view <pr> --json state -q .state` returns `MERGED` — never on a log line or batch `bd --json` query (those flake to null/0).
6. **Epic-close gate.** **NEVER close a parent epic before EVERY child PR is independently confirmed `MERGED`** — re-query `gh pr view --json state` per child first. One non-merged child aborts the close. (Post-mortem governance checkpoint: this is a hard gate, not advisory.)

> Enforce steps 4–6 with the committed scripts, not by hand: `scripts/reconcile-pr.sh <pr> <bead> [--epic <epic>]` (polls checks, reruns the lone correctness-ubuntu flake once, **verifies a CONFIRMED, evidence-bound, commit-current pawl verdict (fresh-context default; multi-model opt-in) via `scripts/pawl-verdict.sh check <bead> <pr> --head <live-sha>` — exit 5/HOLD with no merge if absent/REFUTED/ESCALATE/diversity-floor-unmet/empty-or-stale-head/no-evidence/schema-invalid; also blocks exit 2 on still-PENDING CI (green is strictly necessary)**, merges `--squash --admin`, closes the bead only on confirmed `MERGED`) and `scripts/check-epic-children-closed.sh <epic>` (the no-epic-close-with-open-child gate). Both are hermetic-tested under `tests/scripts/`.

## The FIRE Loop

Crank repeats FIRE (Find → Ignite → Reap → Vibe → Escalate) for each wave until all issues are CLOSED (beads) or all tasks are completed (TaskList). Read `references/wave-patterns.md` for the loop model, parallel wave rules, and acceptance check details.

## Key Rules

- Auto-detect tracking (`bd` first, TaskList fallback) and use the provided epic or plan input directly.
- Use `/swarm` for every wave, preserve fresh per-issue context, and refuse to continue past unresolved conflicts or the 50-wave cap.
- Per-wave validation is **chaos**, not a pawl ([docs/contracts/pawls.md](../../docs/contracts/pawls.md)): the wave-acceptance check uses the **lightweight inline judges** described in `references/wave-patterns.md` ("Wave Acceptance Check") — no skill invocations, no cross-family panel, no context explosion. Fix CRITICAL findings before advancing and keep looping until every issue/task is done. The **heavy** validation (full council, `/validate --mixed`, `/pre-land-refuters`) is reserved for the **bead-acceptance / merge-to-main pawl** — the Final Batched Validation (Step 7) and downstream `/validate` closeout, NOT per intermediate wave.
- Load learnings at the start, extract learnings at the end, and always emit `DONE`, `BLOCKED`, or `PARTIAL`.

### Folded triggers (ag-s43tg wave 1): `burndown` + `ship-loop` route here

- **`burndown` → bounded epic mode.** Use when you need to drive a finite epic set to all-merged,
  then stop — finishing a specific list of tasks, burning down a backlog epic, or executing a
  bounded set of beads until done. Crank's per-wave loop with a fixed input set (epic-id or bead
  list) and the epic-close gate IS the burndown: no new-work discovery, terminate on all-closed.
- **`ship-loop` → single-bead fast lane.** Use when running the fast-lane internal ship cycle for
  one closable bead or small slice: claim, test, implement, push, merge, close. That is a one-issue,
  one-wave crank — the Orchestrator-Merge + Reconcile Loop above (confirmed-MERGED before close)
  owns the merge/close half.

### Verb Disambiguation for Worker Prompts

Read `references/worker-verb-disambiguation.md` for the verb clarification table. Ambiguous verbs (extract, remove, update, consolidate) cause workers to implement wrong operations — always use explicit instructions with `wc -l` assertions.

## Examples

**User says:** `/crank ag-m0r` — Beads epic: loads learnings, swarm per wave, loops until all closed, final validation.
**User says:** `/crank .agents/plans/auth-refactor.md` — Plan file: decomposes into tasks, swarm per wave, final validation.
**User says:** `/crank --test-first ag-xj9` — SPEC → TEST → RED Gate → GREEN IMPL. See `references/test-first-mode.md`.

---

## Troubleshooting

Common failure modes: no ready issues, repeated wave gate failures, missing files from workers, bad RED-gate output, or TaskList/beads mismatches. See `references/troubleshooting.md` for fixes and command-level recovery steps.

---

## Inline Work Policy

Most `/crank` steps delegate worker execution via `/swarm` or `Skill()`. A small number of steps are **orchestrator-owned** by design — these are inline gates, scans, and bookkeeping that must stay in the orchestrator's context to make a downstream decision. Orchestrator-owned steps are marked with a `*(orchestrator-owned: …)*` admonition in the body (see STEP 3a.3, STEP 6.5 slop-scan, STEP 8.7).

**Do NOT convert orchestrator-owned steps into `Skill()` or `/swarm` delegations** — they are intentionally inline. Every other step (SPEC wave, TEST wave, IMPL wave, validation, lifecycle checks) should delegate via the documented `Skill(...)` call or `/swarm` invocation.

If unsure whether a step is orchestrator-owned or delegatable, the default is **delegate**. Only steps marked with the admonition above are exempt.

Crank runs as an isolated phase-2 execution context — discovery and validation are sealed off from this skill. See [references/isolation-contract.md](references/isolation-contract.md) for the four-lever enforcement model and the compression patterns `scripts/check-skill-isolation.sh` flags. See [references/best-practices.md](references/best-practices.md) for the lifecycle principle + anti-pattern citation table (cite by number; do not duplicate body content).

## Related skills

- [`/using-atm`](../using-atm/SKILL.md) — out-of-session ATM substrate for long-running `/crank` waves over a bead queue.

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
