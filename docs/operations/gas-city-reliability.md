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

## AgentOps-owned operational adapter

The supported repository-side adapter is
`scripts/gc-maintainer-ops.sh`. It has three deliberately narrow operations:

- `prepare` verifies the accepted official workflow and rig-role commit,
  snapshots upstream validation assets unchanged under the selected rig's
  ignored `.gc` runtime, installs AgentOps-owned check-path wrappers, selects an
  already available PyYAML-capable Python, and links AgentOps skills into the
  city and rig provider sinks;
- `check` replays the same pin/runtime/skill/service/Doctor checks without
  writing adapter files or issuing state-changing Gas City commands;
- `recover-affinity` is dry-run by default and can clear only a ready formula
  bead's stale required-session assignee when explicitly passed `--apply`.

This is not a pack, formula, role, daemon, supervisor, or GC compatibility fork.
The contained runtime keeps the official maintainer bytes and directory shape
intact so its validator finds its own schemas. AgentOps owns only the wrappers
that place those official checks at the rig-local paths declared by the
formula. A foreign file at one of those paths is a conflict; preparation fails
rather than replacing it.

The adapter does not install PyYAML, edit LaunchAgents, kill processes, restart
the supervisor, patch managed hooks, close beads, re-sling work, or reinterpret
GC outcomes. These operations either belong to the host operator or to Gas
City. On macOS, `check` fails if an existing supervisor LaunchAgent points to a
missing or different GC binary, preventing the login/reboot failure from being
discovered after a run.

Compatibility qualification uses deterministic toolchain, registry, import
lock, config, formula, scoped-dispatcher, and worktree checks before any model
session. A live canary is disposable, uses required OTEL, and is repaired only
from outside Gas City. It qualifies the selected upstream pack plus AgentOps
skill availability; it does not qualify an AgentOps-owned factory.

The 1.4 maintainer canary established additional operational boundaries:

- formula artifact gates require a rig-local check surface even though the
  implementation lives in the upstream pack;
- skill links prove availability, not invocation, and free-form source-bead
  skill requirements are not necessarily propagated into decomposed work;
- a workflow root can pass while its caller-owned input bead stays open;
- disabled publishing is a successful no-op and leaves the approved commit in
  its source anchor;
- a drained session can remain on a future affinity-bound bead, so recovery
  clears that one ready assignee rather than re-slinging the workflow;
- `gc status` can disagree with `gc session list`; roster/pane evidence governs
  liveness, bead/run state governs workflow progress, and Doctor governs city
  metabolism.

Doctor warnings for upstream formula deprecations, managed-hook projection
drift, local-only backup, retention, or unused local providers stay visible.
The adapter does not downgrade them to PASS or claim that a successful canary
proves heavy-load readiness.

Development is two-speed by design. The offline unit and lint contract is the
inner loop. Native import, formula preview and route, doctor, and quiescent
teardown checks run at a candidate boundary. Live model sessions remain an
explicit compatibility test.
