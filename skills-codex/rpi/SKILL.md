---
name: rpi
description: Run Discovery, Crank, Validate, and Learn as
---
# $rpi - Full Lifecycle Orchestrator

## Codex Lifecycle Guard

When this skill runs in Codex hookless mode (`CODEX_THREAD_ID` is set or
`CODEX_INTERNAL_ORIGINATOR_OVERRIDE` is `Codex Desktop`), run:

```bash
ao codex ensure-start 2>/dev/null || true
```

The CLI records startup once per thread and skips duplicates automatically.

> Quick ref: `$discovery` -> `$crank` -> `$validate` -> `$learn`, then report.

**Execute this workflow. Do not only describe it.** RPI is autonomous unless `--interactive` is set. The user touchpoint is after Learn returns control to the orchestrator or after a real
blocked state exhausts retries. Read [autonomous-execution.md](references/autonomous-execution.md)
when you need the full autonomy contract.

**`--auto` means *pivot autonomously*, NOT *execute the initial plan to the letter*.** Autonomy is agility, not waterfall: between waves the orchestrator re-plans the remaining work and changes course on its own — refactoring, adding, dropping, reordering waves as evidence arrives — without the operator saying so (touched only at the terminal objective or a circuit-breaker trip that survives its bounded helper pass). See [Agile Re-Plan Loop](#agile-re-plan-loop-the-anti-waterfall-rule).

## Critical Constraints

- `Validate -> Learn -> orchestrator` is the only legal post-execution
  transition. Learn is the only post-verdict handoff; Validate never jumps to
  Crank, Discovery, Premortem, retry, or delivery.
- Only the orchestrator may invoke Premortem, and only after it has accepted a
  material Learn result, changed the remaining plan, and still has work to do.
- `no_change` is a valid result. The orchestrator may retry, continue, stop, or
  escalate without fabricating a lesson or plan mutation.
- `terminal` closes the tick. No remaining work means no re-plan and no
  Premortem.
- RPI ends at the four receipts and its report. It does not push Git refs,
  operate a Git queue, close tracker state through delivery, or require another
  LLM landing verdict. Repository-selected delivery is a separate adapter.
- Preserve one objective, acceptance surface, and evidence chain across every retry. **Why:** narrowing to a convenient child task can manufacture green while the requested behavior remains incomplete.

## Loop position

`$rpi` is the orchestrator across **every move** of the [operating loop](../../docs/architecture/operating-loop.md): BDD intent → vertical slices → per-slice [narrow-waist micro-cycle](../../docs/architecture/operating-loop.md#the-narrow-waist-micro-cycle-canonical--every-loop-skill-cites-this) (**acceptance test RED → green → refactor-under-green**) → conflict-free wave → acceptance proof → Learn receipt → orchestrator decision. It delegates each move to the skill that owns it (`$discovery`, `$premortem`, `$crank`, `$validate`, `$learn`) and enforces these loop-level invariants:

- **Agile, not waterfall — the plan is a hypothesis.** Every wave closes with a **re-plan, not just a retry** (the [Agile Re-Plan Loop](#agile-re-plan-loop-the-anti-waterfall-rule), autonomous under `--auto`).
- **No move-skipping.** Intermediate slices use cheap deterministic checks;
  scoped or final Validate produces the independent verdict, then Learn records
  plan impact before the orchestrator selects another move.
- **The first failing test is the bead's contract.** With `--test-first` on (the default), `$crank` is invoked with the TDD-per-slice discipline; `--no-test-first` is an explicit opt-out, not a fast path. `$crank` runs **refactor-under-green as its own step after green** — the load-bearing quality move — and a refactor must never change a test (S4; test-first *ordering* alone is not the quality lever).
- **Acceptance examples close the bead, not activity.** Every validation
  verdict routes through Learn; only the orchestrator may choose to re-crank
  the same objective. DONE requires the acceptance roll-up in the
  [slice-validation template](../../docs/templates/slice-validation.md) to be
  fully green.
- **Ports stay visible.** Preserve the [Intent-to-Loop Hexagon](../../docs/architecture/intent-to-loop-hexagon.md) boundary as the objective crosses `shape_intent`, `persist_intent`, `plan_slices`, `execute_wave`, `validate_acceptance`, and `record_evidence`.
- **Context density survives phase boundaries.** Apply the [Context Density Rule](../domain/references/context-density-rule.md) to every phase handoff and final report: keep intent, boundary, evidence, decision, constraint, and next action; omit or link anything else.

### Folded triggers (ag-s43tg): `operating-loop-skill` + `operating-loop-workflow` route here

- **operating-loop-skill** — driving one bead end-to-end through claim, work, independent validation, closeout, and persistence: `$rpi <bead-id>` runs that exact arc.
- **operating-loop-workflow** — installing or running the seven-move operating-loop Workflow for AgentOps plugin users and multi-agent orchestration: `$rpi` is the in-session orchestrator of the same seven moves.

## Core Contract

RPI delegates via `Skill(skill="discovery", ...)`, `Skill(skill="crank", ...)`,
`Skill(skill="validate", ...)`, and `Skill(skill="learn", ...)` as separate calls.
Do not compress phases, replace phase skills with direct agent spawns, or skip validation. Read the [strict-delegation contract](../shared/references/strict-delegation-contract.md),
[isolation contract](references/isolation-contract.md), and [best practices](references/best-practices.md).

When phase isolation exists, keep `$rpi` visible and pass phase skill name plus bounded handoff in, then artifact/verdict/next action out.
The transport may be a process or subagent wrapper, but it must execute the declared phase contract rather than doing phase work directly.

RPI owns one lifecycle objective. Preserve the discovered `epic_id` or original goal and packet objective; a child bead or ready slice is context, not a replacement.
`<promise>PARTIAL</promise>` from `$crank` means retry Phase 2 on the same objective.

## Phase Receipt Contract

RPI cannot rely on memory or a final narrative to prove delegated skills ran.
Every execution packet and phase summary MUST carry compact receipts — JSON
`skills_loaded` + `phase_receipts` (canonical slugs, no sigils) and a
`## Skill Receipts` bullet list in each markdown phase summary. Receipts do not
replace transcript/runtime proof; they make delegation auditable from disk when
the transcript is unavailable and give downstream proof consumers a
deterministic surface to reject missing phase execution. Full schema + example
(the phase-receipt rule + fields): [references/phase-data-contracts.md](references/phase-data-contracts.md).

## Route And Classify

1. Create `.agents/rpi/`.
2. Resolve `--from`:
   - default, `research`, `plan`, `pre-mortem`, `brainstorm` -> discovery
   - `implementation` or `crank` -> implementation
   - `validation` or `vibe` -> validation
   - `learn` or `postmortem` -> learn
3. If the input is a bead and `--from` is absent, resolve it with `ao beads exec show`:
   - epic -> implementation with that epic
   - child with parent -> implementation with the parent epic
4. Classify complexity:
   - `fast`: short/simple goal or `--fast-path`
   - `standard`: medium goal or one scope keyword
   - `full`: `--deep`, complex-operation keyword, 2+ scope keywords, or >120 chars
5. Log `RPI mode: rpi-phased (complexity: <level>)`.

Track state compactly as `rpi_state`: `goal` (string), `epic_id` (null until
discovered), `phase` (discovery|crank|validate|learn), `complexity`
(fast|standard|full), `test_first` (true unless `--no-test-first`), `cycle`
(from 1), and `verdicts` ({}).

## Phase DAG

Enter at the routed phase and run every phase after it.

1. **Discovery:** invoke `$discovery <goal> [--interactive] --complexity=<level>`
   directly or through phase-isolated skill transport.
   On DONE, read `.agents/rpi/execution-packet.json` or the run archive and
   preserve its objective spine. On BLOCKED, return the evidence to the
   orchestrator; never stop or retry on the label alone.
2. **Crank:** invoke `$crank <epic-id>` when the packet has `epic_id`;
   otherwise invoke `$crank .agents/rpi/execution-packet.json`, directly or
   through phase-isolated skill transport. Pass `--test-first` or
   `--no-test-first` through. On DONE, record `ao ratchet record implement
   2>/dev/null || true` and continue. On PARTIAL, auto-redo the same objective;
   on BLOCKED, classify it through bounded recovery. Use 3 total attempts before
   `EXHAUSTED-BUDGET`. **Before accepting a slice/wave the orchestrator reads the actual diff itself** (scope + claim match) — not just the `<promise>DONE</promise>` and evidence JSON, but its own diff-read, distinct from the delegated sub-judges.
   `$crank` enforces this as the anti-green-washing Step 3.5 of its Wave Acceptance ([crank wave-patterns.md §Wave Acceptance Check](../crank/references/wave-patterns.md)).
3. **Validate:** invoke `$validate <epic-id> --complexity=<level>` when an
   epic exists; otherwise invoke `$validate --complexity=<level>`, directly
   or through phase-isolated skill transport. Add `--strict-surfaces` when
   `--quality` is set. Preserve the immutable PASS/WARN/FAIL verdict and hand
   it to Learn regardless of value. Validate does not retry, mutate the plan,
   or invoke Premortem.
4. **Learn:** invoke `$learn` with the immutable Validate verdict and its
   evidence reference. Record a `learn` receipt with status `DONE`, `PARTIAL`,
   or `BLOCKED` and a file-backed `.agents/rpi/phase-4-summary.md`. Learn binds
   observations to the verdict and emits `remaining_work` plus a `plan_impact`
   disposition. It cannot change the verdict, mutate the plan, invoke
   Premortem, or operate delivery state.
5. **Orchestrator decision (the loop's hinge).** Consume the Learn receipt:
   - remaining work + `material_change`: invoke Discovery for a bounded
     re-plan, persist the changed plan, then invoke Premortem on that exact
     changed plan before Crank continues;
   - remaining work + `no_change`: explicitly retry, continue, stop, or
     escalate without inventing a plan delta;
   - no remaining work + `terminal`: close the tick and proceed to Report.
   This preserves the legal `Validate -> Learn -> orchestrator` sequence and
   prevents direct `validate -> premortem` or `learn -> premortem` routing.
6. **Report:** summarize phase verdicts, the re-plan deltas taken, and epic
   status using [references/report-template.md](references/report-template.md).
   With `--loop`, apply the Learn disposition while `cycle < max_cycles`. With
   `--spawn-next`, read `.agents/rpi/next-work.jsonl` and suggest the next
   command without invoking it. Before emitting the report, apply the Context
   Density Rule: every line should carry intent, boundary, evidence, decision,
   constraint, or next action.

## Orchestrator Decision State Machine

The orchestrator, not Validate or Learn, owns retry and re-plan decisions.
Every verdict first becomes a Learn receipt. A material plan impact with work
remaining routes to Discovery, then the changed plan through Premortem. A
`no_change` result makes the next action explicit without manufacturing a
learning. A `terminal` result closes the tick without Premortem.

## Agile Re-Plan Loop (the anti-waterfall rule)

The initial plan is a **hypothesis**; each wave is an experiment. Its evidence
flows through `Validate -> Learn -> orchestrator`. Learn reports whether the
remaining plan has a material impact; it does not apply one. When the impact is
material, the orchestrator invokes Discovery to change the remaining plan and
sends that changed plan through Premortem before the next Crank wave. With
`no_change`, the orchestrator makes an explicit continue/retry/stop/escalate
decision. With `terminal`, it closes the tick. Anti-patterns: **waterfall**,
**retry-not-replan**, **validate-to-premortem**, and **permission-seeking**.
**Full detail:** [references/agile-replan-loop.md](references/agile-replan-loop.md).

## Phase Data Contract

The execution packet carries the repo execution profile through
`contract_surfaces`, `done_criteria`, and queue claim/finalize metadata. Keep
the latest alias at `.agents/rpi/execution-packet.json` and read
[references/phase-data-contracts.md](references/phase-data-contracts.md) for
schemas and archive paths.

## Complexity-Scaled Review

Complexity scales the depth of Premortem and Validate, never the phase order.
Routine work defaults to one fresh independent validator; deep or mixed review
is explicit. Learn remains bounded bookkeeping at every depth. Delivery policy
belongs to the target repository, outside this lifecycle. Read
[references/complexity-scaling.md](references/complexity-scaling.md).

## Flags

| Flag | Default | Purpose |
|------|---------|---------|
| `--from=<phase>` | discovery | Start at discovery, implementation, or validation |
| `--interactive` | off | Human gates in discovery/validate |
| `--auto` | on | Fully autonomous default — **pivots between waves on its own** (re-plans remaining work; not a fixed-plan/waterfall executor). See [Agile Re-Plan Loop](#agile-re-plan-loop-the-anti-waterfall-rule) |
| `--loop --max-cycles=<n>` | off / 3 | Repeat only after an explicit orchestrator decision |
| `--spawn-next` | off | Surface follow-up work after reporting |
| `--test-first` / `--no-test-first` | on / off | Enable or explicitly opt out of TDD ordering |
| `--fast-path` / `--deep` | auto | Force fast or full complexity |
| `--quality` | off | Make validation strict surfaces blocking |
| `--dry-run` / `--no-budget` | off | Report only, or disable phase budgets |

## Examples

- `$rpi "add user authentication"` — discovery → implementation → validation → report.
- `$rpi --from=implementation ag-23k` — resolve the bead scope, run implementation + validation.
- `$rpi --deep "refactor payment module"` — full council gates across the lifecycle.

Read [references/examples.md](references/examples.md) for resume, interactive, loop, and artifact-mode examples.

## Output Specification

**Artifact directory:** `.agents/rpi/`.
**Filename convention:** mutable `execution-packet.json`, immutable `runs/<run-id>/execution-packet.json`, `phase-<n>-summary.md`, and optional `next-work.jsonl`.
**Serialization/schema format:** packet JSON matches `schemas/execution-packet.schema.json` plus the `skills_loaded`/`phase_receipts` extension in [phase-data-contracts](references/phase-data-contracts.md); summaries follow the markdown [report template](references/report-template.md).
**Validator command:** `python3 skills/rpi/scripts/validate-execution-packet.py .agents/rpi/execution-packet.json`.
**Downstream handoff:** discovery creates the packet, crank updates
implementation evidence, validate appends the immutable acceptance verdict,
Learn records post-verdict observations plus plan impact, the orchestrator owns
any plan mutation and Premortem transition, and Report emits the human-readable
roll-up.
**Exit signal:** the per-phase verdict roll-up; `<promise>PARTIAL</promise>` from `$crank` means retry Phase 2 on the same objective.

## Quality Checklist

- [ ] The same objective and acceptance examples survive every phase and retry.
- [ ] Each phase has a disk-backed receipt, evidence path, and explicit verdict.
- [ ] Every verdict routes through Learn before the orchestrator decides the next action.
- [ ] Premortem receives only an orchestrator-owned changed plan while work remains.
- [ ] The execution packet passes its validator before Report or downstream handoff.

## Troubleshooting

| Problem | Response |
|---------|----------|
| Phase returns BLOCKED | Return evidence to the orchestrator; stop only on an explicit terminal transition |
| Packet validation fails | Repair the packet or receipts, then rerun the validator before handoff |
| External executor fails | Use direct local checks; raise a breaker only for a reproducible capability stop |

## Related skills

- [`$agent-native`](../agent-native/SKILL.md) + [`$ntm`](../ntm/SKILL.md) — portable out-of-session workers and NTM pane mechanics for whole `$rpi` loops.

## Reference Documents

- Core loop: [agile re-plan](references/agile-replan-loop.md), [executable feature](references/rpi.feature), [compression anti-pattern](references/orchestrator-compression-anti-pattern.md), [installed-version warning](references/installed-plugin-version-not-repo-head.md).
- Modes: [context windowing](references/context-windowing.md), [discovery artifact](references/discovery-artifact-mode.md), [phase budgets](references/phase-budgets.md), [examples](references/examples.md).
- Recovery: [error handling](references/error-handling.md), [gate retry](references/gate-retry-logic.md), [loop/spawn](references/gate4-loop-and-spawn.md), [troubleshooting](references/troubleshooting.md), [Codex executor](references/codex-executor.md).
- Contracts: [autonomous execution](references/autonomous-execution.md), [phase data](references/phase-data-contracts.md), [report template](references/report-template.md).
