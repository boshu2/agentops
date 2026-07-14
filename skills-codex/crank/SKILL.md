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
- Require a durable `authorized:true` admission from RPI's persistent
  [run governor](../rpi/references/pull-flow-governor.md) before dispatch.
  Crank owns no wave, retry, attempt, cost, disposition, or helper counter.

## Loop position

Move **5 (wave execution)** of the [operating loop](../../docs/architecture/operating-loop.md). Consumes the [slice validation plan](../../docs/templates/slice-validation.md); produces wave-by-wave slice completion via `$swarm` + `$implement`. Each slice runs the canonical [narrow-waist micro-cycle](../../docs/architecture/operating-loop.md#the-narrow-waist-micro-cycle-canonical--every-loop-skill-cites-this): its acceptance test authored RED before code is the slice contract, and **refactor-under-green is its own wave, never optional** (`references/wave-patterns.md`) — a refactor wave must change no test. Hard gate at wave start: every row of the wave-validity check must pass (distinct write scopes, no shared migration/contract/CLI surface, declared integration order, owner per slice, discard path per slice). Any failed row → run those slices sequential, not parallel. **Coupled-chain rule:** two slices that both regenerate a shared *derived* surface (`cli-command-surface` / `registry.json` / `context-map` / codex manifest) collide even with disjoint source files — run them as a sequential chain, each link based on the exact accepted prior link. Parallelism is explicit ownership, not swarm chaos.

Under RPI, one Crank invocation ends at one accepted wave. PARTIAL means work
remains and returns through Validate and Learn before another wave. Standalone
callers fulfill the same orchestrator contract rather than looping silently.

**Feed the orchestrator's decision loop — do not swallow findings into a silent retry.** Crank hands wave evidence to Validate and stops at its phase boundary. It does not invoke Discovery, Learn, or Premortem. Validate produces the immutable verdict, Learn classifies plan impact, and only the orchestrator may retry or change the remaining waves.

**CLI dependencies:** br (read tracker-ready work via `BEADS_DIR="$(ao beads dir)" br`), ao (knowledge flywheel). Both optional — see `skills/shared/SKILL.md` for fallback table. If br is unavailable, use TaskList for wave selection. If ao is unavailable, skip knowledge injection/extraction. Tracker terminal updates remain caller-owned.

For Claude runtime feature coverage (agents/hooks/worktree/settings), the shared source of truth is `skills/shared/references/claude-code-latest-features.md`, mirrored locally at `references/claude-code-latest-features.md`.

## Architecture: Crank + Swarm

Crank owns within-wave execution and evidence collection. RPI owns between-wave
transitions. Swarm owns runtime-native worker spawning, fresh-context isolation,
per-wave execution, and cleanup. In beads mode Crank gets the next wave from
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

## Shared Run Governor

RPI owns the only admissions and hard-cost governor. The default run admits
exactly three Crank waves, persisted across fresh invocations. Crank reports
measured usage and requests admission; it never initializes or resets the run.
Missing/corrupt state, missing meters, or a refused admission stops dispatch.

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
or `ANDON` through the shared governor. `references/failure-recovery.md`
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

Read [references/execution-preflight.md](references/execution-preflight.md) when you need recovery-hook setup, effort/tier mapping, knowledge-context loading (Step 0), tracking-mode detection (0.5), epic identification (Step 1), branch isolation (1.5), persistent-run admission / mutation-trail / shared-task-notes initialization (1a–1a.2), test-first classification (1b), epic details (Step 2), ready-issue listing (Step 3), and the four pre-flight checks (3a, 3a.1 premortem, 3a.2 bd-audit, 3a.3 changed-string grep).

The Branch Isolation Gate (Step 1.5) has its own dedicated contract — see [references/branch-isolation.md](references/branch-isolation.md) for when crank must create or refuse an isolation branch.

### Wave dispatch (Step 3b → Step 4)

Read [references/wave-dispatch.md](references/wave-dispatch.md) when you need SPEC WAVE / TEST WAVE / RED Gate flow (Steps 3b–3c), context-briefing assembly (3b.1), shared-notes injection (3b.2), parallel-wave isolation (3b.3), or Step 4 wave execution detail — GREEN mode, issue-typing + file manifests, grep-for-existing-functions, validation metadata policy, acceptance-criteria injection, language-standards injection, file-ownership table, atomic governor admission, spec-consistency gate, cross-cutting constraint injection, backend dispatch, and cross-cutting validation.

### Wave completion (Step 5 → Step 8.7)

Read [references/wave-completion.md](references/wave-completion.md) when you need verify-and-sync (Step 5, external-gate protocol), wave acceptance check + CI-policy parity gate (5.5), wave checkpoint + per-criterion verdicts + back-compat fallback (5.7), validation-context checkpoint (5.7b), shared-task-notes harvest (5.7c), plan-mutation logging (5.7d), wave status report (5.8), worktree base-SHA refresh (5.9), check-for-more-work loop (Step 6), de-sloppify pass (6.5), pre-validation lifecycle checks (6.9), final batched validation (Step 7), phase-2 summary (Step 8), learnings extraction (8.5), shared-notes archive (8.6), and the scope-completion pre-close gate (8.7).

Step 5.5 includes the **CI-Policy Parity Gate**: if a wave diff touches `.github/workflows/*.yml`, run `bash scripts/validate-ci-policy-parity.sh`; any non-zero exit fails wave acceptance and surfaces the generated drift report. See [references/wave-patterns.md](references/wave-patterns.md) "CI-Policy Parity Gate" for the worked example and trigger pattern.

### Step 9: Report Completion

Report the epic ID/title, slices attempted, run admission ID, persisted usage,
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
invoke another Crank wave only after Validate, Learn, and an explicit
orchestrator decision. Read `references/wave-patterns.md` for the parallel-wave
and acceptance details.

## Key Rules

- Auto-detect tracking (`br` first, TaskList fallback) and use the provided epic or plan input directly.
- Use the selected execution backend for the admitted wave, preserve fresh
  per-issue context, and refuse to dispatch past unresolved conflicts or a
  governor refusal.
- Per-wave deterministic acceptance stays lightweight; the resulting wave
  evidence is handed to Validate rather than interpreted as a re-plan inside
  Crank.
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

- **Path:** slice changes plus wave evidence under `.agents/swarm/results/`; tracker identifiers are read-only context in the handoff.
- **Filename:** preserve each worker's declared result filename; the final response is emitted to stdout and does not invent a second evidence file.
- **Format:** markdown progress/closeout summary with epic ID/title, issue count, iterations, validation result, flywheel status, and per-slice [slice-validation](../../docs/templates/slice-validation.md) roll-ups.
- **Exit code:** run `bash skills/crank/scripts/validate.sh` and require zero; the semantic exit signal is `<promise>DONE</promise>` only when all slices are accepted, `PARTIAL` while work remains, or `BLOCKED` after bounded recovery.
- **Downstream handoff:** pass slice changes and their evidence to Validate.
  Validate hands an immutable verdict to Learn, which returns plan impact to
  the orchestrator; Crank does not choose that transition.

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
- [references/isolation-contract.md](references/isolation-contract.md) and [references/worker-specs.md](references/worker-specs.md) — context and ownership boundaries
- [references/test-first-mode.md](references/test-first-mode.md) and [references/troubleshooting.md](references/troubleshooting.md) — TDD waves and recovery lookup
- Supporting setup: [commit strategies](references/commit-strategies.md), [worktree isolation](references/worktree-per-worker.md), [parallel isolation](references/parallel-wave-isolation.md), [contract template](references/contract-template.md), and [runtime features](references/claude-code-latest-features.md).
- Wave evidence: [shared notes](references/shared-task-notes.md), [plan mutations](references/plan-mutations.md), [phase data](references/phase-data-contracts.md), [UAT integration](references/uat-integration-wave.md), and [spec consistency](references/wave1-spec-consistency-checklist.md).
- Recovery and gates: [failure taxonomy](references/failure-taxonomy.md), [external gate protocol](references/external-gate-protocol.md), [de-sloppify](references/de-sloppify.md), and [FIRE detail](references/fire.md).
- Specialized dispatch: [GC pool](references/gc-pool-dispatch.md), [task examples](references/taskcreate-examples.md), and [ship-loop anti-patterns](references/ship-loop-anti-patterns.md).
