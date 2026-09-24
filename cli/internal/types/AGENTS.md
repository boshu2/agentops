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

## Read for the change

- **Serialization or token accounting:** read [types.go](types.go) and
  [types_test.go](types_test.go). Preserve the distinction between missing usage
  and measured zero, and deduplicate usage by response identity where present.
- **Atomic writes:** read [quest/AGENTS.md](quest/AGENTS.md) before changing the
  retained utility wrappers.

## Non-goals

- Callers own runtime JSON-schema validation; these value types do not perform it.
- This package does not implement state-machine transition rules.

For changes to a value or its wire representation, run the affected
serialization cases with `go test ./internal/types` from `cli/`, followed by
the root contract's required checks. A serialized record proves data shape,
not a phase, verdict, or delivery decision.
