---
name: agy-native
description: Use an explicitly selected AGY runtime for
---
# AGY Native

Use AGY only when the caller explicitly selects that runtime. Discover its live
command surface before acting and scope every session to the supplied workspace
and packet.

- Keep author and validator sessions distinct when AGY supplies both roles.
- Persist the runtime conversation/context identity and artifact references.
- Validators remain read-only and hand judgment to Validate; they do not write
  the core verdict directly.
- AGY plugin, memory, permission, retry, and session state remain substrate facts
  and never become AgentOps phase, queue, or completion state.
- Never invoke `claude -p` through an AGY wrapper.

Return evidence to the caller and stop. Installation, plugin mutation, and
recurring scheduling require separate explicit authorization.
