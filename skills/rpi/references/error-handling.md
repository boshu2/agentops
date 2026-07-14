# Error Handling

| Failure | Behavior |
|---------|----------|
| Skill invocation fails | Preserve the error and return it to the orchestrator for classification through the persistent governor. |
| User abandons at sub-skill gate | /rpi stops with checkpoint (only in --interactive mode) |
| /crank returns BLOCKED | Return evidence to the orchestrator; Crank does not own a retry decision. |
| /crank returns PARTIAL | Return remaining-work evidence to the orchestrator; another wave requires a durable governor admission. |
| Premortem FAIL | Return the immutable finding to the orchestrator; only an admitted changed plan may reach Premortem again. |
| Validate WARN or FAIL | Preserve the verdict, run Learn, then let the orchestrator choose re-plan/retry/continue/stop/escalate. |
| Breaker evidence appears | Submit the blocker class to the persistent governor. `HOLD` authorizes its helper; `ANDON` stops. Phase code never creates a helper allowance. |
| Hard ceiling refuses admission | Stop with the governor receipt. A phase cannot reset usage, buy a helper, or replace the run state. |
| Context feels degraded | Log warning, suggest starting new session with --from |
