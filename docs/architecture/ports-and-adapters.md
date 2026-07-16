# Ports and adapters

The core has four ports:

| Port | Input | Output |
|---|---|---|
| Plan | Existing bead or caller intent | Same source refined in place, or a concise proposed amendment |
| Implement | Resolved intent | Runtime-derived subject manifest and check receipts, or `NOT_BUILT` |
| Validate | Intent digest, exact subject, receipts, independent context identity | durable `verdict.v2` |
| Report | Phase outputs | `rpi-report.v1` |

The pure subject-manifest helper is bundled with Validate and requires no Git,
tracker, queue, network, release, or delivery executable.

Optional adapters may execute explicitly supplied intent slices or append generic
provenance after a verdict exists. Adapter availability, corruption, ordering,
or failure cannot change phase sequencing or semantic outcomes.

The caller may use a tracker as the intent source; AgentOps does not become the
tracker. Git, CI, release systems, and delivery mechanisms are outside the
hexagon. They may consume AgentOps evidence but are never called by the core.
