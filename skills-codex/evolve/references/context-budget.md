# Context Budget — Session Handoff, Not a Stop (soc-5qit)

**Prior framing (deprecated):** context exhaustion was a third stop reason that wrote sticky `DORMANT` and ended the run.

**Current framing (soc-5qit, 2026-05-21):** context exhaustion is a *session-handoff signal*. The loop is continuous across compactions; a heavy session writes a non-sticky `HANDOFF` marker, exits the current cron-fire turn cleanly, and the NEXT cron-fire (running on harness-compacted or fresh context) automatically clears HANDOFF in Step 1 and resumes.

This change was forced by an operator failure mode (2026-05-20→21): a cron-driven /evolve session shipped 6 PRs, accumulated context, then sticky-DORMANT'd despite 10+ unblocked P1 beads in `bd ready` (historical — tracker now `br ready`). The prior design treated "this Claude session can't safely take on more work" as "the work itself is done" — a category error. The cron loop spans many sessions; one session's context is irrelevant to whether the backlog has work.

## Why it's a real failure mode (still true)

A long session that has shipped 5+ productive cycles accumulates context: read files cache, tool-result trails, prior `/rpi` discovery findings. When cycle N+1 selects an item that requires a fresh deep read of unfamiliar surfaces, the cycle either:

- Forces a scout-mode return because the context is too heavy to execute (correct outcome, but the skill must not treat this as terminal).
- Attempts execution and produces shallow work that the regression gate or self-review catches (recovery cost > the cycle's value).

The fix is to surface this as a handoff to the next session, not stop the loop.

## The counter and the signal

Maintain `context_streak` in `.agents/evolve/session-state.json`. Increment when ANY of these are true at end-of-cycle:

- Cycle result is `scout` AND the scout's `disposition` says "context too heavy"
- Cycle result is `harvested` AND the harvest was a context-budget defer (vs. a feature-suggestion)
- The /rpi cycle aborted before commit because discovery context overflowed

Reset to 0 when a productive `improved` cycle lands.

```bash
context_streak=$(jq -r '.context_streak // 0' .agents/evolve/session-state.json)

if [ "$context_streak" -ge 2 ]; then
  echo "CONTEXT_BUDGET_EXHAUSTED after $context_streak heavy-context cycles."
  echo "Writing HANDOFF (non-sticky) and exiting this cron-fire turn."
  # HANDOFF carries ONLY the parked work refs and the timestamp.
  # It is NOT a sticky stop. Step 1 of the next cron-fire clears it automatically.
  cat > .agents/evolve/HANDOFF <<EOF
$(date -u +%FT%TZ)
context_streak=$context_streak (this session's context grew too heavy)
parked_work: $(jq -c '.parked_work // []' .agents/evolve/session-state.json)
next-action: cron-fire will resume on compacted/fresh context
EOF
  # Hand off via cycle-history.jsonl with result: "context-handoff"
  exit 0
fi
```

**Critical:** the HANDOFF marker does NOT block future cron-fires. Step 1 of the next /evolve turn clears it and continues normal work selection.

## Handoff message (logged, not gated)

When the context-handoff fires, write a cycle-history entry that names the parked work concretely:

```json
{"cycle": N, "result": "context-handoff",
 "selected_source": "<source>", "work_ref": "<ref>",
 "milestone": "Context-handoff after 2 heavy cycles. Parked: <work refs>. Next cron-fire (compacted/fresh) resumes."}
```

The next operator session reads this entry, knows what was parked, and can either continue manually or wait for the next cron-fire to handle it.

## Configurable threshold

Default `context_streak` threshold is 2. Override with `EVOLVE_CONTEXT_STREAK_LIMIT=N` env var if a particular session needs to tolerate more heavy cycles (rare).

## ScheduleWakeup interaction

When running the Claude-Code-harness self-perpetuation mode (see `references/autonomous-execution.md`), a context-handoff exit MUST NOT call `ScheduleWakeup` from the same heavy-context turn — re-firing would re-load the heavy context. The cron schedule (or operator's external trigger) handles the re-fire. By the time the next cron-fire runs, the harness has compacted or the session has rolled, and HANDOFF auto-clears.

## What this is NOT

- **Not a stop reason.** Stop reasons are operator override, max-cycles cap, regression-breaker, and genuine stagnation (`ao beads exec ready`=0 AND harvested=0 AND failing-goals=0 AND generators dry).
- **Not a sticky marker.** DORMANT is sticky-with-auto-clear-on-new-work; HANDOFF is fully non-sticky and clears on next read.
- **Not a "the work is done" signal.** It explicitly says "this session can't safely continue; the work is parked." The next session does the work.

## Cross-reference

- `SKILL.md` Step 1 — HANDOFF auto-clear on cron-fire-N+1
- `SKILL.md` Step 7 — context exhaustion explicitly removed from stop reasons list
- `references/scout-mode.md` — scope-filter splits or defers; never bails
