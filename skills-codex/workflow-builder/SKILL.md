---
name: workflow-builder
description: Scaffold a new Claude Workflow script —
---
# Workflow Builder — scaffold a Claude Workflow script

> Counterpart to `skill-builder`. `skill-builder` authors a `SKILL.md` (a leaf
> capability); this authors a **Workflow** (a composite capability — deterministic
> orchestration of subagents). Reach this skill via `automation-shape-routing`
> once the shape is confirmed **Workflow** (deterministic DAG + structured-JSON
> returns + headless). If the shape is NTM or plain skill, you're in the wrong
> builder — go back to `automation-shape-routing`.

## Critical Constraints

- Confirm the automation shape before writing files. **Why:** a Workflow is the
  Claude-only deterministic-DAG adapter, not the universal orchestration
  substrate; stop if a plain skill, NTM, or explicitly selected Gas City fits.
- Start from `.codex/workflows/operating-loop.js`. **Why:** preserving its
  schema-first, delegated-gate shape avoids free-form self-grading and an
  open-loop DAG that cannot provide a grounded promotion verdict.
- Keep the script orchestration-only. **Why:** the author of work cannot be its
  independent judge, so route, bound, and gate agents without putting design
  judgment or implementation work in the orchestrator.
- Give parallel writers disjoint scopes or isolated worktrees. **Why:**
  deterministic control flow does not prevent filesystem collisions; when an
  explicitly multi-writer workflow shares paths, reserve them through
  `agent-mail` first.
- Never add or retune a gate during a run. **Why:** adapting the controller while
  it is converging causes oscillation; route an escape to the slow loop and
  leave the current map fixed.
- Do not launch a runtime merely because this skill can describe one. **Why:**
  authoring a capability is not authorization to start a Workflow, NTM, Agent
  Mail, or Gas City; execution requires the operator to select that runtime.

## Confirm the shape first

Do NOT scaffold a workflow for: an attach-and-steer run (→ `agent-native` +
`ntm`), or a hard-sequential edit-loop with no parallelism (→ plain
skill: `skill-builder`). If unconfirmed, run `automation-shape-routing`.

## The template

Start from `.codex/workflows/operating-loop.js` — the canonical worked example.
Copy its skeleton, don't reinvent it. A Workflow script is plain JS:

```js
export const meta = {                 // REQUIRED — pure literal, no variables
  name: 'my-workflow',
  description: 'one line shown in the permission dialog',
  phases: [ { title: 'Find' }, { title: 'Verify' } ],  // one per phase() call
}

phase('Find')
const found = await parallel(FINDERS.map(f => () =>
  agent(f.prompt, { schema: FINDINGS_SCHEMA, phase: 'Find' })))   // barrier

phase('Verify')
const verified = await pipeline(found.flat().filter(Boolean),
  f => agent(`verify: ${f.title}`, { schema: VERDICT, phase: 'Verify' }))

return { verified }
```

## Building blocks (pick by control-flow shape)

| Primitive | Use when |
|---|---|
| `agent(prompt, {schema})` | one subagent; `schema` forces structured JSON back |
| `parallel([thunks])` | **barrier** — need ALL results together (dedup/merge/early-exit) |
| `pipeline(items, ...stages)` | **default** multi-stage — no barrier, each item flows independently |
| `phase(title)` | progress grouping; match `meta.phases` titles |
| `loop-until-budget` / `loop-until-dry` | unknown-size discovery; guard on `budget.total` |

## Conformance — author it as a control system, not a DAG

A workflow is an **orchestrator** (it gates and routes), so it must be a
traversable control system, not an open-loop DAG. Before scaffolding, read
**[the Workflow Conformance Pattern](../../docs/architecture/workflow-conformance-pattern.md)**
— the copy-paste idiom for the four moves (dispatch skills as black-box
schema-returning agents · gate on a delegated deterministic verdict, never a
self-grade · the bounded-loop guard idiom · orchestrator-routes-never-reasons)
plus the §6 five-rule self-check header to paste into your script and the
`workflows:` ledger row `scripts/check-workflow-governance.sh` requires. That doc
operationalizes [control-loop-model.md §6](../../docs/architecture/control-loop-model.md);
this skill scaffolds the script that satisfies it.

## Authoring checklist

1. **Shape confirmed Workflow** (via `automation-shape-routing`).
2. **Schemas first** — define the JSON schema each `agent()` returns; structured
   output is what makes a workflow deterministic and composable.
3. **Default to `pipeline()`**; reach for `parallel()` only when a stage genuinely
   needs all prior results at once.
4. **Conflict-free fan-out** — if branches write files, give each a disjoint
   write-scope (the wave-validity invariant) or run in worktree isolation.
5. **Budget** — for loops, gate on `budget.total && budget.remaining() > N`.
6. **Conformance self-check** — paste the §6 five-rule header from the
   [conformance pattern](../../docs/architecture/workflow-conformance-pattern.md)
   and mark each rule ✓/HARDENED/PENDING honestly; add the `workflows:` ledger
   row and run `scripts/check-workflow-governance.sh`.
7. **Dry-run to validate** — invoke the workflow on a tiny input; confirm the
   `meta` block parses and each phase returns its schema. This is the workflow
   analog of the heal-skill deep audit (`audit.sh`).

## Relationship to the SDK

A workflow is a **composite capability**; the portable contract for it (a
`shape: skill|workflow` discriminator, a `StepGraph`, a `control_flow` enum, a
`budget`, an `OrchestrationPort` interface) is net-new `agentops-core-sdk` work.
Author the script here; the SDK is where the *contract* for workflow-capabilities
lives. See `operating-loop-workflow` for installing/running a finished workflow.

## Output Specification

- **Artifact directory:** `.codex/workflows/` for the runnable script, plus the
  matching `workflows:` row in `docs/contracts/skill-dispositions.yaml`.
- **Filename convention:** `.codex/workflows/<kebab-case-name>.js`; `meta.name`
  and the disposition-ledger key must equal `<kebab-case-name>`.
- **Serialization/schema format:** JavaScript with a literal exported `meta`
  object and JSON Schemas on every `agent()` result used by a promotion gate.
- **Validator command:** run `node --check .codex/workflows/<name>.js`,
  `scripts/check-workflow-governance.sh`, and `make regen-check`; a dry run on a
  tiny input must also return the declared schemas before handoff.
- **Downstream handoff:** provide the script path, workflow identity, phase and
  schema summary, write-scope ownership, conformance-rule status, exact commands
  with exit codes, and any PENDING rule or residual risk. The finished workflow
  is consumed through `operating-loop-workflow`, not by this builder.

## Quality Checklist

- [ ] The request was routed as `workflow`, not a plain skill, NTM session, or
  explicitly selected Gas City.
- [ ] Every promoted result is schema-bound and grounded in a delegated
  deterministic verdict; no agent self-grade controls promotion.
- [ ] Loops terminate on the verdict, carry the prior failure, and use counters
  or wall-clock limits only as backstops.
- [ ] Parallel writers have disjoint scopes, isolated worktrees, or explicit
  Agent Mail reservations.
- [ ] All five §6 conformance rules are marked honestly; PENDING is not reported
  as compliant.
- [ ] Script parsing, governance, regeneration drift, and the tiny dry run are
  green with captured exit codes.
