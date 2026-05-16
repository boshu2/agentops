# Skill Ports and Adapters

> **Status:** V0 skill vocabulary.
> **Owner bead:** `soc-m6v5.9.9.3`.
> **Purpose:** define the words skills use before skill bodies or CLI code are
> refactored around DDD + Hexagonal architecture.

This contract applies the project-wide [Ports and Adapters](../architecture/ports-and-adapters.md)
model to `skills/*/SKILL.md` and `skills-codex/*/SKILL.md`. It does not replace
the generated [Context Map](context-map.md) or the [Skill Domain Map](skill-domain-map.md).
It gives agents the vocabulary to describe skill boundaries without inventing
new terms inside each refactor.

## Rule

Ports name the boundary. Adapters name the concrete mechanism.

If a skill hands work to another skill, CLI command, hook, daemon, file, remote
API, model runtime, or issue tracker, the handoff should be described as a
port crossing first. The runtime, command, hook, or file format is the adapter.

## Core Terms

| Term | Meaning | Good name shape |
|---|---|---|
| Skill domain | A named responsibility cluster from the [Skill Domain Map](skill-domain-map.md). | Product and Discovery, Planning and Work Graph |
| Inbound port | A behavior-level entry into a skill domain. It says why the domain is being asked to act. | `shape_intent`, `plan_slices`, `validate_acceptance` |
| Outbound port | A behavior-level dependency a skill domain needs from another domain or system. | `retrieve_context`, `persist_issue`, `run_gate` |
| Driving adapter | A concrete caller that drives a skill domain through an inbound port. | slash skill, `ao` command, human prompt, scheduled job |
| Driven adapter | A concrete dependency used behind an outbound port. | `bd`, git, filesystem, search, GitHub, LLM provider |
| Guard adapter | A concrete enforcement surface that allows, warns, or blocks. It should not inject context unless evidence proves value. | hook, schema check, pre-push gate, scope guard |
| Runtime adapter | A concrete packaging or invocation surface for one agent runtime. | Claude skill, Codex skill, OpenCode skill |
| Context packet | A typed or structured artifact crossing a boundary with dense intent and evidence. | execution packet, density block, verdict, handoff |

Use these terms in skill docs before adding CLI behavior. The CLI should support
the skill contract after the boundary is explicit.

## Artifact Boundaries

These artifacts are the preferred things to pass across skill boundaries. They
carry context without dragging the whole session forward.

| Artifact | Boundary role | Typical producer | Typical consumer |
|---|---|---|---|
| Density block | Six-field summary: intent, boundary, evidence, decision, constraint, next action | Discovery, Plan, advisory auditors | Planning, Crank, Validation |
| Execution packet | Runnable packet for downstream lifecycle phases | Discovery, Plan | Crank, Validation, RPI |
| Acceptance criteria | Testable BDD/TDD completion contract | Discovery, Plan, Beads | Implement, Crank, Validation |
| Verdict | PASS/WARN/FAIL judgment with evidence | Validate, Validation, Council, Vibe | Beads, Release, Ratchet |
| Learning | Durable behavior change candidate | Retro, Forge, Post-mortem | Compile, Flywheel, future skills |
| Finding | Indexed evidence or risk record | Research, Bug-hunt, Council | Plan, Validation, Context assembly |
| Handoff | Compact session continuity packet | Handoff, Recover, RPI | Next operator or agent session |

Raw child-skill prose is not a boundary artifact. Link to it when needed; pass
one of the artifacts above when another skill needs to act.

## Adapter Classes

### Driving Adapters

Driving adapters initiate work. Examples:

- `/discovery` invoked by an operator goal.
- `/plan` invoked from an execution packet or bead.
- `ao context assemble` invoked by a runtime or script.
- A scheduled Dream/Evolve run.

Driving adapters should translate the caller's request into the domain language
of the target skill. They should not smuggle runtime-specific assumptions into
the skill domain.

### Driven Adapters

Driven adapters satisfy dependencies. Examples:

- `bd` for issue persistence.
- git for changed files and branch state.
- Search and lookup commands for corpus retrieval.
- Files under `.agents/` for local runtime state.
- GitHub CLI/API for PR work.

Driven adapters can fail open or fail closed depending on the skill contract.
The skill body must say which behavior applies.

### Guard Adapters

Guard adapters enforce constraints. Examples:

- Scope hooks that block writes outside declared surfaces.
- JSON Schema validation for execution packets.
- Pre-push and CI gates.
- Markdownlint, shellcheck, and test runners.

Guard adapters are not default context-injection surfaces. A guard may emit a
small error or verdict. Resident prose belongs behind an explicit evidence and
token-budget decision.

### Runtime Adapters

Runtime adapters package the same skill contract for different agents. Examples:

- `skills/<name>/SKILL.md` as the shared source skill contract.
- `skills-codex/<name>/SKILL.md` as the Codex runtime artifact.
- `skills-codex-overrides/<name>/` as Codex-only tailoring.

When shared behavior changes, update the shared contract first, then the runtime
adapter. Runtime-specific phrasing belongs in the adapter, not in the domain
language.

## Workflow Examples

### Discovery

```gherkin
Feature: Discovery creates a dense packet
  Scenario: Discovery delegates child research
    Given an operator goal and child artifacts from research, design, plan, and pre-mortem
    When Discovery crosses the planning boundary
    Then the inbound port is `shape_intent`
    And the outbound ports include `research_facts`, `plan_slices`, and `stress_test_plan`
    And the context packet contains density fields plus artifact links
```

| Boundary piece | Discovery example |
|---|---|
| Inbound port | `shape_intent` from an operator goal |
| Driving adapter | `/discovery` skill invocation |
| Outbound ports | `research_facts`, `plan_slices`, `stress_test_plan`, `persist_issue` |
| Driven adapters | `/research`, `/plan`, `/pre-mortem`, `bd`, `.agents/plans` |
| Context packet | `.agents/rpi/execution-packet.json` plus phase summary |

### Plan

```gherkin
Feature: Planning turns intent into slices
  Scenario: Plan consumes a BDD intent issue
    Given a BDD-shaped goal with acceptance examples
    When Plan decomposes the work
    Then the inbound port is `plan_slices`
    And each output slice names its write scope, first failing test, and acceptance criteria
```

| Boundary piece | Plan example |
|---|---|
| Inbound port | `plan_slices` from BDD intent or execution packet |
| Driving adapter | `/plan` skill invocation |
| Outbound ports | `persist_issue`, `verify_symbols`, `retrieve_context` |
| Driven adapters | `bd`, `rg`, `.agents/findings`, `.agents/plans` |
| Context packet | slice plan, file dependency matrix, acceptance criteria |

### Crank

```gherkin
Feature: Crank executes a validated wave
  Scenario: Crank dispatches a conflict-free wave
    Given a slice plan with disjoint write scopes
    When Crank starts wave execution
    Then the inbound port is `execute_wave`
    And the runtime adapter dispatches workers through Swarm
    And the guard adapters verify scope, tests, and completion markers
```

| Boundary piece | Crank example |
|---|---|
| Inbound port | `execute_wave` from bead, plan, or execution packet |
| Driving adapter | `/crank` skill invocation |
| Outbound ports | `dispatch_worker`, `sync_issue_state`, `run_acceptance_gate` |
| Driven adapters | `/swarm`, `/implement`, `bd`, git, test commands |
| Guard adapters | wave validity check, scope-completion gate, validation gates |
| Context packet | worker brief, wave result, completion marker, phase-2 handoff |

## Naming Rules

- Prefer behavior verbs for ports: `shape_intent`, `plan_slices`,
  `execute_wave`, `validate_acceptance`, `record_learning`.
- Prefer concrete nouns for adapters: `bd`, `git`, `ao lookup`,
  `skills-codex/discovery/SKILL.md`, `hooks/scope-guard.sh`.
- Do not use `BC1`, `BC2`, or similar abbreviations as the main prose name.
- Do not name a port after a tool. `persist_issue` is a port; `bd` is an
  adapter.
- Do not name an adapter after a policy. `validate_acceptance` is a port;
  `scripts/pre-push-gate.sh` is an adapter.

## Refactor Use

Before editing a skill body, identify:

1. The skill domain.
2. The inbound port the skill exposes.
3. Any outbound ports it consumes.
4. The concrete adapters it currently uses.
5. The context packet or artifact that crosses each boundary.
6. The guard adapter that proves the boundary still works.

If any row is unknown, mark it `needs-follow-up` in the refactor plan rather
than guessing.
