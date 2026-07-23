# AgentOps Gas City pack

**Status: preview — this is the 3.3 supported-candidate flow.** The mayor-driven
door below is the flow proposed for support. Promotion to supported is still
gated on the next official Gas City release being pinned, deterministically
qualified, and one clean mixed-provider canary passing.

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

**Demand-driven spawn is broken for rig work (#4586) — the shepherd Mayor is the
supported propulsion path.** On v1.3.5 the controller never loads external-rig
bead stores, so demand-driven worker spawning never fires for rig-routed step
beads: ready formula steps sit un-run because no session is spawned to claim
them. Nudge-driven dispatch is unaffected — slinging a ready step bead to its
run-target with `--nudge` spawns that session, which claims the rig-scoped bead
and runs it. The standing Mayor shepherd does exactly this: it watches ready rig
work and sling-nudges each ready step bead to its run-target. This is also the
stock Gas City mayor's documented job (`gc bd create -> gc sling <agent>
<bead-id> -> monitor`), so once #4586 is fixed upstream the shepherd becomes
redundant-but-harmless for propulsion — which is the stock design anyway.

**Cross-store claim bug — not exercised by the pack's flow.** On v1.3.5, a
*city-scoped* agent that claims a *rig-homed* bead federated-reads it across rig
stores and finds it, but the claim mutation runs against the agent's own city
store and returns `bead not found`. The pack never does this: `feed` homes the
source bead in the rig store and the formula routes to *rig-scoped* roles, and
the Mayor dispatches but never claims (rig roles claim). The bug affects only
city-scoped agents that claim — for example a Mayor rewired to claim source
beads. It is fixed upstream after v1.3.5.

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

## Default flow: the mayor-driven door (human or agent)

After bootstrap, the loop is: start the Mayor, drive it (a human attaches, or an
agent tells it), author intent with `create`/`feed`, and let the Mayor shepherd
each step through the native formula onto `main`.

Think of controlling the Mayor like driving any tmux-resident agent (NTM-style):
one standing session you steer. The difference is that Gas City ships first-class
control primitives — mail and sling — so you steer it with messages, not
keystroke injection. There are two doors into the same session:

```sh
# 1. Start the standing Mayor shepherd (idempotent; it stays resident).
deploy/gc/invoke.sh --city /path/to/city mayor start

# 2a. HUMAN door: attach to the Mayor's tmux session and drive interactively.
deploy/gc/invoke.sh --city /path/to/city mayor status   # prints the attach line:
#   tmux -L <socket> attach -t <mayor-session>

# 2b. AGENT door: drive the Mayor with messages. Reference BEAD IDS, never prose.
deploy/gc/invoke.sh --city /path/to/city mayor tell "dispatch test-abc"

# 3. Author intent (either door). create writes a source bead with exact
#    acceptance; feed homes it in the rig store and attaches the native formula.
deploy/gc/invoke.sh --city /path/to/city create "Add a widget" -d "why and how"
deploy/gc/invoke.sh --city /path/to/city feed BEAD-ID

# 4. Propulsion is automatic: a scheduled heartbeat order re-nudges the Mayor
#    every few minutes to run a dispatch pass, so each ready step is slung to its
#    run-target (plan -> implement -> validate -> deliver) as it becomes ready.
#    `mayor tell "dispatch <id>"` is for on-demand nudges between heartbeats.
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
Only rig-scoped roles claim, so the v1.3.5 cross-store claim bug is never
exercised. No AgentOps program graph or delivery ledger is created; the Beads
graph is the program graph.

**Why the Mayor propels — and how it stays awake.** On v1.3.5 demand-driven
spawning is broken for rig work (#4586), so ready formula steps would otherwise
sit un-run. The standing Mayor shepherd sling-nudges each ready step bead to its
run-target — the supported propulsion path, and also the stock Gas City mayor
pattern, so it survives the upstream fix (see Known upstream issues). But
`mode=always` only keeps the session RESIDENT; nothing re-prompts it when a step
completes. So the pack ships a scheduled heartbeat order
(`packs/agentops-factory/orders/shepherd-heartbeat.toml`, a `cooldown` order on a
few-minute interval) that re-nudges the Mayor to run a fresh dispatch pass. Each
pass is cheap when nothing is ready. `mayor tell` remains the on-demand path for
dispatching a specific bead id between heartbeats.

### Provider wedge prerequisite

Codex must be current — a pending update prompt wedges the provider pane — and
the rig and worktree-root paths must be trusted in `~/.codex/config.toml` as
exact-path `trust_level` entries; the bootstrap does not yet write them, so add
them yourself before the first run.

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
