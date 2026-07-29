# Gas City reliability boundary

AgentOps ships configuration and small operational helpers, not an
orchestration backend.

| Fact | Authority |
|---|---|
| Work and dependency readiness | Beads |
| Sessions, routing, formulas, scoped control dispatch, dashboard/API, OTEL | official Gas City |
| Pack discovery and resolved import locks | Gas City registries and `packs.lock` |
| Candidate identity | Git commit and tree |
| Semantic acceptance | AgentOps `subject-manifest.v1` and `verdict.v2` |
| Pull request, hosted checks, merge | GitHub |

The source bead flows through one native formula: fresh Sol planning, one
isolated Terra-high or Opus-medium implementation, fresh Sol validation, then
a separate Fable Refiner delivery step. Validation may close the semantic bead
before delivery completes. Moving main never blocks semantic work; the Refiner
rebases once and returns stale or conflicting work to the Mayor.

Operational scripts must remain bounded. They may materialize the pinned
official pair, render a city, set up one worktree, invoke native GC commands,
perform one delivery attempt, and stop the city. They must not mirror Beads,
GC, Git, or GitHub state in a schema family, daemon, reducer, or receipt graph.

The AgentOps pack composes the registry-pinned `gascity` workflow used by
Maintainer City and binds its sibling role pack at the stock default-rig
namespace, `gc.*`, while keeping `agentops-experiment` as its bounded default.
Every graph-owning scope retains an unsuspended `core.control-dispatcher`; the
optional Mayor is on-demand and no longer runs a v1.3 heartbeat workaround.

Release qualification uses deterministic toolchain, registry, pack, config,
formula, scoped-dispatcher, and worktree checks before any model session. A live
canary is disposable, uses required OTEL, and is repaired only from outside GC.
Two failed canaries reject the release shape.

Development is two-speed by design. The offline unit and lint contract is the
inner loop. The pinned native bootstrap, formula preview and route, doctor, and
quiescent teardown suite runs once at a candidate boundary. It does not launch
model sessions. Only a green, freshly validated and merged candidate reaches a
mixed-provider canary.
