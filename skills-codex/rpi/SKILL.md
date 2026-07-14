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

**Execute this workflow. Do not only describe it.** RPI is autonomous unless `--interactive` is set. The user touchpoint is after Learn returns control to the orchestrator or after a real blocked state exhausts retries. Read [autonomous-execution.md](references/autonomous-execution.md) for the full autonomy contract.

**`--auto` means *pivot autonomously*, NOT *execute the initial plan to the letter*.** Autonomy is agility, not waterfall: between waves the orchestrator re-plans the remaining work and changes course on its own — refactoring, adding, dropping, reordering waves as evidence arrives — without the operator saying so (touched only at the terminal objective or a circuit-breaker trip that survives its bounded helper pass). See [Agile Re-Plan Loop](#agile-re-plan-loop-the-anti-waterfall-rule).

## Critical Constraints

- `Validate -> Learn -> orchestrator` is the only legal post-execution transition because the immutable verdict must reach Learn before any plan or control decision. Learn is the only post-verdict handoff; Validate never jumps to Crank, Discovery, Premortem, retry, or delivery.
- Only the orchestrator may invoke Premortem, after it has accepted Learn and written the remaining-plan snapshot. Completion of the prior leaf is a real plan delta even when Learn reports `no_change`.
- Every admitted Crank wave with remaining work must end with exactly one bounded Premortem before another wave is requested.
- `no_change` is valid; the orchestrator may retry, continue, stop, or escalate without fabricating a lesson. A continue/retry still receives the bounded remaining-plan Premortem above.
- `terminal` closes the tick because no remaining work means no re-plan or Premortem.
- RPI ends at the four receipts and its report. It does not push Git refs, operate a Git queue, close tracker state through delivery, or require another LLM landing verdict. Repository-selected delivery is a separate adapter.
- Preserve one objective, acceptance surface, and evidence chain across every retry. **Why:** narrowing to a convenient child task can manufacture green while the requested behavior remains incomplete.
- Keep one active leaf per writer. Goal and epic parents are aggregate demand;
  they never occupy WIP, and the next leaf is not pulled until the current leaf
  is terminally reported.
- An initial introduced acceptance defect may receive one consolidated repair.
  Evidence of a second distinct repair need must be classified `REPLAN` and
  re-sliced through Discovery instead of starting another review loop.
- RPI owns the one persistent [run governor](references/pull-flow-governor.md);
  Crank and Validate request admission through it, and phases create no private
  state. `NOTE`, `REPAIR`, `REPLAN`, `HOLD`, and `ANDON` are the canonical
  dispositions; the reference owns meters, breakers, helpers, and hard ceilings.

## Loop position

`$rpi` is the orchestrator across **every move** of the [operating loop](../../docs/architecture/operating-loop.md): BDD intent → vertical slices → per-slice [narrow-waist micro-cycle](../../docs/architecture/operating-loop.md#the-narrow-waist-micro-cycle-canonical--every-loop-skill-cites-this) (**acceptance test RED → green → refactor-under-green**) → conflict-free wave → acceptance proof → Learn receipt → orchestrator decision. It delegates each move to the skill that owns it (`$discovery`, `$premortem`, `$crank`, `$validate`, `$learn`) and enforces these loop-level invariants:

- **Agile, not waterfall — the plan is a hypothesis.** Every nonterminal wave closes with one bounded Premortem of the remaining plan; a material or second-repair delta re-slices through Discovery (the [Agile Re-Plan Loop](#agile-re-plan-loop-the-anti-waterfall-rule), autonomous under `--auto`).
- **No move-skipping.** Intermediate slices use cheap deterministic checks; scoped or final Validate produces the independent verdict, then Learn records plan impact before the orchestrator selects another move.
- **The first failing test is the bead's contract.** With `--test-first` on (the default), `$crank` is invoked with the TDD-per-slice discipline; `--no-test-first` is an explicit opt-out, not a fast path. `$crank` runs **refactor-under-green as its own step after green** — the load-bearing quality move — and a refactor must never change a test (S4; test-first *ordering* alone is not the quality lever).
- **Acceptance examples close the bead, not activity.** Every validation verdict routes through Learn; only the orchestrator may choose to re-crank the same objective. DONE requires the acceptance roll-up in the [slice-validation template](../../docs/templates/slice-validation.md) to be fully green.
- **Ports stay visible.** Preserve the [Intent-to-Loop Hexagon](../../docs/architecture/intent-to-loop-hexagon.md) boundary as the objective crosses `shape_intent`, `persist_intent`, `plan_slices`, `execute_wave`, `validate_acceptance`, and `record_evidence`.
- **Context density survives phase boundaries.** Apply the [Context Density Rule](../domain/references/context-density-rule.md) to every phase handoff and final report: keep intent, boundary, evidence, decision, constraint, and next action; omit or link anything else.

### Folded triggers (ag-s43tg): `operating-loop-skill` + `operating-loop-workflow` route here

- **operating-loop-skill** — driving one bead end-to-end through claim, work, independent validation, closeout, and persistence: `$rpi <bead-id>` runs that exact arc.
- **operating-loop-workflow** — installing or running the seven-move operating-loop Workflow for AgentOps plugin users and multi-agent orchestration: `$rpi` is the in-session orchestrator of the same seven moves.

## Core Contract

RPI delegates via `Skill(skill="discovery", ...)`, `Skill(skill="crank", ...)`, `Skill(skill="validate", ...)`, and `Skill(skill="learn", ...)` as separate calls. Do not compress phases, replace phase skills with direct agent spawns, or skip validation. Read the [strict-delegation contract](../shared/references/strict-delegation-contract.md), [isolation contract](references/isolation-contract.md), and [best practices](references/best-practices.md).

When phase isolation exists, keep `$rpi` visible and pass phase skill name plus bounded handoff in, then artifact/verdict/next action out. The transport may be a process or subagent wrapper, but it must execute the declared phase contract rather than doing phase work directly.

RPI owns one lifecycle objective. Preserve the discovered `epic_id` or original goal and packet objective; a child bead or ready slice is context, not a replacement. `<promise>PARTIAL</promise>` from `$crank` is evidence for the orchestrator, not an automatic retry.

## Phase Receipt Contract

RPI cannot rely on memory or a final narrative to prove delegated skills ran. Every execution packet and phase summary MUST carry compact receipts: JSON `skills_loaded` + `phase_receipts` (canonical slugs, no sigils), plus a `## Skill Receipts` bullet list in each markdown phase summary. Receipts supplement transcript/runtime proof with a deterministic disk surface. Full schema and example: [phase-data-contracts.md](references/phase-data-contracts.md).

## Route And Classify

1. Create `.agents/rpi/`.
2. Resolve `--from`:
   - default, `research`, `plan`, `premortem`, `brainstorm` -> discovery
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

Track lifecycle state as `rpi_state`: `goal` (string), `epic_id` (null until discovered), `phase` (discovery|crank|validate|learn), `complexity` (fast|standard|full), `test_first` (true unless `--no-test-first`), `run_id`, and `verdicts` ({}). Admissions, charges, breaker state, and helper use live only in the persistent run governor.

## Phase DAG

Enter at the routed phase and run every phase after it.

1. **Discovery:** invoke `$discovery <goal> [--interactive] --complexity=<level>`. On DONE, read the current or archived execution packet and preserve its objective spine; on BLOCKED, return evidence without treating the label as a retry decision.
2. **Crank:** after the governor durably records `authorized:true`, invoke `$crank <epic-id>` or `$crank .agents/rpi/execution-packet.json`; pass the test-first choice through. Every completion marker returns evidence to the orchestrator, which reads the actual diff for scope and claim match under [Wave Acceptance](../crank/references/wave-patterns.md#wave-acceptance-check).
3. **Validate:** after a durably authorized `semantic-review` charge, invoke `$validate <epic-id> --complexity=<level>` or standalone `$validate --complexity=<level>`; add `--strict-surfaces` with `--quality`. Preserve its immutable verdict for Learn; Validate owns no retry, plan mutation, Premortem, or budget.
4. **Learn:** invoke `$learn` with that verdict and evidence, record `.agents/rpi/phase-4-summary.md`, and consume only its `remaining_work` and `plan_impact`; Learn cannot change the verdict, plan, or delivery state.
5. **Orchestrator decision:** with remaining work, write the current remaining-plan snapshot. `material_change` or a second distinct repair need routes through bounded Discovery for re-slicing; `no_change` requires an explicit continue/retry/stop/escalate decision. Before any next Crank admission, invoke exactly one bounded Premortem on that snapshot. `terminal` proceeds to Report. No phase bypasses `Validate -> Learn -> orchestrator`.
6. **Report:** use [references/report-template.md](references/report-template.md), apply any loop disposition only after the governor records the next admission or protected stop, and only suggest `next-work.jsonl` entries. Apply the Context Density Rule to every line.

## Orchestrator Decision State Machine

The orchestrator, not Validate or Learn, owns retry and re-plan decisions.
Every verdict first becomes a Learn receipt; its plan impact selects the branch
above. The [persistent pull-flow governor](references/pull-flow-governor.md)
canonically defines every admission, disposition, breaker, helper, hard-ceiling,
and protected-state transition used by that decision.

## Agile Re-Plan Loop (the anti-waterfall rule)

The initial plan is a **hypothesis**; each wave is an experiment. Its evidence
flows through `Validate -> Learn -> orchestrator`. Learn reports whether the
remaining plan has a material impact; it does not apply one. When the impact is
material, the orchestrator invokes Discovery to change the remaining plan.
With `no_change`, it makes an explicit continue/retry/stop/escalate decision.
Either nonterminal branch writes the remaining-plan snapshot and runs one
bounded Premortem before the next Crank wave. With `terminal`, it closes the tick. Anti-patterns: **waterfall**,
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
| `--loop` | off | Repeat only after an explicit orchestrator decision and a recorded governor admission |
| `--run-id=<id>` | required for dispatch | Resume the persistent run state across invocations |
| `--max-waves=<n>` | 3 | Declare the run-wide Crank admission ceiling at initialization |
| `--max-reviewer-tokens=<n>` | required | Declare the hard reviewer-token ceiling |
| `--max-elapsed-seconds=<n>` | required | Declare the hard elapsed-time ceiling |
| `--max-review-contexts=<n>` | required | Declare the hard review-context ceiling |
| `--max-deterministic-executions=<n>` | required | Declare the hard deterministic-execution ceiling |
| `--spawn-next` | off | Surface follow-up work after reporting |
| `--test-first` / `--no-test-first` | on / off | Enable or explicitly opt out of TDD ordering |
| `--fast-path` / `--deep` | auto | Force fast or full complexity |
| `--quality` | off | Make validation strict surfaces blocking |
| `--dry-run` | off | Report only; never creates an admission receipt |

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
**Exit signal:** the per-phase verdict roll-up; `<promise>PARTIAL</promise>` from `$crank` requires a canonical orchestrator disposition and never implies a retry.

## Quality Checklist

- [ ] The same objective and acceptance examples survive every phase and retry.
- [ ] Each phase has a disk-backed receipt, evidence path, and explicit verdict.
- [ ] Every verdict routes through Learn before the orchestrator decides the next action.
- [ ] Premortem receives only an orchestrator-owned remaining-plan snapshot after Learn while work remains.
- [ ] The execution packet passes its validator before Report or downstream handoff.
- [ ] Every dispatch has a durable admission in the same run state.
- [ ] No phase-local wave, retry, attempt, cost, or helper counter exists.

## Troubleshooting

| Problem | Response |
|---------|----------|
| Phase returns BLOCKED | Classify through the governor; ordinary repair stays autonomous, while a real breaker enters HOLD |
| Packet validation fails | Repair the packet or receipts, then rerun the validator before handoff |
| External executor fails | Use direct local checks; raise a breaker only for a reproducible capability stop |

## Related skills

- [`$agent-native`](../agent-native/SKILL.md) + [`$ntm`](../ntm/SKILL.md) — portable out-of-session workers and NTM pane mechanics for whole `$rpi` loops.

## Reference Documents

- Core loop: [agile re-plan](references/agile-replan-loop.md), [executable feature](references/rpi.feature), [compression anti-pattern](references/orchestrator-compression-anti-pattern.md), [installed-version warning](references/installed-plugin-version-not-repo-head.md).
- Modes: [context windowing](references/context-windowing.md), [discovery artifact](references/discovery-artifact-mode.md), [persistent pull-flow governor](references/pull-flow-governor.md), [examples](references/examples.md), and the [phase-budget migration](references/phase-budgets.md) into that sole governor.
- Recovery: [error handling](references/error-handling.md), [gate retry](references/gate-retry-logic.md), [loop/spawn](references/gate4-loop-and-spawn.md), [troubleshooting](references/troubleshooting.md), [Codex executor](references/codex-executor.md).
- Contracts: [autonomous execution](references/autonomous-execution.md), [phase data](references/phase-data-contracts.md), [report template](references/report-template.md).
