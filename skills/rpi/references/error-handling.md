# Error Handling

| Failure | Behavior |
|---------|----------|
| Skill invocation fails | Log error, retry once. If still fails, stop with checkpoint. |
| User abandons at sub-skill gate | /rpi stops with checkpoint (only in --interactive mode) |
| /crank returns BLOCKED | Return evidence to the orchestrator; an explicit retry may re-crank with context (max 2). |
| /crank returns PARTIAL | Return remaining-work evidence to the orchestrator; an explicit retry may re-crank (max 2). |
| Premortem FAIL | Re-plan with fail feedback, re-run premortem (max 3 total attempts) |
| Validate WARN or FAIL | Preserve the verdict, run Learn, then let the orchestrator choose re-plan/retry/continue/stop/escalate. |
| Max retries exhausted | Take ONE bounded helper pass before the operator: hand the blocker, the evidence, and what was tried to a fresh context or cross-family model (`codex exec`, `/council`); resume on UNSTUCK. Only if the blocker survives that pass (or the class is refusal-lane / explicit-judgment / budget-exhausted — those skip the helper; no consult on a spent time/cost ceiling), stop with message + path to last report — that is what needs human attention. Never a second helper pass on the same blocker class. |
| Context feels degraded | Log warning, suggest starting new session with --from |
