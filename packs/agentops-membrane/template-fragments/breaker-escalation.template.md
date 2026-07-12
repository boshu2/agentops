{{- define "breaker-escalation" -}}
## Circuit-breaker escalation (mandatory)

- Plain `REFUTED -> AUTO-REDO`. Repair from the findings and re-gate while the
  breaker remains closed; a semantic rejection is routine evidence, not an
  operator andon.
- The default max-attempts threshold is three re-work/re-gate cycles and is
  tunable. Reaching it is `CIRCUIT-BREAKER-TRIP -> HOLD`, not direct human
  escalation and never authorization to land.
- `HOLD -> ONE-HELPER`. The coordinator gives the blocker, cumulative evidence,
  and attempted approaches to exactly one advisor in a fresh context or an available cross-family model.
  The helper does not mutate the work or open the door.
- `HELPER-UNSTUCK -> AUTO-REDO`. Apply the concrete new approach, reset the
  breaker counter for that approach, and re-earn the normal independent verdict.
- `HELPER-ESCALATE -> HUMAN`. Consult the operator only when the one helper
  confirms that authority or judgment is required, or when the blocker is an
  explicit refusal/judgment lane.
- Skip the helper only when a hard time, cost, or quota ceiling is actually spent
  and cannot fund one consultation. A retry count is a stuckness signal, not a
  spent hard budget.
- A goal controller's repeated-turn threshold governs status bookkeeping only;
  it never substitutes for this work-escalation path and never permits skipping
  the helper.
{{- end -}}
