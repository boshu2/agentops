---
name: account-rotation
user-invocable: false
skill_api_version: 1
hexagonal_role: supporting
consumes: []
produces: []
context_rel: []
metadata:
  dependencies: []
  capabilities: [account_rotation]
  effects: []
  canonical_status: canonical
  disposition: keep_specialist
  tier: execution
description: 'Switch a caller-selected coding-agent account and report the observed identity.'
practices:
- pragmatic-programmer
---
# Account rotation — credential adapter

Choose the credential tool from both host and agent family, perform only the
explicit account switch, and report the identity observed by the matching
runtime.

## Boundary

- On macOS with Claude credentials, use the operator's `claude-acct` route.
- For Codex, Gemini, Linux, or WSL file-backed credentials, use `caam`.
- Verify account identity through the target runtime; token bytes are not account
  identity.
- Existing processes retain credentials already loaded in memory. Rotation
  affects a new process.
- This skill does not restart work, resume a task, select a pane, move repository
  state, or decide what happens after the switch.

Return the host, agent family, selected tool, requested account/profile, observed
identity/status, command exit code, and whether a new process is required.
