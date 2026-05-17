# Slice Validation Plan Template

> Copy this file when planning the validation surface for a bead. One row per vertical slice; the roll-up at the bottom proves the bead's acceptance examples. Skill that produces this artifact: [`/plan`](../../skills/plan/SKILL.md) (drafts) and [`/validation`](../../skills/validation/SKILL.md) (executes and rolls up).
>
> See [`docs/architecture/operating-loop.md`](../architecture/operating-loop.md) for the loop position and [`docs/templates/intent-issue.md`](intent-issue.md) for the upstream artifact this validation plan checks against.

---

## Bead reference

- **Bead id:** `<bd id>`
- **Intent issue:** `<path to filled-in intent-issue.md or bd issue url>`
- **Bounded context:** `<from intent issue>`

## Slice-level validation

> One row per slice from the intent issue's slice candidates. Each slice must have a **first failing test** before any implementation begins (the TDD discipline of move 4 in the operating loop).
> In beads and CycleTrace payloads this may be named **First failing proof** when the proof is a gate, smoke script, or eval rather than a Go/Python unit test.

| Slice ID | Behavior under test | First failing proof/test | Implementation scope | Validation lane | Evidence on green | Acceptance example covered |
|----------|--------------------|--------------------|----------------------|-----------------|--------------------|----------------------------|
| S1 | <single Given/When/Then behavior> | `<file:test_name>` — must fail for the right reason (missing behavior, not syntax) | `<files / packages this slice modifies>` | L1 unit / L2 integration / L3 e2e / property / snapshot / council | <test verdict + snapshot id + ratchet entry> | <Scenario name from intent issue> |
| S2 | … | … | … | … | … | … |
| … | … | … | … | … | … | … |

### Validation lane reference

| Lane | What it covers | When to choose it |
|------|----------------|-------------------|
| L1 unit | Single function or method, no external deps | Pure logic, parsers, invariants on small types |
| L2 integration | Multiple internal units, real adapters where cheap | The default for behavioral slices — where bugs are actually found |
| L3 e2e | Full workflow including external systems | Reserve for slices that prove a whole-system contract |
| property | Invariant under generated input | Aggregate roots, parsers, state machines |
| snapshot / golden | Output stability against a frozen baseline | Generators, formatters, doc emitters |
| council | Multi-judge verdict on non-mechanical correctness | Design decisions, plan quality, taste-level checks |

## Wave validity

> If any slices are planned to run in parallel, every row of this gate must pass. Any failed row → those slices run **sequential**. See [`docs/architecture/operating-loop.md`](../architecture/operating-loop.md) move 5 for the full rationale.

| Check | Status | Notes |
|-------|--------|-------|
| Distinct write scopes (modified-files sets are disjoint) | [ ] | <list overlapping files if any> |
| Distinct test targets (no shared fixture mutation) | [ ] | |
| No shared migration / schema / generated file | [ ] | |
| No shared CLI surface (flags / arguments) | [ ] | |
| Integration order declared if it matters | [ ] | <named order or "n/a"> |
| Owner per slice (one agent or one human, no joint) | [ ] | <S1: agent-a, S2: agent-b, …> |
| Discard path per slice (rollback or drop-and-re-plan) | [ ] | |

**Wave decision:** [ ] parallel  [ ] sequential

## Roll-up acceptance

> The bead closes only when every Given/When/Then from the intent issue has a passing test linked to it. Activity logs do not close beads.

| Acceptance example from intent issue | Slice(s) that cover it | Passing-test evidence | Status |
|---------------------------------------|------------------------|------------------------|--------|
| Scenario: <happy path name> | S1 | `<file:test_name>` — green at `<git sha>` | [ ] |
| Scenario: <edge name> | S2 | `<file:test_name>` — green at `<git sha>` | [ ] |
| Scenario: … | … | … | [ ] |

## Residual gaps at close

> Anything descoped or deferred during the loop. Each entry must say where it goes next: a new bead, a learning, a planning rule, a gate proposal, or explicit acceptance of the gap.

- <Gap 1> — → <new bead id / learning slug / gate proposal / accepted>
- <Gap 2> — → …

## Evidence index

> Concrete artifacts produced during this bead's execution, suitable for `.agents/ratchet/` and for council retrieval.

- Tests added: `<paths>`
- Snapshots updated: `<paths>`
- Council verdicts: `<verdict ids>`
- Pre-mortem outcome: `<.agents/council/YYYY-MM-DD-pre-mortem-*.md>`
- Post-mortem learning: `<.agents/learnings/*.md or "stayed in handoff per ratchet">`
- Ratchet entry: `<entry id>`

---

## Closing checklist

- [ ] Every slice has a first failing proof/test linked, and that proof failed before implementation began
- [ ] Every slice's evidence row is filled in with a concrete artifact, not a description
- [ ] If any slices ran in parallel, every wave-validity row was checked at the time of wave start
- [ ] Every acceptance example from the intent issue maps to at least one passing test
- [ ] Every non-goal from the intent issue is still untouched
- [ ] Residual gaps each have a next-step disposition
- [ ] At most one learning was promoted to `.agents/learnings/` (most observations died at handoff per the promotion ratchet)
