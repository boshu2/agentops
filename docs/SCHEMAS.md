# Schemas

## Core loop

| Schema | Purpose |
|---|---|
| [`subject-manifest.v1`](../schemas/subject-manifest.v1.schema.json) | Deterministic content identity without Git |
| [`verdict.v2`](../schemas/verdict.v2.schema.json) | Independent `PASS`, `FAIL`, or `NOT_PROVEN` judgment |
| [`rpi-report.v1`](../schemas/rpi-report.v1.schema.json) | Single-pass phase report |

## Legacy compatibility

These deprecated schemas remain readable for previously produced artifacts;
the current loop does not author them:

- [`plan-packet.v1`](../schemas/plan-packet.v1.schema.json)
- [`candidate-packet.v1`](../schemas/candidate-packet.v1.schema.json)
- [`revision-packet.v1`](../schemas/revision-packet.v1.schema.json)

Other schemas support read-only CLI observations, generic provenance, tests,
or repository release tooling. They do not control phase sequencing,
continuation, Git, delivery, or semantic validity.
