# Error Handling

| Failure | Behavior |
|---------|----------|
| Skill invocation fails | Log error, retry once. If still fails, stop with checkpoint. |
| User abandons at sub-skill gate | /rpi stops with checkpoint (only in --interactive mode) |
| /crank returns BLOCKED | Re-crank with context (max 2 retries). If still blocked, stop. |
| /crank returns PARTIAL | Re-crank remaining items with context (max 2 retries). If still partial, stop. |
| Pre-mortem FAIL | Re-plan with fail feedback, re-run pre-mortem (max 3 total attempts) |
| Vibe FAIL | Re-crank with fail feedback, re-run vibe (max 3 total attempts) |
| Max retries exhausted | Stop with message + path to last report. Manual intervention needed. |
| Context feels degraded | Log warning, suggest starting new session with --from |
