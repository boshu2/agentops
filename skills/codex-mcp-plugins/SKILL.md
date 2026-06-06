---
name: codex-mcp-plugins
user-invocable: false
description: |
  Wire MCP servers and plugins into the Codex CLI — the harness-native path
  to reach flywheel binaries (Agent-Mail, br/beads) and to ship the AgentOps
  skill bundle to Codex.

  Triggers: "add an MCP server to Codex", "wire Agent-Mail into codex",
  "reach br/beads from codex", "ship the skill bundle to Codex", "install the
  AgentOps plugin in codex", "codex mcp add/list/get/login/logout", "codex plugin
  add/list/marketplace", "skill_mcp_dependency_install", "plugin_sharing",
  "connect Codex to a tool server", standing up a Codex lane in the flywheel.

  Perfect for:
  - Operator setup of a Codex lane in the flywheel (caam profile, bushido node, Mac cockpit)
  - Making a freshly-built MCP server reachable from `codex exec` workers

  Not ideal for:
  - Client-facing AI Partner work (this drives operator binaries — keep it backstage)
  - Claude Code MCP/plugin config (use that harness's own surfaces, not codex CLI)
practices:
- data-contracts
- twelve-factor-app
hexagonal_role: driven-adapter
consumes:
- mcp-server
- codex-plugin
produces:
- codex-config
context_rel:
- kind: supplier-to
  with: codex-exec
skill_api_version: 1
user-invocable: true
context:
  window: inherit
  intent:
    mode: task
  sections:
    exclude: [HISTORY]
  intel_scope: topic
metadata:
  tier: execution
  dependencies: [agent-mail, beads-br, caam]
  stability: stable
output_contract: "Idempotent edits to ~/.codex/config.toml (MCP servers + marketplaces) plus verification output from `codex mcp list --json` / `codex plugin list --json`."
---

# codex-mcp-plugins

Wire MCP servers and plugins into the Codex CLI so a Codex worker can reach the
flywheel's shared substrate (Agent-Mail coordination, br/beads tracking) and run
the AgentOps skill bundle natively in its own harness.

## Overview / When to Use

Codex has two harness-native extension surfaces, both backed by `~/.codex/config.toml`:

- **`codex mcp`** — register external MCP servers (stdio command OR streamable
  HTTP URL), then `login`/`logout` for OAuth-protected ones. This is how a Codex
  worker reaches a tool server: Agent-Mail, a br/beads MCP bridge, a Notion/Drive
  connector, or any home-built MCP.
- **`codex plugin`** — install plugins from a configured **marketplace snapshot**
  (local path or Git repo). This is the path to ship a bundle (skills, prompts,
  hooks) to Codex the same way a Claude plugin ships to Claude Code.

Use this skill when standing up a Codex lane in the flywheel, when a new MCP
server has just been built and a Codex worker must reach it, or when distributing
the AgentOps bundle to Codex via a marketplace.

This is **operator-side**: it drives the flywheel binaries and harness. Never
surface these commands or this framing in client-facing AI Partner content.

## ⚠️ Critical Constraints

- **Verify the binary first.** Run `codex mcp --help` and `codex plugin --help`
  before acting. **Why:** the subcommand surface changes between Codex releases;
  acting on a remembered shape silently writes a malformed `config.toml`.
- **stdio vs HTTP is mutually exclusive.** `codex mcp add <NAME> -- <COMMAND>...`
  is a stdio server; `codex mcp add <NAME> --url <URL>` is streamable HTTP. `--env`
  and a bare command are stdio-only; `--bearer-token-env-var` is HTTP-only.
  **Why:** mixing them is rejected and a half-written entry breaks every later `codex` run.
- **Never inline secrets.** Pass tokens by env-var reference (`--env KEY=$VAR` is
  the value at write-time; for HTTP use `--bearer-token-env-var ENV_VAR` so the
  token is read at launch, not stored). **Why:** `config.toml` is plaintext and
  often synced/committed — an inlined token leaks across the fleet.
- **Plugins require a marketplace first.** `codex plugin add` resolves against a
  *configured marketplace snapshot* — run `codex plugin marketplace add <SOURCE>`
  before `codex plugin add <PLUGIN>@<MARKETPLACE>`. **Why:** `add` with no
  marketplace fails; the snapshot is the resolution root.
- **Idempotency.** Check `codex mcp get <NAME>` / `codex plugin list` before
  adding. **Why:** re-adding an existing name churns config and can clobber a
  working entry; the flywheel expects these edits to be replayable.
- **OAuth is a separate step.** `codex mcp add` registers; `codex mcp login <NAME>`
  authenticates. **Why:** an OAuth-protected server is registered-but-dead until
  login completes, and the failure mode looks like a broken server.

## Workflow / Methodology

### Phase 1: Inventory what exists

```bash
codex mcp list --json        # configured MCP servers (machine-readable)
codex plugin marketplace list # configured marketplaces + their roots
codex plugin list --json      # installed plugins (add --available for uninstalled)
```

**Checkpoint:** Confirm the target name is NOT already present before adding. If
it is, decide repair (`remove` then re-`add`) vs. leave-as-is — do not blind-add.

### Phase 2: Wire an MCP server (the tool-reach path)

**stdio server** (local command — the common case for Agent-Mail / a br bridge):

```bash
codex mcp add agent-mail -- mcp-agent-mail            # bare command after --
codex mcp add beads-br --env BR_DB=/path/db -- br mcp # with env, stdio-only
```

**streamable HTTP server** (remote / hosted MCP):

```bash
codex mcp add notion --url https://mcp.example.com/sse \
  --bearer-token-env-var NOTION_TOKEN \
  --oauth-client-id <id> --oauth-resource <resource>
```

Inspect and (if OAuth) authenticate:

```bash
codex mcp get agent-mail --json   # confirm the written entry
codex mcp login notion            # OAuth flow; --scopes a,b,c if scoped
```

This is the **skill_mcp_dependency_install** motion: a skill that depends on an
MCP server declares it, and the operator runs `codex mcp add` (+ `login`) so the
dependency is reachable before the skill runs.

**Checkpoint:** `codex mcp list --json` shows the server; for OAuth servers,
`login` returned success. A registered-but-unauthenticated server is not done.

### Phase 3: Ship a plugin bundle (the distribution path)

Register the marketplace snapshot, then install from it — this is **plugin_sharing**:

```bash
# Local bundle (e.g. the staged skill bundle on disk):
codex plugin marketplace add /Users/bo/acfs/staged-skills

# Git-hosted bundle (owner/repo[@ref], HTTPS, or SSH):
codex plugin marketplace add owner/repo --ref main --sparse plugins/agentops

codex plugin list --available --json          # see what the snapshot offers
codex plugin add agentops@<marketplace-name>  # or: add agentops -m <marketplace>
```

Keep a Git marketplace fresh:

```bash
codex plugin marketplace upgrade              # all Git marketplaces
codex plugin marketplace upgrade <name>       # one
```

**Checkpoint:** `codex plugin list --json` shows the plugin installed and sourced
from the expected marketplace.

### Phase 4: Verify end-to-end

Run a Codex worker that exercises the wiring (a one-shot `codex exec` that calls
an MCP tool or uses a bundled skill) and confirm it reaches the server / loads the
plugin. The config is not "wired" until a worker actually uses it.

## Output Specification

**Format:** TOML config edits (applied by the `codex` binary, not hand-edited) +
machine-readable verification output.
**Filename / path:** `~/.codex/config.toml` (the binary owns the write; never
hand-edit MCP/plugin tables — let `codex mcp add` / `codex plugin add` do it).
**Structure:** `[mcp_servers.<name>]` tables for servers; configured-marketplace +
installed-plugin entries for plugins. Verification artifacts: `codex mcp list --json`
and `codex plugin list --json` output captured into the session/handoff.

## Quality Rubric

- [ ] Ran `codex mcp --help` / `codex plugin --help` before acting (no remembered shape)
- [ ] Chose stdio (`-- <cmd>`) vs HTTP (`--url`) correctly; did not mix them
- [ ] No secret inlined; tokens passed via `--env`/`--bearer-token-env-var`
- [ ] Idempotent: checked `get`/`list` before `add`; no blind re-add
- [ ] OAuth servers: `codex mcp login <NAME>` completed (not just registered)
- [ ] Plugins: marketplace added BEFORE `plugin add`; install verified in `plugin list`
- [ ] End-to-end verified by a real Codex worker using the server/plugin
- [ ] Backstage: no client-facing surface mentions these commands or the framing

## Examples

- **Reach Agent-Mail from a Codex flywheel worker:**
  `codex mcp add agent-mail -- mcp-agent-mail && codex mcp get agent-mail --json`
- **Reach br/beads via an MCP bridge:**
  `codex mcp add beads-br --env BR_DB=$HOME/.beads/db -- br mcp`
- **Ship the staged AgentOps bundle to Codex (local marketplace):**
  `codex plugin marketplace add /Users/bo/acfs/staged-skills && codex plugin add agentops@staged-skills`
- **OAuth-protected HTTP MCP:**
  `codex mcp add svc --url https://mcp.svc/sse --bearer-token-env-var SVC_TOKEN && codex mcp login svc`

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| `add` rejected with usage error | Mixed stdio + `--url`, or command not after `--` | Use `add <NAME> -- <cmd>...` (stdio) XOR `add <NAME> --url <URL>` (HTTP) |
| Server registered but tools never appear | OAuth server not authenticated | `codex mcp login <NAME>` (add `--scopes a,b` if scoped) |
| `--env` rejected | Used on an HTTP (`--url`) server | `--env` is stdio-only; for HTTP use `--bearer-token-env-var` |
| `plugin add` can't find plugin | No marketplace configured / wrong name | `codex plugin marketplace add <SOURCE>` first; then `PLUGIN@MARKETPLACE` or `-m` |
| Plugin stale after upstream change | Git marketplace snapshot not refreshed | `codex plugin marketplace upgrade [<name>]` |
| Entry exists / churn on re-add | Blind `add` over an existing name | `codex mcp get`/`codex plugin list` first; `remove` then `add` to repair |
| Token leaked in config | Secret inlined into the command | Remove, re-add with `--env KEY=$VAR` / `--bearer-token-env-var` |

## See Also / References

- `codex mcp --help`, `codex plugin --help`, `codex plugin marketplace --help` — the live, authoritative surface
- `agent-mail` skill — the multi-agent coordination MCP this most often wires
- `beads-br` skill — the br/beads tracker reached via an MCP bridge
- `caam` skill — per-lane Codex account profiles the flywheel runs these under
- `~/.codex/config.toml` — the backing config (binary-owned; do not hand-edit MCP/plugin tables)
