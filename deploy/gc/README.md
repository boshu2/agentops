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

**Cross-store claim failure — the default automated feed loop does not work on
v1.3.5.** The pack's default automated path feeds a rig-owned source bead to the
city-scoped Mayor: `invoke.sh` (line ~69) runs `gc sling agentops.mayor BEAD
--nudge`, and the Mayor is `scope = "city"` (`agents/mayor/agent.toml` line 1,
wired through `packs/agentops-factory/pack.toml` line ~8). On v1.3.5 the
city-scoped Mayor federated-reads the bead across rig stores and finds it, but
the claim mutation runs against the Mayor's own city store and returns `bead not
found`. This is deterministic: the first Mayor claim of a rig-fed bead fails
every time, for every default user. The preview is usable at v1.3.5 for
bootstrap, inspection, teardown, and manual flows; the automated feed loop
becomes functional at the next official Gas City release (fix already merged
upstream in commits `a135117945e2` and `3fa9bb38e916`).

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
