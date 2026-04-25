---
title: Agents Control Plane Hardening Plan
date: 2026-04-25
skill: agentops:discovery
epic: agentops-gpu
complexity: standard
tracker: bd
---

# Plan: `.agents` Operator Control Plane Hardening

## Theme

The post-release theme is hardening AgentOps' repo-native `.agents` operating
surface into an auditable, operator-friendly control plane across contracts,
CLI inspection and linting, hooks, Codex runtime proof, and validation gates.

## Objective

Enhance that theme with a focused refactor packet: make the control-plane
commands repo-root aware, add a read-only diagnostic summary, improve lint
evidence, harden local hash-gate behavior, and document the operator workflow.

## Complexity

Standard: six beads, three implementation waves, cross-cutting validation across
Go CLI code, shell scripts, BATS tests, docs, and pre-push behavior.

## Beads

Source of truth is `bd`; the epic is `agentops-gpu`.

| Bead | Title | Dependency |
|---|---|---|
| `agentops-gpu.1` | Refactor `ao agents` path resolution to repo-root aware helpers | none |
| `agentops-gpu.3` | Enhance write-surface lint with source-location evidence | none |
| `agentops-gpu.5` | Stabilize agents-hub content-hash gate for concurrent local agents | none |
| `agentops-gpu.2` | Add `ao agents doctor` diagnostic summary | after `.1` |
| `agentops-gpu.4` | Add smoke coverage for documented surface references | after `.3` |
| `agentops-gpu.7` | Update operator docs for doctor, lint evidence, and hash-gate triage | after `.2`, `.3`, `.5` |

## Wave Plan

### Wave 1: Independent Foundations

Work can proceed in parallel by file ownership:

- `agentops-gpu.1`: `cli/cmd/ao/agents.go`,
  `cli/cmd/ao/agents_lint.go`, and paired tests.
- `agentops-gpu.3`: `scripts/check-agents-write-surfaces.sh` and BATS tests.
- `agentops-gpu.5`: hash snapshot/pre-push scripts and focused BATS tests.

### Wave 2: Diagnostics And Smoke Coverage

- `agentops-gpu.2` follows `.1` and owns the new `ao agents doctor` command and
  generated CLI docs.
- `agentops-gpu.4` follows `.3` and extends write-surface smoke coverage.

### Wave 3: Operator Documentation

- `agentops-gpu.7` follows `.2`, `.3`, and `.5` so docs reflect implemented
  behavior instead of planned behavior.

## File Conflict Matrix

| Area | Conflict | Ordering |
|---|---|---|
| `cli/cmd/ao/agents*.go` | `.1` and `.2` both touch command code | serialize via `.2` after `.1` |
| `scripts/check-agents-write-surfaces.sh` | `.3` and `.4` both touch lint behavior | serialize via `.4` after `.3` |
| hash gate scripts | `.5` owns behavior; `.7` only documents it | docs after implementation |
| docs | `.2`, `.3`, `.5` can change command output; `.7` finalizes docs | `.7` last |

## Validation Levels

Required:

- L0: unit tests for path resolution, command registration, parser behavior, and
  doctor exit-code mapping.
- L1: script and BATS integration for write-surface lint and hash-gate behavior.
- L2: command-level workflow from repo root and `cli/`.

Recommended:

- L3: full local fast gate before merge because this touches CLI, scripts,
  docs, and hooks-adjacent validation.

## Validation Commands

Run focused checks while implementing:

```bash
cd cli && go test ./cmd/ao -run 'TestAgents|TestAgentsLint|TestAgentsDoctor' -count=1
bats tests/scripts/check-agents-write-surfaces.bats
shellcheck --severity=error scripts/check-agents-write-surfaces.sh scripts/check-agents-hash-snapshot.sh scripts/pre-push-gate.sh
scripts/generate-cli-reference.sh --check
npx -y markdownlint-cli docs/agents-operator-guide.md docs/INDEX.md README.md
bash tests/docs/validate-doc-release.sh
AGENTS_HUB_OVERRIDE=/tmp/empty-agents-hub-for-agent-control HASH_GATE_IGNORE_UNTRACKED=1 scripts/pre-push-gate.sh --fast
```

Before push, run the smart gate when local shared-agent state is stable:

```bash
scripts/pre-push-gate.sh --fast
```

If concurrent local agents make the shared hash gate noisy, preserve CI
strictness and use the documented override only for local validation of this
packet.

## Applied Knowledge

- `f-2026-04-14-001`: pair command refactors with command tests.
- `f-2026-04-14-002`: close with durable committed artifact paths.
- `2026-04-07-v2.35.0-release-postmortem`: validate both local and remote
  surfaces.
- `warn-then-fail-ratchet`: keep CI strict while local concurrency warnings are
  bounded and actionable.

## Completion Criteria

The epic is complete when all six beads are closed, the discovery artifacts and
execution packet are committed, validation evidence is recorded in bead
closure reasons, and the branch is pushed.
