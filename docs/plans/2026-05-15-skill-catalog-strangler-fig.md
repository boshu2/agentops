---
id: plan-2026-05-15-skill-catalog-strangler-fig
type: plan
date: 2026-05-15
goal: Refactor the skill catalog through a strangler-fig migration to CDLC first-principles skill definitions
detail_level: standard
research_refs:
  - PRODUCT.md
  - GOALS.md
  - PRACTICE-REGISTRY.md
  - docs/cdlc.md
  - docs/architecture/operating-loop.md
  - docs/contracts/ubiquitous-language.md
  - skills/domain/references/context-density-rule.md
  - skills/rpi/SKILL.md
  - skills/skill-builder/SKILL.md
  - skills/skill-auditor/SKILL.md
  - skills/heal-skill/SKILL.md
  - skills/converter/SKILL.md
---

# Skill Catalog Strangler Fig Plan - 2026-05-15

## Purpose

The skill catalog should move from "many useful prompt artifacts" toward a
bounded, testable skill-definition system without a big-bang rewrite. The
first-principles target is CDLC for agents under token scarcity:

- BDD/Gherkin gives observable behavior and acceptance language.
- DDD gives names, bounded contexts, and anti-sprawl boundaries.
- Hexagonal architecture separates core skill intent from runtime adapters.
- TDD gives the local done condition.
- AgentOps gates give repeatable evidence and provenance.

The migration must use the existing primitives to build the next generation of
skills. `rpi` is the lifecycle exemplar, but the first catalog-wide leverage
point is the factory pair: `skill-builder` creates skills and `skill-auditor`
judges whether they carry dense, verifiable context.

## Current System

The old primitives remain valuable and should not be thrown away:

| Primitive | Current role | Strangler role |
|-----------|--------------|----------------|
| `heal-skill` | Mechanical hygiene for existing `SKILL.md` files | Structural safety net; stays narrow and deterministic |
| `skill-auditor` | External validator for template quality | Adds report-only density checks before they become gates |
| `skill-builder` | Materializes skills from the current template | First producer of the new skill definition shape |
| `converter` | Produces runtime-specific artifacts | Adapter bridge for Codex and other runtime projections |
| `domain` | Shared vocabulary | Source of names, bounded contexts, and density rules |
| `rpi` | End-to-end lifecycle orchestrator | Exemplar of dense phase boundaries and acceptance proof |
| `plan` | Work decomposition | Turns catalog migration into bounded waves |
| `validation` / `vibe` / `council` | Proof gates | Verify skill behavior and judge ambiguous changes |

The catalog should continue to run through the existing `SKILL.md` contract
while the new contract grows around it.

## Target Skill Definition

A next-generation skill definition is not a larger prompt. It is a compact
contract where every retained token carries one of the Context Density Rule
payloads:

| Payload | Skill definition field | Example question |
|---------|------------------------|------------------|
| Intent | Trigger, behavior, scenario | What behavior should the agent produce? |
| Boundary | Bounded context, dependencies, non-goals, write scope | What should the skill not absorb? |
| Evidence | Acceptance example, test, gate, output contract | How does the agent know it is done? |
| Decision | Rationale and tradeoff | Why does this skill exist instead of a sibling skill? |
| Constraint | Safety, runtime, token, and process limits | What must not be violated? |
| Next action | Step, command, workflow transition | What does the agent do now? |

That definition becomes the narrow waist for skill evolution. The Markdown
skill remains the human-readable source, but the migration should make it
possible to derive machine-checkable contracts from the same content.

## Strangler Architecture

The strangler should wrap the existing catalog rather than replace it:

1. **V1 runtime stays live.** Existing `skills/*/SKILL.md` and
   `skills-codex/*/SKILL.md` continue to be the shipped runtime surface.
2. **V2 contract appears beside V1.** Add a `SkillDefinition` contract and
   template that names intent, boundary, evidence, decision, constraint, and
   next action explicitly.
3. **Builder emits both shapes.** `skill-builder` keeps its current modes, then
   gains a V2 template path that can materialize the new shape without breaking
   old callers.
4. **Auditor reports before it blocks.** `skill-auditor` adds advisory density
   findings first. Gates promote only after the existing catalog has a stable
   pass rate.
5. **Converter stays adapter-only.** `converter` projects the definition into
   Codex and other runtime variants. It should not own domain decisions.
6. **RPI proves the loop.** `rpi` is the exemplar for how a skill carries phase
   intent, DDD boundaries, proof checkpoints, and closeout evidence.

## Planned Artifacts

These are the artifacts to introduce in the first migration wave:

| Artifact | Purpose | Promotion rule |
|----------|---------|----------------|
| `docs/contracts/skill-definition.md` | Human-readable contract for V2 skill definitions | Documentation first, then linked from builder and auditor |
| `schemas/skill-definition.v1.schema.json` | Machine schema for dense skill payloads | Advisory validation before any blocking gate |
| `skills/skill-builder/references/skill-template-v2.md` | New template using the density payloads | Available as explicit V2 mode or option |
| `skills/skill-auditor/references/context-density-checks.md` | Report-only audit definitions | Promote individual checks after pilot wave |
| `skills/converter/references/skill-definition-bundle.md` | Runtime adapter intermediate form | Added only after builder/auditor agree on shape |

## Migration Phases

### S0: Keep the old system green

Do not rewrite the catalog before the factory tools can express the new shape.
Run current skill frontmatter, codex artifact, parity, doc-release, and
markdown checks as the guardrail.

**Acceptance:** current gates still pass after every wave.

### S1: Contract tracer bullet

Add the V2 `SkillDefinition` contract, schema, and template. Keep it advisory
and do not make it the default generator path yet.

**Acceptance:** a single exemplar skill can be represented in V2 without losing
the existing `SKILL.md` runtime shape.

### S2: Builder and auditor report-only support

Teach `skill-builder` to produce the V2 shape on request. Teach
`skill-auditor` to report density findings without failing the build.

**Acceptance:** `skill-builder` can scaffold one V2 candidate, and
`skill-auditor` reports intent, boundary, evidence, decision, constraint, and
next-action coverage.

### S3: RPI backbone migration

Use `rpi` and its immediate lifecycle children as the pilot set:

- `rpi`
- `discovery`
- `plan`
- `implement`
- `crank`
- `validation`
- `vibe`
- `council`
- `post-mortem`

**Acceptance:** each pilot skill has explicit Context Density Rule coverage,
hexagonal role, dependencies, outputs, and acceptance proof.

### S4: Factory skill migration

Migrate the skills that build or police other skills:

- `skill-builder`
- `skill-auditor`
- `heal-skill`
- `converter`
- `domain`
- `standards`

**Acceptance:** new skills can be generated, audited, converted, and repaired
without manual first-principles reconstruction.

### S5: Catalog role sweeps

Migrate the remaining catalog by role, not alphabetically. This keeps review
semantics coherent:

| Wave | Role | Examples |
|------|------|----------|
| A | Orchestrators | `dream`, `evolve`, `swarm`, `codex-team` |
| B | Execution skills | `autodev`, `pr-implement`, `release`, `push` |
| C | Validation skills | `review`, `security`, `deps`, `perf`, `test` |
| D | Knowledge skills | `harvest`, `forge`, `compile`, `ratchet`, `flywheel` |
| E | Product/docs skills | `product`, `readme`, `doc`, `oss-docs` |

**Acceptance:** each wave exits with a report of kept, merged, split, and
retired candidates.

### S6: Gate promotion

Promote only checks that have proven useful:

1. Report-only density audit.
2. Advisory CI job over pilot skills.
3. Advisory CI job over full catalog.
4. Blocking only after one clean cycle and a documented false-positive review.

**Acceptance:** no density check becomes blocking while it still produces noisy
or ambiguous findings against valid skills.

## Lease On Life Audit

Every skill gets one of four dispositions during migration:

| Disposition | Meaning | Evidence required |
|-------------|---------|-------------------|
| Keep | The skill has a distinct bounded context and active use | Trigger, output, and proof remain unique |
| Merge | The skill is a thin alias or duplicate of another skill | Shared intent and overlapping output contract |
| Split | The skill mixes multiple bounded contexts | At least two independent intents or audiences |
| Retire | The skill is stale, obsolete, or better handled by another layer | No current trigger, no unique output, or broken dependency |

The audit should not be subjective taste. A skill earns its lease by carrying
dense intent, clear boundaries, and verifiable evidence.

## First Slice

The first implementation slice should change the factory, not the whole forest:

1. Add the V2 skill-definition contract and schema.
2. Add a V2 template under `skill-builder`.
3. Add report-only context-density checks under `skill-auditor`.
4. Run the checks against `rpi`, `domain`, `plan`, and `validation`.
5. Use the findings to update the template before touching the broad catalog.

This is the tracer bullet. It proves the system can generate and judge the new
shape before asking humans or agents to migrate dozens of skills by hand.

## Non-goals

- No big-bang rewrite of all skills.
- No immediate removal of current `SKILL.md` conventions.
- No blocking density gate until the advisory checks prove low noise.
- No expansion of `heal-skill` into subjective content judgment.
- No runtime-specific decisions inside the domain contract.

## Success Criteria

The migration is working when:

- a new skill can be generated from first principles without bespoke prompting;
- an existing skill can be audited for density without human interpretation;
- the Codex runtime artifact remains an adapter projection, not a second
  source of truth;
- `rpi` and its children carry behavior, boundary, and proof across phase
  transitions;
- catalog-wide migration reports which skills are kept, merged, split, or
  retired with evidence.
