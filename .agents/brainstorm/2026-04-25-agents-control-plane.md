---
title: Agents Control Plane Brainstorm
date: 2026-04-25
skill: agentops:discovery
theme: Repo-native .agents operating surface hardening
---

# Agents Control Plane Brainstorm

## Prompt

Identify the theme of work since `v2.38.0`, then enhance it with more
implementation and refactor work.

## Release-Range Signal

The latest release baseline is `v2.38.0`, published on 2026-04-23. The current
main range `v2.38.0..HEAD` contains 31 commits, 11 merged PRs, 345 changed
files, and a diffstat of 6,960 insertions and 3,013 deletions.

Merged PRs since release cluster around `.agents` contracts, CLI inspection,
linting, harvest recursion, hook/runtime validation, and Codex runtime
hardening:

- `#146` Fix commit-review redaction and add agents guide.
- `#145` and `#142` harden `FindAgentsDir` temp-directory skipping.
- `#144` records the `.agents` hygiene contract pattern.
- `#143` adds `ao agents lint`.
- `#141` adds `ao agents inspect`.
- `#140` and `#138` strengthen harvest nested-directory behavior.
- `#139` adds the `.agents` write-surface contract and lint.
- `#135` and `#134` carry nightly runtime and validation hardening.

## Theme

The post-release theme is hardening AgentOps' repo-native `.agents` operating
surface into an auditable, operator-friendly control plane across contracts,
CLI inspection and linting, hooks, Codex runtime proof, and validation gates.

## Options Considered

### Option A: Path-Stability Patch

Fix only the observed `ao agents inspect` and `ao agents lint` failures from
subdirectories. This is small and high confidence, but it does not complete the
operator workflow.

### Option B: Lint Evidence Upgrade

Improve the write-surface lint so it reports source locations for each unknown
`.agents/<subdir>` literal. This sharpens failures but leaves discovery split
across shell scripts and CLI commands.

### Option C: Operator Control Plane

Treat the existing contract, lint, inspect command, docs, and pre-push gate as
one control plane. Refactor path resolution, add a diagnostic `doctor`, enrich
lint evidence, add smoke coverage, and document the workflow.

### Option D: Broad `.agents` Rewrite

Redesign the `.agents` lifecycle around a new schema or storage abstraction.
This is premature. The recent commits intentionally built simple repo-native
surfaces first.

## Selected Direction

Option C is the right next step. It respects the current theme, builds on the
shipped PRs, and converts the current pieces into an operator-grade workflow
without replacing the underlying contract.

## Out Of Scope

- Migrating or rewriting existing `.agents` content.
- Adding repair or mutation behavior to `ao agents doctor`.
- Changing the canonical `.agents` directory taxonomy except where the lint
  smoke test proves a contract row is stale.
- Weakening CI enforcement for `.agents` hash or write-surface gates.

## Bead Packet

The work is tracked under epic `agentops-gpu`, with six child beads:

- `agentops-gpu.1`: repo-root-aware `ao agents` path resolution.
- `agentops-gpu.2`: `ao agents doctor` diagnostic summary.
- `agentops-gpu.3`: write-surface lint source-location evidence.
- `agentops-gpu.4`: smoke coverage for documented surface references.
- `agentops-gpu.5`: bounded local hash-gate behavior under concurrency.
- `agentops-gpu.7`: operator docs for doctor, lint evidence, and triage.
