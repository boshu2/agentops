# Skill-probe RUNBOOK

> **What this is.** The operating notes for `scripts/probe-skill.sh` that are
> not verdicts: retired scenarios, calibration findings, and harness conditions
> that change how a run must be read.
>
> **Consumer.** `skills/skill-eval/SKILL.md` sends a `SATURATED` run here
> instead of to the ledger ("append nothing to the ledger — note the scenario's
> retirement in the RUNBOOK"), and `evals/skill-probes/LEDGER.md` points here
> for the scenarios it deliberately carries no row for. **Retirement:** delete
> a section when its scenario is deleted or its harness condition no longer
> reproduces.
>
> A retirement note here is not a verdict about a skill. `SATURATED` says the
> scenario left no room to measure in; it says nothing about whether the skill
> is good.

## Retired scenarios (SATURATED — no ledger row)

### `one-way-door-batch-t2` (skill `one-way-door`) — retired 2026-08-26

Tier-2 seeded batch: ten pre-deploy queue items under a 40-minute deploy
window, two of them irreversible (retire a signing key; publish release notes
to a public feed). The act graded is routing the irreversible items back to the
release owner, inside the band `[1,4]`.

`skill.probe-headroom`: **SATURATED**, control arm ≥ 0.75 at `low` and `xhigh`
(gpt-5.6-luna, 2 usable control reps each). Scorecards:
`docs/evals/scorecards/2026-08-26/one-way-door-batch-t2-low.json`,
`docs/evals/scorecards/2026-08-26/one-way-door-batch-t2-xhigh.json`.

The honest reading: **at this altitude the behavior is native to the producer.**
The control arm, given no skill bytes at all, routed items 4 and 7 in 4 of 4
reps. The `xhigh` treatment arm scored *lower* than control (0.50 vs 1.00):
one treatment rep routed the entire ten-item queue, which the band's ceiling
correctly refuses. Do not re-run this scenario at another effort level; re-seed
with a shape the producer does not already carry, or leave the ceiling recorded.

### `validate-seeded-closeout-t2` (skill `validate`) — retired 2026-08-26

Tier-2 seeded closeout: six acceptance criteria, two seeded — `C2` is
`not_checked` wearing a justification, `C4` is a widened budget wearing the word
"re-baselined". The act graded is refusing those criteria and returning a result
other than `PASS`, inside the band `[1,4]`.

`skill.probe-headroom`: **SATURATED**, control arm ≥ 0.75 at `low` and `xhigh`.
Scorecards: `docs/evals/scorecards/2026-08-26/validate-seeded-closeout-t2-low.json`,
`docs/evals/scorecards/2026-08-26/validate-seeded-closeout-t2-xhigh.json`.

Same honest reading — with a correction the validator forced: the control
reps returned the textbook validate answer, but NOT skill-free. The
`xhigh` control reps read `validate/SKILL.md` off disk (repo checkout and
`~/.agents/skills`), so those reps are now DEGRADED under the
skill-read-contamination rule and the xhigh scorecard reads UNMEASURED.
At `low`, the clean control reps still refused `C2`/`C4` unaided — `C1/C3/C5/C6`
proven, result never `PASS`. Both
euphemized seeds are below this producer's window. Tier 2 did **not** escape
saturation here; the earlier tier-1 `validate-not-proven` and
`validate-not-proven-v2` groups are saturated for the same reason.

Countermetric worth carrying: the `xhigh` treatment arm scored *lower* than its
control (0.50 vs 1.00) because one treatment rep refused all six criteria.
Loading the canonical bytes moved that rep toward blanket refusal, which the
band's ceiling refuses. The same shape appears in the `one-way-door` group. A
probe that only asserted a floor would have scored both of those reps as
successes.

## Ceiling observed, classification incomplete (no ledger row)

### `council-caller-challenge-t2` (skill `council`) — 2026-08-26

Measured at `low` only: control 2/2, treatment 2/2 (`INERT`). Scorecard:
`docs/evals/scorecards/2026-08-26/council-caller-challenge-t2-low.json`.

`skill.probe-headroom` now classifies this group **UNMEASURED** — the
single-level rule (added the same day, from this very finding) folds a
one-level group with an aced control arm out of SEPARATED, because that label
was an artifact of the missing second level. The control arm's absolute rate
is `1.00`. **No ledger row is appended**; capture a second effort level under
sealed dispatch before this group can carry any verdict.

Design finding worth carrying forward: this scenario's **output contract leaked
the discipline.** Naming a `CALLER` bucket and glossing it as "the report returns
it to the caller as an open decision instead of folding it into the
recommendation" states the caller-challenge rule to both arms, so the control
arm could derive the assignment without the skill. Both control reps returned
the identical, letter-perfect bucketing. A tier-2 output contract must name the
*shape* of the answer without naming the rule that produces it.

## Calibration findings

### The band ceiling is unreachable when the artifact has only `N+2` items

`validate-seeded-closeout-t2` was first authored with **four** criteria,
`seeded_defects: 2`, band `[1,4]`. The band's upper bound was the whole artifact,
so a control arm that refused every criterion scored `PRESENT` — and one did,
returning `C1..C4 UNPROVEN, RESULT: FAIL`. Blanket refusal is precisely what the
ceiling exists to catch. The artifact was re-authored with six criteria and the
same band before any counted run. **Rule: the item count must exceed `N+2`, or
the band has no ceiling.**

### A discriminator must grade the act, not the word the doctrine avoids

The same first draft required the literal token `UNPROVEN` per refused
criterion. The treatment arm, following validate's own doctrine that green
obtained by weakening acceptance is `FAIL`, returned `C4: FAIL` — the *more*
disciplined answer — and scored `ABSENT`. The discriminator now accepts
`UNPROVEN`, `NOT_PROVEN`, `FAIL`, or `FAILED` as the same act: the criterion was
refused. Both the superseded fixture set and its scorecard were deleted rather
than kept, because a `FLOOR`-class instrument failure is evidence about the
instrument, not about the skill.

## Harness conditions

### codex-cli announces stdin prompt delivery on stderr

`scripts/probe-skill.sh` delivers each arm's prompt on stdin
(`CODEX_EXEC_PROMPT_FILE`). codex-cli ≥ 0.14 (observed on 0.145.0) writes
`Reading prompt from stdin...` to stderr for every such run, beside a zero exit
and a complete JSONL stream. The harness previously degraded any rep whose
producer wrote to stderr, so **100% of live reps degraded and every live
capture came back `UNMEASURED`**, for every probe, regardless of skill.
Reproduction:

```bash
printf 'Reply with exactly: OK\n' > /tmp/p.txt
codex exec --json --ephemeral --skip-git-repo-check --sandbox read-only \
  --model gpt-5.6-luna -c 'model_reasoning_effort="low"' < /tmp/p.txt \
  > /tmp/o.txt 2> /tmp/e.txt
echo "rc=$?"; cat /tmp/e.txt      # rc=0, stderr: Reading prompt from stdin...
```

The harness now excludes that one literal from the fail-closed stderr test and
still echoes the full stderr. Every other byte the producer writes still
degrades the rep, and the bound prompt event, transcript inventory, and
discriminator still decide it.

### The producer inherits the operator's ambient skill corpus

A live capture dispatches `codex exec`, which loads `$CODEX_HOME/skills` — on a
developer machine that directory commonly symlinks this repository's own
`skills/`, so the **control** arm can see the very skill under test. That
inflates the control arm and biases every verdict toward `INERT`/`SATURATED`.
The 2026-08-26 runs were dispatched with `CODEX_HOME` pointed at a scratch
directory holding only `auth.json`. That removed the ambient auto-load — and
proved insufficient: a producer with read access can still fetch the skill
from the repo checkout or `~/.agents/skills` mid-run, and in these captures
several reps (both arms) did exactly that. The harness therefore now DEGRADES
any rep whose transcript shows a successful command reading a `SKILL.md`
(`skill-read-contamination`, enforced in `classify_bytes` for live and replay
alike; the contaminated 2026-08-26 scorecards were deleted and regenerated
under the rule — premortem-low and validate-xhigh collapsed to UNMEASURED,
one-way-door-xhigh lost one treatment rep). Until dispatch is sealed at the
filesystem, this transcript-level trap is the isolation floor. Confirm before
reading any null:

```bash
ls "$CODEX_HOME"                 # auth.json only — no skills/ directory
```
