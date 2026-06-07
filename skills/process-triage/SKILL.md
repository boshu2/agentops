---
name: process-triage
user-invocable: false
skill_api_version: 1
metadata:
  tier: execution
description: >-
  Triage system processes with the `pt` wrapper and choose safe remediation.
  Use when diagnosing runaway processes, comparing `pt scan` and `pt deep-scan`,
  or using `pt agent` plan/apply workflows.
---
# process-triage

Use `pt` as the user-facing command and `pt help` for wrapper-aware help.

Core workflows:
- `pt scan --format json`
- `pt deep-scan --format json`
- `pt agent plan`
- `pt agent explain`
- `pt agent apply`

Safety:
- Never kill automatically without explicit user approval
- Prefer `pt agent` over legacy `pt robot`
- Use structured output for agent automation
