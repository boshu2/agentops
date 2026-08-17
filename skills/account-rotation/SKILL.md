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
  effects: [rotate_agent_account]
  canonical_status: canonical
  disposition: keep_specialist
  tier: execution
description: 'Switch a caller-selected coding-agent credential profile and report the credential tool post-state without claiming target-runtime identity. Triggers: "switch account", "rotate coding-agent account".'
practices:
- pragmatic-programmer
output_contract: credential profile post-state, command status, and explicit runtime-identity gap
---
# Account rotation — credential adapter

Choose the credential tool from both host and agent family, perform only the
explicit account switch, and report the credential tool's selected-profile
post-state. This package has no target-runtime identity probe.

## Enforced surface

Run credential mutation only through `scripts/rotate.sh`. It validates the
family/profile grammar, requires an exact `rotate:<family>:<profile>` approval,
attests the selected tool's live command surface, switches once, and requires
the tool's post-switch state to name the requested profile. Missing approval,
capability, platform match, post-state, or the nonrenewing 1–120 second deadline
stops without a successful-rotation claim.

The tool post-state proves only which stored profile was selected. It does not
prove the external credential manager or provider safe, and it is not target
runtime identity. Start a new matching runtime and perform its identity probe
before claiming the account observed by that runtime; until then the receipt
states `runtime_identity_not_checked=true`.

Target-runtime identity remains a separate check because the runtime is the
only party whose opinion matters: credential files can be swapped perfectly
and still authenticate as the old account in an already-running process. Until
a separately authorized runtime-specific adapter performs that check, this
skill returns `runtime_identity_not_checked=true` and makes no account-identity
claim.

Named failure mode — **stale-process identity**: declaring the rotation done
while every live session still holds the previous account's tokens in memory.

Anti-pattern: confirming a switch by diffing credential file bytes.
Corrective: ask the matching runtime who it is now, and report whether a new
process is required for the answer to hold.

## Boundary

- Perform only the account switch the caller explicitly authorized; rotation
  mutates host credential state and is never implied by repository access.
- The credential tool is caller- or operator-selected per host and agent family;
  the names below are this operator's routes, not a universal prescription. On
  macOS with Claude credentials the route is `claude-acct` (Keychain-backed);
  file-backed Codex, Gemini, Linux, or WSL credentials use `caam`. Never use
  `caam` for macOS Claude account operations.
- Treat target-runtime identity as unchecked; token bytes and credential-tool
  profile names are profile state rather than account identity.
- If neither the selected credential tool nor a runtime identity probe is
  available, report that absence as a disclosed fact and stop. Never fall back to
  diffing credential-file bytes to declare a switch done.
- Existing processes retain credentials already loaded in memory. Rotation
  affects a new process.
- This skill does not restart work, resume a task, select a pane, move repository
  state, or decide what happens after the switch.

Return the host, agent family, selected tool, requested profile, tool-reported
profile state before and after the switch, command exit code,
`runtime_identity_not_checked=true`, and whether a new process is required.
Account identity and the credentials held by existing processes remain
`not_checked`.

Done means one authorized switch was attempted, the selected tool reported the
requested post-state, and the receipt explicitly separates tool profile state
from runtime identity. The package test compares an unguarded raw mutation with
the wrapper's pre-action refusal; it does not attest either external tool.
