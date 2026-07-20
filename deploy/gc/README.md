# Gas City deployment for AgentOps

This directory is the source-controlled deployment projection for running an
AgentOps Gas City pack. It creates a new, isolated city with explicit Codex and
interactive Claude role pools. Use `agentops-executor` for caller-supplied
single packets or `agentops-factory` for Mayor-to-Refinery bead orchestration.
Bootstrap does not migrate an older city, start a role session, or make Gas
City the owner of AgentOps completion.

The suggested machine-local city path on bo-mac is
`/Users/bo/dev/gc-agentops`. That path is documentation only; the portable
configuration and bootstrap require an explicit caller-supplied target.

## Exact toolchain

AgentOps does not trust a version string or the ambient `PATH` to select Gas
City. `toolchain.lock.json` lists the exact GC and official Beads source commits
that this deployment accepts. Materialize the default, fully qualified pair
from those commits into a new directory:

```sh
deploy/gc/materialize-toolchain.sh --output /path/to/agentops-gc-toolchain
```

The command checks out both declared commits, uses each project’s canonical
build, verifies the resulting runtime identities, colocates `bin/gc` and
`bin/bd`, and writes `toolchain.json` with the source identities and local
binary digests. It never edits an installed Homebrew or user binary. Use
`--describe` to inspect the default pair or `--pair ID --describe` to inspect
another accepted entry without building it.

The first lock entry with `status=qualified` is the default. It is the exact
AgentOps fork commit that passed the mixed Codex/Claude parallel factory
canary. A `compatible` entry satisfies the SDK and command contracts but has
not passed that same AgentOps qualification matrix. Bootstrap accepts either
only because both are explicit lock entries; an unlisted build is rejected even
when it reports the same version.

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
The deployment declares no always-on or named sessions. Gas City derives a
generic sling target from each registered provider; bootstrap explicitly
suspends those targets plus the scaffold's maintenance pools at city and
managed-rig scope. Only roles explicitly supplied by the selected pack remain
routable in the primary rig. Bootstrap also pins the four direct packet
executor roles to the primary rig's parent directory and verifies that native
`gc agent list` resolves the same roots. Because packet metadata contributes
the candidate directory name, this launches the role in the exact supplied
workspace instead of the invalid `<workspace>/<workspace>` path. The factory
applies the same parent-root rule while patching every dynamic candidate rig so
only its bead-selected Codex/Claude Worker and Validator routes remain active,
and every integration rig so only its two Validator routes remain active.

The private `CODEX_HOME` contains a symlink to the explicitly selected existing
`auth.json`; credentials are not copied. When `--codex-auth` is omitted, the
bootstrap uses the `auth.json` under the caller's original `CODEX_HOME` (or
`$HOME/.codex`). It fails before starting unless `codex login status` succeeds
through the private home. `--gc-bin` must have its paired `bd` beside it. Both
runtime identities must match one exact entry in `toolchain.lock.json` before
the city is created. The marker records the selected qualification id, full
source commits, runtime identities, paths, and binary digests; workspace
sessions inherit the same pair and cannot drift to another `gc` or `bd` on
`PATH`. Moving an existing managed city to a different pair or binary path
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
workspace-write/no-prompts and exposes one additional unrestricted choice used
only by the fenced Refiner for GitHub delivery. Claude roles default to the
interactive unrestricted mode inside their GC-managed worktrees because Claude's
`auto` mode delegates each tool call to a remote permission classifier that can
stall indefinitely; managed roots, safe mode, bead fences, and role-specific
write-scope checks remain enforced. Model choice is role policy, not an ambient CLI
default: the Codex pool exposes Terra for implementation and Sol for
Mayor/Judge/Validator/Refiner work; the Claude pool exposes Opus 4.8 for both
implementation and higher-reasoning roles. There is no Fable alias. Factory
planning records the selected Mayor, plan-review Judge, Refiner, and integration
Validator providers on the program/Refinery beads; judges must use the opposite
provider family from the authoring lifecycle role. Codex's
workspace-write profile enables network access because GC's private Dolt service
is a loopback TCP endpoint; this does not expand Codex's writable filesystem
roots. Managed role sessions explicitly disable inherited OpenTelemetry SDK
exporters so a stale machine-wide collector endpoint cannot delay every GC/bd
command. Both providers receive only the managed city runtime directories and
configured rig roots as additional writable directories. Packets name
`provider = codex | claude`; generic provider targets remain suspended and
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

Factory planning defaults to `--delivery-mode pr`. Reliability canaries may use
`--delivery-mode qualify`: the selected Sol or Opus 4.8 Refiner still assembles the certified train
and obtains a fresh integration verdict, but an exact PASS produces a durable
qualification receipt and closes the canary beads without any push, PR, merge,
or base-branch mutation. Qualification is runtime evidence, not proof of the
external forge/checks/merge portion of PR delivery.

Factory `program plan` and `program admit` default to the bootstrap import
binding, `agentops`. Pass `--binding` only when bootstrap used the same explicit
nondefault binding; the persisted binding is then reused for lifecycle routing
and dynamic worktree rigs.

## Teardown

Quiesce a managed city with the matching bootstrap marker:

```sh
deploy/gc/teardown.sh --city /Users/bo/dev/gc-agentops
```

This preserves the city and its durable beads for a later restart. It selects
the private supervisor with `GC_HOME=<city>/.gc-home`—`--city` alone does not
select supervisor identity—waits for destructive supervisor shutdown, performs
one final idempotent managed-Dolt stop, and fails closed if the path-bound tmux
socket or a city-scoped process remains live. `--gc-bin` is optional and, when
supplied, must match the exact paired toolchain recorded by bootstrap.

## Configuration ownership

| File | Owner | Contents |
|---|---|---|
| `deploy/gc/city.toml` | AgentOps source | Portable dual-provider policy, safe CLI flag mappings, and workspace concurrency cap |
| `deploy/gc/toolchain.lock.json` | AgentOps source | Accepted exact GC/Beads source pairs and their qualification state |
| `deploy/gc/materialize-toolchain.sh` | AgentOps source | Fail-closed source checkout, canonical builds, runtime verification, and local receipt |
| `<city>/pack.toml` | `gc init`, then bootstrap | Current built-in pins plus the city-scoped `agentops` import |
| `<city>/city.toml` | `gc init`, `gc rig add`, then bootstrap | Portable runtime policy, logical rig declaration, and rig-scoped `agentops` import |
| `<city>/.gc/site.toml` | Gas City SDK/CLI | Machine-local workspace identity and physical rig path |
| `<city>/.gc-home/supervisor.toml` | Bootstrap, then Gas City runtime | Persisted loopback supervisor address; selected once and preserved on managed reruns |
| `<city>/.gc-home` | Gas City runtime | Remaining private supervisor/store discovery state |
| `<city>/.gc/codex-home` | Codex runtime | Private session state plus a symlink to the selected external `auth.json` |
| Caller home | Claude runtime | Existing authenticated interactive Claude state; no auth material is copied into the city |
| `<city>/.gc/agentops-bootstrap.json` | Bootstrap | Exact city/rig/pack/auth/paired-toolchain identity, qualification, binary digests, and recovery state |

Do not source-control `.gc/site.toml`, `.gc-home`, or the generated city. To
promote a stable release, point `--pack` at a clean committed pack and let the
Gas City import workflow produce a commit pin. The bootstrap intentionally uses
direct local TOML imports for an uncommitted pack, matching `gc import add`'s
documented read-in-place case.

After starting, inspect the deployment with the same isolated environment:

```sh
GC_HOME=/Users/bo/dev/gc-agentops/.gc-home \
GC_ISOLATED=1 \
/path/to/gc --city /Users/bo/dev/gc-agentops status
```

See `docs/contracts/gas-city-execution-adapter.md` for the semantic boundary
between Gas City transport and the AgentOps experiment.
