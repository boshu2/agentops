---
name: rpi
description: 'Run Discovery, Crank, Validate, and Learn as four ordered, independently receipted umbrellas. Triggers: "run rpi", "research-plan-implement one turn", "drive a turn through the operating loop".'
practices:
- bdd-gherkin
- ddd-bounded-context
- hexagonal-architecture
- tdd
- continuous-delivery
- dora-metrics
- agile-manifesto
- pragmatic-programmer
hexagonal_role: domain
consumes:
- crank
- discovery
- domain
- learn
- validate
produces:
- .agents/rpi/*.md
context_rel:
- kind: customer-of
  with: crank
- kind: customer-of
  with: discovery
- kind: customer-of
  with: learn
- kind: customer-of
  with: validate
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
  tier: meta
  dependencies:
  - domain
  - discovery
  - crank
  - validate
  - learn
  internal: false
output_contract: .agents/rpi/YYYY-MM-DD-*.md
---

# /rpi - Full Lifecycle Orchestrator

> Quick ref: `/discovery` -> `/crank` -> `/validate` -> `/learn`, then report.

**Execute this workflow. Do not only describe it.** RPI is autonomous unless
`--interactive` is set. The user touchpoint is after validation, or after a
real blocked state exhausts retries. Read
[references/autonomous-execution.md](references/autonomous-execution.md) when
you need the full autonomy contract.

**`--auto` means *pivot autonomously*, NOT *execute the initial plan to the letter*.** Autonomy is agility, not waterfall: between waves the orchestrator re-plans the remaining work and changes course on its own — refactoring, adding, dropping, reordering waves as evidence arrives — without the operator saying so (touched only at the terminal objective or a circuit-breaker trip that survives its bounded helper pass). See [Agile Re-Plan Loop](#agile-re-plan-loop-the-anti-waterfall-rule).

## Critical Constraints

- `WARN|FAIL|REFUTED -> AUTO-REDO`: consult the pawl, feed its findings into re-plan, and retry the same lifecycle objective. **Why:** a negative verdict is evidence for the loop, not an andon by itself.
- `BREAKER -> HOLD -> ONE-HELPER`; `HELPER-UNSTUCK -> AUTO-REDO`. A breaker is a capability, permission, safety, or irreducible ambiguity stop—not an ordinary failed check. **Why:** one bounded helper can restore progress without hiding a true stop condition.
- `HELPER-ESCALATE -> HUMAN`; `REFUSAL-LANE|EXPLICIT-JUDGMENT|EXHAUSTED-BUDGET -> HUMAN`. **Why:** human attention is reserved for decisions or terminal recovery lanes the loop cannot own.
- Preserve one objective, acceptance surface, and evidence chain across every retry. **Why:** narrowing to a convenient child task can manufacture green while the requested behavior remains incomplete.

## Loop position

`/rpi` is the orchestrator across **every move** of the [operating loop](../../docs/architecture/operating-loop.md): BDD intent → vertical slices → per-slice [narrow-waist micro-cycle](../../docs/architecture/operating-loop.md#the-narrow-waist-micro-cycle-canonical--every-loop-skill-cites-this) (**acceptance test RED → green → refactor-under-green**) → conflict-free wave → bead acceptance → evidence + learning mined back into the next loop. It delegates each move to the skill that owns it (`/discovery`, `/plan`, `/crank`, `/validate`, `/curate --mode=forge`/`/post-mortem`), and enforces these loop-level invariants:

- **Agile, not waterfall — the plan is a hypothesis.** Every wave closes with a **re-plan, not just a retry** (the [Agile Re-Plan Loop](#agile-re-plan-loop-the-anti-waterfall-rule), autonomous under `--auto`).
- **No move-skipping, but validation cadence is pawl-gated, not per-tread.** The acceptance roll-up plus heavy gates (`/validate`, optional council, `/pawl-review`, then `ao pawl`) fire once at bead acceptance. Intermediate slices use cheap local checks.
- **The first failing test is the bead's contract.** With `--test-first` on (the default), `/crank` is invoked with the TDD-per-slice discipline; `--no-test-first` is an explicit opt-out, not a fast path. `/crank` runs **refactor-under-green as its own step after green** — the load-bearing quality move — and a refactor must never change a test (S4; test-first *ordering* alone is not the quality lever).
- **Acceptance examples close the bead, not activity.** Validation FAIL re-cranks on the same objective up to 3 attempts; DONE requires the acceptance roll-up in the [slice-validation template](../../docs/templates/slice-validation.md) to be fully green.
- **Ports stay visible.** Preserve the [Intent-to-Loop Hexagon](../../docs/architecture/intent-to-loop-hexagon.md) boundary as the objective crosses `shape_intent`, `persist_intent`, `plan_slices`, `execute_wave`, `validate_acceptance`, and `record_evidence`.
- **Context density survives phase boundaries.** Apply the [Context Density Rule](../domain/references/context-density-rule.md) to every phase handoff and final report: keep intent, boundary, evidence, decision, constraint, and next action; omit or link anything else.

### Folded triggers (ag-s43tg): `operating-loop-skill` + `operating-loop-workflow` route here

- **operating-loop-skill** — driving one bead end-to-end through claim, work, independent validation, closeout, and persistence: `/rpi <bead-id>` runs that exact arc.
- **operating-loop-workflow** — installing or running the seven-move operating-loop Workflow for AgentOps plugin users and multi-agent orchestration: `/rpi` is the in-session orchestrator of the same seven moves.

## Core Contract

RPI delegates via `Skill(skill="discovery", ...)`,
`Skill(skill="crank", ...)`, `Skill(skill="validate", ...)`, and
`Skill(skill="learn", ...)` as separate tool invocations. Keep strict
delegation on by default; do not compress phases,
replace phase skills with direct agent spawns, or skip validation. Read
[../shared/references/strict-delegation-contract.md](../shared/references/strict-delegation-contract.md)
for the full anti-compression contract.
See [references/isolation-contract.md](references/isolation-contract.md) for
phase-isolated transport and [references/best-practices.md](references/best-practices.md) for its anti-patterns.

When the runtime supports phase isolation, keep `/rpi` visible in the main
session and run each phase contract through isolated transport: phase skill name in, bounded handoff artifact in, phase artifact/verdict/next action out.
The transport may be a daemon job, process runner, or subagent wrapper, but it must execute the declared phase skill contract rather than doing phase work directly.

RPI owns one lifecycle objective across all phases. Preserve the discovered
`epic_id` when present; otherwise preserve the original goal and execution
packet objective. A child bead or one ready slice is context, not a replacement
objective. `<promise>PARTIAL</promise>` from `/crank` means retry Phase 2 on the
same objective.

## Phase Receipt Contract

RPI cannot rely on memory or a final narrative to prove delegated skills ran.
Every execution packet and phase summary MUST carry compact receipts — JSON
`skills_loaded` + `phase_receipts` (canonical slugs, no sigils) and a
`## Skill Receipts` bullet list in each markdown phase summary. Receipts do not
replace transcript/runtime proof; they make delegation auditable from disk when
the transcript is unavailable and give validation or pre-land review a
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

1. **Discovery:** invoke `/discovery <goal> [--interactive] --complexity=<level>`
   directly or through phase-isolated skill transport.
   On DONE, read `.agents/rpi/execution-packet.json` or the run archive and
   preserve its objective spine. On BLOCKED, classify it through the pawl
   recovery state machine; never stop on the label alone.
2. **Crank:** invoke `/crank <epic-id>` when the packet has `epic_id`;
   otherwise invoke `/crank .agents/rpi/execution-packet.json`, directly or
   through phase-isolated skill transport. Pass `--test-first` or
   `--no-test-first` through. On DONE, record `ao ratchet record implement
   2>/dev/null || true` and continue. On PARTIAL, auto-redo the same objective;
   on BLOCKED, classify it through pawl recovery. Use 3 total attempts before
   `EXHAUSTED-BUDGET`. **Before accepting a slice/wave the orchestrator reads the actual diff itself** (scope + claim match) — not just the `<promise>DONE</promise>` and evidence JSON, but its own diff-read, distinct from the delegated sub-judges.
   `/crank` enforces this as the anti-green-washing Step 3.5 of its Wave Acceptance ([crank wave-patterns.md §Wave Acceptance Check](../crank/references/wave-patterns.md)).
3. **Validate:** invoke `/validate <epic-id> --complexity=<level>` when an
   epic exists; otherwise invoke `/validate --complexity=<level>`, directly
   or through phase-isolated skill transport. Add `--strict-surfaces` when
   `--quality` is set. On FAIL, extract findings, re-run `/crank` on the same
   objective, then re-run `/validate`, up to 3 total validation attempts. On
   DONE, record `ao ratchet record vibe 2>/dev/null || true`. This Phase-3 `/validate` is the bead-acceptance pawl, once per objective. Any work crossing shared trunk obtains fresh evidence through [`/pawl-review`](../pawl-review/SKILL.md); `ao pawl` applies the complexity-scaled diversity and verdict gate.
4. **Learn:** invoke `/learn` with the immutable Validate verdict and its
   evidence reference. Record a `learn` receipt with status `DONE`, `PARTIAL`,
   or `BLOCKED` and a file-backed `.agents/rpi/phase-4-summary.md`. Learn may
   capture observations; it cannot change the verdict or delivery state.
5. **Re-plan (mandatory between waves; the loop's hinge).** With remaining waves, run the [Agile Re-Plan Loop](#agile-re-plan-loop-the-anti-waterfall-rule) before the next — a postmortem/discovery delta that MAY mutate the remaining plan (autonomous under `--auto`). No remaining waves → straight to Report.
6. **Report:** summarize phase verdicts, the re-plan deltas taken, and epic
   status using [references/report-template.md](references/report-template.md).
   With `--loop`, restart from discovery on FAIL while `cycle < max_cycles`. With
   `--spawn-next`, read `.agents/rpi/next-work.jsonl` and suggest the next
   command without invoking it. Before emitting the report, apply the Context
   Density Rule: every line should carry intent, boundary, evidence, decision,
   constraint, or next action.

## Pawl Recovery State Machine

Treat validation WARN, FAIL, and REFUTED as `AUTO-REDO`: persist findings, re-plan remaining work, then re-enter the owning phase. Raise `BREAKER` only when execution cannot safely or meaningfully proceed; hold the objective and dispatch exactly one bounded helper. Helper recovery returns to auto-redo. Helper escalation, an explicit refusal/judgment lane, or exhausted budget is the only human andon path. Record every transition in the execution packet and final report.

## Agile Re-Plan Loop (the anti-waterfall rule)

The initial plan is a **hypothesis**; each wave is an experiment whose evidence re-plans the rest. At every wave boundary (and after validation): **reflect** (a bounded `/post-mortem` + `/discovery` re-plan delta over what shipped/broke) → **re-plan the REMAINING waves** (refactor / insert / drop / reorder / re-scope / escalate, persisting the mutated plan so the next wave reads the *current* one) → **proceed**. Under `--auto` this is autonomous, bounded by the run's circuit breakers (budget / attempt cap / oscillation detection) and the ≥5-ship post-mortem checkpoint; the operator is touched only at the terminal objective or a breaker trip that survives its bounded helper pass. `/crank` and `/validate` surface findings UP for re-planning (never a silent local retry); `/discovery` is the re-plan engine. Anti-patterns: **waterfall** (run the plan to the letter), **retry-not-replan** (re-crank forever instead of changing the remaining plan), **permission-seeking** (pause to approve a pivot `--auto` already authorizes). **Full detail:** [references/agile-replan-loop.md](references/agile-replan-loop.md).

## Phase Data Contract

The execution packet carries the repo execution profile through
`contract_surfaces`, `done_criteria`, and queue claim/finalize metadata. Keep
the latest alias at `.agents/rpi/execution-packet.json` and read
[references/phase-data-contracts.md](references/phase-data-contracts.md) for
schemas and archive paths.

## Complexity-Scaled Gates

> The pawl gates ([pawls.md](../../docs/contracts/pawls.md)) fire at the irreversible doors — bead-acceptance and merge-to-main — never per slice/wave; chaos between pawls. The merge-to-main pawl fires **regardless of complexity** (see Phase 3); complexity below only scales the DEPTH of the gate, never whether it runs.

`fast`/`standard` use a 2-judge minimum panel; `full` uses a full council; all cap at 3 attempts. Pre-mortem is a chaos-side stress test, not a pawl. Final validation and post-mortem sit at bead acceptance. Read [references/complexity-scaling.md](references/complexity-scaling.md) for the complete matrix.

## Flags

| Flag | Default | Purpose |
|------|---------|---------|
| `--from=<phase>` | discovery | Start at discovery, implementation, or validation |
| `--interactive` | off | Human gates in discovery/validate |
| `--auto` | on | Fully autonomous default — **pivots between waves on its own** (re-plans remaining work; not a fixed-plan/waterfall executor). See [Agile Re-Plan Loop](#agile-re-plan-loop-the-anti-waterfall-rule) |
| `--loop --max-cycles=<n>` | off / 3 | Iterate when validation fails |
| `--spawn-next` | off | Surface follow-up work after reporting |
| `--test-first` / `--no-test-first` | on / off | Enable or explicitly opt out of TDD ordering |
| `--fast-path` / `--deep` | auto | Force fast or full complexity |
| `--quality` | off | Make validation strict surfaces blocking |
| `--dry-run` / `--no-budget` | off | Report only, or disable phase budgets |

## Examples

- `/rpi "add user authentication"` — discovery → implementation → validation → report.
- `/rpi --from=implementation ag-23k` — resolve the bead scope, run implementation + validation.
- `/rpi --deep "refactor payment module"` — full council gates across the lifecycle.

Read [references/examples.md](references/examples.md) for resume, interactive, loop, and artifact-mode examples.

## Output Specification

**Artifact directory:** `.agents/rpi/`.
**Filename convention:** mutable `execution-packet.json`, immutable `runs/<run-id>/execution-packet.json`, `phase-<n>-summary.md`, and optional `next-work.jsonl`.
**Serialization/schema format:** packet JSON matches `schemas/execution-packet.schema.json` plus the `skills_loaded`/`phase_receipts` extension in [phase-data-contracts](references/phase-data-contracts.md); summaries follow the markdown [report template](references/report-template.md).
**Validator command:** `python3 skills/rpi/scripts/validate-execution-packet.py .agents/rpi/execution-packet.json`.
**Downstream handoff:** discovery creates the packet, crank updates
implementation evidence, validate appends the immutable acceptance verdict,
Learn records post-verdict observations, and Report emits the human-readable
roll-up.
**Exit signal:** the per-phase verdict roll-up; `<promise>PARTIAL</promise>` from `/crank` means retry Phase 2 on the same objective.

## Quality Checklist

- [ ] The same objective and acceptance examples survive every phase and retry.
- [ ] Each phase has a disk-backed receipt, evidence path, and explicit verdict.
- [ ] Ordinary negative verdicts re-plan through the pawl; only terminal lanes raise the andon.
- [ ] The execution packet passes its validator before Report or downstream handoff.

## Troubleshooting

| Problem | Response |
|---------|----------|
| Phase returns BLOCKED | Classify it through pawl recovery; stop only on a terminal transition |
| Packet validation fails | Repair the packet or receipts, then rerun the validator before handoff |
| External executor fails | Use direct local checks; raise a breaker only for a reproducible capability stop |

## Related skills

- [`/agent-native`](../agent-native/SKILL.md) + [`/ntm`](../ntm/SKILL.md) — portable out-of-session workers and NTM pane mechanics for whole `/rpi` loops.

## Reference Documents

- Core loop: [agile re-plan](references/agile-replan-loop.md), [executable feature](references/rpi.feature), [compression anti-pattern](references/orchestrator-compression-anti-pattern.md), [installed-version warning](references/installed-plugin-version-not-repo-head.md).
- Modes: [context windowing](references/context-windowing.md), [discovery artifact](references/discovery-artifact-mode.md), [phase budgets](references/phase-budgets.md), [examples](references/examples.md).
- Recovery: [error handling](references/error-handling.md), [gate retry](references/gate-retry-logic.md), [loop/spawn](references/gate4-loop-and-spawn.md), [troubleshooting](references/troubleshooting.md), [Codex executor](references/codex-executor.md).
- Contracts: [autonomous execution](references/autonomous-execution.md), [phase data](references/phase-data-contracts.md), [report template](references/report-template.md).
