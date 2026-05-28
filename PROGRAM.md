# PROGRAM.md

## Objective

Run bounded autonomous improvement cycles for AgentOps without relying on
session-only prompt policy. Each cycle is one turn of the
[operating loop](docs/architecture/operating-loop.md): shape BDD-shaped intent,
slice it vertically through behavior, write the first failing test, implement the
smallest change that flips it green, prove acceptance against the bead, push when
the slice is ready to land, and ratchet evidence and learnings. The loop is the
primitive — no artifact is produced unless it advances behavior toward acceptance.

## Mutable Scope

- PROGRAM.md
- PRODUCT.md
- README.md
- GOALS.md
- cli/**
- evals/**
- docs/**
- hooks/**
- lib/**
- scripts/**
- schemas/**
- skills/**
- skills-codex/**
- skills-codex-overrides/**
- tests/**
- .github/workflows/**
- .beads/**
- .agents/** runtime state when the active command owns it
- .agents/overnight/*/generator-results/*.json when Dream owns the active run
- .agents/dream/external-watchlist.yaml when Dream is the active command (RFC 0001 Proposal 2 source)

## Immutable Scope

- .git/** and linked worktree internals
- secrets, tokens, credentials, and private key material
- user shell/profile files and machine-local configuration outside this repo
- release tags, GitHub releases, and Homebrew tap state unless the active bead is
  a release task
- production or customer data, external service state, and credentials-backed
  resources unless the active bead explicitly authorizes that operation
- unrelated user edits, foreign worktrees, or preserved branches without a
  recorded disposition

## Experiment Unit

One bead-backed vertical slice, shaped per the operating loop:

- The slice maps to exactly one Given/When/Then row of the bead's acceptance
  examples — a behavior, not a layer. "Refactor then feature" is two slices.
- It touches one bounded context per the
  [context map](docs/contracts/context-map.md). A slice that crosses contexts is
  two slices.
- It has a nameable first failing test that fails for the right reason (missing
  behavior, not syntax) before any implementation change.
- Its write scope is reviewable in one pass.

Per slice: claim or create the bead, write the first failing test, make the
smallest change that flips it green, refactor under green as a separate commit,
record evidence into the bead, run the relevant local gates, update/close the
bead, commit, rebase, push, and verify the remote gate when the slice is intended
to land. A slice that touches `cli/internal/domain/` must preserve the
no-import-from-`internal/*` invariant; a slice that touches a port writes its
first failing test against the port interface, not an adapter internal.

## Validation Commands

- `cd cli && env -u AGENTOPS_RPI_RUNTIME go run ./cmd/ao autodev validate --file ../PROGRAM.md --json`
- `cd cli && env -u AGENTOPS_RPI_RUNTIME go test ./cmd/ao ./internal/autodev`
- `cd cli && env -u AGENTOPS_RPI_RUNTIME go test ./internal/domain/... ./internal/ports/... ./internal/adapters/...`
- `env -u AGENTOPS_RPI_RUNTIME bash skills/heal-skill/scripts/heal.sh --strict`
- `bash scripts/check-worktree-disposition.sh`
- `env -u AGENTOPS_RPI_RUNTIME scripts/pre-push-gate.sh --fast`

## Decision Policy

- Start from `bd ready --json` or a user-selected bead; create a discovered bead
  before editing when the work is new. A bead is not ready to work until its
  acceptance examples are testable Given/When/Then rows.
- For Nightly, evolve, or RPI-auto maintenance work, inspect the last 14 days of
  Nightly PR/run evidence before selecting the slice, and separate code-driven
  failures from runtime-artifact-only or corpus-state-only movement.
- Write the first failing test before the implementation change. Code with no
  failing test has no acceptance surface — it has no defined "done".
- Keep a slice only when it maps to one Given/When/Then row, touches one bounded
  context, the changed files are inside mutable scope, the acceptance examples
  pass, and the applicable validation commands pass.
- Default to running slices sequentially. Group slices into a parallel wave only
  when every row of the wave-validity check passes: disjoint write scopes,
  independent test targets, at most one slice per shared migration / schema /
  generated file, at most one slice per CLI surface, declared integration order,
  one owner per slice, and a discard path per slice. Any failed row → sequential.
- Treat domain purity as a shared wave concern: at most one slice per wave
  touches `cli/internal/domain/` types, and that slice must keep the
  no-import-from-`internal/*` invariant.
- Prefer source-of-truth order from AGENTS.md when docs disagree: executable code
  and generated artifacts first, skill contracts second, explanatory docs third.
- Prefer the repo's existing patterns and validation scripts over new policy
  surfaces or ad hoc checks.
- A slice that adds or changes an `ao` CLI surface follows the agent-ergonomic
  CLI contract (GOALS.md Directive 13): read-side commands expose `--json` with
  stdout-as-data / stderr-as-diagnostics separation, errors name the exact
  corrective command, unknown flags return a typo hint, and machine-readable
  introspection stays consistent with `ao capabilities` / `ao robot-docs`.
  Mirror the doctor surface in `cli/cmd/ao/doctor_surface.go`.
- Revert or narrow a slice that expands beyond its bead, crosses immutable scope,
  crosses a bounded-context boundary, or produces no measurable improvement after
  validation.
- Capture under the promotion ratchet, not into a landfill: a one-off observation
  dies in the handoff; only a learning that repeats twice earns
  `.agents/learnings/`, and only a must-never-regress fact earns a gate.
- Record every deferred follow-up in bd with a discovered-from relationship.

## Escalation Rules

Stop local edits and update or create a bead when the work requires credentials,
release authority, external service mutation, or a change outside mutable scope.

Stop and preserve work on a `codex/preserve-*` branch when the slice is valuable
but cannot be landed before the session ends.

Escalate instead of widening scope when a validation failure exposes a security
or data-loss risk, when a foreign worktree has no disposition, or when unrelated
user edits conflict with the current slice.

Stop and re-slice instead of proceeding when the work cannot be expressed as a
single Given/When/Then row, when it crosses a bounded-context boundary, or when
its first failing test cannot be named — these are signs the slice is actually
two or more slices.

## Stop Conditions

- `ao autodev validate --json` reports `valid: true` for this contract.
- Every Given/When/Then in the active bead maps to a passing test; every non-goal
  is still untouched; every rollback path is reachable. Activity logs do not close
  beads — acceptance evidence does.
- The active bead is closed or updated with concrete remaining blockers.
- The relevant validation bundle is green, including the fast pre-push gate for
  landed changes.
- The worktree is clean, pushed, and up to date with origin for landed changes.
- Every foreign worktree is marked merged, preserved, exported, or deleted.
- Evidence is recorded in the bead and `.agents/ratchet/`; learnings are promoted
  only if they cleared the promotion ratchet bar.
- New follow-up work discovered during the cycle is tracked in bd.
