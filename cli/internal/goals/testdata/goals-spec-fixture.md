# Goals

A fixture GOALS.md exercising the executable-spec directive-block patcher.

## North Stars

<!-- agentops:claim:AOP-CLAIM-FIXTURE -->
- The patcher preserves every byte it does not deliberately change

## Anti Stars

- Lossy round-trips that drop prose, comments, or table columns

## Directives

### 1. Keep the patcher non-lossy

The patcher must never re-render the file from a model. It edits a single
directive block in place.

**Steer:** increase (preserved bytes)

### 2. Carry structured attribute metadata

A directive declares stable metadata as bold attribute lines.

**Directive ID:** d-existing-two
**Steer:** decrease (lossy writes)
**Setpoint:** AOP-CLAIM-FIXTURE | exact wording | GOALS.md
**Scenarios:** s-2026-05-17-001, s-2026-05-17-002
**Scenario threshold:** 0.8
**Tags:** fixture, executable-spec

### 3. Survive directives that have no attributes

This directive intentionally carries no attribute lines so the patcher's
attribute-insertion path is exercised on a bare block.

## Three-Gap Contract Proof Surface

This section does not exist in the GoalFile model and must survive patching.

| Gap | What fails without it | Currently enforcing |
|-----|-----------------------|---------------------|
| 1   | judgment              | some-gate            |

## Gates

| ID | Check | Weight | Description | Tags |
|----|-------|--------|-------------|------|
| fixture-gate | `true` | 5 | A fixture gate with a Tags column | warn-only |
