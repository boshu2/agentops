# Provenance orphan fixtures (ag-x31t.7)

Inert seed fixtures for the **no-orphan provenance gate** built in **ag-x31t.6**
(`ao provenance trace --orphans --strict`, generalizing `goals_trace_orphans`).

Each `.jsonl` file is a tiny provenance graph (one JSON record per line; `node`
and `edge` records use the `goalstrace.Node` / `goalstrace.Edge` JSON contract in
`cli/internal/goalstrace/graph.go`). Every fixture seeds **one engineered
artifact node that has no inbound authored/inferred edge** — the orphan condition
the future gate must flag. The directive + edge present in each file deliberately
point at a *different* scenario, so the engineered artifact stays orphaned.

These are **failing-by-design** fixtures: they reproduce the three audit gaps the
epic (ag-x31t) closes. Once a provenance edge wires each artifact back to its
authoring directive, the gate flips them green.

| Fixture | Orphan artifact | Audit gap |
|---|---|---|
| `orphan-scenario-hash-stability.jsonl` | `gate:scenario-hash-stability` | phantom CI gate with no authoring directive |
| `orphan-retired-pre-push-gate.jsonl` | `artifact:scripts/pre-push-gate.sh` | retired-but-present script reference |
| `orphan-stale-65-jobs.jsonl` | `claim:65-jobs` | stale doctrine claim ("65 jobs") |

`expected-orphans.json` is the contract the ag-x31t.6 gate asserts against: for
each fixture it names the orphan artifact id and the finding the gate should emit.

**Scope of ag-x31t.7:** seed the fixtures only. The gate itself (ag-x31t.6), the
ledger schema (ag-x31t.2), and any `ao` subcommand are out of scope. The
format-check test (`tests/scripts/provenance-orphan-fixtures.bats`) only verifies
the fixtures are well-formed JSON/JSONL and internally consistent with
`expected-orphans.json`, so they are usable the moment the gate lands.
