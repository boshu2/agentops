# RPI Run Registry

The RPI phased orchestrator (`ao rpi phased`) writes structured artifacts to a well-known directory layout. This document defines the registry layout, file naming conventions, required fields per phase transition, and contract guarantees.

## Directory Layout

All RPI artifacts live under `.agents/rpi/` relative to the working directory (which may be a worktree):

```
.agents/rpi/
  phased-state.json           # Orchestrator state (survives across phases)
  phased-orchestration.log    # Append-only log of phase transitions
  phase-1-result.json         # Phase result artifact (discovery)
  phase-2-result.json         # Phase result artifact (implementation)
  phase-3-result.json         # Phase result artifact (validation)
  phase-1-summary.md          # Phase summary (written by Claude or fallback)
  phase-2-summary.md          # Phase summary
  phase-3-summary.md          # Phase summary
  phase-1-handoff.md          # Handoff file (context degradation signal)
  phase-2-handoff.md          # Handoff file
  phase-3-handoff.md          # Handoff file
  live-status.md              # Live status file (optional, --live-status flag)
```

## File Naming Conventions

| File | Purpose | Lifecycle |
|------|---------|-----------|
| `phased-state.json` | Orchestrator state: goal, epic ID, phase, verdicts, attempts | Written after each phase; read on resume (`--from`) |
| `phased-orchestration.log` | Append-only transition log for debugging | Appended at every transition point |
| `phase-{N}-result.json` | Structured phase outcome matching `rpi-phase-result.schema.json` | Written atomically after each phase completes or fails |
| `phase-{N}-summary.md` | Human-readable summary for cross-phase context | Written by Claude (preferred) or orchestrator fallback |
| `phase-{N}-handoff.md` | Context degradation signal from Claude | Written by Claude when it detects context degradation |
| `live-status.md` | Real-time progress for external watchers | Continuously updated when `--live-status` is enabled |

Where `{N}` is the phase number: 1 (discovery), 2 (implementation), 3 (validation).

## Required Fields Per Phase Transition

Each `phase-{N}-result.json` must contain at minimum:

| Field | Type | Description |
|-------|------|-------------|
| `schema_version` | integer | Always `1` (current version) |
| `run_id` | string | Hex run identifier |
| `phase` | integer | Phase number (1-3) |
| `phase_name` | string | `discovery`, `implementation`, or `validation` |
| `status` | string | `started`, `completed`, `failed`, or `retrying` |
| `started_at` | string | ISO 8601 timestamp |

Optional fields populated when available:

| Field | Type | Description |
|-------|------|-------------|
| `retries` | integer | Number of retry attempts (default 0) |
| `error` | string | Error message on failure |
| `backend` | string | Execution backend: `direct` or `stream` |
| `artifacts` | object | Map of artifact names to paths |
| `verdicts` | object | Map of gate names to verdict strings |
| `completed_at` | string | ISO 8601 timestamp |
| `duration_seconds` | number | Wall-clock duration |

Runtime selection is controlled by `ao rpi phased --runtime`, `ao rpi phased --runtime-cmd`,
or config/env (`rpi.runtime_mode`, `rpi.runtime_command`, `AGENTOPS_RPI_RUNTIME[_MODE]`,
`AGENTOPS_RPI_RUNTIME_COMMAND`).

## Phase Transition Validation

Before starting phase N (for N > 1), the orchestrator validates that `phase-{N-1}-result.json` exists and has `status: "completed"`. This ensures phases execute in order and that prior phases completed successfully.

Validation failures produce a clear error message indicating which prior phase result is missing or incomplete.

## Contract Guarantees

### Atomic Writes

Phase result files are written atomically using a write-to-temp-then-rename pattern:

1. Marshal JSON to `phase-{N}-result.json.tmp`
2. Rename `.tmp` to final path

This ensures readers never see a partial write. The orchestration log (`phased-orchestration.log`) uses append-only writes which are atomic at the OS level for reasonable line lengths.

### Schema Version

Every result file includes `schema_version: 1`. Consumers must check this field and handle unknown versions gracefully (fail open or warn). When the schema evolves, the version will increment and the orchestrator will maintain backward compatibility for at least one major version.

### Idempotent Resume

The orchestrator can resume from any phase using `--from=<phase>`. On resume, it reads `phased-state.json` to recover epic ID, verdicts, and attempt counts. Phase result files from prior phases are preserved and not overwritten on resume.

### Clean Start

When starting from phase 1 (fresh run), the orchestrator removes stale phase summaries and handoff files from prior runs. Phase result files from the current run are written fresh.

## Worktree Lifecycle Semantics

`ao rpi phased` creates sibling worktrees named `../<repo>-rpi-<run-id>/` (unless `--no-worktree` is set). Cleanup behavior is intentional and asymmetric:

- Success path: after all phases complete, the orchestrator merges `rpi/<run-id>` into the source branch and removes the worktree + branch.
- Failure path: worktree is preserved for debugging (no auto-destroy on failed phase).
- Interrupt path (`SIGINT`/`SIGTERM`): worktree is preserved and terminal metadata is written (`terminal_status: interrupted`).

This design prevents data loss on partial runs while still auto-cleaning successful runs.

## Monitoring and Liveness

Use:

```bash
ao rpi status
ao rpi status --watch
```

Status classification is registry-first:

- Primary source: `.agents/rpi/runs/<run-id>/phased-state.json`
- Liveness signal: `.agents/rpi/runs/<run-id>/heartbeat.txt` (fresh heartbeat => active)
- Fallback liveness probe: tmux session check (legacy runs) if heartbeat is stale/missing

Stale reasons include `worktree missing` when state references a removed worktree directory.

## Stale Cleanup Workflow

Use manual cleanup commands:

```bash
ao rpi cleanup --all --dry-run
ao rpi cleanup --all
ao rpi cleanup --all --stale-after 24h
ao rpi cleanup --all --prune-worktrees
ao rpi cleanup --run-id <id>
```

Behavior:

- Marks stale runs with terminal metadata (`terminal_status: stale`, reason, timestamp)
- Removes orphaned worktree directories when safe
- Optionally runs `git worktree prune`
- Supports age-gated cleanup via `--stale-after` to avoid touching recently interrupted runs

Safety guards:

- Refuses to remove non-sibling paths
- Refuses to remove repo root as a worktree target

## Optional Pre-Run Auto-Cleanup

`ao rpi phased` supports optional stale-run cleanup before orchestration:

```bash
ao rpi phased --auto-clean-stale --auto-clean-stale-after 24h "<goal>"
```

Behavior:

- Runs stale cleanup at phased startup before phase execution
- Uses dry-run cleanup semantics when `ao rpi phased --dry-run` is set
- Persists startup state (including `run_id`, backend, and phase) to state files before entering the phase loop

## Current Limitation

`ao rpi cleanup` operates on run-registry state entries. If a historical/log-only run never wrote `.agents/rpi/runs/<run-id>/phased-state.json`, it may appear in log views but not be selected by stale cleanup. In that case, use standard git worktree hygiene (`git worktree list`, `git worktree remove --force <path>`, `git branch -D rpi/<run-id>`) after verifying the branch has no unique commits.
