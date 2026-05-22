# ADR-0007: Deterministic `/evolve` Loop — Only the Operator Stops It

- **Status:** Accepted (2026-05-22)
- **Author:** AgentOps maintainers
- **Tracking:** bead `soc-sfjx`
- **Origin:** ported from the mt-olympus unbounded-evolve substrate (`docs/decisions/2026-05-21-deterministic-loop-only-operator-stops.md`), which has driven ~245 cycles without self-halting.

## Context

`/evolve` is meant to run continuously while open work exists, with the operator **on** the loop (curating intent in `GOALS.md`, `PRODUCT.md`, ADRs, and the `bd ready` queue) rather than **in** it. Its self-regulation defaults were written assuming the agent IS the operator in a single burst session — so several defaults let any cycle's agent self-halt the loop on heuristics the operator never sanctioned (`CONTEXT_BUDGET_EXHAUSTED`, scout-streak halt, "honest stop"). `soc-5qit` already removed sticky `DORMANT`. This ADR makes the no-self-stop rule doctrine the loop re-reads every cycle, and — load-bearing — **mechanical**, not prose the agent can rationalize past.

## Decision

1. **The agent MAY NOT self-halt.** The only stop signals are operator-written:
   - `~/.config/evolve/KILL` (global), `.agents/evolve/STOP` (repo) — honored within `EVOLVE_KILL_TTL_DAYS` (default 7); stale markers are surfaced and bypassed.
   - `.agents/evolve/DORMANT` — non-sticky: auto-cleared whenever `bd ready` or harvested next-work has items (`soc-5qit`).
   - Explicit operator removal of the driving cron / session.
2. **The check is mechanical.** `scripts/evolve/halt-check.sh` runs before every cycle (Step 1 of the skill calls it). Marker checks, goal-regression, and prior-cycle-FAIL are evaluated in code, not in skill prose the agent might skip. `goal_regression` (latest `goals_passing` < prior productive cycle, read from `cycle-history.jsonl`) halts the loop for operator attention — revert-on-red, enforced.
3. **When blocked, reason — don't stop.** The unblock ladder is mandatory: re-read the bead, grep for the sibling pattern, decompose into a smaller primitive, scout productively, pick a different ready bead or a bug-fix, file a discovered-from sub-bead, log via `ao evolve blocked`. NEVER write a STOP marker as an escape hatch.

## Consequences

- An empty claimable queue produces honest **operator-wait** (idle/sanity cycles), never false dormancy.
- A genuine regression halts the loop so a human looks — the loop never papers over red.
- The guarantee is testable: `tests/scripts/evolve-halt-check.bats` asserts both the gate's behavior and that the skill actually invokes it (no orphaned-primitive regression).

## Evidence this is needed

The `soc-g2qd` epic (2026-05-21) shipped six `/evolve` CLI primitives whose enforcement was *prose in the skill* — and the skill called none of them. An unsupervised session built the wrong thing for ~10 hours because its only gate was green CI. Mechanical, doctrine-anchored guardrails read every cycle are what keep mt-olympus's loop honest across hundreds of cycles; this ADR ports that property. See `.agents/research/2026-05-21-mt-olympus-evolve-loop.md`.
