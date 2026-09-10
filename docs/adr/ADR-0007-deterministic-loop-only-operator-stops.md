# ADR-0007: Deterministic `/evolve` Loop — Only the Operator Stops It

- **Status:** Accepted, amended for selected CDLC 2026-09-06 (2026-05-22)
- **Author:** AgentOps maintainers
- **Tracking:** bead `soc-sfjx`
- **Origin:** ported from the mt-olympus unbounded-evolve substrate (`docs/decisions/2026-05-21-deterministic-loop-only-operator-stops.md`), which has driven ~245 cycles without self-halting.

## Active disposition — 2026-09-06 CDLC adoption

The Decision below's unlimited/operator-only stop rule and automatic bypass of
stale stop markers are superseded for selected finite campaigns. An explicitly
selected outer goal owns experiment authorization within its accepted envelope;
native time, cost, quota, stop and continuation controls bound execution. A
human stop or native HOLD cannot be bypassed by autonomy. RPI still stops under
its admitted repair law and returns honest outcomes; it acquires no goal
controller, helper loop, budget account or work-selection authority.

Under the 2026-09-07 stopping amendment, informative red may support a different
experiment within unchanged acceptance. A selected goal's causal HOLD allows
exactly one bounded fresh helper per incident inside the existing allowance;
recurrence is a reason to examine cause, not proof the design is wrong.
Cancellation, explicit refusal/judgment, and spent hard time/cost/quota skip the
helper. Repeated continuation never resets totals or helper use. A retry count
alone is not a spent hard budget, and status bookkeeping is not permission for
more work. Goal objective text and terminal reports demonstrate neither a native
pause nor an aggregate allowance operation; report observed enforcement and
measurement gaps truthfully.

Preserve the lesson demonstrated below: an unused helper enforces nothing.
Native continuation/stop behavior needs observed consumer execution and bounded
recovery evidence, not prose, elapsed time or green CI. T23/T24 own the native
continuation amendments with implemented behavior; T25 alone owns evolve
restoration after that proof. The old evolve commands, marker machinery and
operator-only rules remain retired. This disposition neither starts recurrence
nor reinstates unlimited operation. Earlier evidence is preserved as history.

## Historical context

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
- The same operator-markers-are-authoritative principle generalizes to the **in-session** path: an autonomous-drive directive (a `/goal` Stop-hook, a `/loop`) cannot override a human STOP marker or a pawl HOLD any more than it can self-halt the evolve loop here. See `pawls.md` § Directive precedence (historical reference; retired path: `../contracts/pawls.md#directive-precedence-autonomy-never-overrides-a-human-gate`).

## Evidence this is needed

The `soc-g2qd` epic (2026-05-21) shipped six `/evolve` CLI primitives whose enforcement was *prose in the skill* — and the skill called none of them. An unsupervised session built the wrong thing for ~10 hours because its only gate was green CI. Mechanical, doctrine-anchored guardrails read every cycle are what keep mt-olympus's loop honest across hundreds of cycles; this ADR ports that property. See `.agents/research/2026-05-21-mt-olympus-evolve-loop.md`.
