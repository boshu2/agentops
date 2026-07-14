# Error Handling

| Failure | Behavior |
|---------|----------|
| Skill invocation fails | Preserve the error and return it to the orchestrator for one evidence-bound disposition. |
| User abandons at sub-skill gate | /rpi stops with checkpoint (only in --interactive mode) |
| /crank returns BLOCKED | Return evidence to the orchestrator; Crank does not own a retry decision. |
| /crank returns PARTIAL | Return remaining-work evidence to the orchestrator; the soft tranche boundary does not escalate. |
| Premortem FAIL | Return the immutable finding to the orchestrator; only a materially changed plan may reach Premortem again. |
| Validate WARN or FAIL | Preserve the verdict, run Learn, then let the orchestrator choose re-plan/retry/continue/stop/escalate. |
| Stuckness evidence appears | Record `HOLD` with the blocker class and freeze mutation for one bounded fresh-context consultation. |
| A hard external ceiling is spent | Record `ANDON` with the ceiling evidence and notify the operator. |
| Context feels degraded | Log warning, suggest starting new session with --from |
