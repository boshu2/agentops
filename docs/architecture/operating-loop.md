# Operating Loop

> One-page spine. The operational discipline every AgentOps process skill executes. Companion to [Component Map](component-map.md) (product/component routing), [Ports and Adapters](ports-and-adapters.md) (the runtime seams), [Intent-to-Loop Hexagon](intent-to-loop-hexagon.md) (the process-level ports), and [CDLC](../cdlc.md) (the context lifecycle inside the SDLC control plane).

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
7. **The map is fixed; the route is re-routed.** This loop is a deterministic
   role-topology — its stages, legal transitions, and gates do not change per
   goal (the map). The path a given goal takes through it is dynamic and
   recalculated on failure (the route). Because the worker is stochastic, you
   trust the map and the gates, not the agent: the gate at move 6 is the
   *windshield* — deterministic ground-truth that catches a confident
   hallucination (a road that was never there) which re-routing alone cannot.
   See [3.0 → the navigator model](../3.0.md#why-a-loop-and-not-a-pipeline-the-navigator-model).
8. **Single-agent-first; orchestration is opt-in escalation.** The default
   execution shape is one capable agent working in-session with good
   bookkeeping. Multi-agent orchestration — parallel waves, ATM swarms, Agent
   Mail coordination — is an *escalation you reach for*, never a substrate you
   start from. **Escalation trigger (observable):** escalate only when you are
   creating **two or more active lanes** — independent read/review lanes whose
   outputs a lead will merge, or independent implementation slices with
   **disjoint write scopes**. When ≥2 lanes/panes share the repo, Agent Mail
   registration and file reservations are mandatory before writes. With only
   one active writer, stay single-agent and use normal bookkeeping.
   **ATM and AM are *separate* escalations on different axes — never a package.**
   ATM (the out-of-session substrate) answers a **durability/wall-clock** need —
   work must outlive your session or run unattended. AM (coordination) answers a
   **contention** need — ≥2 writers can touch the same path. You reach for either
   *alone*: AM-without-ATM is the common case (two in-session lanes sharing a
   repo); ATM-without-AM is an unattended **file-disjoint** queue. **Asymmetry
   guardrail:** the de-mandate removes the single-writer *session-start tax*, not
   the *collision guard* — the `≥2-writers → reserve` reflex stays non-negotiable
   (an unneeded AM call costs one command; a missing one silently clobbers a
   shared file). Full 4-case matrix: [`using-atm`](../../skills/using-atm/SKILL.md#when-to-use-atm-vs-am-the-4-case-matrix).
   (Shape routing detail: [`automation-shape-routing`](../../skills/automation-shape-routing/SKILL.md)
   — "shape 0" is the default front door; `AGENTOPS_ORCHESTRATION=off` pins the
   beads floor.)

## The seven moves

### 1. Shape intent as BDD

The intent issue is not ready until the acceptance examples are testable. Required surface:

- Feature / capability name
- Given / When / Then examples (one happy path + at least one edge)
- Domain terms used (anchored to the repo's ubiquitous-language register; for AgentOps that is [`skills/domain/references/`](../../skills/domain/references/) and [`skills/standards/references/architecture-terms.md`](https://github.com/boshu2/agentops/blob/main/skills/standards/references/architecture-terms.md))
- Component and bounded-context route per the [Component Map](component-map.md); generated skill-role context per the [context map](../contracts/context-map.md)
- Non-goals
- Rollback / containment path
- Evidence needed for completion (test names, snapshot keys, eval suites, council verdicts)

Template: [`docs/templates/intent-issue.md`](../templates/intent-issue.md). Skills that produce this artifact: `/brainstorm`, `/discovery`, `/design`.

### 2. Track as a bead when it leaves the head

A bead is the linked-intent packet for one BDD-shaped behavior change. It carries the acceptance examples, the bounded-context tag, the slice list, the wave plan, accumulating evidence, and residual gaps at close. One-shot work that stays inside a single prompt does not need a bead. Skill: `/beads` (via `br`; while legacy `.beads/` retirement is in progress, invoke as `BEADS_DIR=$PWD/_beads br ...`).

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

When a cycle is logged, the CycleTrace can carry the closeout join explicitly:
`bead_id`, `acceptance_examples`, `validation_commands`, and
`closeout_verdict`. That join is the reviewer path from a bead's Gherkin
example to the test, gate, or eval that proved it.

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

**R3 self-enforcement (no learning without a constraint).** The "Must never regress → add a validation gate" rung used to be prose only — a learning could be promoted to a durable maturity tier without ever compiling into a gate/test/rule. `scripts/check-ratchet-r3-constraint.sh` enforces it against the live (gitignored) `.agents/learnings/` corpus: any durable-tier learning (`candidate`/`established`/`canonical`/`stable`/`promoted`) that cites no constraint — a `scripts/`/`.github/workflows/` gate, a `_test.go`/`tests/` reference, a `skills/**/SKILL.md` step, or a `constraint:`/`enforced_by:` frontmatter field — is flagged. Warn-only by default; `--strict` (or `RATCHET_R3_BLOCKING=true`) makes it blocking, mirroring the same warn-then-fail ladder. A CI path-filter gate is intentionally *not* used because the learnings corpus is gitignored (dead-by-design, like the retired learning-coherence job); the script's own correctness is gated by `tests/scripts/check-ratchet-r3-constraint.bats`.

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

- **Bounded contexts** ([Component Map](component-map.md), generated [context map](../contracts/context-map.md)) — every slice declares which bounded context it touches. A slice that crosses contexts is two slices.
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
- Does not require new tooling — `br`, `ratchet`, and existing validation gates carry the load.
- Does not enforce parallelism — parallel waves are an optimization unlocked by the conflict-free check, not a default.

## See also

- [`.agents/research/2026-05-15-cdlc-dojo-doctrine.md`](https://github.com/boshu2/agentops/blob/main/.agents/research/2026-05-15-cdlc-dojo-doctrine.md) — doctrine source (promote changes here first)
- [Component Map](component-map.md) — product/component routing and trim/defer posture
- [Ports and Adapters](ports-and-adapters.md) — architectural seams the loop runs through
- [Fungibility Charter](fungibility-charter.md) — the six doctrinal commitments behind the loop's stateless, role-free, single-model-default agents
- [Intent-to-Loop Hexagon](intent-to-loop-hexagon.md) — process-level ports/adapters from BDD intent through evidence ratchet
- [ADR-0001](../adr/ADR-0001-ddd-hexagonal-adoption.md) — DDD + Hexagonal adoption
- [CDLC](../cdlc.md) — conceptual seven phases this loop runs inside
- [Context Map](../contracts/context-map.md) — bounded contexts and skill roles
- [`docs/templates/intent-issue.md`](../templates/intent-issue.md) — BDD intent issue template
- [`docs/templates/slice-validation.md`](../templates/slice-validation.md) — per-slice validation plan template
- [`PRACTICE-REGISTRY.md`](https://github.com/boshu2/agentops/blob/main/PRACTICE-REGISTRY.md) — practice slug registry
- [`GOALS.md`](https://github.com/boshu2/agentops/blob/main/GOALS.md) Directive #12 — fitness gate that enforces this loop for non-trivial work
