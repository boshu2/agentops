# Orchestration Ports

> **Status:** V1 dual-runtime orchestration seam.
> **Owner plan:** `.agents/plans/2026-05-29-dual-runtime-orchestration-foundation.md`.
> **Purpose:** name the boundary that selects an orchestration engine for a unit
> of agent work, and the safety property that makes engine degradation free.

This contract documents the **`OrchestrationPort`** — the typed seam that routes
a unit of work onto an execution engine and records *why*. It is the foundation
that gates Bo's application epics (the dual-runtime *AgentOps × Claude Managed
Agents* integration): every fan-out skill, crank wave, and autodev loop depends
on this selection layer.

It follows the project-wide [Ports and Adapters](../architecture/ports-and-adapters.md)
model and the established `cli/internal/ports/` triplet pattern (interface +
`inmemory_*` adapter + test), exactly like `convergence_check.go` and
`ci_status.go`.

## Source of truth

| Concern | File |
|---|---|
| Port interface + contract | `cli/internal/ports/orchestration.go` |
| Deterministic adapter | `cli/internal/ports/inmemory_orchestration.go` |
| Capability detection (NTM) | `cli/internal/orchestration/ntm_probe.go` |
| Backend-selection schema | `schemas/orchestration-backend.v1.schema.json` |
| Output-contract parity schema | `schemas/orchestration-result.v1.schema.json` |
| Skill-side selection prose | `skills/shared/SKILL.md` (spawn-backend selection) |
| Shape routing (Workflow/NTM/skill) | `skills/automation-shape-routing/SKILL.md` |

When prose and code disagree, the Go port and the JSON schemas win (see
`CLAUDE.md` Source-of-Truth Precedence).

## The three-category model

Before selecting an engine, decide the **shape** of the automation. This is the
front door — `skills/automation-shape-routing/SKILL.md` owns it.

| Shape | What it is | Mechanism |
|---|---|---|
| **Claude Workflow** | Deterministic, reproducible orchestration of subagents | Claude `Workflow` tool — `agent({schema})`, `parallel()`, `pipeline()`, `phase()`, loop-until-budget. In-process, headless. |
| **NTM swarm** | Long-lived, human-in-the-loop multi-agent run | `ntm` / `*-with-ntm` — persistent tmux panes, robot API, mail/locks, attach + nudge + relaunch. |
| **Plain skill** | One model reasoning through a procedure | A single `SKILL.md`. No fan-out, or a strictly sequential edit-loop. |

The `OrchestrationPort` is invoked **once the shape calls for orchestration** —
it does not decide *whether* to orchestrate (that is the routing rule above); it
decides *which engine* runs the work and how to degrade if the preferred one is
absent.

## The degradation ladder

`OrchestrationPort.Select()` resolves a `WorkSpec` to a backend along a safe
degradation ladder. The ladder is **NTM → Claude-native → beads floor**:

| Tier | Backend | Role |
|---|---|---|
| Top | `ntm` | Preferred swarm runtime when available |
| Fallback | `claude` | Claude-native runtime when NTM is unavailable |
| Floor | `beads` | Always-available floor; every path terminates here |
| Pinnable | `codex` | Selectable **only** by an explicit `Pin`; the default ladder never auto-selects it |

The contract (verbatim from `cli/internal/ports/orchestration.go`):

- A non-empty `WorkSpec.Pin` **MUST** win over everything else, including
  `OptOut` and availability.
- `WorkSpec.OptOut` (with no `Pin`) **MUST** resolve to `BackendBeads`.
- Otherwise `Select` **MUST** choose `BackendNTM` when NTM is available, else
  `BackendClaude` when Claude is available, else `BackendBeads`.
- `BackendBeads` is the floor: it **MUST** always be selectable so the port
  never fails to place work for lack of an engine.
- `SelectionTrace.Considered` **MUST** record the ladder steps evaluated, in
  order, for auditability.
- Context cancellation **MUST** be honored on a best-effort basis.

`SelectionTrace` (`Chosen`, `Reason`, `Considered`) is the resolved routing
decision plus its reasoning, serialized as
`schemas/orchestration-backend.v1.schema.json` so every degradation is
auditable.

## The global opt-out: `AGENTOPS_ORCHESTRATION=off`

Setting `AGENTOPS_ORCHESTRATION=off` skips **all** spawn backends and degrades
straight to the **beads floor** (single-agent inline / `--quick`; workers' work
is still tracked through `br` — `BEADS_DIR="$(ao beads dir)" br`). This mirrors the `AGENTOPS_HOOKS_DISABLED=1`
convention. At the port level this is the `WorkSpec.OptOut` path: it routes to
`BackendBeads` regardless of NTM/Claude availability, but a non-empty `Pin`
still overrides it.

## Capability detection: `ntm --robot-capabilities`, NOT `command -v`

NTM availability is detected by **capability**, not presence. A binary on PATH
is not a usable swarm runtime. `cli/internal/orchestration/ntm_probe.go` invokes
`ntm --robot-capabilities`, parses the reported capability tokens, and checks
them against the hard dependencies a swarm cannot run without:

```
NTMHardDeps = [tmux, git, persistent-host, agent-CLIs]
```

Deliberately **not** hard deps: cursors and pipeline-state — those are
host-bound, non-portable runtime artifacts (they describe one host's live
session, not a portable capability), so their absence is not a missing
dependency.

Degradation contract of the probe:

- If `ntm` is absent (the runner returns an error), `ProbeNTM` returns
  `NTMCapabilities{Available: false}` and a **nil error** — absence is a normal
  degradation signal, not a failure, so callers branch on `Available` without an
  error path for the common "ntm not installed" case.
- A hard error is returned **only** when `ntm` IS present but its output cannot
  be parsed — a genuine contract violation worth surfacing.

This is why detection is capability-based rather than `command -v ntm`: a missing
hard dep means the host cannot drive a swarm even when the `ntm` binary exists.

## Safety property: output-contract parity

The property that makes degradation **free** (correctness-preserving) is
**output-contract parity**: *every* tier emits the same result shape,
`schemas/orchestration-result.v1.schema.json`. Whether the backend was NTM, a
Claude-native team, Codex sub-agents, or the beads floor, the run writes a
result carrying:

- `schema_version` (const `1`)
- `backend` — the tier that produced the result (`ntm` / `claude` / `codex` / `beads`)
- `result_paths[]` — repo-relative artifact paths
- `verdict` — `{ status: PASS|WARN|FAIL, confidence: HIGH|MEDIUM|LOW }`
- `task_id` (optional) — e.g. the bead ID

Because the *output* is invariant across tiers, the lead can verify-then-trust
the artifact identically no matter which engine ran. Degradation changes *who
does the work*, never *what a finished result looks like*. This is the invariant
already stated in `skills/shared/SKILL.md` ("Output-contract parity is unchanged
across all tiers"), now pinned to a versioned schema.

## Two ladders — do not conflate

There are **two distinct orchestration ladders** in the codebase. They govern
different boundaries and must stay separate:

| | Ladder (A) — spawn-backend | Ladder (B) — CLI phase-executor |
|---|---|---|
| **Question** | Which *engine* spawns the workers? | Which *transport mode* runs an RPI phase? |
| **Tiers** | `ntm` → `claude` → `beads` (+ pinnable `codex`) | `auto` \| `direct` \| `stream` \| `tmux` |
| **Owner** | `OrchestrationPort` + `skills/shared/SKILL.md` spawn-backend selection | `validateRuntimeMode` in `cli/cmd/ao/rpi_phased_context.go` |
| **Opt-out** | `AGENTOPS_ORCHESTRATION=off` → beads floor | n/a (mode is an explicit flag) |

This contract governs **ladder (A)** only. Ladder (B) (the phase-executor
runtime mode) gains new modes **only if/when** phases route through the
`OrchestrationPort` — that is a deliberate follow-up, out of scope for this
foundation. Future work must not collapse the two: a phase-executor mode
(`stream`/`tmux`) is *not* a spawn backend, and a spawn backend (`ntm`/`claude`)
is *not* a phase-executor mode.

> **`gc` is NOT a selectable tier.** The Gas City (`gc`) CLI bridge was removed
> (soc-2rtm0); `runtime=gc` is rejected by the CLI. Any `gc`-based dispatch
> prose in the swarm/crank references is historical and is never selected.

## Paired schemas

This contract is the prose half of a paired contract. Its schemas live at:

- `schemas/orchestration-backend.v1.schema.json` — the selection/degradation
  trace (`chosen`, `reason`, `considered`, `opt_out`, `pin`).
- `schemas/orchestration-result.v1.schema.json` — the output-contract parity
  shape every tier emits (`backend`, `result_paths`, `verdict`, `task_id`).

Both are validated as JSON by `scripts/check-contracts-structural-floor.sh` in
CI; the per-field shape is enforced by the JSON Schemas themselves at runtime.

## BC6 instrument layer (out-of-session waist)

`ao orchestrate` is **not** a driving adapter and **not** a daemon. It is the
BC6 **instrument lane** for out-of-session orchestration — deterministic probes
and verdicts that operators and skills call before/after human `atm`/`am`
procedure.

| Role | Examples | Hex |
|------|----------|-----|
| **Driving adapters** (who wakes the loop) | Agent session, ATM panes, `/goal`, cron | Swappable per ADR-0009 |
| **Instrument lane** (windshield) | `preflight`, `verify`, `tools`, `status`, `route`, `select`, `shape` | Guard + driven instruments |
| **Earn-it wrappers** (not committed Phase 1) | `spawn`, `send`, `reserve`, `teardown` | Thin passthrough — defer |

Membrane wire: `preflight`/`verify` append `orchestration.preflight.v1` /
`orchestration.verify.v1` events to `docs/provenance/ledger.jsonl` (same audit
chain as gate check). Profiles contract SOT:
`docs/contracts/orchestration-profiles.yaml`; tool matrix:
`docs/contracts/orchestration-tools.yaml`.

At the zoom level of classic ports-and-adapters prose, **ingress** is the
driving adapter (agent/ATM/`/goal`); **instruments** are the `ao` commands the
controller calls for ground truth. Both statements are true at different zoom
levels.
