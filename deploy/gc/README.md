# AgentOps Gas City pack

This is a thin AgentOps role pack over official Gas City. Gas City owns
sessions, routing, formulas, orders, and OTEL. Beads owns work and dependencies.
Git owns candidate commits, AgentOps owns the final semantic verdict, and
GitHub owns PR, CI, and merge state.

## Materialize the official pair

```sh
deploy/gc/materialize-toolchain.sh --output /path/to/toolchain
```

The adjacent lock pins Gas City v1.3.5 at
`8ffc009ded781a2ada2077f3a29bd712b2def0bf` and Beads v1.1.0 at
`8e4e59d39f3459a43cf21a3236a13eca4dd874f7`.

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

## Feed and observe

```sh
deploy/gc/invoke.sh --city /path/to/city feed BEAD-ID
deploy/gc/invoke.sh --city /path/to/city status
deploy/gc/invoke.sh --city /path/to/city doctor
```

`feed` is intentionally a small wrapper around native `gc sling
agentops.mayor BEAD-ID --nudge`. The Mayor creates Beads and attaches the native
`agentops-experiment` formula. No AgentOps program graph or delivery ledger is
created.

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
