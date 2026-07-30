# Gas City reliability boundary

AgentOps ships skills, evidence contracts, and small operational guidance, not
an orchestration backend or a competing Gas City pack.

| Fact | Authority |
|---|---|
| Work and dependency readiness | Beads |
| Sessions, routing, formulas, scoped control dispatch, dashboard/API, OTEL | official Gas City |
| Pack discovery and resolved import locks | Gas City registries and `packs.lock` |
| Candidate identity | Git commit and tree |
| Semantic acceptance | AgentOps `subject-manifest.v1` and `verdict.v2` |
| Pull request, hosted checks, merge | GitHub |

Caller-owned work flows through the upstream pack selected by the operator.
AgentOps prefers the official `gascity` build pack used by Maintainer City, but
also supports the Agentic Coding Flywheel as an independent factory choice.
Agents inside either factory consume AgentOps skills through their provider
runtime's normal skill discovery.

Operational guidance must remain bounded. It may install a pinned upstream
pack, configure providers, invoke native commands, observe runs, and stop a
city. It must not mirror Beads, Gas City, Git, or GitHub state in an AgentOps
schema family, daemon, reducer, receipt graph, formula, or role pack.

For Gas City, install the registry-pinned `gascity` workflow at city scope and
its sibling role pack at rig scope, preserving the stock `gc.*` namespace.
Every graph-owning scope retains an unsuspended
`core.control-dispatcher`. The upstream `gc.mayor` skill and
`gc.run-operator` own guided coordination and formula launch.

Compatibility qualification uses deterministic toolchain, registry, import
lock, config, formula, scoped-dispatcher, and worktree checks before any model
session. A live canary is disposable, uses required OTEL, and is repaired only
from outside Gas City. It qualifies the selected upstream pack plus AgentOps
skill availability; it does not qualify an AgentOps-owned factory.

Development is two-speed by design. The offline unit and lint contract is the
inner loop. Native import, formula preview and route, doctor, and quiescent
teardown checks run at a candidate boundary. Live model sessions remain an
explicit compatibility test.
