---
package: cli/internal/types
status: active
owner: agentopsd
contract_source: package-local Go types and retained generic evidence schemas
---

# cli/internal/types

Shared Go value types retained by the read-only CLI surfaces.

## Status

**Active supporting package.** It contains values used by retained inspection
and evidence features. It is not an AgentOps lifecycle state machine.

## Ownership

- Package-local tests own serialization compatibility.
- Retained record schemas describe generic evidence only and cannot decide a
  core phase, verdict, continuation, or delivery outcome.

## Existing surface (pre-l12)

| File | Domain |
|---|---|
| `types.go` | Claude Code transcript pipeline types (TranscriptMessage, etc.) |
| `errors.go` | Shared sentinel errors |
| `*_test.go` | L1/L2 coverage for the above |

## Non-goals

- This package does NOT validate against the JSON schemas at runtime — that's
  the caller's responsibility (likely a separate `cli/internal/schemas`-style
  package or an external `jsonschema` lib).
- This package does not implement state-machine transition rules.
