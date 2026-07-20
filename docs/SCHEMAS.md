# Schemas

## Core loop

| Schema | Purpose |
|---|---|
| [`subject-manifest.v1`](https://github.com/boshu2/agentops/blob/main/schemas/subject-manifest.v1.schema.json) | Deterministic content identity without Git |
| [`verdict.v2`](https://github.com/boshu2/agentops/blob/main/schemas/verdict.v2.schema.json) | Independent `PASS`, `FAIL`, or `NOT_PROVEN` judgment |
| [`rpi-report.v1`](https://github.com/boshu2/agentops/blob/main/schemas/rpi-report.v1.schema.json) | Single-pass phase report |

## Legacy compatibility

These deprecated schemas remain readable for previously produced artifacts;
the current loop does not author them:

- [`plan-packet.v1`](https://github.com/boshu2/agentops/blob/main/schemas/plan-packet.v1.schema.json)
- [`candidate-packet.v1`](https://github.com/boshu2/agentops/blob/main/schemas/candidate-packet.v1.schema.json)
- [`revision-packet.v1`](https://github.com/boshu2/agentops/blob/main/schemas/revision-packet.v1.schema.json)

Other schemas support read-only CLI observations, generic provenance, tests,
or repository release tooling. They do not control phase sequencing,
continuation, Git, delivery, or semantic validity.
