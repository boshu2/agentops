# `skill.probe-headroom` fixtures

Two committed scorecard **pairs** that drive the hermetic
`skill.probe-headroom` gate (`scripts/check-skill-probe-headroom.sh` ->
`cli/cmd/probe-headroom` -> `cli/internal/probeheadroom`). They exist so the
detector's discrimination is proven on every run instead of asserted:

| Directory | Probe id | Control arm | Expected classification |
|---|---|---|---|
| `saturated/` | `fixture-saturated-quiz` | aces the scenario (1.00) at two effort levels | `SATURATED` — **must be flagged** (no headroom, the row is void) |
| `headroom/` | `fixture-headroom-quiz` | 0.50 at two effort levels | `SEPARATED` — **must NOT be flagged** (INERT here is a real null) |

Both pairs carry the `INERT` verdict on purpose: equal arm rates. That is the
whole point of the gate — `INERT` alone cannot tell "the skill did nothing"
apart from "the scenario had no room left to measure in". Only the control
arm's absolute rate separates them.

The bytes are modelled on real persisted scorecards, not hand-invented shapes:

- `saturated/` mirrors `docs/evals/scorecards/2026-08-05/validate-not-proven-v2-{low,xhigh}.json`
  (control 2/2 and treatment 2/2 at both effort labels — the run that motivated
  the rule).
- `headroom/` mirrors the same writer's field set with the arm counts of a
  genuinely unsaturated scenario (control 1/2).

Fixtures are immutable inputs. Editing a rate here changes what the gate
claims to prove, so change the rule and its unit tests instead.
