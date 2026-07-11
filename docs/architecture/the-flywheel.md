# The Flywheel — a 4-loop control system for high-yield autonomous RPI

> **Status:** Proposed / north-star (2026-07-09). This is the control-theory view of the
> [operating loop](operating-loop.md) plus the autonomy+discipline layer above it. It is a
> **model**, not a finished build — the status ledger at the end marks what is landed vs
> specced vs aspirational. Honest scope: this makes **no** compounding-moat claim
> ([ADR-0004](../adr/ADR-0004-corpus-moat-unproven-position-on-the-system.md),
> [ADR-0011](../adr/ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md)
> stay unproven). It claims a *shape*, and marks each edge's evidence.

## The one-line claim

**RPI is agile/CD/XP applied to stochastic agents.** Naming it that de-risks the design:
we are not inventing a loop shape and hoping — we are porting a control system with twenty
years of proof (small batches, red→green→refactor, continuous integration, the andon /
stop-the-line). The "team" is agents; the CI acceptance gate is the membrane. Everything
below is that port.

## The four loops (goal-driven)

A single **goal/intent** is the setpoint. It does not command "build X for 8 hours"; it
commands "run *this loop* toward X, and park what needs me." The operating loop is the
**filter** the goal passes through:

```
goal (bounded: done-condition + scope + andon policy)
  → /rpi  ── the outer driver
      → DISCOVERY loop   — re-plans until the plan survives the PRE-MORTEM loop
      → CRANK loop       — iterates small waves until each is complete
      → VALIDATE         — emits a verdict; on FAIL, routes back to DISCOVERY
```

1. **/rpi** — the outer driver; the goal is its input.
2. **Discovery loop** — iterates (with the inner pre-mortem adversarial loop) until the plan
   is sound. Decomposes the goal into a DAG of **small vertical slices**. It is also the
   **re-plan engine**, re-invoked whenever a slice teaches something (not only at the start).
3. **Crank loop** — runs the slices in small batches until each wave is complete.
4. **Validate** — **does not loop.** It is a *sensor*: it emits a verdict + a reason. On FAIL,
   the *driver* re-plans (routes back to discovery). See "Validate is a sensor" below.

## Validate is a sensor, not a controller

The single most load-bearing shape decision. Validate's one job is to emit a trustworthy
verdict + reason. It must **not** own the loop, because the decision on FAIL (re-crank?
re-discover? the acceptance criteria were wrong? this is a one-way door — stop and ask a
human?) needs context validate does not have. Fusing judgment and control makes both worse —
the same separation the operating loop already draws between deterministic gates and the
orchestrator.

So the loop belongs to the **driver**, at three timescales, and conflating them is the trap:
- **Crank waves** — innermost (re-implement until the wave's tests pass).
- **/rpi's re-plan engine** — the mid loop (validate FAIL → discovery re-shapes → re-crank).
  This is where the *validate → discovery* feedback edge lives.
- **The goal / Stop-hook** — the outermost governor; human-set.

The breaker is the **andon**, and it is **three-tier, not binary** (auto vs escalate). A
one-way-door decision routes by a *policy* to the cheapest tier that can safely own it:

| Tier | For | Machinery (built) |
|---|---|---|
| **Auto** | routine, deterministically checkable | AUTO-REDO on REFUTED; the gates |
| **Helper** | model-adjudicable one-way doors — architecture forks, scoring, plan shape — **and breaker-tripped stuck states** (N failed rounds, oscillation, scope creep): stuckness is model-adjudicable too; the context that ground to a halt is in a rut a fresh one is not in | [`/council`](../../skills/council/SKILL.md) (multi-judge consensus) + `ao plan-pawl decide` (deterministic PASS/REDO/BLOCKED — the windshield, no model gets the last word) + [`converge`](../../skills/converge/SKILL.md) (fix→re-judge to agreement or hard-BLOCK); for stuck states, one bounded helper pass — a fresh context or cross-family model (`codex exec`) gets the blocker + what was tried, returns UNSTUCK or ESCALATE. An advisor, never a second driver |
| **Human** | genuine human-judgment (the refusal lane: money, legal, hiring, irreversible external), an explicit judgment flag, an exhausted budget — plus any blocker that survived its one helper pass | `ESCALATE`/`HOLD`; the refusal lane |

"Keep going until it passes" without a breaker grinds forever on an unpassable goal — and a
Stop-hook that *blocks stopping* is exactly the mechanism that would. So the primary re-do
loop is bounded (`ao plan-pawl decide` PASS/REDO/**BLOCKED**; `converge` → hard-BLOCK); the
Stop-hook sits outside as governor; the human is the ultimate breaker.

**The tiers all exist; the load-bearing piece is the *router* — a per-goal policy** mapping a
one-way-door *class* to a *tier* (`{arch fork → helper, stuck → helper, external/money/
irreversible → me, routine → auto}`). The goal-crafting skill carries that policy; it is
*not* a flat "escalate to me." The full ladder — breaker trip → one bounded helper pass →
human only if it survives — is the contract in
[`pawls.md` §Escalation](../contracts/pawls.md#escalation-the-circuit-breaker-model).

## Small batches and the flywheel property

Every loop's context accretes into **three durable sinks** — the difference between a
flywheel (gets smarter across turns) and a pipeline (starts cold each time):

| Sink | What lands there |
|---|---|
| `.agents/` | the working corpus (yield ledger, catches, checks, learnings) |
| the **bead** | the tracked intent packet + its accruing evidence (the thinnest sink today) |
| the **artifacts** | the code / docs / skills actually produced |

**A closed bead is a sensor reading, not a checkbox.** Waterfall treats DONE as terminal;
the flywheel treats it as the *highest-signal evidence in the loop* — the only membrane-
verified thing. So closing a bead can (and should) update the plan: re-analyze the sibling
cohort, split/re-order/drop, or spawn new beads. Guard against thrash with the andon rule —
re-plan on a **falsified plan assumption**, not on every close (most closes teach nothing).

**Small beads maximize sensor *frequency*.** A big bead teaches once, late; ten small beads
teach ten times, early. That — not just blast-radius — is why small-batch is load-bearing:
it maximizes the learning rate, which maximizes **yield** (verified-done per unit cost). The
unit of a batch is **one behavior = one Gherkin scenario** with a runnable executed-red
acceptance test (LOC is gameable; the behavior is the true unit). The same scenario is the
plan contract *and* the acceptance criterion, so plan and validate cannot drift.

## Autonomy ⟂ batch size (the resolution of the factory fear)

The fear: agents run autonomously for hours, but that drifts into one big blob. The
resolution: **autonomy and batch size are orthogonal.** Autonomy = no human *between*
iterations. Batch size = work *per* iteration. An 8-hour autonomous run should be **50
small-batch iterations, not one 8-hour blob.** You are not trading yield for autonomy — you
run the *small-batch* loop *unattended*.

But this makes the discipline **load-bearing**: attended, the human enforces small-batch (and
intervenes on drift); autonomous, **nothing does unless the system does.** So the mechanical
enforcement — batch-size gate, close-time re-plan checkpoint, the re-plan engine, the andon —
is not polish; it is the **prerequisite for safe autonomy.** Attended you can be sloppy;
autonomous the system must be strict.

## The human moves from *in* the loop to *on* it

- **In the loop** — gate every iteration. Doesn't scale; kills the long autonomy.
- **On the loop** — set the setpoint (the goal), then review **asynchronously** the two things
  the loop accumulates: the **yield** (verified increments + what each taught) and the
  **andon queue** (the handful it refused to guess on: refusal-lane doors, explicit judgment
  flags, and blockers that survived their helper pass). The andon is what makes long autonomy
  *safe* — grind the slices you can verify, consult a fresh context on the ones that stall,
  **stop and ask** only on the ones no model can own, never hallucinate a road that isn't
  there.

## Substrate split: gc runs it, agentops disciplines it

- **gc (Gas City)** — the always-on autonomous **substrate**: durable supervised agents
  running for hours.
- **The agentops operating loop + membrane** — the small-batch **discipline** + verification,
  composed on top.

Autonomy without the loop = a fast blob-maker. The loop without autonomy = a slow human-gated
crawl. Together = a high-frequency **verified**-increment machine. That is the thesis: make
the stochastic agent reliable enough that you can walk away for 8 hours and trust the yield.

## The shift-left feedback edge (validate → discovery)

The catch corpus is the concrete instance of loop-4's feedback edge: what the pawl/validate
*catches* is mined and fed to the **start** of the loop (discovery/pre-mortem), so plans catch
a defect class *before* it reaches the pawl. Detail + honest status:
[ADR-0014](../adr/ADR-0014-catch-to-producer-loop-judgment-catches-need-a-producer-route.md),
[the producer-defect register](producer-defect-register.md).

## Status ledger (what is real)

| Piece | State | Evidence |
|---|---|---|
| The operating loop (moves + gates) | **landed** | [operating-loop.md](operating-loop.md), the pawl membrane |
| Discovery as re-plan engine | **landed** (wave-boundary + per-close) | discovery skill; the per-close trigger landed via crank's close checkpoint (`age-cysr`) |
| Validate = sensor / bounded driver loop / andon | **partly landed** | `ao plan-pawl decide`, `converge`; the full wiring is a model |
| Catch → producer (shift-left) | **partly landed** | ADR-0014 + `ao membrane digest` (`age-xbmf`); injection (S2) not yet closed |
| Small-batch-by-Gherkin enforcement | **specced** | `age-74yi` |
| Goal-crafting skill | **specced** | `age-znst` |
| Close-time learning checkpoint | **landed** | crank Land Loop "Close checkpoint" + implement close rule (`age-cysr`) |
| Async governance surface (yield + andon) | **landed** | `ao yield report` (`age-mv67`) |
| gc as autonomous substrate | **exists**, composition proven | the gc adoption arc |

This doc will be wrong the moment a slice teaches us something — which is the point. It is a
hypothesis, tracked; update it when a close falsifies it.
