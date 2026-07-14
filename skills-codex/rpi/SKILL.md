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

> Quick ref: `$discovery` + `$premortem` -> `$crank` for a bounded tranche -> one `$validate` -> one `$learn`, then report.

**Execute this workflow. Do not only describe it.** RPI is autonomous unless `--interactive` is set. The user touchpoint is after Learn returns control to the orchestrator or after a real blocked state exhausts retries. Read [autonomous-execution.md](references/autonomous-execution.md) for the full autonomy contract.

**`--auto` means *pivot autonomously*, NOT *execute the initial plan to the letter*.** Autonomy is agility, not waterfall: between waves the orchestrator re-plans the remaining work and changes course on its own — refactoring, adding, dropping, reordering waves as evidence arrives — without the operator saying so (touched only at the terminal objective or a circuit-breaker trip that survives its bounded helper pass). See [Agile Re-Plan Loop](#agile-re-plan-loop-the-anti-waterfall-rule).

## Critical Constraints

- `Validate -> Learn -> orchestrator` is the only legal post-execution transition because the immutable verdict must reach Learn before any plan or control decision. Learn is the only post-verdict handoff; Validate never jumps to Crank, Discovery, Premortem, retry, or delivery.
- Only the orchestrator may invoke Premortem. One verdict binds the tranche plan,
  acceptance, dependency shape, write scope, and risk class; reuse it while those
  inputs remain unchanged. A changed input receives one bounded fresh-context
  Premortem before another wave is admitted.
- A routine tranche contains one to three sequential low-risk waves, one active
  mutation at a time, in one bounded context. Intermediate waves run targeted
  deterministic checks and return evidence to the orchestrator; **no per-wave
  Validate, Learn, delivery, or duplicate summary is required**.
- `no_change` is valid; the orchestrator may continue the admitted tranche
  without fabricating a lesson. Material scope, risk, dependency, or acceptance
  change is `REPLAN`, not another review round.
- `terminal` closes the tick because no remaining work means no re-plan or Premortem.
- RPI ends at the four receipts and its report. It does not push Git refs, operate a Git queue, close tracker state through delivery, or require another LLM landing verdict. Repository-selected delivery is a separate adapter.
- Preserve one objective, acceptance surface, and evidence chain across every retry. **Why:** narrowing to a convenient child task can manufacture green while the requested behavior remains incomplete.
- Keep one active leaf per writer. Goal and epic parents are aggregate demand;
  they never occupy WIP. The leaf is the bounded tranche and may take one to
  three implementation waves, but no second leaf is pulled until the current
  one is terminally reported.
- An initial introduced acceptance defect may receive one consolidated repair.
  Evidence of a second distinct repair need must be classified `REPLAN` and
  re-sliced through Discovery instead of starting another review loop.
- RPI owns the one [run disposition contract](references/pull-flow-governor.md),
  not an execution controller. `NOTE`, `REPAIR`, `REPLAN`, `HOLD`, and `ANDON`
  are the canonical dispositions. Crank and Validate return evidence; they do
  not reserve work, maintain counters, grant helpers, or authorize dispatch.

## Loop position

`$rpi` is the orchestrator across **every move** of the [operating loop](../../docs/architecture/operating-loop.md): BDD intent → vertical slices → per-slice [narrow-waist micro-cycle](../../docs/architecture/operating-loop.md#the-narrow-waist-micro-cycle-canonical--every-loop-skill-cites-this) (**acceptance test RED → green → refactor-under-green**) → one-to-three-wave bounded tranche → one frozen-candidate proof → Learn receipt → orchestrator decision. It delegates each move to the skill that owns it (`$discovery`, `$premortem`, `$crank`, `$validate`, `$learn`) and enforces these loop-level invariants:

- **Agile, not waterfall — the plan is a hypothesis.** Intermediate wave evidence may reorder or narrow the remaining admitted tranche. Reuse the existing Premortem when its bound inputs are unchanged; a material or second-repair delta re-slices through Discovery and receives a new Premortem.
- **One proof transaction.** Intermediate slices use cheap deterministic checks. After the bounded tranche is complete, freeze once, run one fresh independent Validate, run one Learn bookkeeping pass, and report. Validate and Learn never sit between unchanged low-risk waves.
- **The first failing test is the bead's contract.** With `--test-first` on (the default), `$crank` is invoked with the TDD-per-slice discipline; `--no-test-first` is an explicit opt-out, not a fast path. `$crank` runs **refactor-under-green as its own step after green** — the load-bearing quality move — and a refactor must never change a test (S4; test-first *ordering* alone is not the quality lever).
- **Acceptance examples close the bead, not activity.** Every validation verdict routes through Learn; only the orchestrator may choose to re-crank the same objective. DONE requires the acceptance roll-up in the [slice-validation template](../../docs/templates/slice-validation.md) to be fully green.
- **Ports stay visible.** Preserve the [Intent-to-Loop Hexagon](../../docs/architecture/intent-to-loop-hexagon.md) boundary as the objective crosses `shape_intent`, `persist_intent`, `plan_slices`, `execute_wave`, `validate_acceptance`, and `record_evidence`.
- **Context density survives phase boundaries.** Apply the [Context Density Rule](../domain/references/context-density-rule.md) to every phase handoff and final report: keep intent, boundary, evidence, decision, constraint, and next action; omit or link anything else.

## Core Contract

RPI preserves four typed responsibilities: Discovery shapes the tranche, Crank
executes admitted waves, Validate independently judges the frozen tranche, and
Learn bookkeeps the immutable verdict. Runtime-native skill calls or thin phase
runners may carry those responsibilities, but only semantic judgment requires a
fresh independent context. Do not replace a typed responsibility with ad hoc
work or skip validation. Read the [strict-delegation contract](../shared/references/strict-delegation-contract.md), [isolation contract](references/isolation-contract.md), and [best practices](references/best-practices.md).

When phase isolation exists, keep `$rpi` visible and pass phase skill name plus bounded handoff in, then artifact/verdict/next action out. The transport may be a process or subagent wrapper, but it must execute the declared phase contract rather than doing phase work directly.

RPI owns one lifecycle objective. Preserve the discovered `epic_id` or original goal and packet objective; a child bead or ready slice is context, not a replacement. `<promise>PARTIAL</promise>` from `$crank` is evidence for the orchestrator, not an automatic retry.

## Phase Receipt Contract

RPI cannot rely on memory or a final narrative to prove responsibilities ran.
The execution packet carries one ordered receipt index whose entries point at
the canonical Discovery packet, Crank tranche evidence, Validate verdict, and
Learn receipt. Do not restate the same analysis in four Markdown summaries or
mirror `skills_loaded` into every artifact. Legacy phase summaries, when a
consumer still requires them, are link-only compatibility projections. Full
contract: [phase-data-contracts.md](references/phase-data-contracts.md).

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

Track lifecycle state as `rpi_state`: `goal` (string), `epic_id` (null until discovered), `phase` (discovery|crank|validate|learn), `complexity` (fast|standard|full), `test_first` (true unless `--no-test-first`), `run_id`, and `verdicts` ({}). When evidence changes the next move, write one immutable run-disposition record bound to the objective and evidence. Do not add phase counters, reservations, or helper state.

## Phase DAG

Enter at the routed phase and run every phase after it.

1. **Discovery:** invoke `$discovery <goal> [--interactive] --complexity=<level>`. On DONE, read the current or archived execution packet and preserve its objective spine; on BLOCKED, return evidence without treating the label as a retry decision.
2. **Crank tranche:** invoke `$crank` for one ready wave and read the actual diff
   for scope and claim match. If targeted checks are green and the bound plan
   inputs are unchanged, the orchestrator may pull the next sequential wave
   without Validate or Learn. A completed leaf proceeds
   to freeze. At three waves or 90 minutes with work incomplete, persist
   `PARTIAL` resume evidence and stop without proof authorization. Scope/risk
   drift or failed acceptance returns to the appropriate repair/replan move. A
   soft tranche boundary is not HOLD or ANDON.
3. **Freeze and Validate once:** commit the complete tranche, pin one candidate
   identity, and consume exact-input deterministic receipts. Invoke one fresh
   independent `$validate`. Missing,
   stale, suspicious, or invalidated facts are rerun; unchanged facts are not.
4. **Repair closure:** one introduced blocker set may receive one consolidated
   repair batch. Refreeze, rerun invalidated facts, and re-review only affected
   claims. A second distinct repair need is `REPLAN`. After closure, run the
   repository's full deterministic terminal gate once on the final exact
   candidate, persist its reusable receipt, and seal the final Validate result.
5. **Learn once:** invoke `$learn` in the orchestrator context with the immutable
   final verdict. Learn performs no model review and emits one canonical receipt;
   any phase summary is a link-only compatibility projection.
6. **Report:** use [references/report-template.md](references/report-template.md)
   after the one proof transaction. Apply the Context Density Rule and preserve
   exact resume state when aggregate demand remains.

## Orchestrator Decision State Machine

The orchestrator, not Validate or Learn, owns retry and re-plan decisions.
Every final tranche verdict becomes one Learn receipt; its plan impact selects
the next-tranche branch. Intermediate wave facts go directly to the
orchestrator's remaining-plan decision and never impersonate a semantic verdict.
The [run disposition contract](references/pull-flow-governor.md) defines the
five evidence classifications and their legal next moves. It records decisions;
it does not run another state machine around the work.

## Agile Re-Plan Loop (the anti-waterfall rule)

The initial plan is a **hypothesis**; each wave is an experiment. Targeted wave
evidence may change the remaining tranche before any semantic review. When the
plan's bound inputs change materially, the orchestrator invokes Discovery and
one fresh Premortem; otherwise it reuses the accepted plan verdict. At tranche
completion, evidence flows once through `Validate -> Learn -> orchestrator`.
Anti-patterns: **waterfall**,
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
Routine work uses one fresh validator; deeper review is explicit. Learn stays bounded and delivery remains a repository adapter. See [complexity scaling](references/complexity-scaling.md).

## Flags

| Flag | Default | Purpose |
|------|---------|---------|
| `--from=<phase>` | discovery | Start at discovery, implementation, or validation |
| `--interactive` | off | Human gates in discovery/validate |
| `--auto` | on | Fully autonomous default — **pivots between waves on its own** (re-plans remaining work; not a fixed-plan/waterfall executor). See [Agile Re-Plan Loop](#agile-re-plan-loop-the-anti-waterfall-rule) |
| `--loop` | off | Pull additional sequential waves inside the current bounded tranche |
| `--run-id=<id>` | generated if absent | Correlate receipts for one lifecycle objective |
| `--max-waves=<n>` | 3 | Soft tranche boundary; preserve resume evidence instead of escalating |
| `--max-elapsed-seconds=<n>` | 5400 | Soft 90-minute tranche boundary; preserve resume evidence instead of escalating |
| `--test-first` / `--no-test-first` | on / off | Enable or explicitly opt out of TDD ordering |
| `--fast-path` / `--deep` | auto | Force fast or full complexity |
| `--dry-run` | off | Report the selected moves without mutating work |

These are orchestration defaults, not authorization or phase-local retry
budgets. Hard external ceilings are facts supplied by the runtime or operator.

## Examples

**User says:** `$rpi "add user authentication"` for a new goal, or `$rpi --from=implementation ag-23k` for an already-shaped leaf.

## Output Specification

- Canonical state: `.agents/rpi/execution-packet.json` plus immutable per-run artifacts and one ordered receipt index.
- Schema: `schemas/execution-packet.schema.json`; validate with `python3 skills/rpi/scripts/validate-execution-packet.py .agents/rpi/execution-packet.json`.
- Handoff: Discovery shapes, Crank records targeted evidence, Validate writes the immutable verdict, Learn records plan impact, and the orchestrator reports.
- `<promise>PARTIAL</promise>` preserves resume state; it never implies retry or terminal proof.

## Quality Checklist

- [ ] No per-wave Validate or Learn ran before the bounded tranche froze.
- [ ] One final independent verdict routes through Learn before the next-tranche decision.
- [ ] The execution packet passes its validator before Report or downstream handoff.
- [ ] One closed disposition record binds each next-move decision to evidence.

## Troubleshooting

- Classify failures with the five run dispositions; repair invalid packets locally, and use direct checks when an optional executor fails. See [troubleshooting.md](references/troubleshooting.md).

## Reference Documents

- Core: [agile re-plan](references/agile-replan-loop.md), [run dispositions](references/pull-flow-governor.md), [phase data](references/phase-data-contracts.md), [compression](references/orchestrator-compression-anti-pattern.md), and [executable feature](references/rpi.feature).
- Operation: [autonomy](references/autonomous-execution.md), [context windows](references/context-windowing.md), [Discovery artifact mode](references/discovery-artifact-mode.md), [bounded tranches](references/phase-budgets.md), [repair and escalation](references/gate-retry-logic.md), [loop/spawn](references/gate4-loop-and-spawn.md), [Codex executor](references/codex-executor.md), [installed-version warning](references/installed-plugin-version-not-repo-head.md), [examples](references/examples.md), [recovery](references/error-handling.md), and [report](references/report-template.md).
