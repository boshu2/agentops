# PROGRAM.md

## Objective

Run bounded autonomous improvement cycles for AgentOps without relying on
session-only prompt policy. Each cycle should select a tracked slice, improve the
repo, validate the result, close or update the bead, and push when the work is
ready to land.

## Mutable Scope

- PROGRAM.md
- README.md
- GOALS.md
- cli/**
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

One bead-backed vertical slice: claim or create the bead, make the smallest
coherent change that satisfies its acceptance criteria, run the relevant local
gates, update/close the bead, commit, rebase, push, and verify the remote gate
when the slice is intended to land.

## Validation Commands

- `cd cli && env -u AGENTOPS_RPI_RUNTIME go run ./cmd/ao autodev validate --file ../PROGRAM.md --json`
- `cd cli && env -u AGENTOPS_RPI_RUNTIME go test ./cmd/ao ./internal/autodev`
- `env -u AGENTOPS_RPI_RUNTIME bash skills/heal-skill/scripts/heal.sh --strict`
- `bash scripts/check-worktree-disposition.sh`
- `env -u AGENTOPS_RPI_RUNTIME scripts/pre-push-gate.sh --fast`

## Decision Policy

- Start from `bd ready --json` or a user-selected bead; create a discovered bead
  before editing when the work is new.
- For Nightly, evolve, or RPI-auto maintenance work, inspect the last 14 days of
  Nightly PR/run evidence before selecting the slice, and separate code-driven
  failures from runtime-artifact-only or corpus-state-only movement.
- Keep a slice only when the changed files are inside mutable scope, the
  acceptance criteria are satisfied, and the applicable validation commands pass.
- Prefer source-of-truth order from AGENTS.md when docs disagree: executable code
  and generated artifacts first, skill contracts second, explanatory docs third.
- Prefer the repo's existing patterns and validation scripts over new policy
  surfaces or ad hoc checks.
- Revert or narrow a slice that expands beyond its bead, crosses immutable scope,
  or produces no measurable improvement after validation.
- Record every deferred follow-up in bd with a discovered-from relationship.

## Escalation Rules

Stop local edits and update or create a bead when the work requires credentials,
release authority, external service mutation, or a change outside mutable scope.

Stop and preserve work on a `codex/preserve-*` branch when the slice is valuable
but cannot be landed before the session ends.

Escalate instead of widening scope when a validation failure exposes a security
or data-loss risk, when a foreign worktree has no disposition, or when unrelated
user edits conflict with the current slice.

## Stop Conditions

- `ao autodev validate --json` reports `valid: true` for this contract.
- The active bead is closed or updated with concrete remaining blockers.
- The relevant validation bundle is green, including the fast pre-push gate for
  landed changes.
- The worktree is clean, pushed, and up to date with origin for landed changes.
- Every foreign worktree is marked merged, preserved, exported, or deleted.
- New follow-up work discovered during the cycle is tracked in bd.
