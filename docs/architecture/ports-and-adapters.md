# Ports and adapters

The core has four ports:

| Port | Input | Output |
|---|---|---|
| Plan | Caller intent | `plan-packet.v1` |
| Implement | `plan-packet.v1` | `candidate-packet.v1` or `NOT_BUILT` report status |
| Validate | Candidate plus independent context identity | durable `verdict.v2` |
| Report | Phase outputs | `rpi-report.v1` |

The pure subject-manifest helper is bundled with Validate and requires no Git,
tracker, queue, network, release, or delivery executable.

Optional adapters may execute explicitly supplied packets or append generic
provenance after a verdict exists. Adapter availability, corruption, ordering,
or failure cannot change phase sequencing or semantic outcomes.

Git, trackers, CI, release systems, and delivery mechanisms are outside the
hexagon. They may consume AgentOps evidence but are never called by the core.
