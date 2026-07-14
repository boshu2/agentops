# The Control-Loop Model

> The sixth view of the loop ([loop-map.md](loop-map.md)): **why it converges and why it self-improves.** The [operating loop](operating-loop.md) names the seven moves and states (principle 7) that *the map is fixed and the route is re-routed*; this doc specifies the control system that makes re-routing **terminate** and the map **improve** — the layer principle 7 only gestures at. Composes with, does not duplicate, the seven moves. Cross-links: the membrane reframe ([ADR-0004](../adr/ADR-0004-corpus-moat-unproven-position-on-the-system.md), [PRODUCT.md](../../PRODUCT.md) "validation membrane"), the self-improving-membrane epic `age-cwo`, the admission gates ([pawls.md](../contracts/pawls.md)), and the **structure sibling** [the-agent-factory.md](the-agent-factory.md) — this doc is the *behavior* (how the citizens converge + self-improve), that doc is the *citizens* (the control-plane primitives + adapter taxonomy).
>
> **Discipline of this doc:** mechanism, not vocabulary. The run-level
> orchestrator owns transitions; individual skills emit artifacts and verdicts,
> not private controllers. Local-first, no cloud.

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

- **Terminate on evidence, not ceremony.** Deterministic checks prove facts;
  one fresh independent verdict judges the frozen semantic claim. A counter is
  neither proof nor a reason to duplicate the gate.
- **Guard against degeneration-of-thought** — the failure where an agent repeats the same flawed reasoning each retry and converges *stable-but-wrong* (confident, consistent, incorrect):
  1. The verdict must hit **deterministic ground truth** (the windshield — operating-loop move 6: a real test, a real gate, a real `ao provenance verify`). A model grading itself converges on confident-wrong; that is not a verdict.
  2. A repair carries the complete blocker evidence forward. Repeating the same
     objective without changing the approach is not progress.
- **Oscillation usually comes from reviewing a moving artifact.** Freeze the
  exact plan or candidate, bind its digest, and return one complete blocker set.
  Any edit creates a new artifact identity.
- **One governor.** The orchestrator classifies results as NOTE, REPAIR, REPLAN,
  HOLD, or ANDON. Skills do not own attempt maps, retry multipliers, helper
  quotas, or operator notification.

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
| 1 | **Exact-input proof + one run disposition** | fast (stability) | **Landed** — frozen artifact identities, author-distinct semantic verdicts, and orchestrator-owned NOTE/REPAIR/REPLAN/HOLD/ANDON disposition. |
| 2 | **Escape → shift-left check** | slow (engine) | **Open** — `age-cwo` (escape-tracking → finding → one-altitude-earlier gate); the postmortem→finding-compiler shape exists, the escape→gate closure does not. |
| 3 | **SPC governor** (special-cause-only, two-sided fitness, error budget) | slow (setpoint) | **Built** (`age-wy3`) for across-run learning economics. It does not decide a plan verdict or create phase-local control state. |

## 6. Conformance contract

A Workflow script or skill is **loop-model-compliant** iff:

1. **Gates are deterministic.** Every promotion decision hits ground truth (a test, a gate, a fixed-rubric quorum) — never a free-form LLM self-grade. (A stochastic gate is the #1 cause of non-convergence.)
2. **The fast loop terminates on evidence and one orchestrator disposition.** Not a bare counter or a skill-local retry ladder.
3. **No self-modification inside a run.** No gate is added, removed, or re-tuned while a goal is in flight. Map fixed within a run.
4. **Escapes route to the slow loop.** A defect caught downstream emits an escape record that feeds escape→shift-left, governed by SPC — it is not silently fixed in place and forgotten.
5. **The orchestrator gates and routes; it never reasons about the work.** The orchestrator's only jobs are: read the verdict, apply the bounding primitives, route to the next altitude (or escalate). The moment an orchestrator starts *doing* the work it is supposed to *gate*, the membrane has a conflict of interest and the control system is broken.

A script that satisfies all five is a traversable control system. A script that fails any of them is an open-loop DAG wearing the loop's vocabulary.
