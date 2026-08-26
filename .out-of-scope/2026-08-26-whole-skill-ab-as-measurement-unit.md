# Whole-skill A/B as the measurement unit

**Request:** keep measuring skill efficacy the way `evals/skill-probes/` did —
run a task with the whole skill injected against the same task without it, and
read the difference as the skill's effect.

**Decision:** not built further. Existing whole-skill A/B rows stay as history;
no new skill gets measured this way.

**Why:**

- Ceiling saturation, not tooling, nulled the local A/Bs. When a frontier model
  already passes the control arm, both arms score the same and the row carries
  no information about the skill.
- `evals/skill-probes/LEDGER.md` reports 0/12 product and judgment skills
  measured. Several rows read INERT when the honest reading is UNMEASURED — the
  scenario had no headroom for the skill to occupy.
- A design whose null result is indistinguishable from its no-effect result is
  not a measurement.

**Replaced by:** seeded-defect probes (a work artifact with planted defects the
skill cannot honestly miss, scored with floor and band assertions) and line-level
ablation, each gated on a passing headroom pre-screen that classifies SATURATED
before any row is written.

**Reopens when:** a scenario is shown to have real headroom — the control arm
demonstrably fails it — in which case the A/B is a valid shape for that scenario
specifically, not the corpus default.
