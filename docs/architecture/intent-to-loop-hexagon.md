# Intent-to-Loop Hexagon

> The process-level hexagon for AgentOps work: one operator intent enters the
> loop, crosses named ports as dense artifacts, and exits only after acceptance
> evidence and learning disposition are recorded.

This page connects the structural hexagon in
[Ports and Adapters](ports-and-adapters.md) to the execution discipline in the
[Operating Loop](operating-loop.md). It is the contract agents use when turning
an idea into BDD/Gherkin, beads, vertical slices, implementation work, validation
verdicts, and ratcheted evidence.

## Inner Hexagon

The inner domain is not a CLI command or a specific agent runtime. It is the
behavior contract for one loop turn:

```text
Intent
  -> AcceptanceExample
  -> WorkItem / Bead
  -> Slice
  -> Wave
  -> CriterionVerdict
  -> Evidence
  -> LearningDisposition
```

These domain objects may be represented by Markdown, JSON, bd/br rows, Go
aggregates, or `.agents/` artifacts, but the names above are the stable language.
Adapters may change; the domain contract does not.

## Port Chain

| Loop move | Inbound port | Required artifact crossing the port | Driving adapter | Driven / guard adapters | Done state |
|---|---|---|---|---|---|
| 1. Shape intent | `shape_intent` | Intent issue with `Feature` and testable `Scenario` blocks | `/discovery`, `/brainstorm`, `/design`, operator prompt | domain register, GOALS directives, scenario schema | Every scenario is testable, bounded context is named, non-goals and evidence are explicit |
| 2. Track as bead | `persist_intent` | Bead body carrying the Gherkin or linking to the intent issue | `/beads`, `bd create`, `br create` | bd/br, `bv --robot-*`, dependency graph checks | Bead is self-contained, dependency-linked, and has validation commands |
| 3. Slice vertically | `plan_slices` | Slice validation plan, one row per scenario or behavior | `/plan` | symbol verification, file matrix, planning rules | Every slice has a first failing proof, write scope, owner, and bounded context |
| 4. Execute slice | `execute_slice` | Worker brief with slice, proof, scope, and rollback path | `/implement`, `/crank`, `/swarm` | git, test runners, scope guard, runtime agents | Failing proof failed for the right reason, then passes after the smallest implementation |
| 5. Execute wave | `execute_wave` | Wave packet with independent slice ownership | `/crank`, `/swarm`, `/autodev` | file-conflict matrix, worktrees, agent messaging | Same-wave writes do not collide, integration order is declared |
| 6. Validate acceptance | `validate_acceptance` | Criterion verdicts and roll-up validation report | `/validation`, `/validate`, `/vibe`, `/council`, `/scenario` | tests, evals, GOALS measure, completion-claim kernel | Every Given/When/Then maps to fresh passing evidence; no test theater |
| 7. Record evidence | `record_evidence` | Ratchet entry, evidence index, residual-gap disposition | `/ratchet`, `/post-mortem`, `/retro`, `/forge` | `.agents/ratchet/`, learning promotion gates, findings registry | Evidence is cited, residual gaps have next-step beads or accepted disposition |
| 8. Steer loop | `steer_goal` | Goal trace, learning, or next-work packet | `/goals`, `/flywheel`, `/harvest`, `/dream` | GOALS.md, scenarios, knowledge compile, scheduler | Durable behavior changed, or the observation dies at handoff |

## BDD and Done-State Rules

1. **Every behavior bead starts from Gherkin.** A feature, bug, or product-facing
   behavior must carry a fenced `gherkin` block or link to an intent issue that
   does. Chores may omit Gherkin only when their acceptance criteria are
   explicitly mechanical.
2. **Every scenario becomes a slice row.** `/plan` maps each Scenario to at
   least one slice with a first failing proof/test and a write scope.
3. **Every slice has one owner.** If ownership or write scope is shared, the
   work is sequential or re-sliced.
4. **Every close claim has fresh proof.** Closed/DONE/green status is a claim
   until `/validation` or `/validate` records command output, test names,
   file:line evidence, and parent/dependency reconciliation.
5. **Every residual gap is routed.** A gap becomes a new bead, learning,
   planning rule, gate proposal, or explicit accepted risk. It does not vanish
   into the report prose.

## Adapter Responsibilities

| Adapter class | Examples | Responsibility |
|---|---|---|
| Driving adapters | slash skills, `$` Codex skills, `ao` commands, operator prompts, scheduled jobs | Translate outside requests into the current inbound port without smuggling raw chat context |
| Driven adapters | bd/br, git, filesystem, search, eval runners, model providers, GitHub | Persist, retrieve, execute, or observe through narrow outbound ports |
| Guard adapters | scope guard, schema validation, pre-push gates, CI, pre-mortem, wave-validity matrix | Warn or block when a boundary contract is not met |
| Runtime adapters | `skills/`, `skills-codex/`, OpenCode skill bundles, hooks | Package the same domain contract for a specific agent runtime |

## Agent Output Contract

When an agent produces an artifact for the loop, it should make the hexagon
visible enough for the next agent to continue without chat memory:

```yaml
hexagon:
  inbound_port: shape_intent | persist_intent | plan_slices | execute_slice | execute_wave | validate_acceptance | record_evidence | steer_goal
  bounded_context: bc-corpus | bc-validation | bc-loop | bc-factory | bc-runtime | <repo-specific>
  driving_adapter: "<skill, command, prompt, or scheduler that initiated this>"
  driven_adapters:
    - "<bd/br/git/test/eval/filesystem/model/etc. used behind the port>"
  guard_adapters:
    - "<pre-mortem/scope/schema/CI/completion-claim-kernel/etc.>"
  context_packet: "<artifact path or bead id crossing the boundary>"
  done_state: "<specific proof required before the next port may accept it>"
```

This block is not required for every small note, but it is required in durable
plans, execution packets, substantial beads, and validation reports that future
agents are expected to consume.

## Skill Responsibilities

| Skill | Hexagonal responsibility |
|---|---|
| `/discovery` | Owns `shape_intent`, emits the dense execution packet, and preserves BDD/Gherkin acceptance examples |
| `/beads` | Owns `persist_intent`, makes work self-contained, dependency-linked, and proof-bearing |
| `/plan` | Owns `plan_slices`, maps scenarios to vertical slices, file ownership, and wave validity |
| `/pre-mortem` and `/council` | Guard intent and plan quality before implementation starts |
| `/implement` | Owns `execute_slice`, runs first failing proof then smallest green implementation |
| `/crank` and `/swarm` | Own `execute_wave`, coordinate workers without collapsing ownership boundaries |
| `/validation`, `/validate`, `/vibe`, `/scenario`, `/goals` | Own `validate_acceptance`, prove claims with fresh evidence |
| `/ratchet`, `/post-mortem`, `/retro`, `/forge` | Own `record_evidence`, promote only reusable learnings |
| `/rpi` | Orchestrates the port chain without replacing the phase skills that own each port |

## Failure Modes

| Failure mode | Corrective action |
|---|---|
| Bead has prose acceptance but no Gherkin or mechanical criteria | Send back to `shape_intent` / `/discovery` |
| Slice touches multiple bounded contexts | Split the slice before `/crank` |
| Same-wave slices write the same file | Serialize, merge, or re-slice |
| Agent marks DONE from stale CI or summary text | Apply the completion-claim kernel and rerun proof |
| Validation finds a real gap | Re-crank the same objective or create/reopen completion-debt beads |
| Learning is interesting but not reusable | Keep it in handoff; do not promote |

## See Also

- [Operating Loop](operating-loop.md)
- [Ports and Adapters](ports-and-adapters.md)
- [Skill Ports and Adapters](../contracts/skill-ports-and-adapters.md)
- [Intent Issue Template](../templates/intent-issue.md)
- [Slice Validation Plan Template](../templates/slice-validation.md)
- [Context Map](../contracts/context-map.md)
