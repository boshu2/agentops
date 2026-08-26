---
name: skill-eval
description: 'Author and tier behavioral probes for a skill, including seeded-defect probes that escape ceiling saturation. Triggers: "measure this skill", "the probe came back INERT", "the control arm aces it", "harden this scenario", "is this skill actually doing anything".'
practices:
- measurement-over-assertion
- ab-testing
skill_api_version: 1
hexagonal_role: supporting
consumes:
- skill-source-package
produces:
- probe-package
- probe-result.v1
context_rel:
- kind: supplier-to
  with: skill-builder
user-invocable: true
metadata:
  tier: meta
  dependencies: []
  capabilities: ["author_seeded_probe","run_probe_tier"]
  effects: ["write_probe_package","dispatch_probe_producer"]
  canonical_status: canonical
  disposition: keep_specialist
  stability: experimental
---

# /skill-eval

Author one behavioral probe for one skill, at the cheapest tier that can still
separate the arms, and report the verdict honestly. A probe measures
**behavior-change** — did loading the skill change what the agent *did* — never
quality-uplift. This skill authors and tiers probes. `scripts/probe-skill.sh`
runs them.

**Insight:** when a probe returns INERT because the control arm already aces the
scenario, the measurement failed, not the skill. Weakening the producer is one
escape and it costs realism. The cheaper escape is to **plant the defect**: build
a scenario containing exactly one flaw the discipline catches and a skim does
not, then grade whether the agent acted on it. Signal you manufacture is signal
you can reproduce.

**The failure mode this exists to prevent:** a skill catalog whose tier badges are
editorial. A skill nobody measured is a skill nobody can defend, and re-running a
saturated scenario at a lower effort level produces more rows in the ledger
without producing more knowledge.

## Modes

| Trigger phrases | Mode | Entry point |
|---|---|---|
| "measure this skill", "does this skill do anything" | author tier 1 (quiz probe) | `evals/skill-probes/<id>/` |
| "the control arm aces it", "harden this scenario" | author tier 2 (seeded-defect probe) | [`references/seeding.md`](references/seeding.md) |
| "the probe came back INERT" | diagnose headroom | gate `skill.probe-headroom` |
| "run the probes" | run a tier | `scripts/probe-skill.sh` |

## Inputs

Required: the skill slug, and one sentence naming the **action** the skill should
cause — a tool call made, an artifact written, a question raised, a sequence
followed. If the sentence names a belief instead of an action ("understands
that…", "considers…"), stop: that is not probeable, and rewriting it as an action
is the actual work.

Optional: an existing probe id to harden.

**Non-goals.** This skill does not score output quality, rank skills, claim a
skill is good, or gate a release. It does not run `claude -p`. It does not
generalize from N=2 — small N is directional and every artifact it produces says
so.

## The two tiers

| | Tier 1 — quiz | Tier 2 — seeded task |
|---|---|---|
| Scenario | asks the agent a question about a situation | hands the agent work containing a planted defect |
| Grades | which answer it gave | whether it acted on the defect |
| Saturates | fast — frontier models answer doctrine questions correctly unaided | slowly — skimming is a real failure mode at every altitude |
| Cost | low | higher (real task, longer transcript) |
| Use when | the skill's whole content is a decision rule | tier 1 saturated, or the skill's value is *noticing* |

Our ledger records five tier-1 probes saturating at both `xhigh` and `low` effort
with the note "scenario needs hardening, not the skill." Harder quizzes did not
fix it — `validate-not-proven-v2` re-saturated. Tier 2 is the escape, because it
changes what is being measured from *knowing the rule* to *applying it while busy*.

## Procedure

1. **State the action.** One sentence, an observable act. Reject beliefs.
2. **Pick the tier.** Start at tier 1 unless a prior probe for this skill is
   saturated; then go straight to tier 2.
3. **Build the scenario.** For tier 2, seed exactly one forcing defect using the
   rules in [`references/seeding.md`](references/seeding.md). One defect for a
   floor probe; N defects for a band probe.
4. **Write the discriminator.** Deterministic, over one transcript. Exit `0`
   present, `1` absent, `2` infra. It checks the **act**, never a mention — a
   discriminator that greps for a word the prelude contains measures the prelude.
5. **Calibrate on replay** against committed fixtures before spending a live run.
6. **Run both arms** at two effort levels. Same scenario, same reps, the prelude
   is the only variable.
7. **Pre-screen headroom before believing the verdict** — gate
   `skill.probe-headroom`. A verdict over a saturated scenario is void.
8. **Record one ledger row** in `evals/skill-probes/LEDGER.md` and stop.

```bash
# Calibrate deterministically against committed fixtures.
bash scripts/probe-skill.sh --probe <id> --replay

# Live A/B (codex exec — the sanctioned headless path).
bash scripts/probe-skill.sh --probe <id> --live --capture --reps 3 --output out.json

# Is the verdict trustworthy, or did the control arm ace it?
bash scripts/check-skill-probe-headroom.sh
```

### Floor and band

- **Floor probe** — one seeded defect, assert the agent acted **at least once**.
  Catches the total no-op: the review that produced a polished report naming
  nothing.
- **Band probe** — N seeded defects, assert findings land in **[N-1, N+2]**.
  The lower bound catches rubber-stamping; the upper bound catches spray, where
  an agent lists every conceivable concern and is credited for the one that
  happened to be planted. A probe with only a floor rewards noise.

### Saturation rule — owned by the gate, not by this skill

The rule is **deterministic, so it does not live here.** Gate
`skill.probe-headroom` owns it: a scenario is **SATURATED** when the control arm
scores **≥ 0.75 at two or more effort levels** with at least 2 usable control
reps each. The rule, its thresholds, and its exit codes are
`cli/internal/probeheadroom`; the gate script is
[`scripts/check-skill-probe-headroom.sh`](../../scripts/check-skill-probe-headroom.sh)
and the helper it drives is `cli/cmd/probe-headroom`. Do not restate the
thresholds in a probe package or re-derive them by hand — read the gate's answer.
This skill's job is what to DO with that answer:

- **SATURATED** — retire the scenario. Promote it to tier 2, or record the honest
  finding (*at this altitude the behavior is native to the model, and this
  skill's marginal effect is nil*), which is a real result about the skill's
  value and belongs in the ledger. Never re-run it at a lower effort.
- **FLOOR** — the treatment arm never acted at any level. Check the
  discriminator against a hand-written passing transcript before re-seeding.
- **UNMEASURED** — the run did not happen. Not INERT; do not record it as one.
- **SEPARATED** — the scenario left room, so the verdict is about the skill.

Never resolve saturation by lowering the discriminator's bar. That converts a
measurement problem into a false positive.

## Anti-patterns

| Anti-pattern | Corrective |
|---|---|
| Discriminator greps for a term that appears in the treatment prelude | Grade the act (file written, tool called, question raised), never the vocabulary |
| Re-running a saturated scenario at a lower effort to find separation | Retire it; promote to tier 2 or record the ceiling finding |
| Seeding a defect so obvious both arms catch it | Calibrate: the control arm must plausibly miss it. See `references/seeding.md` |
| Seeding a defect so obscure neither arm catches it | The defect must be *derivable from the discipline*, not from trivia |
| Floor-only band on a multi-defect scenario | Add the ceiling; an agent that flags everything is not detecting anything |
| Reporting N=2 as evidence the skill works | Say "directional, not statistical" in the same sentence as the number |
| Deleting a losing probe | Append the row. A skill measured INERT is knowledge; a missing row is a gap |

## Output

A probe package under `evals/skill-probes/<id>/` (`probe.json`, `question.md`,
`treatment-prelude.md`, `discriminator.sh`, `fixtures/`), plus one appended
ledger row.

`probe.json` for a tier-2 probe declares its seeding:

```json
{
  "id": "validate-not-proven-t2",
  "skill": "validate",
  "tier": "judgment",
  "probe_tier": 2,
  "reps": 3,
  "seeded_defects": 1,
  "band": [1, 3],
  "behavior": "the agent returns NOT_PROVEN rather than PASS when one in-scope acceptance criterion has no evidence",
  "discriminator": "discriminator.sh",
  "budget_note": "N=3 — DIRECTIONAL, not statistical",
  "honesty": "measures behavior-change on a seeded task, NOT quality-uplift"
}
```

**Done when:** `probe-skill.sh --replay` reproduces the recorded verdict from
committed fixtures, `skill.probe-headroom` classifies the scorecard group
`SEPARATED` (or the run is recorded as `SATURATED` with the scenario retired),
and exactly one row was appended to the ledger.

## Checks

- The discriminator passes on a hand-written transcript that performs the act
  without using the prelude's wording, and fails on one that uses the wording
  without performing the act. Both directions, or it is not a discriminator.
- Control and treatment prompts differ **only** by the prelude.
- The seeded defect count in `probe.json` equals the count actually present in
  `question.md`.
- The ledger row names the producer model and effort levels.
- The ledger row cites a headroom pre-screen: a row over a `SATURATED` group is
  a void row, not evidence.
- No claim of quality-uplift appears anywhere in the output.

## Provenance

- Harness this extends: [`scripts/probe-skill.sh`](../../scripts/probe-skill.sh), [`evals/skill-probes/README.md`](../../evals/skill-probes/README.md).
- The saturation evidence that motivated tier 2: [`evals/skill-probes/LEDGER.md`](../../evals/skill-probes/LEDGER.md) — five INERT rows dated 2026-08-04/05 annotated "scenario needs hardening, not the skill."
- Coverage gate (does a result exist): [`scripts/check-skill-probe-coverage.sh`](../../scripts/check-skill-probe-coverage.sh), whose denominator is declared in `scripts/.skill-probe-denominator-exclusions`.
- Headroom gate (could a result have existed): [`scripts/check-skill-probe-headroom.sh`](../../scripts/check-skill-probe-headroom.sh) — gate id `skill.probe-headroom`, rule in `cli/internal/probeheadroom`.
- Seeded-forcing-defect and floor/band mechanism analysis (§2.1, §2.5): not on main; read it at `git show 9872483bd:docs/research/gstack-teardown-2026-08-08.md` (branch `recover/gstack-clean-room`).
- Overclaim discipline: ADR-0011.

## Failure behavior

If the live producer errors or the transcript is truncated, the rep is `infra`
(discriminator exit 2), not `absent`. Infra failures are excluded from rates and
named in the ledger row. A run whose usable treatment reps reach zero is
`UNMEASURED` — never INERT. Scoring an infra failure as a miss manufactures the
result the harness exists to prevent.
