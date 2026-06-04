# Orchestration Result Parity Contract

Schema: [`schemas/orchestration-result.v1.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/orchestration-result.v1.schema.json)

The output-contract **parity shape** every orchestration tier (NTM swarm,
Claude-native, beads floor) MUST emit. This is the load-bearing safety property
behind safe degradation: a downstream consumer (validation, ledger, provenance)
stays tier-agnostic only because all tiers produce an identical result *shape* —
values legitimately differ (the beads floor advertises `WARN`/`MEDIUM`; richer
tiers earn `PASS`/`HIGH`), but the key set is invariant. Enforced by the
degradation-conformance test in `cli/internal/orchestration/conformance_test.go`.
See [orchestration-ports.md](orchestration-ports.md) for the full port contract.

## Fields

- `schema_version` (const `1`) — contract version.
- `backend` — which tier produced the result: `ntm` \| `claude` \| `codex` \| `beads`.
- `result_paths` — artifact locations written by the run (e.g. `.agents/swarm/results/*.json`).
- `verdict` — `{ status: PASS|WARN|FAIL, confidence: HIGH|MEDIUM|LOW }`.
- `task_id` — the unit of work this result corresponds to.

Required: `schema_version`, `backend`, `result_paths`, `verdict`.
