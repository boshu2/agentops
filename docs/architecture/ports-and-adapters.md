# Ports and adapters

AgentOps sits in the federated integration graph as a semantic hexagon. The
nodes around it — callers, context sources, execution factories, deterministic
checks, and validators — stay outside and keep their own state; every crossing
is a typed handoff, never shared ownership.

The core has four ports:

| Port | Input | Output |
|---|---|---|
| Plan | Existing bead or caller intent | Same source refined in place, or a concise proposed amendment |
| Implement | Resolved intent | Runtime-derived subject manifest and check receipts, or `NOT_BUILT` |
| Validate | Intent digest, exact subject, receipts, independent context identity | `PASS | FAIL | NOT_PROVEN`; optional `verdict.v2` |
| Report | Phase outputs | Interactive result; optional `rpi-report.v1` |

One pass through these ports is the [RPI traversal](rpi-traversal.md).

The pure subject-manifest helper is bundled with Validate and requires no Git,
tracker, queue, network, release, or delivery executable.

Optional adapters may execute explicitly supplied intent slices or append
generic provenance after a persisted verdict exists. Adapter availability,
corruption, ordering, or failure cannot change phase sequencing or semantic
outcomes. A factory or runtime adapter hands over native execution facts;
runtime completion never enters the hexagon as validation.

The caller may use a tracker as the intent source; AgentOps does not become the
tracker. Git, CI, release systems, and delivery mechanisms are outside the
hexagon. They may consume AgentOps evidence but are never called by the core.
