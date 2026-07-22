# Gas City deployment for AgentOps

This directory is the source-controlled deployment projection for running an
AgentOps Gas City pack. It creates a new, isolated city with explicit Codex and
interactive Claude role pools. Use `agentops-executor` for caller-supplied
single packets or `agentops-factory` for checked, bead-native workflow
orchestration. In 3.3 the factory is Fable Mayor → fresh Sol plan check →
one-shot graph admission → Terra/Opus implementation → fresh Sol validation
→ semantic terminal → deterministic delivery. Refiner is not a clean-path
worker and Luna is support-only; no Mayor retry/rescope or drain topology is
enabled.
Bootstrap does not migrate an older city, start a role session, or make Gas
City the owner of AgentOps completion.

The suggested machine-local city path on bo-mac is
`/Users/bo/dev/gc-agentops`. That path is documentation only; the portable
configuration and bootstrap require an explicit caller-supplied target.

## Exact toolchain

AgentOps does not trust a version string or the ambient `PATH` to select Gas
City. `toolchain.lock.json` pins the official Gas City v1.3.5 and Beads v1.1.0
source commits and their official release-checksum asset digests. Materialize the default pair
from those commits into a new directory:

```sh
deploy/gc/materialize-toolchain.sh --output /path/to/agentops-gc-toolchain
```

The command checks out both declared commits, uses each project’s canonical
build, verifies the resulting runtime identities, builds `bin/ao` and
`bin/agentops-gc-delivery` from one exact committed AgentOps CLI tree, and
writes a schema-3 `toolchain.json` with all four source and binary identities.
It never edits an installed Homebrew or user binary. Use
`--describe` to inspect the default pair or `--pair ID --describe` to inspect
another accepted entry without building it.

The first lock entry with `status=qualified` is the default. The PR #3985
exception is deliberately absent: it may be admitted only after an exact,
current reproducer proves stable v1.3.5 fails and the exact PR head passes.
An unlisted build is rejected even when it reports the same version.

## Bootstrap

Use a disposable rig/worktree rather than the live AgentOps checkout. The pack
may be an uncommitted local pack; the bootstrap records that source as a plain
local path so Gas City reads it in place.

```sh
deploy/gc/bootstrap.sh \
  --city /Users/bo/dev/gc-agentops \
  --rig /path/to/disposable/agentops-rig \
  --pack /path/to/agentops-factory-pack \
  --gc-bin /path/to/agentops-gc-toolchain/bin/gc \
  --ao-bin /path/to/agentops-gc-toolchain/bin/ao \
  --codex-auth /path/to/codex/auth.json
```

The default is configure-and-preflight only. Add `--start` to start the city
and resume the registered rig after lint, Codex and Claude authentication,
resolved-config, config-provenance, and import-status checks pass. Gas City
must also complete one city-store and one rig-store read before bootstrap
reports the started deployment ready; this moves Dolt cold-start latency out of
the first agent turn. Readiness also requires Gas City's
`beads.native_store_eligible=true` status: a city whose `bd context` cannot
cross-check its backend may still answer through the subprocess fallback, but
AgentOps refuses to call that degraded and substantially slower path ready. Gas
City records that resume as a persistent runtime preference, so it survives an
ordinary city restart; use `gc rig suspend` when the rig must remain dormant.
Rig registration validates and reverses only Gas City's canonical `.gitignore`
projection, preserving the caller's exact tracked bytes while keeping the
runtime-owned `.beads` state local.
The deployment declares no always-on sessions. Gas City derives a generic
sling target from each registered provider only after applying patches.
Bootstrap therefore projects explicit suspended city convention agents, and
the executor pack supplies explicit suspended rig agents, so GC's documented
"explicit wins" rule prevents those generic identities from being injected.
Scaffold maintenance pools are suspended with ordinary patches because they
already exist when patches run. Only roles explicitly supplied by the selected
pack remain routable. Direct packet work uses the registered rig root. Factory Formula
tasks instead stamp the exact absolute `work_dir` selected for that bead;
official GC v1.3.5 launches the fresh Terra/Opus/Sol session there. The 3.3
factory does not create a second rig, Dolt server, or integration rig per
candidate.

The private `CODEX_HOME` contains a symlink to the explicitly selected existing
`auth.json`; credentials are not copied. When `--codex-auth` is omitted, the
bootstrap uses the `auth.json` under the caller's original `CODEX_HOME` (or
`$HOME/.codex`). It fails before starting unless `codex login status` succeeds
through the private home. `--gc-bin` and `--ao-bin` are explicit paths;
bootstrap never resolves `gc`, `bd`, `ao`, or the delivery reducer from ambient
`PATH` after admission. `--gc-bin` must have its paired `bd`, `ao`,
`agentops-gc-delivery`, and schema-3 `toolchain.json` beside it. The receipt
must contain exactly those four runtimes; `ao` and the delivery reducer must
bind the same source commit and committed CLI tree. The marker records selected
source commits, release-checksum provenance, runtime paths/digests, exact pack
content, reducer and schema/config digests, and the Gas City-generated
`packs.lock` digest. The native delivery context separately binds the exact
Git, GitHub CLI, and `/bin/bash` bytes used for delivery effects and check-only
gates. Moving an existing managed city to a different pair or binary path
requires `--replace-gc-bin --start`; every other recorded city identity field
must still match, and bootstrap replaces the supervisor then verifies the new
pair before rewriting the marker. Bootstrap prepends the pair's directory to
`PATH`. It also reserves and persists a private
loopback supervisor port in `<city>/.gc-home/supervisor.toml`; this prevents a
service-manager launch from falling back to Gas City's machine-wide default
port when other cities are already running. It also pins `[session].socket` to
a stable digest of the canonical city path. Tmux sockets are machine-global,
so this keeps two isolated cities with the same basename (for example repeated
`.../city` canaries) from seeing, restarting, or stopping each other's agents.
The managed session setup deadline is 60 seconds because Gas City materializes
assigned skills in `PreStart`; cold controller and per-session materialization
can briefly contend on the same catalog, and the SDK's 10-second default is not
a safe production deadline for that legitimate startup work.

Bootstrap disables Beads' Git-to-Dolt remote synthesis while registering the
rig. A Git URL (especially a local clone path) is not a Dolt replication URL;
off-box Beads replication must be configured explicitly instead of inferred.

Claude roles run as fresh GC-owned interactive tmux sessions. The provider
clears inherited `print_args`, and bootstrap rejects `-p` or `--print` in both
base and appended arguments, so no AgentOps path can turn a Claude role into a
headless print sink. `wake_mode = "fresh"`, the claimed bead, and the attested
GC session ID provide role freshness. Bootstrap requires `claude auth status
--json` to report first-party login. The managed city also projects
`remoteControlAtStartup = false` into Claude's city-local settings. This keeps
an operator's optional account-wide Remote Control preference from joining GC
workers to one competing remote epoch and terminating an otherwise healthy
interactive session; it does not disable Claude authentication or interaction.
The city fully replaces Gas City's inherited option schemas. Codex defaults to
workspace-write/no-prompts; delivery is model-free and does not need a Refiner
session. Claude roles default to the
interactive unrestricted mode inside their GC-managed worktrees because Claude's
`auto` mode delegates each tool call to a remote permission classifier that can
stall indefinitely; managed roots, safe mode, bead fences, and role-specific
write-scope checks remain enforced. Model choice is role policy, not an ambient
CLI default: Fable-adaptive owns Mayor and at most one ambiguity consultation,
Sol-high owns plan binding and fresh validation, Terra-high is the default
implementer, and Opus-medium is the explicitly admitted overflow implementer.
Luna has no execution route. No role may silently fall back to another profile.
Codex's
workspace-write profile enables network access because GC's private Dolt service
is a loopback TCP endpoint; this does not expand Codex's writable filesystem
roots. GC v1.3.5 telemetry policy is `auto`, `required`, or `off` (default
`auto`). Bootstrap clears the generic ambient OTLP fallback and sets only the
explicit GC metrics/logs pair. `auto` records a durable degraded state when
either endpoint is unavailable; `required` fails before city mutation and is
the release-canary mode. Both providers receive only the managed city runtime directories and
configured rig roots as additional writable directories. Packets name
`provider = codex | claude`; explicit disabled agents keep generic provider targets unroutable and
cannot receive packet work.

At dispatch, the adapter requires the packet `workspace` to equal the physical
root of the selected Gas City rig. This keeps the packet's mutation boundary,
the provider's additional-directory boundary, and GC's routing scope identical.
The transport description also carries the adapter script's resolved absolute
path, so imported local packs work without relying on prompt-template directory
variables that are not available in direct local-pack role rendering.

The command is recoverably idempotent for the same resolved city, rig, pack,
rig name, binding, paired toolchain, and auth source, including a prior failure after
city creation but before rig registration. A nonempty city without
`.gc/agentops-bootstrap.json` is refused. This is intentional: preserve
historical cities as evidence and choose a fresh path instead of rewriting them
in place.

Factory delivery defaults to `--delivery-mode auto`: the deterministic reducer
creates or adopts the PR, observes protected CI, rebases against moving `main`,
and requests the lawful merge. `--delivery-mode manual` keeps the same checked
path but waits for external review/merge. Neither mode locks the base branch or
makes delivery a condition of semantic bead closure.

Start one bounded factory program through the imported pack command:

```sh
deploy/gc/invoke.sh --city /path/to/city -- agentops program start --source-bead age-example --max-parallel 2
```

The managed invoker reads the bootstrap marker, verifies the exact recorded
Gas City binary digest, selects the private supervisor, projects the effective
telemetry pair (or explicit disabled state), and clears an ambient generic OTLP
fallback. Its `--` is the wrapper boundary and is consumed; do not put another
`--` between the discovered `program start` leaf and `--source-bead`.

The command snapshots that exact source Bead once, freezes the first observed
base OID for the program, runs Fable Mayor and fresh Sol plan binding, and then
admits only the checked graph. A later replay adopts the same program identity
even if `main` has moved; moving-base reconciliation belongs to delivery.

## Teardown

Quiesce a managed city with the matching bootstrap marker:

```sh
deploy/gc/teardown.sh --city /Users/bo/dev/gc-agentops
```

This preserves the city and its durable beads for a later restart. It selects
the private supervisor with `GC_HOME=<city>/.gc-home`—`--city` alone does not
select supervisor identity—waits for destructive supervisor shutdown, performs
one final idempotent managed-Dolt stop, and fails closed if the path-bound tmux
socket or a city-scoped process remains live. AgentOps 3.3 sets GC v1.3.5's
supported `event_hooks = false` because its event-propulsion orders are also
disabled; clean managed cities therefore have no per-write Beads hook chain.
Teardown defensively leaves a legacy GC-stamped hook non-executable, and the
next managed bootstrap removes that projection before runtime admission.
`--gc-bin` is optional and, when supplied, must match the exact paired toolchain
recorded by bootstrap.

## Configuration ownership

| File | Owner | Contents |
|---|---|---|
| `deploy/gc/city.toml` | AgentOps source | Portable dual-provider policy, safe CLI flag mappings, and workspace concurrency cap |
| `deploy/gc/agents/*/agent.toml` | AgentOps source | Explicit suspended city agents that shadow GC's late implicit provider injection |
| `deploy/gc/toolchain.lock.json` | AgentOps source | Accepted exact GC/Beads source pairs and their qualification state |
| `deploy/gc/materialize-toolchain.sh` | AgentOps source | Fail-closed source checkout, canonical builds, runtime verification, and local receipt |
| `deploy/gc/invoke.sh` | AgentOps source | Marker-bound operator invocation, private supervisor selection, exact GC digest, and effective telemetry environment |
| `<city>/pack.toml` | `gc init`, then bootstrap | Current built-in pins plus the city-scoped `agentops` import |
| `<city>/city.toml` | `gc init`, `gc rig add`, then bootstrap | Portable runtime policy, logical rig declaration, and rig-scoped `agentops` import |
| `<city>/.gc/site.toml` | Gas City SDK/CLI | Machine-local workspace identity and physical rig path |
| `<city>/.gc-home/supervisor.toml` | Bootstrap, then Gas City runtime | Persisted loopback supervisor address; selected once and preserved on managed reruns |
| `<city>/.gc-home` | Gas City runtime | Remaining private supervisor/store discovery state |
| `<city>/.gc/codex-home` | Codex runtime | Private session state plus a symlink to the selected external `auth.json` |
| Caller home | Claude runtime | Existing authenticated interactive Claude state; no auth material is copied into the city |
| `<city>/.gc/agentops-bootstrap.json` | Bootstrap | Exact city/rig/pack/auth/toolchain, telemetry, reducer identity, `packs.lock` digest, binary digests, and recovery state |

Do not source-control `.gc/site.toml`, `.gc-home`, or the generated city. To
promote a stable release, point `--pack` at a clean committed pack and let the
Gas City import workflow produce a commit pin. The bootstrap intentionally uses
direct local TOML imports for an uncommitted pack, matching `gc import add`'s
documented read-in-place case.

After starting, inspect the deployment with the same isolated environment:

```sh
deploy/gc/invoke.sh --city /Users/bo/dev/gc-agentops -- status
```

See `docs/contracts/gas-city-execution-adapter.md` for the semantic boundary
between Gas City transport and the AgentOps experiment.
