---
name: workflow-builder
description: "Run workflow builder."
---
# $workflow-builder — author a deterministic Codex orchestration

> Counterpart to `$skill-builder`. `$skill-builder` authors a `SKILL.md` (a leaf
> capability); this authors an **orchestration** (a composite capability —
> deterministic fan-out / pipeline / loop over sub-agents). Reach this skill via
> `$automation-shape-routing` once the shape is confirmed **Orchestration**
> (deterministic DAG + structured-JSON returns + headless). If the shape is NTM or
> plain skill, you're in the wrong builder — go back to `$automation-shape-routing`.

## Confirm the shape first

Do NOT author an orchestration for: an attach-and-steer run (→ NTM: `ntm` /
`vibing-with-ntm`), or a hard-sequential edit-loop with no parallelism (→ plain
skill: `$skill-builder`). If unconfirmed, run `$automation-shape-routing`.

## The shape

A Codex orchestration is a script that drives `codex exec` to launch sub-agents
via `spawn_agents`, each constrained to return JSON against an `output_schema`,
then composes those structured results across phases. The control flow is the same
three primitives regardless of how you wire them: **fan-out barrier**, **streaming
pipeline**, and **bounded loop**.

```
phase: Find    — fan out N finders in parallel (spawn_agents), each returning
                 FINDINGS_SCHEMA; barrier = collect ALL before continuing.
phase: Verify  — stream each finding into a verifier sub-agent returning
                 VERDICT_SCHEMA; no barrier, items flow independently.
return         — the verified structured results.
```

Each sub-agent is a `codex exec` call carrying its prompt plus the
`output_schema` it must satisfy; the orchestrator owns sequencing, the barrier vs
streaming choice, the budget guard, and the merge.

## Building blocks (pick by control-flow shape)

| Primitive | Use when |
|---|---|
| sub-agent with `output_schema` | one `codex exec` sub-agent; the schema forces structured JSON back |
| parallel fan-out (`spawn_agents`) | **barrier** — need ALL results together (dedup/merge/early-exit) |
| streaming pipeline | **default** multi-stage — no barrier, each item flows independently |
| phase markers | progress grouping; one per orchestration stage |
| bounded loop (loop-until-budget / loop-until-dry) | unknown-size discovery; guard on remaining budget |

## Conformance — author it as a control system, not a DAG

An orchestration gates and routes, so it must be a traversable control system,
not an open-loop DAG. The four conformance moves (from `control-loop-model.md`
§6) are runtime-agnostic: (a) dispatch each move's discipline as a black-box
sub-agent returning a structured schema, never inline the generative work;
(b) gate on a DELEGATED deterministic verdict (a captured test exit code,
`$validate`, a fixed-rubric quorum) — never a free-form self-grade, the #1 cause
of non-convergence; (c) terminate on that grounded verdict, with an attempt-cap
as a backstop only, carrying the prior failure forward each retry; (d) the
orchestrator routes and gates — it never reasons about the work it gates. The
conformance contract is `docs/architecture/control-loop-model.md` §6; a worked
copy-paste idiom lives in `docs/architecture/workflow-conformance-pattern.md`.

## Authoring checklist

1. **Shape confirmed Orchestration** (via `$automation-shape-routing`).
2. **Schemas first** — define the `output_schema` each sub-agent returns;
   structured output is what makes an orchestration deterministic and composable.
3. **Default to the streaming pipeline**; reach for the parallel fan-out barrier
   only when a stage genuinely needs all prior results at once.
4. **Conflict-free fan-out** — if branches write files, give each a disjoint
   write-scope (the wave-validity invariant) or run in worktree isolation.
5. **Budget** — for loops, gate on a remaining-budget check before each round.
6. **Conformance self-check** — confirm the four §6 moves above hold; gate on a
   delegated deterministic verdict, never a self-grade.
7. **Dry-run to validate** — invoke the orchestration on a tiny input; confirm
   each phase launches its sub-agents and returns its `output_schema`. This is the
   orchestration analog of `$skill-auditor`.

## Relationship to the SDK

An orchestration is a **composite capability**; the portable contract for it (a
`shape: skill|workflow` discriminator, a `StepGraph`, a `control_flow` enum, a
`budget`, an `OrchestrationPort` interface) is net-new `agentops-core-sdk` work.
Author the orchestration here; the SDK is where the *contract* for
composite-capabilities lives.
