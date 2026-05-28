# AOP-CLAIM-README-EVOLVE-AUTONOMOUS — evidence (v2.39.0)

**Claim location:** README.md sections describing `/evolve` as an
autonomous improvement loop that runs without a human in the inner
loop.

**Claim summary:** `/evolve` autonomously selects work, executes it
through `/rpi`, gates regression, and harvests follow-ups, looping
until a kill switch or real dormancy.

## Repo surfaces that demonstrate it

- `skills/evolve/SKILL.md` — the operator-facing definition of the
  loop, selection ladder, and gates.
- `scripts/evolve-measure-fitness.sh` — fitness measurement at the
  top of every cycle.
- `scripts/evolve-log-cycle.sh` — durable cycle ledger writer.
- `scripts/evolve-capture-daily-learning.sh` — end-of-day
  self-reflection consolidator.
- `.agents/evolve/cycle-history.jsonl` — the running ledger of every
  cycle (productive / harvested / idle / scout).
- `.agents/evolve/daily-learning-log-YYYY-MM-DD.md` — per-cycle
  micro-captures.
- `.agents/learnings/YYYY-MM-DD-evolve-loop-learnings.md` — daily
  consolidated reflections.

## Verification surface

Recent session evidence under `.agents/evolve/cycle-history.jsonl`:
20+ cycles ledgered in a single day, including productive
implementation cycles that landed real commits (D2, D11, D10, G1,
G5, PG1, etc.) and harvested audit cycles that spawned 60+
follow-up beads. Companion: LC1 (bead soc-p1ac.1) which verified
all three layers (micro-capture, every-5th reflect, end-of-day
consolidator) operate end-to-end on a real day.

## Why this is enough

The claim is verifiable from the persistent record. Anyone can read
`cycle-history.jsonl` and the daily learning files to see real
autonomous-loop behavior, not aspirational copy.

## Anti-claim

Not claiming "evolve is unsupervised forever" — the operator can
interrupt at any cycle boundary, the kill switches fire on demand,
and ScheduleWakeup yields control between cycles.
