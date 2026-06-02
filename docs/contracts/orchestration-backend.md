# Orchestration Backend Selection Contract

Schema: `schemas/orchestration-backend.v1.schema.json`

The selection trace emitted by the `OrchestrationPort` when it resolves which
backend runs a unit of orchestrable work. The full port semantics, the
`NTM → Claude-native → beads floor` degradation ladder, the
`AGENTOPS_ORCHESTRATION=off` opt-out, and capability-detection live in
[orchestration-ports.md](orchestration-ports.md) — this contract pins the wire
shape of one selection decision so the structural-floor gate validates the schema.

## Fields

- `schema_version` (const `1`) — contract version.
- `chosen` — the selected backend: `ntm` \| `claude` \| `codex` \| `beads`.
- `reason` — human-readable explanation of why this backend was chosen.
- `considered` — ordered ladder steps evaluated before the choice.
- `opt_out` — whether the global orchestration opt-out forced the beads floor.
- `pin` — an explicit backend pin (empty/null = auto-select down the ladder).

Required: `schema_version`, `chosen`, `reason`.
