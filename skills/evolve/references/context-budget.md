# Context Budget — A Third Stop Reason

The skill's two declared stop reasons (kill switch / dormancy) don't catch the failure mode where the loop has actionable work but the work is the wrong shape for the current cycle's context budget. This reference defines `CONTEXT_BUDGET_EXHAUSTED` as a third stop reason.

## Why it's a real failure mode

A long session that has shipped 10+ productive cycles accumulates context: read files cache, tool-result trails, prior `/rpi` discovery findings. When cycle 11 selects an item that requires a fresh deep read of unfamiliar surfaces, the cycle either:

- Forces a scout-mode return because the context is too heavy to execute (correct outcome, but the skill doesn't currently classify this as anything but `harvested` or `idle`).
- Attempts execution and produces shallow work that the regression gate or self-review catches (recovery cost > the cycle's value).

Neither path is captured by the existing dormancy gate (`IDLE_STREAK >= 2 AND GENERATOR_EMPTY_STREAK >= 2`). Work IS found; it just can't be safely executed.

## The counter and the gate

Maintain `context_streak` in `.agents/evolve/session-state.json`. Increment when ANY of these are true at end-of-cycle:

- Cycle result is `scout` AND the scout's `disposition` says "context too heavy"
- Cycle result is `harvested` AND the harvest was a context-budget defer (vs. a feature-suggestion)
- The /rpi cycle aborted before commit because discovery context overflowed

Reset to 0 when a productive `improved` cycle lands.

```bash
context_streak=$(jq -r '.context_streak // 0' .agents/evolve/session-state.json)

if [ "$context_streak" -ge 2 ]; then
  echo "CONTEXT_BUDGET_EXHAUSTED after $context_streak consecutive heavy-context cycles."
  echo "Parked work:"
  jq -r '.parked_work[] // empty' .agents/evolve/session-state.json
  # Hand off via cycle-history.jsonl with result: "context-budget-exit"
  exit 0
fi
```

## Handoff message

When the gate fires, write a handoff entry that names the parked work concretely:

```json
{"cycle": N, "result": "context-budget-exit",
 "selected_source": "<source>", "work_ref": "<ref>",
 "milestone": "CONTEXT_BUDGET_EXHAUSTED after 2 heavy-context cycles. Parked: <work refs>. Resume in a fresh session."}
```

The next operator session reads this entry, knows what was parked, and can either continue manually or fire a fresh `/evolve` with cleared context.

## Configurable threshold

Default `context_streak` threshold is 2. Override with `EVOLVE_CONTEXT_STREAK_LIMIT=N` env var if a particular session needs to tolerate more heavy cycles (rare).

## ScheduleWakeup interaction

When running the Claude-Code-harness self-perpetuation mode (see `references/autonomous-execution.md`), a context-budget exit MUST NOT call `ScheduleWakeup`. The loop terminates and surfaces the handoff message. Re-firing would re-load the heavy context and repeat the failure.
