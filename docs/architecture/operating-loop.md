# Operating Loop

> One-page spine. The operational discipline every AgentOps process skill executes. Companion to [Ports and Adapters](ports-and-adapters.md) (the architectural seams) and [CDLC](../cdlc.md) (the context lifecycle inside the SDLC control plane).

AgentOps' execution discipline is one repeatable loop inside the SDLC control plane, not a phased waterfall of documents. Every process skill is one move within it. No artifact exists unless it advances the loop.

```text
BDD-shaped intent issue
  → vertical slices (each one a behavior, not a layer)
  → TDD per slice (first failing test, then implementation)
  → conflict-free parallel wave (only if write scopes do not collide)
  → integrated bead completion (acceptance examples pass)
  → evidence + learning capture (under the promotion ratchet)
```

The doctrine source for this spine is [`.agents/research/2026-05-15-cdlc-dojo-doctrine.md`](https://github.com/boshu2/agentops/blob/main/.agents/research/2026-05-15-cdlc-dojo-doctrine.md). Promote changes there first, then update this doc.

## Governing principles

1. **The loop is the primitive, not the documents.** If an artifact does not advance behavior toward acceptance, enable parallel work, preserve human authority, or become a reusable gate, it is token drag.
2. **Behavior is the unit of work, not a layer.** A slice cuts vertically through whatever layers are needed to demonstrate one Given/When/Then.
3. **The first failing test is the slice's contract.** Code without a failing test has no acceptance surface; an agent has no way to know when it is done.
4. **Parallelism is explicit ownership.** Waves are valid only when the conflict-free check below passes. Default to sequential.
5. **Less process, more executable shared language.** The promotion ratchet kills artifacts that do not change future behavior.
6. **Context crosses boundaries as artifacts.** RPI keeps orchestration visible,
   but phase execution should cross through bounded packets and summaries, not
   raw accumulated chat context.

## The seven moves

### 1. Shape intent as BDD

The intent issue is not ready until the acceptance examples are testable. Required surface:

- Feature / capability name
- Given / When / Then examples (one happy path + at least one edge)
- Domain terms used (anchored to the repo's ubiquitous-language register; for AgentOps that is [`skills/domain/references/`](../../skills/domain/references/) and [`skills/standards/references/architecture-terms.md`](https://github.com/boshu2/agentops/blob/main/skills/standards/references/architecture-terms.md))
- Bounded context per the [context map](../contracts/context-map.md)
- Non-goals
- Rollback / containment path
- Evidence needed for completion (test names, snapshot keys, eval suites, council verdicts)

Template: [`docs/templates/intent-issue.md`](../templates/intent-issue.md). Skills that produce this artifact: `/brainstorm`, `/discovery`, `/design`.

### 2. Track as a bead when it leaves the head

A bead is the linked-intent packet for one BDD-shaped behavior change. It carries the acceptance examples, the bounded-context tag, the slice list, the wave plan, accumulating evidence, and residual gaps at close. One-shot work that stays inside a single prompt does not need a bead. Skill: `/beads` (via `bd`).

### 3. Slice vertically through behavior

A good slice maps to one Given/When/Then row, has a nameable first failing test, has a review-in-one-pass write scope, and touches one bounded context. "Refactor then feature" is two slices. Skill: `/plan` produces the slice list.

### 4. TDD per slice

Per slice, in order:

1. First failing test — must fail for the *right reason* (missing behavior, not syntax).
2. Smallest change that flips it to green.
3. Refactor under green. Refactor is its own commit.
4. Record evidence into the bead.

Skill: `/implement` operates on one slice at a time.

### 5. Group into a wave only when write scopes do not collide

Wave validity is a hard gate, applied row by row:

| Check | Pass means |
|------|-----------|
| Distinct write scopes | Each slice's modified-files set is disjoint |
| Distinct test targets | Tests run independently; no shared fixture mutation |
| No shared migration | At most one slice per migration / schema / generated file |
| No shared CLI surface | At most one slice per command's flags or arguments |
| Integration order declared | Merge order is named if it matters |
| Owner per slice | One agent or one human per slice — no joint ownership |
| Discard path per slice | Every slice has a rollback or drop-and-re-plan exit |

Any failed row → slices run **sequential**. Skill: `/plan` declares the wave; `/crank`, `/swarm`, `/autodev` execute it.

### 6. Close the bead by proving its acceptance

Every Given/When/Then maps to a passing test. Every non-goal is still untouched. Every rollback path is reachable. Evidence is recorded. Activity logs do not close beads. Skills: `/validation`, `/council`, `/vibe`.

### 7. Capture evidence and learning, then ratchet

Two outputs per loop turn — evidence into `.agents/ratchet/` and the bead; learnings only if they cleared the promotion bar (next section). Skills: `/post-mortem`, `/forge`, `/retro`, `/ratchet`, `/flywheel`, `/harvest`.

## The promotion ratchet

Do not run full ceremony for every observation. Promote progressively:

| Trigger | Goes to |
|---------|---------|
| Noticed once | Stays in the handoff. Dies when the handoff ages out. |
| Repeats twice across sessions or beads | `.agents/learnings/<slug>.md` |
| Changes future agent behavior | Update a SKILL.md or a template under `docs/templates/` |
| Must never regress | Add a validation gate (warn-only first, then blocking) |
| Becomes core doctrine | Promote into PRODUCT.md / GOALS.md / docs/cdlc.md |

The ratchet is what keeps `.agents/` from becoming a landfill. Compounding only happens when capture meets pruning.

## Skill → loop-move map

| Loop move | Primary skills | Produces |
|-----------|----------------|----------|
| Shape intent | `brainstorm`, `discovery`, `design` | BDD intent issue with acceptance examples |
| Track as bead | `beads` | Bead with slice list + acceptance contract |
| Slice + wave plan | `plan` | Slice list + wave grouping + ownership map |
| Pre-flight check | `pre-mortem`, `council` | Verdict on plan + wave validity |
| TDD per slice | `implement` | First failing test → green → refactor |
| Wave execution | `crank`, `swarm`, `autodev` | Parallel slices with explicit ownership |
| Slice validation | `vibe`, `validate`, `validation` | Per-slice acceptance proof |
| Bead acceptance | `validation`, `council` | Roll-up acceptance verdict |
| Capture | `post-mortem`, `forge`, `retro`, `ratchet` | Evidence + ratcheted learnings |
| Compound | `flywheel`, `harvest`, `dream` | Learnings → patterns → rules → gates |

## How the loop composes with the architectural seams

The loop is operational discipline. The architectural seams are structural. They are orthogonal and they compose:

- **Bounded contexts** ([context map](../contracts/context-map.md)) — every slice declares which bounded context it touches. A slice that crosses contexts is two slices.
- **Ports** (`cli/internal/ports/`) — the first failing test for a slice that touches a port can be written against the port interface before any adapter exists.
- **Adapters** (`cli/internal/adapters/`) — adapter changes are slices like any other. The first failing test calls the adapter through the port; the port stays stable.
- **Domain purity** ([ADR-0001](../adr/ADR-0001-ddd-hexagonal-adoption.md)) — slices that change `cli/internal/domain/` must keep the no-import-from-internal/* invariant. The wave check treats domain-purity as a shared concern: at most one slice per wave touches domain types.

## Failure modes the loop prevents

| Failure mode | Loop move that prevents it |
|--------------|----------------------------|
| Agent writes code with no contract | Move 4: first failing test before implementation |
| Two agents stomp on the same file in parallel | Move 5: wave-validity write-scope check |
| Bead closes with "looks good" instead of evidence | Move 6: every Given/When/Then maps to a passing test |
| `.agents/` accumulates one-off observations forever | Move 7 + ratchet: most observations die at handoff |
| A "refactor + feature" PR mixes contracts | Move 3: refactor and feature are two slices |
| Layer-by-layer waterfall reappears under "phases" | Move 3 + move 1: slices are vertical and BDD-shaped |

## What this doctrine deliberately does NOT do

- Does not introduce a new `skills/cdlc/` skill — the spine is doc-shaped, referenced by every process skill.
- Does not introduce new practice slugs — the loop is a composition of `bdd-gherkin` + `tdd` + `ddd-bounded-context` + `hexagonal-architecture` + `agile-manifesto` + `pragmatic-programmer` + `continuous-delivery`.
- Does not couple AgentOps to any consumer's domain vocabulary — bounded contexts are named by the consuming repo.
- Does not require new tooling — `bd`, `ratchet`, and existing validation gates carry the load.
- Does not enforce parallelism — parallel waves are an optimization unlocked by the conflict-free check, not a default.

## See also

- [`.agents/research/2026-05-15-cdlc-dojo-doctrine.md`](https://github.com/boshu2/agentops/blob/main/.agents/research/2026-05-15-cdlc-dojo-doctrine.md) — doctrine source (promote changes here first)
- [Ports and Adapters](ports-and-adapters.md) — architectural seams the loop runs through
- [ADR-0001](../adr/ADR-0001-ddd-hexagonal-adoption.md) — DDD + Hexagonal adoption
- [CDLC](../cdlc.md) — conceptual seven phases this loop runs inside
- [Context Map](../contracts/context-map.md) — bounded contexts and skill roles
- [`docs/templates/intent-issue.md`](../templates/intent-issue.md) — BDD intent issue template
- [`docs/templates/slice-validation.md`](../templates/slice-validation.md) — per-slice validation plan template
- [`PRACTICE-REGISTRY.md`](https://github.com/boshu2/agentops/blob/main/PRACTICE-REGISTRY.md) — practice slug registry
- [`GOALS.md`](https://github.com/boshu2/agentops/blob/main/GOALS.md) Directive #12 — fitness gate that enforces this loop for non-trivial work
