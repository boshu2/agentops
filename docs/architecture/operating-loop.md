# Operating Loop

> One-page spine. The operational discipline every AgentOps process skill executes. Companion to [Component Map](component-map.md) (product/component routing), [Ports and Adapters](ports-and-adapters.md) (the runtime seams), [Intent-to-Loop Hexagon](intent-to-loop-hexagon.md) (the process-level ports), and [CDLC](../cdlc.md) (the context lifecycle inside the SDLC control plane). RPI naming (`/rpi` skill vs the now-removed `ao rpi` CLI — removed in 3.0, use the operating loop — vs this loop): [codebase-overview — RPI terminology](codebase-overview.md#rpi-terminology).

AgentOps' execution discipline is one repeatable loop inside the SDLC control plane, not a phased waterfall of documents. Every process skill is one move within it. No artifact exists unless it advances the loop.

```text
BDD-shaped intent issue
  → vertical slices (each one a behavior, not a layer)
  → TDD per slice (first failing test, then implementation)
  → conflict-free parallel wave (only if write scopes do not collide)
  → integrated bead completion (acceptance examples pass)
  → evidence + learning capture (under the promotion ratchet)
```

## The narrow-waist micro-cycle (canonical — every loop skill cites this)

The 3.0 narrow waist ([3.0 → four load-bearing practices](../3.0.md#the-four-load-bearing-practices-the-narrow-waist)) executes, per slice, as **one** repeatable micro-cycle. This is the canonical statement of the shape; every operating-loop skill reinforces the SAME sequence rather than restating it:

```text
small batch          one behavior per slice — never a big-batch bundle           [S1]
  → BDD              the behavior written as Gherkin (Given/When/Then)            [S2]
  → ATDD             its acceptance test authored + run RED before code          [S3]
  → green            smallest implementation that flips the test green           (S3→S4)
  → refactor         refactor-UNDER-green as its own step; NEVER change a test   [S4]
  → membrane         an independent verdict binds the slice's acceptance test    [S5]
  → mine back        by-products of inference + lessons ratchet into the NEXT    [S6]
                     loop (a gate catch → a check; an escape → a new gate)
```

Authoritative source per stage (cite these, don't restate): **S1 small batch + S4 refactor-after-green** — [`agentic-workflow-evidence.md`](../../skills/standards/references/agentic-workflow-evidence.md) findings #1–#2, #6 (refactor-after-green is the load-bearing quality move; test-first *ordering* alone contributed nothing measurable — the acceptance test as *contract* is what matters, not its position); **S2/S3 BDD→ATDD** — [`behavior-first-planning`](../../skills/behavior-first-planning/SKILL.md) (no runnable acceptance test, no bead); **S4 test-shape + thoroughness-to-stakes** — [`test-pyramid.md`](../../skills/standards/references/test-pyramid.md); **S5 membrane** — [`/validate`](../../skills/validate/SKILL.md), [`/pawl-review`](../../skills/pawl-review/SKILL.md), and the [pawl-gate](../contracts/pawls.md) (`no verdict = not done`); **S6 ratchet** — move 7 below + the [3.0 ratchet rules](../3.0.md#what-makes-the-loop-compound-instead-of-repeat-the-ratchet-rules).

> **The unit of value is the proof, not the artifact.** A slice is *done* only when the membrane (S5) has written an independent verdict on it (no verdict = not done) — this is the move every skill feeds. The corpus/ratchet beneath is the (unproven, [ADR-0004](../adr/ADR-0004-corpus-moat-unproven-position-on-the-system.md)) compounding layer, not the headline; the membrane's own self-improvement (S6: escape → new check → re-measure) is the compounding that has a deterministic gradient.

The doctrine source for this spine is [`.agents/research/2026-05-16-agentops-3-cdlc-context-validation.md`](https://github.com/boshu2/agentops/blob/main/.agents/research/2026-05-16-agentops-3-cdlc-context-validation.md). Promote changes there first, then update this doc.

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
   **Why the re-routing terminates, and how the map itself improves between
   runs without oscillating, is specified in the [Control-Loop Model](control-loop-model.md)**
   (two timescales + the governor): the map is fixed *within* a run; the slow
   loop tunes it *across* runs, governed so it doesn't thrash.
8. **Single-agent-first; orchestration is opt-in escalation.** The default
   execution shape is one capable agent working in-session with good
   bookkeeping. Multi-agent orchestration — parallel waves, persistent NTM workers, Agent
   Mail coordination — is an *escalation you reach for*, never a substrate you
   start from. **Escalation trigger (observable):** escalate only when you are
   creating **two or more active lanes** — independent read/review lanes whose
   outputs a lead will merge, or independent implementation slices with
   **disjoint write scopes**. When ≥2 lanes/panes share the repo, Agent Mail
   registration and file reservations are mandatory before writes. With only
   one active writer, stay single-agent and use normal bookkeeping.
   **NTM and Agent Mail are separate adapters on different axes.**
   Persistent NTM workers answer a **durability/wall-clock** need —
   work must outlive your session or run unattended. AM (coordination) answers a
   **contention** need — ≥2 writers can touch the same path. You reach for either
   alone: Agent Mail without NTM is common for in-session lanes; NTM without
   Agent Mail is valid for one writer or a file-disjoint queue. **Asymmetry
   guardrail:** the de-mandate removes the single-writer *session-start tax*, not
   the *collision guard* — the `≥2-writers → reserve` reflex stays non-negotiable
   (an unneeded AM call costs one command; a missing one silently clobbers a
   shared file). Portable lifecycle: [`agent-native`](../../skills/agent-native/SKILL.md);
   pane mechanics: [`ntm`](../../skills/ntm/SKILL.md).
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

Template: [`docs/templates/intent-issue.md`](../templates/intent-issue.md). Skills that produce this artifact: `/discovery`, `/product`, `/plan`.

### 2. Track as a bead when it leaves the head

A bead is the linked-intent packet for one BDD-shaped behavior change. It carries the acceptance examples, the bounded-context tag, the slice list, the wave plan, accumulating evidence, and residual gaps at close. One-shot work that stays inside a single prompt does not need a bead. Skill: `beads-br` (via `br`; while legacy `.beads/` retirement is in progress, invoke as `BEADS_DIR="$(ao beads dir)" br ...`).

### 3. Slice vertically through behavior

A good slice maps to one Given/When/Then row, has a nameable first failing test, has a review-in-one-pass write scope, and touches one bounded context. "Refactor then feature" is two slices. Skill: `/plan` produces the slice list.

### 4. TDD per slice

Per slice, in order:

1. First failing test — must fail for the *right reason* (missing behavior, not syntax).
2. Smallest change that flips it to green.
3. Refactor under green. Refactor is its own commit.
4. Record evidence into the bead.

Skill: `/implement` operates on one slice at a time.

**Step 3 is the load-bearing move, and one behavior per cycle is the batch.** A controlled study of
agent-run workflows (Finster 2026 — see the `standards` skill's `agentic-workflow-evidence` reference)
found that stripping refactor-under-green out of TDD erased its entire quality advantage, while
test-first *ordering* alone contributed nothing measurable, and small batches (one behavior per
cycle) beat all-at-once across the board. Two invariants follow: refactor after **every** green
(not deferred to one final pass — deferred-refactor workflows were the worst-performing cluster),
and **never let a refactor step change a test** (a test change during refactor means behavior
changed — that is a new slice, not a refactor). The repo keeps the first failing test as the
default because it is the slice's contract and its regression guard; code-first / test-after
(`--no-tdd`) is a defensible cost-efficient variant on fully-specified small tasks *as long as*
those two invariants hold.

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

Any failed row → slices run **sequential**. Skill: `/plan` declares the wave; `/crank`, `/swarm`, `/evolve` execute it.

### 6. Close the bead by proving its acceptance

Every Given/When/Then maps to a passing test. Every non-goal is still untouched. Every rollback path is reachable. Evidence is recorded. Activity logs do not close beads. Skills: `/validate`, `/council`, `/pawl-review`; `ao pawl` binds the verdict.

The land itself is one verb: **`ao land <bead>`** is the canonical land path. It builds a fresh in-checkout `ao` and re-execs through it so the pawl review runs the LIVE/trusted path (an installed `ao` fails `aoBinaryInside` and takes the cold, un-auto-binding stranger path), pins `AO_BIN` for preflight + the gate, runs `ao pawl review <bead> --scope head` (cross-family codex refuter; **auto-binds** the commit-bound verdict on CONFIRM — no hand `ao provenance emit-verdict`/`#trivial` bind step), then hands off to the atomic land machinery (`scripts/pawl-land.sh`: rebase `origin/main` → restamp → single push through the deterministic pre-push gate — the *windshield*). REFUTED / NO-VERDICT stops the land (exit non-zero); no verdict = not done.

When a cycle is logged, the CycleTrace can carry the closeout join explicitly:
`bead_id`, `acceptance_examples`, `validation_commands`, and
`closeout_verdict`. That join is the reviewer path from a bead's Gherkin
example to the test, gate, or eval that proved it.

### 7. Capture evidence and learning, then ratchet

Two outputs per loop turn — evidence into `.agents/rpi/`, the bead, and the relevant council/validation artifacts; learnings only if they cleared the promotion bar (next section). Skill: `/post-mortem` (primary).

**S6 is not "write a learning" — it is "make the NEXT loop consume it."** A by-product of inference or a lesson learned is durable only when it lands as something a later move will *read* (the 3.0 rule "knowledge becomes constraints"). Route by class: a **membrane escape** (CONFIRMED-but-later-wrong) compiles into a mechanical gate/check (the escape→check ratchet); a **judgment lesson** compiles into a `/plan` planning-rule, a `/pre-mortem` check, or — when a gate caught a defect a green test missed — a new dimension appended to `docs/gate/findings-ledger.md`, which is exactly the ledger [`behavior-first-planning`](../../skills/behavior-first-planning/SKILL.md) reads to ratchet its Standing Review Dimensions. If nothing downstream reads the artifact, S6 did not happen — it is a landfill entry, not a ratchet ([3.0 ratchet rules](../3.0.md#what-makes-the-loop-compound-instead-of-repeat-the-ratchet-rules)). The corpus-flywheel skills `/curate`, `/flywheel`, and `/compile` are retired (2026-07-07 wave — folded into `/post-mortem`'s mining surface; mechanical surfaces live on the CLI: `ao compile`, `ao flywheel status`) and are no longer part of the primary ratchet.

### The loop closes here: re-plan on evidence, not just on failure

Move 7 feeds **back into move 1** — this is where the route gets re-routed (principle 7). The sharpening principle 7 leaves implicit: re-routing is triggered by **evidence, not only by failure.** A wave that *succeeds* still teaches something the plan didn't know, and that evidence may **refactor, insert, drop, reorder, or re-scope the *remaining* waves** before the next one runs. The wave plan is a hypothesis; each wave is the experiment that tests it. Under `--auto` the orchestrator executes those pivots itself — it is not gated on a wave *failing* first, and it does not run the initial wave-list to the letter. Two failure modes this kills: **retry-not-replan** (re-cranking a failed wave forever instead of asking whether the *remaining plan* should change) and **waterfall** (executing the pre-written wave list because "that was the plan"). Bounded by the run's circuit breakers (budget / attempt cap / oscillation detection) and the ≥5-arc post-mortem checkpoint; the operator is surfaced only at the terminal objective or a breaker trip — never just to approve a pivot. The orchestrator that owns this across a turn is `/rpi`; full mechanics: [Agile Re-Plan Loop](../../skills/rpi/references/agile-replan-loop.md).

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
| Shape intent | `discovery`, `product`, `plan` | BDD intent issue with acceptance examples |
| Track as bead | `beads-br` | Bead with slice list + acceptance contract |
| Slice + wave plan | `plan` | Slice list + wave grouping + ownership map |
| Pre-flight check | `pre-mortem`, `council` | Verdict on plan + wave validity |
| TDD per slice | `implement` | First failing test → green → refactor |
| Wave execution | `crank`, `swarm`, `evolve` | Parallel slices with explicit ownership |
| Slice validation | `validate`, `council`, `pawl-review` | Acceptance proof plus independent lane evidence |
| Bead acceptance | `validate`, `council` | Roll-up acceptance verdict |
| Capture | `post-mortem`, `forge` | Evidence + promoted learnings |
| Compound | `pattern-mining`, `operationalize` | Earned patterns → rules → weakest durable mechanism |

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
| `.agents/` accumulates one-off observations forever | Move 7 + promotion rule: most observations die at handoff |
| A "refactor + feature" PR mixes contracts | Move 3: refactor and feature are two slices |
| Layer-by-layer waterfall reappears under "phases" | Move 3 + move 1: slices are vertical and BDD-shaped |

## What this doctrine deliberately does NOT do

- Does not introduce a new `skills/cdlc/` skill — the spine is doc-shaped, referenced by every process skill.
- Does not introduce new practice slugs — the loop is a composition of `bdd-gherkin` + `tdd` + `ddd-bounded-context` + `hexagonal-architecture` + `agile-manifesto` + `pragmatic-programmer` + `continuous-delivery`.
- Does not couple AgentOps to any consumer's domain vocabulary — bounded contexts are named by the consuming repo.
- Does not require new tooling — `br` and existing validation gates carry the load.
- Does not enforce parallelism — parallel waves are an optimization unlocked by the conflict-free check, not a default.

## See also

- [`.agents/research/2026-05-16-agentops-3-cdlc-context-validation.md`](https://github.com/boshu2/agentops/blob/main/.agents/research/2026-05-16-agentops-3-cdlc-context-validation.md) — doctrine source (promote changes here first)
- [Component Map](component-map.md) — product/component routing and trim/defer posture
- [Ports and Adapters](ports-and-adapters.md) — architectural seams the loop runs through
- [Fungibility Charter](fungibility-charter.md) — the six doctrinal commitments behind the loop's stateless, role-free, single-model-default agents
- [Effective Feedback Compute](../doctrine/effective-feedback-compute.md) — *why* moves 6 (prove acceptance) and 7 (ratchet) are load-bearing: harness success scales with useful feedback (I·V·R·M), not raw spend
- [Control-Loop Model](control-loop-model.md) — *why* the loop converges and self-improves: the two-timescale control system (fast=convergence, slow=governed improvement) + the conformance contract every Workflow/skill must satisfy
- [Intent-to-Loop Hexagon](intent-to-loop-hexagon.md) — process-level ports/adapters from BDD intent through evidence ratchet
- [ADR-0001](../adr/ADR-0001-ddd-hexagonal-adoption.md) — DDD + Hexagonal adoption
- [CDLC](../cdlc.md) — conceptual seven phases this loop runs inside
- [Context Map](../contracts/context-map.md) — bounded contexts and skill roles
- [`docs/templates/intent-issue.md`](../templates/intent-issue.md) — BDD intent issue template
- [`docs/templates/slice-validation.md`](../templates/slice-validation.md) — per-slice validation plan template
- [`PRACTICE-REGISTRY.md`](https://github.com/boshu2/agentops/blob/main/PRACTICE-REGISTRY.md) — practice slug registry
- [`GOALS.md`](https://github.com/boshu2/agentops/blob/main/GOALS.md) Directive #12 — fitness gate that enforces this loop for non-trivial work
