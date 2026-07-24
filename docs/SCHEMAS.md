# Schemas

## Core loop

| Schema | Purpose |
|---|---|
| [`subject-manifest.v1`](https://github.com/boshu2/agentops/blob/main/schemas/subject-manifest.v1.schema.json) | Deterministic content identity without Git |
| [`verdict.v2`](https://github.com/boshu2/agentops/blob/main/schemas/verdict.v2.schema.json) | Independent `PASS`, `FAIL`, or `NOT_PROVEN` judgment |
| [`rpi-report.v1`](https://github.com/boshu2/agentops/blob/main/schemas/rpi-report.v1.schema.json) | Single-pass phase report |
| [`proof-contract.v1`](https://github.com/boshu2/agentops/blob/main/schemas/proof-contract.v1.schema.json) | Exact proof implementation, schema, corpus, and known-gap identity |
| [`proof-contract-active.v1`](https://github.com/boshu2/agentops/blob/main/schemas/proof-contract-active.v1.schema.json) | Atomic pointer to the active proof epoch |
| [`proof-contract-transition.v1`](https://github.com/boshu2/agentops/blob/main/schemas/proof-contract-transition.v1.schema.json) | Prior-contract-qualified activation of the next proof epoch |
| [`proof-bootstrap-root-replacement.v1`](https://github.com/boshu2/agentops/blob/main/schemas/proof-bootstrap-root-replacement.v1.schema.json) | Operator-authorized replacement of a rejected bootstrap root before any PASS is minted |

The rejected bootstrap epoch is preserved under
`docs/contracts/proof-contracts/epoch-0/`; its binding FAIL identified a
component-verification bypass. The active corrected root is frozen under
`epoch-0b/`. Its recorder can perform only the epoch-0b to epoch-1 transition.
The replacement is explicit because no PASS was minted under the rejected
root; ordinary activated epochs must advance through a proof transition.

## Legacy compatibility

These deprecated schemas remain readable for previously produced artifacts;
the current loop does not author them:

- [`plan-packet.v1`](https://github.com/boshu2/agentops/blob/main/schemas/plan-packet.v1.schema.json)
- [`candidate-packet.v1`](https://github.com/boshu2/agentops/blob/main/schemas/candidate-packet.v1.schema.json)
- [`revision-packet.v1`](https://github.com/boshu2/agentops/blob/main/schemas/revision-packet.v1.schema.json)

Other schemas support read-only CLI observations, generic provenance, tests,
or repository release tooling. They do not control phase sequencing,
continuation, Git, delivery, or semantic validity.
