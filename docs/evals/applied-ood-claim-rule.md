# Applied-OOD moat claim rule — PRE-REGISTRATION (the sentinel-recall ban)

> **HISTORICAL:** Locked preregistration for a retired evaluation harness.

> **Pre-registered BEFORE the next valid moat run (age-6ys, 2026-06-17).** This
> fixes — in advance — what a scenario must be for its A/B scorecard to count as
> evidence that *"AgentOps' knowledge corpus improves agent work."* Locked: do not
> edit the decision rule to fit a result. Append a dated erratum instead.
>
> Companion to `evals/workbench/corpus-delta-w1c-prereg.md` (the *context-axis*
> prereg). This doc governs the *gold/corpus-axis* `ao eval scenario-ab` harness
> (`cli/internal/eval/scenario_ab.go`).

## Why this exists (the failure it bans)

The first "valid corpus-A/B verdict" (s-2026-06-16-100, delta=1.0) used a
**sentinel-recall** scenario: the goal was to state an invented string
(`QXR-7731-VERIFY`) that exists only in the corpus. Re-running it days later
produced a **ceiling violation** — the control scored 1.0 by reading the sentinel
from `docs/`, `_beads/`, and `.agents/learnings/`, where it had leaked *because we
documented the win*. Two structural defects, neither fixable by better isolation:

1. **Tautological by construction.** An invented secret is corpus-only *by
   definition*, so the control *must* fail. The scenario proves the corpus can
   deliver a string — not that it improves an agent's **work**. A delta of 1.0
   here is a measurement of "we hid a string and revealed it," nothing more.
2. **Self-burning.** The discriminator is a *secret*, and the act of running,
   celebrating, and auditing the eval leaks that secret into the paper trail
   (`docs/`, `_beads/`, learnings) — surfaces the arm isolation **cannot** deny
   (it scopes only `.agents/`/`.ao/`). So the verdict is un-reproducible: every
   run erodes the very property it depends on.

A real moat verdict therefore requires **applied-OOD** scenarios: the corpus holds
**doctrine** the agent must **apply** to succeed; the model cannot guess it from
training; and the discriminator is **not a leakable secret**.

## Scenario taxonomy (the mechanical signal)

The grading mode of a `scenario.v1` file is the mechanical class signal, enforced
by `classifyVerdict` and stamped on every scorecard as `verdict_class` +
`moat_eligible`:

| Class | Graded by | `moat_eligible` | Role |
|---|---|---|---|
| **fact-recall** | `answer_key` (deterministic string match) | **false** | PLUMBING/smoke only — proves the corpus delivers + arm isolation holds. Tautological + self-burning. **BANNED as moat evidence.** |
| **applied-ood** | `acceptance_vectors` (cross-family LLM judge) | **true** | The only class admissible as moat evidence — *if* it also clears the ceiling pre-screen and passes its gate. |
| **unspecified** | neither | **false** | No measurable success dimension — neither plumbing nor evidence. Invalid. |

`fact-recall` scenarios remain **useful and allowed** — as plumbing tests that the
corpus-delivery + isolation machinery works. They are simply never the moat
verdict. The CLI prints `NOT-moat-evidence(plumbing)` on a passing fact-recall
scorecard precisely so a plumbing PASS is not misread as the moat claim.

## What makes an applied-OOD scenario VALID (author checklist)

All four must hold. The harness mechanically enforces #2 and #4; #1 and #3 are
authoring discipline a reviewer confirms.

1. **The doctrine is in the corpus and APPLIED, not recalled.** Success is
   *better work* (a correct design, a passed gate, an engaged process), graded by
   acceptance vectors — never the presence of a string.
2. **The discriminator is not a leakable secret.** Mechanically: no `answer_key`.
   Substantively: there is nothing a future doc/bead/learning could leak that
   would let the control pass. The discriminator is *applied judgment*, which
   cannot leak into a denylist gap.
3. **The model cannot guess it from training.** The strongest shape is a
   **contrarian-default**: the model's *standard* answer is **wrong** for this
   repo, and the corpus holds the repo's non-obvious decision. The control fails
   not from ignorance-of-a-string but from confidently applying the wrong default.
4. **The task has headroom (control genuinely fails).** Enforced by the **ceiling
   pre-screen** (age-707): the control runs first; if it already clears the
   satisfaction threshold, the run aborts as a `ceiling_violation` with no delta.
   A guessable "applied" scenario is caught here, not waved through.

## Publication rule (LOCKED)

Claim **moat positive** for the gold/corpus axis ONLY IF, on an applied-OOD
scenario (`verdict_class == "applied-ood"`, `moat_eligible == true`):

- the control arm ran under genuine filesystem isolation (age-9a9), AND
- the run did **not** ceiling-violate (control had headroom), AND
- `aggregate_delta > 0` with the treatment clearing the satisfaction threshold,
  AND the gate passed, AND
- the delta is attributable to the applied doctrine (judge's per-vector verdicts
  show the with-gold arm applied the corpus decision and the control applied the
  wrong default), AND
- a fresh **non-author** reviewer confirms the scenario meets the author checklist
  above (especially #1–#3, which the harness cannot check).

A `fact-recall`/`moat_eligible:false` scorecard is **never** sufficient for this
claim, regardless of its delta or gate result.

Claim **honest null** when an applied-OOD scenario had headroom (no ceiling
violation), isolation held, and the delta did not clear the bar — the corpus did
not improve the work on that task. Anything else is **inconclusive**.

## Open frontier (what is NOT yet closed)

Authoring + grading valid applied-OOD scenarios is research-grade and only
partly done:

- **One-shot-gradable** applied-OOD scenarios (design/judgment questions, e.g.
  s-2026-06-16-003, and the `evals/scenarios/applied-ood/` set) are runnable now:
  the single-turn runner produces a design/answer the acceptance-vector judge can
  grade.
- **Execution-required** applied-OOD scenarios (ship code that trips a gate;
  dispatch a process that must engage — s-2026-06-16-001/002) use
  `runner_mode: "agentic"` on `scenario.v1`. The agentic runner (age-5tv) runs a
  multi-turn worker in an isolated workspace whose RESULT is graded; the default
  one-shot runner remains for design/judgment scenarios.
- A single applied-OOD run is **n=1** over a stochastic LLM judge. A durable moat
  verdict needs multiple scenarios and seeds with the corpus-delta prereg's
  statistic — the moat remains **UNPROVEN** until then. This rule makes the next
  run *valid*; it does not by itself prove the moat.

## Change log

- 2026-06-17 (age-6ys): created. Banned sentinel-recall (fact-recall) scorecards
  as moat evidence; defined the applied-OOD class + validity checklist + locked
  publication rule. Mechanical enforcement landed as `verdict_class` /
  `moat_eligible` on `ScenarioDeltaScorecard`.
- 2026-06-18 (age-sb0): moat claim aggregation surface (`ao eval scenario-moat`)
  fail-closes on `moat_eligible=false` inputs; renders moat_positive/honest_null/
  inconclusive over eligible scorecards only.
- 2026-06-18 (age-5tv): agentic runner (`runner_mode: agentic`) for
  execution-required applied-OOD scenarios in scenario-ab.
