# AgentOps Gas City pack

**Status: preview.** This pack is promoted to supported after the next official
Gas City release is pinned, deterministically qualified, and one clean canary
passes.

This is a thin AgentOps role pack over official Gas City. Gas City owns
sessions, routing, formulas, orders, and OTEL. Beads owns work and dependencies.
Git owns candidate commits, AgentOps owns the final semantic verdict, and
GitHub owns PR, CI, and merge state.

## Materialize the official pair

```sh
deploy/gc/materialize-toolchain.sh --output /path/to/toolchain
```

This downloads the official prebuilt Gas City and Beads release archives,
verifies each against its pinned official checksums file, installs `bin/gc` and
`bin/bd`, and writes `toolchain.json`. No compiler, `git`, or `make` is
required; the tools it uses are `curl`, `tar`, `python3`, and
`shasum`/`sha256sum`. It fetches the plain GitHub release download URLs, so no
`gh` authentication is needed.

The adjacent lock pins Gas City v1.3.5 at
`8ffc009ded781a2ada2077f3a29bd712b2def0bf` and Beads v1.1.0 at
`8e4e59d39f3459a43cf21a3236a13eca4dd874f7`.

## Known upstream issues (Gas City v1.3.5)

**Cross-store claim bug — not exercised by the pack's default flow.** On v1.3.5,
a *city-scoped* agent that claims a *rig-homed* bead federated-reads it across
rig stores and finds it, but the claim mutation runs against the agent's own
city store and returns `bead not found`. The pack's default intake never does
this: `invoke.sh feed` homes the source bead in the rig store and slings the
native `agentops-experiment` formula to the *rig-scoped* planner, so only rig
roles ever claim it (a rig-scoped agent claiming a rig-homed bead is unchanged
and safe). The bug affects only city-scoped agents that claim — for example a
custom Mayor rewired to claim source beads. It is fixed upstream after v1.3.5.

**Teardown hang.** On v1.3.5 the tmux teardown can rarely hang under process
churn, from a recursive live process-walk with PID reuse. Workaround: kill the
hung teardown and re-run `teardown.sh`. The fix is in upstream review
(gastownhall/gascity PR #3985).

## Bootstrap

Use a clean or disposable rig clone whose official Beads sync remote contains
the source bead. The bootstrap uses pinned `bd bootstrap` to clone that durable
database into the fresh city's one managed Dolt server, proves a bead is
present, and then lets `gc rig add --adopt` own endpoint normalization. It does
not invent or mirror Beads state:

```sh
deploy/gc/bootstrap.sh \
  --city /path/to/city \
  --rig /path/to/rig \
  --gc-bin /path/to/toolchain/bin/gc \
  --delivery-mode auto \
  --telemetry-mode required \
  --start
```

`manual` delivery leaves a checked PR open. `auto` asks GitHub to merge it after
hosted checks. The bootstrap uses the user default Git and GitHub identity.

## Default flow: create, feed, observe, refine, teardown

After bootstrap, the loop is create a bead, feed it, watch it flow through the
native formula onto `main`, then tear the city down:

```sh
# 1. Create a source bead directly in the managed rig store.
deploy/gc/invoke.sh --city /path/to/city create "Add a widget" -d "why and how"

# 2. Feed it. This homes the bead in the rig store, prepares one isolated
#    worktree, and slings the native agentops-experiment formula to the
#    rig-scoped planner (plan -> implement -> validate -> deliver).
deploy/gc/invoke.sh --city /path/to/city feed BEAD-ID

# 3. Observe.
deploy/gc/invoke.sh --city /path/to/city status
deploy/gc/invoke.sh --city /path/to/city doctor

# 4. The Refiner rebases the validated candidate and delivers it. In auto mode
#    GitHub merges it onto main after hosted checks; in manual mode a checked PR
#    is left open.

# 5. Teardown (see below).
```

`create` writes one `task` bead into the city's single managed Dolt server —
the same store `feed`, the rig roles, and the formula read — using the pinned
`bd` binary and the exact server environment the bootstrap uses. It prints the
new bead id (or the raw `bd` JSON with `--json`). This removes any need to drive
`bd` by hand.

`feed` is the official single-bead intake: `gc sling
RIG/agentops.plan-reviewer BEAD --on agentops-experiment --nudge`, plus the role
targets as formula vars. Because the bead is rig-homed and only rig-scoped roles
claim it, the v1.3.5 cross-store claim bug is never exercised (see Known upstream
issues). No AgentOps program graph or delivery ledger is created. The Beads
graph is the program graph.

The city-scoped Mayor is now an optional, observe-only coordinator: intake does
not route through it, and it never claims, routes, or edits. You can leave it
unused, or wake it to summarize ready, in-flight, blocked, and stuck work.

## Teardown

```sh
deploy/gc/teardown.sh --city /path/to/city
```

Teardown preserves durable city and Beads state while proving the private
supervisor and city-scoped processes are stopped.

## Bounded development loop

Run the fast, offline contract after each edit:

```sh
scripts/check-gc-executor.sh
```

Run the native bootstrap/formula/doctor/teardown qualification once after the
fast contract is green, using the materialized pinned binary:

```sh
GC_BIN=/path/to/toolchain/bin/gc \
AGENTOPS_GC_INTEGRATION=1 \
python3 -m unittest tests.python.test_gc33_thin_pack
```

The native qualification is deliberately opt-in: it starts disposable Beads
and Dolt services and is evidence for a candidate boundary, not an inner-loop
test. A live mixed-provider canary runs only after fresh semantic validation and
protected delivery.
