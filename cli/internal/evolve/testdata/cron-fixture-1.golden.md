
# /evolve loop-mode cron prompt (cycle 7)

You are in /evolve --mode=loop. This is cycle 7.

## Last cycle outcome

Shipped: abc1234 (soc-ij7e #cron-template); def5678 (soc-hwax); 
Tests delta: +4 passing, 0 new failures

## Sub-beads filed this cycle

- soc-zzz1
- soc-zzz2


## Recommended next work

soc-6svt (advisory; Layer-3 authority may override)

<!-- VERBATIM-PRESERVE:start name="3-adr-cite" -->
Load-bearing context: docs/plans/2026-05-21-evolve-loop-epic-design.md (this epic's design memo) and the bead's per-cycle acceptance refinements.
<!-- VERBATIM-PRESERVE:end -->

<!-- VERBATIM-PRESERVE:start name="unblock-ladder" -->
## 7-step unblock ladder

If `bd ready` returns ≥1 unblocked bead, claim and work one.
If you feel "blocked": log via `ao evolve blocked --reason '<text>'` (Wave 2). NEVER write a STOP marker.
The ladder (programmatic enforcement: soc-mlbm Wave 2):
1. Shape-compatible ready bead exists → pick it
2. Sibling pattern in another bead's description → surface match
3. Decomposable epic → recommend smaller child
4. Cross-Hop pickup → traverse discovered-from chain
5. Bug-fix fallback → smallest-surface bug
6. Still nothing? Log blocked event, pick smallest available P1 anyway
7. NEVER write STOP. Operator owns stop signals.
<!-- VERBATIM-PRESERVE:end -->

<!-- VERBATIM-PRESERVE:start name="layer-3-authority" -->
## Layer-3 shaping authority

The agent can:
- File new beads as discoveries land (`bd create` + `discovered-from`)
- Restructure the DAG
- Update the cron's recommended-next pointer
- Write ADRs for emergent precedents in `docs/decisions/`

Bounded by: bead-graph coherence, operator-set scope (preferences.yaml), no-self-stop principle.
<!-- VERBATIM-PRESERVE:end -->

<!-- VERBATIM-PRESERVE:start name="no-self-stop" -->
## The no-self-stop principle

In `--mode=loop`, the agent NEVER writes a STOP/DORMANT/KILL marker. `ao evolve write-stop-marker` mechanically refuses (exit 1). When stuck: log via `ao evolve blocked`, pick smallest-available P1 anyway. Stop signals come from the operator, never the agent.
<!-- VERBATIM-PRESERVE:end -->
