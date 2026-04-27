---
title: Agents Control Plane Design
date: 2026-04-25
skill: agentops:discovery
status: selected
---

# Design: `.agents` Operator Control Plane

## Product Fit

AgentOps already treats `.agents` as repo-native operational state. The recent
release-range work added a contract, linter, inspection command, guide, and
validation hooks. The missing product shape is a coherent operator flow:
discover the contract, inspect the state, diagnose drift, fix source evidence,
and run gates with clear local versus CI semantics.

## User

The primary user is an operator or coding agent landing work in this repo. They
need low-friction answers to:

- What `.agents` surfaces are expected?
- Which command proves the repo is in contract?
- Where did a bad `.agents/<subdir>` reference come from?
- Is a local hash-gate warning caused by real drift or shared-agent
  concurrency?

## Design Choice

Build on the existing CLI/script/docs surfaces instead of adding a new storage
model.

The selected design adds three narrow improvements:

- Make `ao agents inspect` and `ao agents lint` resolve paths from the repo
  root so commands work from subdirectories.
- Add `ao agents doctor` as a read-only diagnostic that composes contract,
  lint, skill discovery, and observed on-disk top-level `.agents` directories.
- Improve the shell lint and docs so drift includes source-location evidence
  and hash-gate triage stays strict in CI.

## Why This Design

The existing implementation already has the right foundations:

- `docs/contracts/agents-write-surfaces.md` is the canonical contract.
- `scripts/check-agents-write-surfaces.sh` is already wired into validation.
- `ao agents inspect` exposes contract and skill ownership.
- `ao agents lint` provides the command bridge into the shell gate.
- `docs/agents-operator-guide.md` gives users the starting workflow.

The design finishes the loop without making `ao agents` a mutating migration
tool.

## Non-Goals

- No automatic cleanup of unknown `.agents` directories.
- No migration command in this packet.
- No schema replacement for existing markdown knowledge artifacts.
- No local fail-open behavior in CI.

## Acceptance Summary

The packet is complete when:

- `ao agents inspect` and `ao agents lint` work from repo root and `cli/`.
- `ao agents doctor --json` emits structured diagnostics and documented exit
  codes.
- write-surface lint failures point to at least one source path per unknown
  subdir.
- smoke coverage classifies contract rows as referenced, skill-owned, or
  lifecycle-only.
- local hash-gate concurrency has bounded runtime and clear messaging while CI
  remains strict.
- operator docs describe the end-to-end workflow.
