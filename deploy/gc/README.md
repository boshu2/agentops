# Retired AgentOps Gas City pack prototype

**Status: retired on 2026-07-29. Do not use this as the supported setup path.**
AgentOps now recommends the upstream
[`gascity` build pack](https://github.com/gastownhall/gascity-packs/tree/main/gascity)
and contributes skills through the provider runtime instead of owning formulas
or roles. See [`using-gc`](../../skills/using-gc/SKILL.md).

The scripts and notes below are retained temporarily as migration and test
evidence for the former prototype. They do not define current product behavior.

## Historical prototype

Gas City 1.4 is pinned and deterministically qualified below. This prototype
was a thin AgentOps role pack over official Gas City. Gas City owns sessions,
routing, formulas, orders, and OTEL. Beads owns work and dependencies. Git owns
candidate commits, AgentOps owns the final semantic verdict, and GitHub owns PR,
CI, and merge state.

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

The adjacent toolchain lock pins Gas City v1.4.0 at
`a7297c511d637a3609947386f3389d76ddb2f23b` and Beads v1.1.0 at
`8e4e59d39f3459a43cf21a3236a13eca4dd874f7`.
`pack-registry.lock.json` separately accepts the built-in `main` registry and
the official `gascity` workflow pack v0.1.6 at
`3b3b89f2011e06d84459aa7bea1552382f13930a`.

## Gas City 1.4 integration

The v1.3 heartbeat workaround is gone. Every graph-owning city or rig scope uses
the bundled `core.control-dispatcher`; workers claim routed work at their own
store boundary. The AgentOps Mayor remains an on-demand status/manual-dispatch
door and consumes no standing seat.

The pack composes the registry-pinned upstream `gascity` workflows and rig roles
used by the public Maintainer City at
[factory.gascity.com](https://factory.gascity.com). This adds upstream
`do-work`, build, review, issue, and PR formula families plus
`gc.run-operator`/`gc.implementation-worker` in every AgentOps rig. The
workflow pack is composed into `agentops-factory`, while its sibling role pack
is deliberately bound at `defaults.rig.imports.gc`: that is the stock Gas City
binding and the namespace the official formulas target by default.
`agentops-experiment` remains the default AgentOps flow and semantic completion
still requires `verdict.v2`.

Bootstrap refreshes the built-in registry, verifies `gascity` 0.1.6 against the
repository registry lock, installs all imports, and records the resulting
`packs.lock`. The optional community catalog can be added with:

```sh
gc pack registry add community https://registry.gascity.com/registry.toml
gc pack registry refresh
gc pack registry search --registry community --all
```

### Upgrading an existing city

Run the v1.4 convergence before starting the orchestrator:

```sh
gc doctor --fix
gc import install
gc supervisor stop --wait
gc start
```

Confirm `gc version` resolves the intended 1.4 binary, each graph-owning scope
has an unsuspended `core.control-dispatcher`, and `gc doctor` has no blocking
failures. Repair or explicitly unregister stale registered cities that block
startup. The dashboard is now served by the supervisor; remove any old
standalone-dashboard service or reverse-proxy target.

### Retire old HQ/canary registrations and start the replacement

Stop cities individually; do not stop the machine-wide supervisor that will
host the replacement. Resolve the exact registered names first:

```sh
gc cities --json

old_hq=/path/to/old-hq
old_canary=/path/to/old-canary
gc stop "$old_hq" --timeout 45s
gc unregister "$old_hq"
gc stop "$old_canary" --timeout 45s
gc unregister "$old_canary"

gc cities --json
```

`gc unregister` intentionally fails for an unknown target. If `gc cities`
already omits an old city, it is retired from the supervisor registry; inspect
and remove any separately installed legacy dashboard service outside GC.
Preserve the old directories until their Beads state is backed up or confirmed
disposable.

Bootstrap and start the replacement only after the old registrations are
settled:

```sh
new_city=/path/to/gc-agentops
rig_root=/path/to/agentops
deploy/gc/bootstrap.sh \
  --city "$new_city" \
  --rig "$rig_root" \
  --gc-bin /path/to/toolchain/bin/gc \
  --delivery-mode auto \
  --telemetry-mode required \
  --start

gc cities --json
gc --city "$new_city" status
gc --city "$new_city" doctor --json
```

`bootstrap.sh --start` registers the new city with the v1.4 machine-wide
supervisor, starts reconciliation, resumes the adopted rig, and runs doctor.

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

## Default flow: run-centered, with an optional Mayor door

After bootstrap, author intent with `create`, start one run with `feed`, and use
the v1.4 dashboard/run API while the scoped control dispatcher advances the
formula.

Think of controlling the Mayor like driving any tmux-resident agent (NTM-style):
one on-demand session you steer. The difference is that Gas City ships
first-class control primitives — mail and sling — so you steer it with messages,
not keystroke injection. There are two doors into the same session:

```sh
# 1. Author intent and launch the bounded AgentOps formula.
deploy/gc/invoke.sh --city /path/to/city create "Add a widget" -d "why and how"
deploy/gc/invoke.sh --city /path/to/city feed BEAD-ID

# 2. Print the supervisor-hosted run dashboard.
deploy/gc/invoke.sh --city /path/to/city dashboard

# 3. Optional HUMAN door: start and attach to the on-demand Mayor.
deploy/gc/invoke.sh --city /path/to/city mayor start
deploy/gc/invoke.sh --city /path/to/city mayor status   # prints the attach line:
#   tmux -L <socket> attach -t <mayor-session>

# 4. Optional AGENT door: reference BEAD IDS, never prose.
deploy/gc/invoke.sh --city /path/to/city mayor tell "dispatch test-abc"

# 5. Read state from GC and Beads.
deploy/gc/invoke.sh --city /path/to/city mayor status
deploy/gc/invoke.sh --city /path/to/city status
deploy/gc/invoke.sh --city /path/to/city doctor

# 5. The Refiner rebases the validated candidate and lands it on main. In auto
#    mode GitHub merges after hosted checks; in manual mode a checked PR is left
#    open.

# 6. State-preserving teardown (see below).
```

**Author vs. dispatch — the line that must hold.** `create` and `feed` are the
only way work enters the graph: `create` writes one `task` bead with exact
acceptance into the managed store, `feed` homes it in the rig store, prepares one
isolated worktree, and slings the native `agentops-experiment` formula to the
rig-scoped planner (`gc sling RIG/agentops.plan-reviewer BEAD --on
agentops-experiment --nudge`, plus the role targets as formula vars). The Mayor
DISPATCHES existing beads; it never authors work and never claims. So drivers
(human or agent) hand the Mayor BEAD IDS, never paraphrased work — a paraphrase
is a telephone game that drifts from the acceptance the bead already carries.
Only rig-scoped roles claim. No AgentOps program graph or delivery ledger is
created; the Beads graph is the program graph. The Mayor is a manual escape
hatch for a ready bead, not the v1.4 propulsion engine.

### Provider wedge prerequisite

Codex must be current — a pending update prompt wedges the provider pane — and
the rig and worktree-root paths must be trusted in `~/.codex/config.toml` as
exact-path `trust_level` entries; the bootstrap does not yet write them, so add
them yourself before the first run.

### Visibility

Four observability layers; each lies in its own way, so cycle all four:

- **Run/supervisor state** — `invoke.sh --city C dashboard`, `status`, and
  `gc session list`: stage ladder, structured transcript, token rate, burn rate,
  and lifecycle shape; an active roster can still hide a wedged pane.
- **Bead graph** — `gc bd --rig <rig> ready|show <id>`: the only completion truth.
- **Pane truth** — `tmux -L <socket> capture-pane -t <session> -p`: ground truth
  for wedges (update nags, trust prompts, API/DNS failures print here first).
- **Health machinery** — `gc doctor`, `gc order history <order>`: the city's
  metabolism, not any specific work's progress.

Robot state lies by omission; pane capture is ground truth — when they disagree,
trust the pane. Full discipline: the `using-gc` skill.

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
