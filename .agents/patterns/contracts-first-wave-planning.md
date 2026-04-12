---
maturity: candidate
utility: 0.64
---

# Contracts-First Wave Planning

## Pattern

When parallel waves touch shared behavior, define the contract before assigning
implementation lanes. The contract can be an interface, schema, manifest shape,
CLI output format, test fixture, or documented invariant that every lane agrees
to preserve.

## Why It Works

Parallel agents fail when they infer different boundaries. A contracts-first
plan gives each lane a stable interface boundary, clear ownership, and a shared
validation target before files diverge.

## How To Apply

1. Identify the shared boundary that all lanes depend on.
2. Write the smallest durable contract for that boundary.
3. Assign file ownership around the contract, not around vague feature names.
4. Add contract tests or fixture validation before broad implementation.
5. Merge waves only after every lane proves it still satisfies the same contract.

## Retrieval Cues

Contracts first wave planning, interface boundary, contract test, parallel wave
ownership, file ownership, shared schema, output contract, implementation lane.
