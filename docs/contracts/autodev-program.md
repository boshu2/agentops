# Autodev Program Contract

> **Status:** Draft
> **Consumers:** `ao autodev`, `/rpi`, future `/evolve` loaders

This contract defines the repo-local operational program for bounded autonomous development. It is intentionally separate from `GOALS.md`.

- `GOALS.md` describes fitness: what should be true of the repo.
- `PROGRAM.md` describes execution: how the autonomous loop should behave in this repo right now.

`PROGRAM.md` is the operator-facing control surface for autonomous work. It answers which files may change, how to validate a slice, when to revert, and when to stop or escalate.

## Purpose

The program contract exists to give `/rpi` and future autonomous loops a small, explicit runtime policy instead of inferring repo behavior from broad strategic docs alone.

Use it when you want bounded autonomy:
- explicit mutable scope
- explicit immutable scope
- one clear experiment unit
- a fixed validation bundle
- keep/revert decision rules
- explicit escalation and stop conditions

## File Discovery

Consumers should resolve the program file in this order:
1. `PROGRAM.md`
2. `AUTODEV.md`

`PROGRAM.md` is the preferred canonical name. `AUTODEV.md` remains a compatibility alias for repos that already use that spelling.

## Required Sections

### `Objective`
Short description of what the loop is trying to achieve in this repo or phase.

### `Mutable Scope`
List of paths or glob-like scopes the agent may edit.

### `Immutable Scope`
List of paths or scopes the agent must not modify.

### `Experiment Unit`
Description of what counts as one bounded slice. This should be small enough to validate and revert cleanly.

### `Validation Commands`
Ordered shell commands that must run before the slice can be considered complete. This list is the **declared gate**: the full set a slice must pass, not a menu to pick from. Consumers run it in its entirety — see [Gate Runner Discipline](#gate-runner-discipline).

### `Decision Policy`
Rules for keep vs revert. This is where the repo states how to interpret passing tests, partial improvements, tie-breaks, and simplicity bias.

### `Escalation Rules`
Rules for when the loop must stop making local edits and hand work off to a bead, issue, or human decision.

### `Stop Conditions`
Concrete conditions that allow the loop to stop. Avoid vague prose.

## Gate Runner Discipline

The single most common silent failure of an autonomous loop is **running a subset of the declared validation bundle** — an agent reconstructs "the gate" from memory or prose, drops a command it forgot, and reports the slice green while a real check never ran. (Origin: 2026-06-04, ag-8rns — a `/evolve` cycle skipped `PROGRAM.md` grounding greps from its validation bundle for several slices; an independent `/validate` pass caught the red.)

The contract defense is a **single declared gate runner**, not a remembered command list:

- **One canonical invocation.** A repo SHOULD expose its full validation bundle behind a single declared entrypoint — a gate script (e.g. `scripts/pre-push-gate.sh`) or, per gate, the typed BC2 `GateRunnerPort` surfaced as `ao gate run <name>` (resolves `scripts/check-<name>.sh`, emits a structured `GateVerdict`). `Validation Commands` then *documents* what that runner runs; it is not an a-la-carte list for the agent to sample.
- **Full bundle, every time.** Consumers MUST run the **entire** declared bundle before marking a slice complete. They MUST NOT reconstruct a subset from memory, "skip the slow one", or infer the gate from prose. Selecting the smallest *lane set* is a property of the declared profile (see the repo execution profile's `validation_lanes`), not of the agent's recollection.
- **Fail-closed.** A gate that cannot run, errors, or returns `UNKNOWN` blocks the slice — it is never treated as a pass. Absence of a green is a red, not a skip.
- **Completion-claim assertion.** A slice is "validated" only once the full declared gate ran and passed. A keep decision predicated on a partial run is invalid regardless of the partial result.

This mirrors the repo execution profile's `validation_commands` rule — both contracts treat the declared bundle as a unit run via the canonical runner.

## Minimal Example

```markdown
# PROGRAM.md

## Objective

Introduce a repo-local autodev loop without rewriting the full evolve stack.

## Mutable Scope

- cli/cmd/ao/**
- cli/internal/autodev/**
- docs/contracts/autodev-program.md

## Immutable Scope

- hooks/**
- skills/**
- schemas/**

## Experiment Unit

One TDD-sized vertical slice: tests first, implementation second, validation third.

## Validation Commands

- `cd cli && go test ./cmd/ao ./internal/autodev`
- `bash scripts/generate-cli-reference.sh`

## Decision Policy

- Keep the slice only if all validation commands pass.
- Prefer smaller scope when two options satisfy the objective.
- Revert the slice if it expands beyond mutable scope.

## Escalation Rules

If the required change crosses skill contracts or hook semantics, open a bead instead of widening scope in place.

## Stop Conditions

- `ao autodev validate` passes.
- Relevant Go tests pass.
- CLI reference docs are in sync.
```

## Relationship to Other Contracts

- `GOALS.md` remains the strategic fitness contract.
- `docs/contracts/repo-execution-profile.md` remains the repo bootstrap and landing-policy contract.
- The RPI execution packet may include the resolved program contract as an additive phase-stable surface.

## Evolve Integration

`/evolve` should consume the program contract before cycle 1 and compose it with the repo execution profile:
- use the repo execution profile for repo bootstrap, tracker wrappers, and session-level landing policy
- use the program contract for mutable scope, immutable scope, per-cycle validation, keep/revert rules, and stop conditions
- prefer work that fits mutable scope and escalate work that clearly requires immutable-scope edits
- do not consume queue items or mark a cycle productive until the program validation bundle and stop conditions pass

## Design Notes

- Keep this contract human-editable first. Manual clarity matters more than early schema complexity.
- Prefer explicit scopes and validation commands over prose preferences.
- Treat this as a repo-local operational layer, not a replacement for skill behavior contracts.
