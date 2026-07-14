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

> **Quick Ref:** Execute the next ready wave, run targeted acceptance once, and
> return canonical wave evidence to RPI's bounded tranche.

**YOU MUST EXECUTE THIS WORKFLOW. Do not just describe it.**

## Constraints

- Execute only tracker-ready vertical slices because crank consumes an accepted plan; it does not silently redefine intent.
- Parallelize only disjoint write scopes and serialize shared derived surfaces to prevent workers from invalidating one another's base.
- Return unresolved wave evidence to the orchestrator instead of choosing a
  cross-phase retry or re-plan inside Crank.
- Execute only the accepted leaf and wave selected by RPI. Crank owns no wave,
  retry, attempt, cost, disposition, or helper counter. It returns evidence to
  the orchestrator's [run disposition contract](../rpi/references/pull-flow-governor.md).

## Loop position

Move **5 (wave execution)** of the [operating loop](../../docs/architecture/operating-loop.md). Consumes the [slice validation plan](../../docs/templates/slice-validation.md) and uses one direct `$implement` worker for the routine wave. Each slice runs the canonical [narrow-waist micro-cycle](../../docs/architecture/operating-loop.md#the-narrow-waist-micro-cycle-canonical--every-loop-skill-cites-this): its acceptance test authored RED before code is the slice contract, and **refactor-under-green is its own wave, never optional** (`references/wave-patterns.md`) — a refactor wave must change no test. Admit `$swarm` only when the plan has at least two explicitly owned, disjoint write scopes; any shared migration, contract, CLI, or generated surface makes the work sequential. Parallelism is explicit ownership, not swarm chaos.

Under RPI, one Crank invocation ends at one accepted wave. PARTIAL means work
remains in the admitted bounded tranche and returns to the orchestrator for a
remaining-plan check. When plan inputs and risk are unchanged, the next wave may
run without Validate or Learn. The tranche freezes and pays semantic proof cost
once after at most three waves or 90 minutes.

**Feed the orchestrator's decision loop — do not swallow findings into a silent retry.** Crank hands canonical wave evidence to RPI and stops. Crank does not invoke Discovery, Learn, or Premortem; it also does not invoke Validate. RPI may admit another unchanged low-risk wave or close the tranche. Only after freeze does the orchestrator send the accumulated wave evidence to Validate, then Learn.

**CLI dependencies:** br (read tracker-ready work via `BEADS_DIR="$(ao beads dir)" br`), ao (knowledge flywheel). Both optional — see `skills/shared/SKILL.md` for fallback table. If br is unavailable, use TaskList for wave selection. If ao is unavailable, skip knowledge injection/extraction. Tracker terminal updates remain caller-owned.

For Claude runtime feature coverage (agents/hooks/worktree/settings), the shared source of truth is `skills/shared/references/claude-code-latest-features.md`, mirrored locally at `references/claude-code-latest-features.md`.

## Architecture: Crank + Swarm

Crank owns within-wave execution and evidence collection. RPI owns between-wave
transitions and the one proof transaction. One direct implementer is the routine
default. Swarm is admitted only when at least two disjoint lanes have explicit
owners and independent write scopes; it then owns runtime-native worker spawning,
fresh-context isolation, per-wave execution, and cleanup. In beads mode Crank gets the next wave from
`ao beads exec ready`, bridges issues into worker tasks, verifies results, and
returns proposed tracker changes without applying terminal updates. TaskList
mode uses the same one-wave boundary.

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

## Orchestrator boundary

RPI selects one accepted tranche and pulls at most three routine waves before a
soft return boundary. Crank executes the selected wave and returns targeted
facts. It never initializes lifecycle control state, meters work, or authorizes
the next wave.

## Completion Enforcement (The Sisyphus Rule)

Not done until you emit an explicit completion marker after each wave:
- `<promise>DONE</promise>` when the selected wave has accepted evidence
- `<promise>BLOCKED</promise>` when progress cannot continue
- `<promise>PARTIAL</promise>` when work remains

Never claim completion without one of these markers. The marker does not close
an epic, tracker, or delivery workflow.

## Failure Evidence

When a task fails during wave execution, report whether the evidence suggests
a transient repair, decomposition, or blocked path, but do not act on a private
counter. The orchestrator classifies it as `NOTE`, `REPAIR`, `REPLAN`, `HOLD`,
or `ANDON` through the shared disposition contract. `references/failure-recovery.md`
supplies evidence taxonomy only; any older numeric retry/helper text there is
non-authorizing.

**Mutation logging on failure classification:**
- **DECOMPOSE:** Log `task_removed` for the original task, then `task_added` for each new sub-task.
- **PRUNE:** Log `task_removed` with the block reason.
- **RETRY:** No mutation (task identity unchanged).

## Execution Steps

Given `$crank [epic-id | .agents/rpi/execution-packet.json | plan-file.md | "description"]`:

**Checkpoint:** verify before dispatch that the slice is ready, its acceptance command is executable, and its write scope does not collide with another lane.

### Preflight (Recovery hooks → Step 3a.3)

Read [references/execution-preflight.md](references/execution-preflight.md) for
the minimum readiness packet: exact leaf, bound plan/Premortem, failing proof,
write scope, isolation, test-first classification, and stable run ID.

The Branch Isolation Gate (Step 1.5) has its own dedicated contract — see [references/branch-isolation.md](references/branch-isolation.md) for when crank must create or refuse an isolation branch.

### Wave dispatch (Step 3b → Step 4)

Read [references/wave-dispatch.md](references/wave-dispatch.md) for the selected
wave identity, direct single-writer route, minimum worker metadata, test-first
flow, and the explicit disjoint-lane threshold for parallel dispatch.

### Wave completion (Step 5 → Step 8.7)

Read [references/wave-completion.md](references/wave-completion.md) when you need verify-and-sync (Step 5, external-gate protocol), wave acceptance check + CI-policy parity gate (5.5), wave checkpoint + per-criterion verdicts + back-compat fallback (5.7), validation-context checkpoint (5.7b), shared-task-notes harvest (5.7c), plan-mutation logging (5.7d), wave status report (5.8), worktree base-SHA refresh (5.9), check-for-more-work decision (Step 6), or the scope-completion gate (8.7). Intermediate waves skip broad pre-validation lifecycle suites and final validation; RPI runs those once after tranche freeze. Legacy phase-2 summaries are link-only projections of canonical wave evidence.

Step 5.5 includes the **CI-Policy Parity Gate**: if a wave diff touches `.github/workflows/*.yml`, run `bash scripts/validate-ci-policy-parity.sh`; any non-zero exit fails wave acceptance and surfaces the generated drift report. See [references/wave-patterns.md](references/wave-patterns.md) "CI-Policy Parity Gate" for the worked example and trigger pattern.

### Step 9: Report Completion

Report the epic ID/title, slices attempted, base and checkpoint identity,
acceptance evidence, and remaining work. End with exactly one completion marker:
`DONE` only when every selected slice is accepted, `PARTIAL` while selected or
later work remains, or `BLOCKED` with the surviving reason and issue count.

## Delivery boundary

Crank stops after it writes wave evidence and the phase-2 handoff. It does not
push, open or merge a PR, operate a Git queue, require a landing verdict, or
close tracker state as a side effect of delivery. The repository/operator may
later choose direct push, a PR, user-owned CI, or the optional deterministic
`$push` adapter. That decision is outside Crank and cannot change its evidence.

Wave acceptance still uses deterministic checks because implementation needs a
ground-truth handoff before Validate. Those checks prove the wave artifact; they
do not authorize or perform Git delivery.

## The FIRE Loop

Crank runs FIRE (Find → Ignite → Reap → Escalate → Return) for one wave. RPI may
invoke another selected wave after targeted acceptance and a remaining-plan
decision; Validate and Learn run once at the tranche boundary. Read `references/wave-patterns.md` for the parallel-wave
and acceptance details.

## Key Rules

- Auto-detect tracking (`br` first, TaskList fallback) and use the provided epic or plan input directly.
- Use the selected execution backend for the selected wave, preserve fresh
  per-issue context, and refuse to dispatch past unresolved conflicts or a
  dispatch-packet mismatch.
- Per-wave deterministic acceptance stays lightweight; the resulting wave
  evidence is handed to RPI, not directly to Validate and not interpreted as a
  re-plan inside Crank.
- Load relevant prior evidence at the start, emit current evidence at the end,
  and always return `DONE`, `BLOCKED`, or `PARTIAL`.

### Folded triggers (ag-s43tg wave 1): `burndown` + `ship-loop` route here

- **`burndown` → bounded epic mode.** Drive a finite epic set through accepted
  wave evidence, then stop. No new-work discovery or delivery ownership.
- **`ship-loop` → single-bead wave.** Claim, test, implement, and emit accepted
  evidence for one bounded issue. Repository-selected delivery and tracker
  closeout remain caller-owned after Crank returns.

### Verb Disambiguation for Worker Prompts

Read `references/worker-verb-disambiguation.md` for the verb clarification table. Ambiguous verbs (extract, remove, update, consolidate) cause workers to implement wrong operations — always use explicit instructions with `wc -l` assertions.

## Examples

**User says:** `$crank ag-m0r` — execute the next ready wave and return evidence.
**User says:** `$crank .agents/plans/auth-refactor.md` — execute the plan's next ready wave.
**User says:** `$crank --test-first ag-xj9` — SPEC → TEST → RED Gate → GREEN IMPL. See `references/test-first-mode.md`.

---

## Output Specification

- **Path:** slice changes plus canonical wave evidence under `.agents/swarm/results/`; tracker identifiers are read-only context in the handoff.
- **Filename:** preserve each worker's declared result filename; the final response is emitted to stdout and does not invent a second evidence file.
- **Format:** markdown progress/closeout summary with epic ID/title, issue count, iterations, validation result, flywheel status, and per-slice [slice-validation](../../docs/templates/slice-validation.md) roll-ups.
- **Exit code:** run `bash skills/crank/scripts/validate.sh` and require zero; the semantic exit signal is `<promise>DONE</promise>` only when all slices are accepted, `PARTIAL` while work remains, or `BLOCKED` after bounded recovery.
- **Downstream handoff:** pass wave evidence to RPI's bounded tranche. RPI alone
  decides whether to admit another unchanged wave or freeze once for Validate;
  Crank does not choose that transition.

## Quality Checklist

- Every completed slice has an executable acceptance result and owned files;
  Crank does not infer tracker or delivery completion from that evidence.
- Parallel waves contain no shared write or generated-surface collision, and
  sequential dependencies use the freshly integrated prior base.
- The final marker matches reality: no `DONE` while issues or failed checks
  remain, and no cross-phase retry is hidden inside Crank.

## Troubleshooting

Common failure modes: no ready issues, repeated wave gate failures, missing files from workers, bad RED-gate output, or TaskList/beads mismatches. See `references/troubleshooting.md` for fixes and command-level recovery steps.

---

## Execution ownership

The current leaf owner is the routine implementer. Crank keeps wave selection,
targeted acceptance, and remaining-plan facts in the visible
orchestrator context. It does not spawn a worker for bookkeeping or a Swarm for
one write scope. A fresh context is required later for semantic Validate, not
for every mechanical step. See [references/isolation-contract.md](references/isolation-contract.md)
for authority separation and [references/best-practices.md](references/best-practices.md)
for lifecycle principles.

## Related skills

- [`$agent-native`](../agent-native/SKILL.md) — portable persistent-worker lifecycle; use [`$ntm`](../ntm/SKILL.md) for NTM pane mechanics.

## Reference Documents

- [references/crank.feature](references/crank.feature) — executable wave and completion contract
- [references/execution-preflight.md](references/execution-preflight.md) and [references/wave-dispatch.md](references/wave-dispatch.md) — readiness and worker dispatch
- [references/wave-completion.md](references/wave-completion.md) and [references/wave-patterns.md](references/wave-patterns.md) — acceptance, synchronization, and FIRE
- [references/failure-recovery.md](references/failure-recovery.md) — bounded retry/decompose/prune operator
- [references/isolation-contract.md](references/isolation-contract.md) and [references/worker-specs.md](references/worker-specs.md) — context and ownership boundaries
- [references/test-first-mode.md](references/test-first-mode.md) and [references/troubleshooting.md](references/troubleshooting.md) — TDD waves and recovery lookup
- Supporting setup: [commit strategies](references/commit-strategies.md), [worktree isolation](references/worktree-per-worker.md), [parallel isolation](references/parallel-wave-isolation.md), [contract template](references/contract-template.md), and [runtime features](references/claude-code-latest-features.md).
- Wave evidence: [shared notes](references/shared-task-notes.md), [plan mutations](references/plan-mutations.md), [phase data](references/phase-data-contracts.md), [UAT integration](references/uat-integration-wave.md), and [spec consistency](references/wave1-spec-consistency-checklist.md).
- Recovery and gates: [failure taxonomy](references/failure-taxonomy.md), [external gate protocol](references/external-gate-protocol.md), [de-sloppify](references/de-sloppify.md), and [FIRE detail](references/fire.md).
- Specialized dispatch: [GC pool](references/gc-pool-dispatch.md), [task examples](references/taskcreate-examples.md), and [ship-loop anti-patterns](references/ship-loop-anti-patterns.md).
