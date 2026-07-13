---
name: crank
description: Execute the next ready epic wave and return
---
# Crank Skill

## Codex Lifecycle Guard

When this skill runs in Codex hookless mode (`CODEX_THREAD_ID` is set or
`CODEX_INTERNAL_ORIGINATOR_OVERRIDE` is `Codex Desktop`), run:

```bash
ao codex ensure-start 2>/dev/null || true
```

The CLI records startup once per thread and skips duplicates automatically.

> **Quick Ref:** Execute the next ready wave with runtime-native workers. Output:
> wave evidence + phase-2 handoff for Validate.

**YOU MUST EXECUTE THIS WORKFLOW. Do not just describe it.**

## Constraints

- Execute only tracker-ready vertical slices because crank consumes an accepted plan; it does not silently redefine intent.
- Parallelize only disjoint write scopes and serialize shared derived surfaces to prevent workers from invalidating one another's base.
- Return unresolved wave evidence to the orchestrator instead of choosing a
  cross-phase retry or re-plan inside Crank.

## Loop position

Move **5 (wave execution)** of the [operating loop](../../docs/architecture/operating-loop.md). Consumes the [slice validation plan](../../docs/templates/slice-validation.md); produces wave-by-wave slice completion via `$swarm` + `$implement`. Each slice runs the canonical [narrow-waist micro-cycle](../../docs/architecture/operating-loop.md#the-narrow-waist-micro-cycle-canonical--every-loop-skill-cites-this): its acceptance test authored RED before code is the slice contract, and **refactor-under-green is its own wave, never optional** (`references/wave-patterns.md`) — a refactor wave must change no test. Hard gate at wave start: every row of the wave-validity check must pass (distinct write scopes, no shared migration/contract/CLI surface, declared integration order, owner per slice, discard path per slice). Any failed row → run those slices sequential, not parallel. **Coupled-chain rule:** two slices that both regenerate a shared *derived* surface (`cli-command-surface` / `registry.json` / `context-map` / codex manifest) collide even with disjoint source files — run them as a sequential chain, each link branched off the freshly-MERGED prior link. Parallelism is explicit ownership, not swarm chaos.

Under RPI, one Crank invocation ends at one accepted wave. PARTIAL means work
remains and returns through Validate and Learn before another wave. Standalone
callers fulfill the same orchestrator contract rather than looping silently.

**Feed the orchestrator's decision loop — do not swallow findings into a silent retry.** Crank hands wave evidence to Validate and stops at its phase boundary. It does not invoke Discovery, Learn, or Premortem. Validate produces the immutable verdict, Learn classifies plan impact, and only the orchestrator may retry or change the remaining waves.

**CLI dependencies:** br (issue tracking, via `BEADS_DIR="$(ao beads dir)" br`), ao (knowledge flywheel). Both optional — see `skills/shared/SKILL.md` for fallback table. If br is unavailable, use TaskList for issue tracking and skip beads sync. If ao is unavailable, skip knowledge injection/extraction.

For Claude runtime feature coverage (agents/hooks/worktree/settings), the shared source of truth is `skills/shared/references/claude-code-latest-features.md`, mirrored locally at `references/claude-code-latest-features.md`.

## Architecture: Crank + Swarm

Crank owns within-wave execution and task lifecycle. RPI owns between-wave
transitions. Swarm owns runtime-native worker spawning, fresh-context isolation,
per-wave execution, and cleanup. In beads mode Crank gets the next wave from
`ao beads exec ready`, bridges issues into worker tasks, verifies results, and
syncs status back to beads. TaskList mode uses the same one-wave boundary.

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

When a task fails during wave execution, classify as **RETRY** (transient — re-add with adjustment, max 2), **DECOMPOSE** (too complex — split into sub-issues, terminal), or **PRUNE** (blocked — one bounded helper pass, then escalate what survives). Budget: 2 per task. Read `references/failure-recovery.md` for classification signals and recovery commands.

**Mutation logging on failure classification:**
- **DECOMPOSE:** Log `task_removed` for the original task, then `task_added` for each new sub-task.
- **PRUNE:** Log `task_removed` with the block reason.
- **RETRY:** No mutation (task identity unchanged).

## Execution Steps

Given `$crank [epic-id | .agents/rpi/execution-packet.json | plan-file.md | "description"]`:

**Checkpoint:** verify before dispatch that the slice is ready, its acceptance command is executable, and its write scope does not collide with another lane.

### Preflight (Recovery hooks → Step 3a.3)

Read [references/execution-preflight.md](references/execution-preflight.md) when you need recovery-hook setup, effort/tier mapping, knowledge-context loading (Step 0), tracking-mode detection (0.5), gc-pool detection (0.6), epic identification (Step 1), branch isolation (1.5), wave-counter / mutation-trail / shared-task-notes initialization (1a–1a.2), test-first classification (1b), epic details (Step 2), ready-issue listing (Step 3), and the four pre-flight checks (3a, 3a.1 pre-mortem, 3a.2 bd-audit, 3a.3 changed-string grep).

The Branch Isolation Gate (Step 1.5) has its own dedicated contract — see [references/branch-isolation.md](references/branch-isolation.md) for when crank must create or refuse an isolation branch.

### Wave dispatch (Step 3b → Step 4)

Read [references/wave-dispatch.md](references/wave-dispatch.md) when you need SPEC WAVE / TEST WAVE / RED Gate flow (Steps 3b–3c), context-briefing assembly (3b.1), shared-notes injection (3b.2), parallel-wave isolation (3b.3), or Step 4 wave execution detail — GREEN mode, issue-typing + file manifests, grep-for-existing-functions, validation metadata policy, acceptance-criteria injection, language-standards injection, file-ownership table, wave-counter / 50-cap gate, spec-consistency gate, cross-cutting constraint injection, gc-pool dispatch, and cross-cutting validation.

### Wave completion (Step 5 → Step 8.7)

Read [references/wave-completion.md](references/wave-completion.md) when you need verify-and-sync (Step 5, external-gate protocol), wave acceptance check + CI-policy parity gate (5.5), wave checkpoint + per-criterion verdicts + back-compat fallback (5.7), validation-context checkpoint (5.7b), shared-task-notes harvest (5.7c), plan-mutation logging (5.7d), wave status report (5.8), worktree base-SHA refresh (5.9), check-for-more-work loop (Step 6), de-sloppify pass (6.5), pre-validation lifecycle checks (6.9), final batched validation (Step 7), phase-2 summary (Step 8), learnings extraction (8.5), shared-notes archive (8.6), and the scope-completion pre-close gate (8.7).

Step 5.5 includes the **CI-Policy Parity Gate**: if a wave diff touches `.github/workflows/*.yml`, run `bash scripts/validate-ci-policy-parity.sh`; any non-zero exit fails wave acceptance and surfaces the generated drift report. See [references/wave-patterns.md](references/wave-patterns.md) "CI-Policy Parity Gate" for the worked example and trigger pattern.

### Step 9: Report Completion

Report the epic ID/title, issues completed, iterations used of 50, final validation, and flywheel status. End with exactly one completion marker: `DONE` only when all slices are accepted, `PARTIAL` while work remains, or `BLOCKED` with the surviving reason and issue count. The structured fields are defined below in Output Specification.

## Land Loop (per-bead, direct-main)

Crank lands each bead's slice to `main` **directly** — PR-per-bead is retired for THIS repo (external-repo variant at the end of this section). Land each bead from **its own worktree**, one at a time:

1. **Isolate.** The bead's slice is committed on HEAD in the bead's own `git worktree` (never the shared checkout), the HEAD message citing the bead id (the gate + pawl resolve the bead from it).
2. **Gate.** `ao gate check --fast --scope head` — the local cockpit gate (also the pre-push hook; run it manually to fail fast). Fix-forward stale/transient reds, never revert green work; regenerate drifted derived surfaces (`registry.json` / `contracts-sync` → `make regen-all`, scoped via `--skills` when only some skills changed) and commit them WITH the change.
3. **Review — the mutate-shared-trunk pawl.** `pawl-review` runs immutable fresh reviewer lanes and hands evidence to `ao pawl`; `ao pawl` alone applies diversity, commit binding, and admission ([docs/contracts/pawls.md](../../docs/contracts/pawls.md); [`$pawl-review`](../pawl-review/SKILL.md)). CONFIRMED requires all refuters confirmed, the selected diversity floor, real nonempty evidence, and a verdict bound to the live head. No CONFIRMED verdict means no land. REFUTED routes back to re-work; circuit-breaker exhaustion HOLDs.
4. **Land.** `bash scripts/pawl-land.sh <bead>` — fetch + rebase onto current `origin/main`, restamp the CONFIRMED verdict onto the post-rebase feat, single-shot `push origin HEAD:main`. Aborts without pushing on a rebase conflict (resolve locally, re-run pawl-review if the tree changed, re-land).
5. **Close on landed-only.** `ao beads exec close` a child bead ONLY after its commit is confirmed on trunk — `git fetch origin main && git merge-base --is-ancestor <feat-sha> origin/main` — never on a log line or a batch `br --json` query (those flake to null/0).
6. **Epic-close gate.** **NEVER close a parent epic before EVERY child bead's commit is confirmed an ancestor of `origin/main`** (re-check `git merge-base --is-ancestor` per child; each child `br` CLOSED). One unlanded child aborts the close. (Post-mortem governance checkpoint: hard gate, not advisory.)

### Close checkpoint — a closed bead is a sensor reading (age-cysr)

Not a checkbox ([the flywheel](../../docs/architecture/the-flywheel.md)): the close is the loop's highest-signal, membrane-verified evidence. On EVERY close (step 5) answer two questions before moving on:
1. **What did completing this bead teach?** (one line — usually "nothing new", and that's fine)
2. **Does it CONTRADICT an assumption the remaining plan depends on?**

If **no** → record `no_change` evidence. If **yes** → record the falsified
assumption and its evidence. In both cases, return the evidence through
Validate and Learn; Crank does not invoke Discovery or mutate the remaining
plan. The orchestrator alone decides whether a material Learn packet warrants
re-planning and Premortem.

**Multi-lane serialization + by-hand land.** When several lanes land onto a hot `main` at once, or when you land by hand via the `ao pawl review` CLI (which sets `PAWL_UNTRUSTED_REPO=1` and SKIPS auto-bind, so the sealed bind is manual), follow the serialized land-token discipline and the exact `[feat, #trivial-bind]` command sequence in [references/land-protocol.md](references/land-protocol.md) — one land at a time across lanes, `ao provenance emit-verdict` for the sealed bind (never a hand-appended ledger edge), and `git merge-base --is-ancestor` before every `ao beads exec close`.

> Enforce steps 3–4 with the committed scripts, not by hand: `scripts/pawl-review.sh <bead> --scope head --author-family <family>` (runs the refuter; on CONFIRMED writes + verifies the commit-bound verdict via `scripts/pawl-verdict.sh`, REFUTED exit 3 prints the defects) then `scripts/pawl-land.sh <bead>` (rebase → restamp → single-shot push). The epic-close gate is `scripts/check-epic-children-closed.sh <epic>` (no-epic-close-with-open-child). All are hermetic-tested under `tests/scripts/`.

> **External-repo variant (PR flow).** When crank targets an **external repo** (an upstream fork where you cannot push `main`), the land half becomes a PR: prepare it with [`$pr-prep`](../pr-prep/SKILL.md), then reconcile mechanically with `scripts/reconcile-pr.sh <pr> <bead> [--epic <epic>]` (polls `gh pr checks`, reruns the lone correctness-ubuntu flake once, verifies the CONFIRMED pawl verdict via `scripts/pawl-verdict.sh check <bead> <pr> --head <live-sha>` — exit 5/HOLD if absent/REFUTED/ESCALATE/stale-head/no-evidence, merges `gh pr merge --squash --admin`, closes the bead only on confirmed `MERGED`). This path is for **external targets only** — never for landing AgentOps' own beads.

## The FIRE Loop

Crank runs FIRE (Find → Ignite → Reap → Vibe → Escalate) for one wave. RPI may
invoke another Crank wave only after Validate, Learn, and an explicit
orchestrator decision. Read `references/wave-patterns.md` for the parallel-wave
and acceptance details.

## Key Rules

- Auto-detect tracking (`br` first, TaskList fallback) and use the provided epic or plan input directly.
- Use `$swarm` for the selected wave, preserve fresh per-issue context, and
  refuse to continue past unresolved conflicts or the 50-wave cap.
- Per-wave deterministic acceptance stays lightweight; the resulting wave
  evidence is handed to Validate rather than interpreted as a re-plan inside
  Crank.
- Load relevant prior evidence at the start, emit current evidence at the end,
  and always return `DONE`, `BLOCKED`, or `PARTIAL`.

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

**User says:** `$crank ag-m0r` — execute the next ready wave and return evidence.
**User says:** `$crank .agents/plans/auth-refactor.md` — execute the plan's next ready wave.
**User says:** `$crank --test-first ag-xj9` — SPEC → TEST → RED Gate → GREEN IMPL. See `references/test-first-mode.md`.

---

## Output Specification

- **Path:** committed slice changes plus wave evidence under `.agents/swarm/results/` and tracker state in the ledger resolved by `ao beads dir`.
- **Filename:** preserve each worker's declared result filename; the final response is emitted to stdout and does not invent a second evidence file.
- **Format:** markdown progress/closeout summary with epic ID/title, issue count, iterations, validation result, flywheel status, and per-slice [slice-validation](../../docs/templates/slice-validation.md) roll-ups.
- **Exit code:** run `bash skills/crank/scripts/validate.sh` and require zero; the semantic exit signal is `<promise>DONE</promise>` only when all slices are accepted, `PARTIAL` while work remains, or `BLOCKED` after bounded recovery.
- **Downstream handoff:** pass committed slices and their evidence to Validate.
  Validate hands an immutable verdict to Learn, which returns plan impact to
  the orchestrator; Crank does not choose that transition.

## Quality Checklist

- Every completed slice has an executable acceptance result, owned files, and tracker state consistent with its landed commit.
- Parallel waves contain no shared write or generated-surface collision, and sequential dependencies use the freshly landed prior base.
- The final marker matches reality: no `DONE` while issues or failed checks
  remain, and no cross-phase retry is hidden inside Crank.

## Troubleshooting

Common failure modes: no ready issues, repeated wave gate failures, missing files from workers, bad RED-gate output, or TaskList/beads mismatches. See `references/troubleshooting.md` for fixes and command-level recovery steps.

---

## Inline Work Policy

Most `$crank` steps delegate worker execution via `$swarm` or `Skill()`. A small number of steps are **orchestrator-owned** by design — these are inline gates, scans, and bookkeeping that must stay in the orchestrator's context to make a downstream decision. Orchestrator-owned steps are marked with a `*(orchestrator-owned: …)*` admonition in the body (see STEP 3a.3, STEP 6.5 slop-scan, STEP 8.7).

**Do NOT convert orchestrator-owned steps into `Skill()` or `$swarm` delegations** — they are intentionally inline. Every other step (SPEC wave, TEST wave, IMPL wave, validation, lifecycle checks) should delegate via the documented `Skill(...)` call or `$swarm` invocation.

If unsure whether a step is orchestrator-owned or delegatable, the default is **delegate**. Only steps marked with the admonition above are exempt.

Crank runs as an isolated phase-2 execution context — discovery and validation are sealed off from this skill. See [references/isolation-contract.md](references/isolation-contract.md) for the four-lever enforcement model and the compression patterns `scripts/check-skill-isolation.sh` flags. See [references/best-practices.md](references/best-practices.md) for the lifecycle principle + anti-pattern citation table (cite by number; do not duplicate body content).

## Related skills

- [`$agent-native`](../agent-native/SKILL.md) — portable persistent-worker lifecycle; use [`$ntm`](../ntm/SKILL.md) for NTM pane mechanics.

## Reference Documents

- [references/crank.feature](references/crank.feature) — executable wave and completion contract
- [references/execution-preflight.md](references/execution-preflight.md) and [references/wave-dispatch.md](references/wave-dispatch.md) — readiness and worker dispatch
- [references/wave-completion.md](references/wave-completion.md) and [references/wave-patterns.md](references/wave-patterns.md) — acceptance, synchronization, and FIRE
- [references/failure-recovery.md](references/failure-recovery.md) — bounded retry/decompose/prune operator
- [references/land-protocol.md](references/land-protocol.md) — serialized pawl bind/land and stale-head recovery
- [references/isolation-contract.md](references/isolation-contract.md) and [references/worker-specs.md](references/worker-specs.md) — context and ownership boundaries
- [references/test-first-mode.md](references/test-first-mode.md) and [references/troubleshooting.md](references/troubleshooting.md) — TDD waves and recovery lookup
- Supporting setup: [commit strategies](references/commit-strategies.md), [worktree isolation](references/worktree-per-worker.md), [parallel isolation](references/parallel-wave-isolation.md), [contract template](references/contract-template.md), and [runtime features](references/claude-code-latest-features.md).
- Wave evidence: [shared notes](references/shared-task-notes.md), [plan mutations](references/plan-mutations.md), [phase data](references/phase-data-contracts.md), [UAT integration](references/uat-integration-wave.md), and [spec consistency](references/wave1-spec-consistency-checklist.md).
- Recovery and gates: [failure taxonomy](references/failure-taxonomy.md), [external gate protocol](references/external-gate-protocol.md), [de-sloppify](references/de-sloppify.md), and [FIRE detail](references/fire.md).
- Specialized dispatch: [GC pool](references/gc-pool-dispatch.md), [task examples](references/taskcreate-examples.md), and [ship-loop anti-patterns](references/ship-loop-anti-patterns.md).
