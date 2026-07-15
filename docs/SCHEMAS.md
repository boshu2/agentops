# Schemas

## Core loop

| Schema | Purpose |
|---|---|
| [`plan-packet.v1`](../schemas/plan-packet.v1.schema.json) | One behavior, acceptance, evidence, and write scope |
| [`candidate-packet.v1`](../schemas/candidate-packet.v1.schema.json) | Factual implementation result bound to a Plan digest |
| [`subject-manifest.v1`](../schemas/subject-manifest.v1.schema.json) | Deterministic content identity without Git |
| [`revision-packet.v1`](../schemas/revision-packet.v1.schema.json) | Caller-authored link between separate invocations |
| [`verdict.v2`](../schemas/verdict.v2.schema.json) | Independent `PASS`, `FAIL`, or `NOT_PROVEN` judgment |
| [`rpi-report.v1`](../schemas/rpi-report.v1.schema.json) | Single-pass phase report |

Other schemas support read-only CLI observations, generic provenance, tests,
or repository release tooling. They do not control phase sequencing,
continuation, Git, delivery, or semantic validity.
