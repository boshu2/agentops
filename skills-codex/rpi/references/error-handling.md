# Error Handling

| Failure | Behavior |
|---------|----------|
| Skill invocation fails | Log error, retry once. If still fails, stop with checkpoint. |
| User abandons at sub-skill gate | $rpi stops with checkpoint (only in --interactive mode) |
| $crank returns BLOCKED | Re-crank with context (max 2 retries). If still blocked, stop. |
| $crank returns PARTIAL | Re-crank remaining items with context (max 2 retries). If still partial, stop. |
| Pre-mortem FAIL | Re-plan with fail feedback, re-run pre-mortem (max 3 total attempts) |
| Vibe FAIL | Re-crank with fail feedback, re-run vibe (max 3 total attempts) |
| Max retries exhausted | Take ONE bounded helper pass before the operator: hand the blocker, the evidence, and what was tried to a fresh context or cross-family model ($council, a fresh Codex session); resume on UNSTUCK. Only if the blocker survives that pass (or the class is refusal-lane / explicit-judgment / budget-exhausted — those skip the helper; no consult on a spent time/cost ceiling), stop with message + path to last report — that is what needs human attention. Never a second helper pass on the same blocker class. |
| Context feels degraded | Log warning, suggest starting new session with --from |
