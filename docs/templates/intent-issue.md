# Intent Issue Template (BDD-shaped)

> Copy this file when shaping a new piece of work. The issue is **not ready** until every section below is filled in and the acceptance examples are testable. Skill that produces this artifact: [`/discovery`](../../skills/discovery/SKILL.md) (and `/brainstorm` for the earlier free-text → structured pass).
>
> See [`docs/architecture/operating-loop.md`](../architecture/operating-loop.md) for why this template exists and where it sits in the loop.

---

## Feature

> One sentence. The capability being added or changed, in the consumer's language (not the implementation's).

## Bounded context

> Which bounded context from [`docs/contracts/context-map.md`](../contracts/context-map.md) does this work belong to? If it crosses contexts, this is two issues, not one.

## Domain terms

> Domain terms used below, each anchored to the ubiquitous-language register at [`skills/domain/references/`](../../skills/domain/references/) or [`skills/standards/references/architecture-terms.md`](../../skills/standards/references/architecture-terms.md). New terms must be added to the register before they are used here.

- **<Term 1>** — definition + register link
- **<Term 2>** — …

## Acceptance examples

> At least one happy path and at least one critical edge. Each example must be testable as written. "It should work" is not an example.

```gherkin
Feature: <feature name from above>

  Scenario: <happy path name>
    Given <precondition phrased in domain terms>
    When <action the actor takes>
    Then <observable outcome>
    And <secondary observable outcome, if any>

  Scenario: <critical edge case name>
    Given <precondition>
    When <action>
    Then <observable outcome that proves the edge is handled>
```

## Non-goals

> Things this issue will explicitly **not** do. Anything not listed under acceptance examples and not listed here is out of scope by default — list the ones a reasonable reader might expect to be in scope so the boundary is loud.

- <Non-goal 1>
- <Non-goal 2>

## Rollback / containment path

> How do we undo if this goes wrong? Name the concrete mechanism: feature flag, schema migration with `down`, branch revert, config toggle, etc. If no rollback exists, say so explicitly — that is itself useful information.

- <Rollback step or "not rollback-able; the containment is X">

## Evidence needed for completion

> What proves the acceptance examples passed? Be specific — test names, snapshot keys, eval suite names, council verdicts, citation events. The bead does not close without these artifacts existing.

- Test: `<test path:name>` covering Scenario 1
- Test: `<test path:name>` covering Scenario 2 (edge)
- Snapshot / golden: `<path or "n/a">`
- Eval suite: `<suite name or "n/a">`
- Council verdict: `<required preset(s) or "n/a">`
- Other evidence: `<e.g., ratchet entry id, GOALS measure pass>`

## Vertical slice candidates

> Initial slice list, one per acceptance example (minimum). `/plan` will refine this into the final slice + wave plan. Each slice must have a nameable first failing test, a write-scope sketch, and a bounded-context tag (defaults to the one above).

| Slice ID | Scenario | First failing test (proposed) | Write scope (proposed) | Notes |
|----------|----------|-------------------------------|------------------------|-------|
| S1 | <name from acceptance examples> | `<test path:name>` | `<files / packages>` | <e.g., "depends on S2 — sequential"> |
| S2 | … | … | … | … |

## Linked artifacts

- Parent bead: `<bd id or "to be created">`
- ADR (if architectural): `<adr id or "n/a">`
- Prior research: `<.agents/research/*.md or "n/a">`
- Pre-mortem: `<.agents/council/YYYY-MM-DD-pre-mortem-*.md or "to run">`

---

## Readiness checklist

A `/pre-mortem` or `/council` must verify these before the issue leaves discovery:

- [ ] Acceptance examples are written in Given/When/Then and each is testable as written
- [ ] Bounded context is named and present in the context map
- [ ] All domain terms used are registered in the ubiquitous-language register
- [ ] Non-goals are explicit
- [ ] Rollback or containment path is named (or its absence is named explicitly)
- [ ] Evidence list points to concrete artifacts, not vague descriptions
- [ ] Slice candidates exist (at least one per acceptance example)

If any box is unchecked, the issue is not ready — send it back to `/discovery` or `/brainstorm`.
