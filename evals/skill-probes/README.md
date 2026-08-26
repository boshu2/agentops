# Skill behavioral probes (`evals/skill-probes/`)

> **HONESTY (read first).** A probe measures **BEHAVIOR-CHANGE, not
> quality-uplift.** It answers exactly one question: when the declared
> treatment source is included in the treatment prompt but omitted from the
> control, does the agent actually **DO** the scored thing differently — a tool
> call made, an artifact produced, a sequence followed? It **never** scores
> whether the text merely mentions the guidance, and it **never** claims the
> output is better. `BEHAVIORAL` means the treatment increased the scored
> response behavior, `REGRESSIVE` means it reduced it, and `INERT` means the
> two measured rates were equal. Small N (default 2–3) is **DIRECTIONAL, not
> statistical.** Do not overclaim (ADR-0011 discipline).

## Why this exists

Skills are half the product, but tier badges are editorial — the only
enforcement was an enum-membership check. A 2026-06-30 graphify classifier
regression motivated this harness, but its reconstructed fixtures predate the
capture-manifest contract and are now `LEGACY-UNVERIFIED`; they are not current
behavioral evidence. Documentary acceptance still does not imply behavioral
acceptance. This harness measures the behavior separately and fails closed when
capture provenance is absent.

## What a probe is

A directory `evals/skill-probes/<id>/`:

| File | Role |
|------|------|
| `probe.json` | metadata: id, skill, reps, behavior, discriminator, and required `treatment_source` |
| `question.md` | the scenario question — **IDENTICAL for both arms** |
| `treatment-prelude.md` | distilled guidance used only when `treatment_source` is `injected-prelude` |
| `discriminator.sh` | a **deterministic** behavioral check over the harness-owned response envelope: exit `0`=PRESENT, `1`=ABSENT, `2`=infra. **Checks the ACTION, never a mention.** |
| `fixtures-<run>/` | one immutable live capture: structured JSONL `control-<n>.txt` / `treatment-<n>.txt`; the directory name must be new for every capture |
| `fixtures-<run>/capture-contract.json` | pre-dispatch, self-contained copies of the exact probe inputs, canonical skill bytes, per-arm prompt bytes, requested producer config, runtime executable identity, counterbalanced schedule, and scoring contract |
| `fixtures-<run>/fixture-set.json` | v3 binding over that pre-existing capture contract, exact transcript inventory/hashes, native Codex thread ids, treatment source, and every material evaluator/dispatch helper; required for replay |

`treatment_source` has two deliberately different meanings:

- `canonical-skill`: treatment prompt = the exact bound
  `skills/<skill>/SKILL.md` bytes + the question; control = the question. Only
  this mode can count toward product/judgment skill coverage. It measures the
  response-shape effect of including those canonical bytes, not automatic skill
  discovery, task quality, or an executed outcome.
- `injected-prelude`: treatment prompt = the bound `treatment-prelude.md` + the
  question; control = the question. This is replayable evidence about that
  distilled prelude only. It is not full-skill activation and the coverage gate
  refuses to count it as skill coverage.

In both modes the question is identical and the declared treatment source is
the only arm difference.

## Running

```bash
# Live A/B into a new immutable fixture set and a new scorecard path:
run_tag="low-$(date -u +%Y%m%dT%H%M%SZ)"
scorecard_dir="docs/evals/scorecards/$(date -u +%F)"
mkdir -p "$scorecard_dir"
bash scripts/probe-skill.sh \
  --probe anti-ceremony-creation-gate-v2 --live --capture --reps 2 \
  --fixtures "fixtures-$run_tag" --model gpt-5.6-luna --effort low \
  --output "$scorecard_dir/anti-ceremony-$run_tag.json"

# Compatibility-replay the retained v2 classification without dispatching a
# model. It is historical and coverage-ineligible under the v3 contract:
bash scripts/probe-skill.sh \
  --probe anti-ceremony-creation-gate-v2 --replay \
  --fixtures fixtures-low-2026-08-16-v2b
```

Live capture dispatches `codex exec --json`, the sanctioned headless path. The
harness writes a structured `agentops.probe-input.v1` event from the exact
prompt file passed on stdin, followed by the native Codex JSONL events. It
refuses to replace an existing fixture set or scorecard. Replay verifies every
bound byte, prompt event, Codex thread id, and the exact fixture inventory before
scoring, and refuses legacy directories with no `fixture-set.json`. A v3 fixture
remains self-contained for replay after the current probe or skill changes; the
coverage gate separately requires its captured canonical skill and probe inputs
to match the current repository. Historical v1/v2 manifests remain
compatibility-only and cannot count as current tier coverage.

Live dispatch follows the contract-bound alternating schedule: odd reps run
control then treatment, even reps treatment then control. `--reps`, when
provided, must equal `probe.json.reps` (maximum 20). Before discrimination the
harness structurally extracts only the final completed Codex `agent_message`
from the bound JSONL event stream. Human `codex` / `tokens used` delimiter
parsing exists only for compatibility replay of v1/v2 fixtures and cannot shape
a v3 response. A missing or malformed event boundary, an over-time
discriminator, or a discriminator that mutates its read-only scoring snapshot
degrades that rep instead of scoring prompt-echoed text.

Tier coverage additionally requires an explicit model and effort plus a
PATH-resolved native Codex executable identity captured before dispatch;
explicit binary overrides remain replayable but are coverage-ineligible. This
binds local capture provenance, not model quality, external runtime attestation,
or cross-platform reproducibility.

Replay makes the bound classification replayable; it does not make model
generation deterministic or reproducible.

Verdict: `BEHAVIORAL` iff `treatment_rate > control_rate`; `REGRESSIVE` iff it
is lower; `INERT` iff the rates are equal; `UNMEASURED` iff either arm has no
usable reps.

## The frontier-aces-it caveat

A **frontier** producer often already does the right thing, so a skill's marginal
behavioral effect on it may be nil even for a useful skill. A probe can seek
headroom with a weaker producer or harder task, but a producer-strength claim
requires manifest-backed captures at each compared config. The historical crank
fixtures illustrate a stored null classification only; they are
`LEGACY-UNVERIFIED` and do not establish frontier behavior.

## Two probe tiers

The caveat above proposed two escapes from ceiling saturation: a weaker producer,
or a harder task. Neither worked as run. Five probes dated 2026-08-04/05
saturated at **both** effort levels, and `validate-not-proven-v2` re-saturated
after the scenario was deliberately hardened. The ledger notes conclude: *"the
doctrine is robust in quiz format; next ratchet is task-embedded Tier-2, not
harder quizzes."*

That ratchet is a declared probe tier:

| | Tier 1 — quiz | Tier 2 — seeded task |
|---|---|---|
| Scenario | asks about a situation | hands over work containing a planted defect |
| Grades | which answer was given | whether the agent **acted** on the defect |
| Saturates | fast at frontier | slowly — skimming fails at every altitude |
| `probe.json` | (as before) | adds `probe_tier: 2`, `seeded_defects: N`, `band: [lo, hi]` |

A tier-2 probe plants exactly one forcing defect (floor probe) or N independent
defects (band probe) inside a realistic artifact, and asserts findings land in
`[N-1, N+2]`. The lower bound catches rubber-stamping; the upper bound catches
spray, where an agent lists every conceivable concern and gets credited for the
one that happened to be planted.

Authoring rules, the calibration window, and the seed shapes:
[`skills/skill-eval/SKILL.md`](../../skills/skill-eval/SKILL.md) and
[`skills/skill-eval/references/seeding.md`](../../skills/skill-eval/references/seeding.md).

## The headroom pre-screen (run this before you believe a verdict)

`INERT` is two different results wearing one label. It means the arms matched —
which happens both when the skill changed nothing (a real null, worth a row) and
when the control arm already aced the scenario so nothing could have differed (a
void row: the measurement failed, not the skill). Only the control arm's
absolute rate separates them.

The rule is deterministic, so it is a gate rather than doctrine.
**`skill.probe-headroom`** — [`scripts/check-skill-probe-headroom.sh`](../../scripts/check-skill-probe-headroom.sh),
rule in `cli/internal/probeheadroom`, helper `cli/cmd/probe-headroom`:

```bash
# Classify one probe group (all scorecards for one probe, one per effort level).
cd cli && go build -o bin/probe-headroom ./cmd/probe-headroom && cd ..
cli/bin/probe-headroom docs/evals/scorecards/<date>/<probe>-*.json

# Gate: prove the detector still discriminates, then sweep every group.
bash scripts/check-skill-probe-headroom.sh
```

A scenario is **SATURATED** when the control arm scores ≥ 0.75 at two or more
effort levels with ≥ 2 usable control reps each. Exit `0` SEPARATED · `3`
SATURATED · `4` FLOOR · `5` UNMEASURED. A saturated scenario is **retired, not
re-run** — re-running it produces ledger rows, not knowledge; promote it to a
tier-2 seeded-defect probe or record the honest ceiling finding.

**First reading on the historical corpus: 7 of 11 probe groups are SATURATED.**
That is why those INERT rows could never become coverage, and it is the reason a
new ledger row must cite a passing pre-screen before it counts as evidence.

The gate is advisory (`Blocking:false`): a saturated historical group is a true
finding about the ledger, not a regression introduced by the change under test.

## Spine first, ratchet does the rest

The advisory gate `skill.probe-coverage`
(`scripts/check-skill-probe-coverage.sh`) names every product-/judgment-tier
skill lacking a current, canonical-skill-mode, manifest-backed result. After
the 2026-08-16 provenance migration, the historical rows are excluded and
current coverage is 0/11 against a **declared denominator**: 12 skills carry a
product/judgment badge and `goals` (a pure `alias-of fitness` that delegates
verbatim) is excluded with its written argument in
`scripts/.skill-probe-denominator-exclusions`, because probing it would measure
`fitness` and file the verdict under the wrong name. The gate stays
advisory-first until a deliberately selected spine is recaptured under the
current contract.

## Evidence lands dated

Every counted run has an immutable fixture set, an
`agentops-skill-probe.v3` scorecard under a dated `docs/evals/scorecards/`
directory, a passing `skill.probe-headroom` pre-screen over that scorecard
group, a short dated evidence note when interpretation is needed, and a row
in the **Behavioral Probe Ledger (MEASUREMENT STATUS)** at
`evals/skill-probes/LEDGER.md`. The ledger is hand-maintained and never belongs
inside generated `skills/SKILL-TIERS.md`, where regeneration once wiped it.
