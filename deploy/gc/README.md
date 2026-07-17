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

## Bootstrap

Use a disposable rig/worktree rather than the live AgentOps checkout. The pack
may be an uncommitted local pack; the bootstrap records that source as a plain
local path so Gas City reads it in place.

```sh
deploy/gc/bootstrap.sh \
  --city /Users/bo/dev/gc-agentops \
  --rig /path/to/disposable/agentops-rig \
  --pack /path/to/agentops-factory-pack \
  --gc-bin /path/to/gc \
  --codex-auth /path/to/codex/auth.json
```

The default is configure-and-preflight only. Add `--start` to start the city
and resume the registered rig after lint, Codex and Claude authentication,
resolved-config, config-provenance, and import-status checks pass. Gas City
records that resume as a persistent runtime preference, so it survives an
ordinary city restart; use `gc rig suspend` when the rig must remain dormant.
The deployment declares no always-on or named sessions. Gas City derives a
generic sling target from each registered provider; bootstrap explicitly
suspends those targets plus the scaffold's maintenance pools at city and
managed-rig scope. Only roles explicitly supplied by the selected pack remain
routable in the primary rig. The factory further patches every dynamic
candidate rig so only its bead-selected Codex/Claude Worker and Validator
routes remain active, and every integration rig so only its two Validator
routes remain active.

The private `CODEX_HOME` contains a symlink to the explicitly selected existing
`auth.json`; credentials are not copied. When `--codex-auth` is omitted, the
bootstrap uses the `auth.json` under the caller's original `CODEX_HOME` (or
`$HOME/.codex`). It fails before starting unless `codex login status` succeeds
through the private home. `--gc-bin` is recorded in the managed-city marker and
in workspace session environment, so pack commands and workers do not drift to
a different `gc` on `PATH`. The bootstrap also reserves and persists a private
loopback supervisor port in `<city>/.gc-home/supervisor.toml`; this prevents a
service-manager launch from falling back to Gas City's machine-wide default
port when other cities are already running.

Bootstrap disables Beads' Git-to-Dolt remote synthesis while registering the
rig. A Git URL (especially a local clone path) is not a Dolt replication URL;
off-box Beads replication must be configured explicitly instead of inferred.

Claude remains an interactive provider. The config clears inherited
`print_args`, so Gas City cannot add `-p` for title or one-shot synthesis, and
bootstrap requires `claude auth status --json` to report first-party login.
The city fully replaces Gas City's inherited option schemas. Codex defaults to
workspace-write/no-prompts and exposes one additional unrestricted choice used
only by the fenced Refiner for GitHub delivery; Claude exposes interactive
`auto` mode with safety checks. Codex's workspace-write profile enables network
access because GC's private Dolt service is a loopback TCP endpoint; this does
not expand Codex's writable filesystem roots. Both providers receive only the
managed city runtime directories and configured rig roots as additional
writable directories. Packets name
`provider = codex | claude`; generic provider targets remain suspended and
cannot receive packet work.

At dispatch, the adapter requires the packet `workspace` to equal the physical
root of the selected Gas City rig. This keeps the packet's mutation boundary,
the provider's additional-directory boundary, and GC's routing scope identical.
The transport description also carries the adapter script's resolved absolute
path, so imported local packs work without relying on prompt-template directory
variables that are not available in direct local-pack role rendering.

The command is recoverably idempotent for the same resolved city, rig, pack,
rig name, binding, GC binary, and auth source, including a prior failure after
city creation but before rig registration. A nonempty city without
`.gc/agentops-bootstrap.json` is refused. This is intentional: preserve
historical cities as evidence and choose a fresh path instead of rewriting them
in place.

## Configuration ownership

| File | Owner | Contents |
|---|---|---|
| `deploy/gc/city.toml` | AgentOps source | Portable dual-provider policy, safe CLI flag mappings, and workspace concurrency cap |
| `<city>/pack.toml` | `gc init`, then bootstrap | Current built-in pins plus the city-scoped `agentops` import |
| `<city>/city.toml` | `gc init`, `gc rig add`, then bootstrap | Portable runtime policy, logical rig declaration, and rig-scoped `agentops` import |
| `<city>/.gc/site.toml` | Gas City SDK/CLI | Machine-local workspace identity and physical rig path |
| `<city>/.gc-home/supervisor.toml` | Bootstrap, then Gas City runtime | Persisted loopback supervisor address; selected once and preserved on managed reruns |
| `<city>/.gc-home` | Gas City runtime | Remaining private supervisor/store discovery state |
| `<city>/.gc/codex-home` | Codex runtime | Private session state plus a symlink to the selected external `auth.json` |
| Caller home | Claude runtime | Existing authenticated interactive Claude state; no auth material is copied into the city |
| `<city>/.gc/agentops-bootstrap.json` | Bootstrap | Exact city/rig/pack/auth/GC-binary identity and recovery state |

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
