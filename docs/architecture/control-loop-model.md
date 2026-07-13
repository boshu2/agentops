# The Control-Loop Model

> The sixth view of the loop ([loop-map.md](loop-map.md)): **why it converges and why it self-improves.** The [operating loop](operating-loop.md) names the seven moves and states (principle 7) that *the map is fixed and the route is re-routed*; this doc specifies the control system that makes re-routing **terminate** and the map **improve** — the layer principle 7 only gestures at. Composes with, does not duplicate, the seven moves. Cross-links: the membrane reframe ([ADR-0004](../adr/ADR-0004-corpus-moat-unproven-position-on-the-system.md), [PRODUCT.md](../../PRODUCT.md) "validation membrane"), the self-improving-membrane epic `age-cwo`, the admission gates ([pawls.md](../contracts/pawls.md)), and the **structure sibling** [the-agent-factory.md](the-agent-factory.md) — this doc is the *behavior* (how the citizens converge + self-improve), that doc is the *citizens* (the control-plane primitives + adapter taxonomy).
>
> **Discipline of this doc:** mechanism, not vocabulary. Every principle below changes a real rule in the controller; anything that doesn't is cut. Local-first, no cloud.

## 1. The membrane is a closed-loop control system, not a DAG

The verification membrane is the whole loop run as a **closed-loop control system** over the seven moves — the SDLC left half (shape → slice → TDD) and the DevOps right half (prove → run → observe → feedback). It is not a pipeline and not a workflow DAG.

Every altitude (slice, wave, bead, merge, run) carries the **same gate**: *is this ready to promote, or does it route back?* The control-system property — the thing a DAG cannot do — is that **route-back can target an earlier altitude, not just retry the current stage.** A defect surfacing at the merge gate that should have been caught at the slice gate means an **upstream filter was too loose**; the route goes back to where the filter belongs, not to a blind retry of the same step.

This is a system you **traverse**, not a sequence you execute. The old `workflow.json` DAGs (and `bdd-foundry.js`, which inlined five skills as a sequential waterfall) were **open-loop**: fixed forward order, no gate-driven re-routing, no convergence guarantee. `operating-loop.js` is closer — it has the gates. The model below is what an open-loop DAG is missing.

**This refines operating-loop principle 7:** *the map is fixed within a run; the map is improved across runs.* Stages, legal transitions, and gates do not change while a goal is in flight (the map). The route through them is recalculated on every verdict (the fast loop, §3). The map itself — which gates exist, at which altitude — is tuned only between runs by the governed slow loop (§4). **No self-modification inside a run.**

## 2. Two timescales — the keystone

There are two clocks, and **conflating them is what oscillates** (adaptive control 101: a system that adapts its own rules while still trying to converge never settles).

- **Fast loop — within a run — is CONVERGENCE.** Route-back, bounded, until it converges on a grounded verdict or escalates. The map is fixed here.
- **Slow loop — across runs — is IMPROVEMENT.** Tune the filters from escapes, governed so it doesn't thrash. The map changes only here.

Keep them on separate clocks. The fast loop never edits a gate; the slow loop never runs mid-convergence.

## 3. Fast-loop contract (stability — convergence within a run)

- **Terminate on a GROUNDED VERDICT, not a counter.** The loop stops because move 6 produced a deterministic pass against ground truth — not because a counter ran out. The attempt-cap + wall-clock is the **backstop** that catches a thrash the verdict missed, never the primary terminator.
- **Guard against degeneration-of-thought** — the failure where an agent repeats the same flawed reasoning each retry and converges *stable-but-wrong* (confident, consistent, incorrect):
  1. The verdict must hit **deterministic ground truth** (the windshield — operating-loop move 6: a real test, a real gate, a real `ao provenance verify`). A model grading itself converges on confident-wrong; that is not a verdict.
  2. Each retry **carries WHY the last attempt failed** (injected critique). A retry without the prior failure re-derives the same dead end.
- **Oscillation usually comes from the GATE, not the work.** A *stochastic* gate (an LLM judge with no fixed rubric) will pass → fail → pass the same unchanged artifact and *cause* the flap. **Gates must be deterministic** (or a fixed-rubric quorum, not a free-form judge). This is non-negotiable: a non-deterministic gate makes convergence impossible no matter how good the bounding primitives are.
- **Bounding primitives — the minimal set that bites** (do not import the whole catalogue):
  - **Contraction + ratchet + fixpoint** (the termination guarantee): each route-back round must **shrink the open-defect count** or it escalates; a **passed gate freezes and cannot reopen** (the ratchet); **no-diff between two rounds → done**. This is what makes the traversable system provably terminate rather than wander.
  - **Flap damping** (the direct anti-oscillation primitive, BGP-style): each route-back to a stage accrues a per-stage penalty that **decays on clean passes**; crossing a suppress-limit → **freeze the stage and escalate**, do not route to it again.
  - **Restart-intensity-in-window** (OTP supervision): N route-backs to the same stage inside a window → **escalate to a broader scope** (re-plan the remaining waves / stronger model / human), do not retry the same stage again.
  - **Retry budget** (run-level, not per-stage): cap total route-backs as a fraction of forward progress for the whole run; budget spent → **ship-or-abort**, surfaced to the operator.

## 4. Slow-loop contract (the self-improving membrane — improvement across runs)

- **Engine: escape → shift-left.** When a defect slips a filter and is caught later (or in prod), add the check **one altitude earlier**, where it is cheap — poka-yoke (make the error impossible at the stage that missed it) via the existing **blameless-postmortem → finding → one durable compiled gate** path (`age-cwo`; the postmortem → finding-compiler shape already exists). An escape's output is exactly one new gate at the right altitude, not a one-off fix.
- **The governor — the genuinely missing setpoint (Deming / SPC):**
  - **Adjust ONLY on special-cause signal** — a repeated escape pattern past a control limit. **Never adjust on common-cause noise** (a one-off). Adjusting on noise is *tampering*: it increases variance, and it is itself what makes the membrane oscillate (cry-wolf). A self-improving membrane that adds a gate for *every* escape over-fits and degrades.
  - **Bound every added filter by TWO-SIDED FITNESS:** an added gate must raise catch-rate **and not raise false-alarm-rate** (measured: `ao yield gauge` — catch_rate ↑, false_refute ↓). A gate that catches more by crying wolf is rejected.
  - **The ERROR BUDGET is the top governor:** inside tolerance → keep shipping (don't harden, that's tampering); budget burned → **stop the line and harden** before more work flows. One number decides ship-vs-harden.

## 5. The honest cut — only three mechanisms, each mapped to current state

Importing all ten control-theory patterns would be vocabulary-instead-of-mechanism (the cathedral failure mode this repo keeps paying for). Specify only the three that change a real rule:

| # | Mechanism | Loop | Current AgentOps state |
|---|---|---|---|
| 1 | **Circuit breaker + no-delta-stop** | fast (stability) | **Mostly exists** — 3-attempt cap + circuit-breaker escalation ([pawls.md](../contracts/pawls.md) §Escalation), dry/no-delta stop. Gap: the cap should be the *backstop*, the grounded verdict the *terminator*. |
| 2 | **Escape → shift-left check** | slow (engine) | **Open** — `age-cwo` (escape-tracking → finding → one-altitude-earlier gate); the postmortem→finding-compiler shape exists, the escape→gate closure does not. |
| 3 | **SPC governor** (special-cause-only, two-sided fitness, error budget) | slow (setpoint) | **Built** (`age-wy3`). `cli/internal/governor`: the error-budget burn-rate (`ao governor budget`, ship-vs-harden — SPC.1) and the special-cause noise-band + two-sided-fitness gate + `DetectFalseAlarms` (`ao governor noise-band` — SPC.2). The governor is the canonical SLOW-loop owner; it does NOT merge the fast/outer breakers (different timescales by design — `planpawl/decide.go` is the fast loop, `scripts/evolve/halt-check.sh` the evolve outer loop). The coherence wire (SPC.3): `halt-check.sh` consults `ao governor budget`, so a burned budget halts the evolve cycle (`governor_budget_burned`). `rpi_loop_supervisor.go` was deleted by ADR-0009/soc-2rtm0; the old "3 scattered breakers" framing is obsolete. |

## 6. Conformance contract

A Workflow script or skill is **loop-model-compliant** iff:

1. **Gates are deterministic.** Every promotion decision hits ground truth (a test, a gate, a fixed-rubric quorum) — never a free-form LLM self-grade. (A stochastic gate is the #1 cause of non-convergence.)
2. **The fast loop terminates on a grounded verdict, with the attempt-cap/wall-clock as backstop only.** Not a bare counter.
3. **No self-modification inside a run.** No gate is added, removed, or re-tuned while a goal is in flight. Map fixed within a run.
4. **Escapes route to the slow loop.** A defect caught downstream emits an escape record that feeds escape→shift-left, governed by SPC — it is not silently fixed in place and forgotten.
5. **The orchestrator gates and routes; it never reasons about the work.** The orchestrator's only jobs are: read the verdict, apply the bounding primitives, route to the next altitude (or escalate). The moment an orchestrator starts *doing* the work it is supposed to *gate*, the membrane has a conflict of interest and the control system is broken.

A script that satisfies all five is a traversable control system. A script that fails any of them is an open-loop DAG wearing the loop's vocabulary.
