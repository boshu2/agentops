# Migration Guide

AgentOps is a validation gate that proves an agent's work is actually done, plus a provenance record of that proof kept in your own repo. It runs **in your session**, on your machine, against your own subscription — **hookless**: no runtime hooks to install, no daemon to run, no service to babysit.

This is the living map from every AgentOps surface that was removed or retired to what you use instead. If a command you relied on is gone, find its row below: it names the replacement and, where the old surface still exists behind a build tag, the exact command to bring it back. For 3.0-era detail see [MIGRATION-3.0.md](MIGRATION-3.0.md); for version-keyed breaking changes see [UPGRADING.md](UPGRADING.md).

## If you used X, use Y now

| You used | Use instead | Notes (why + restore path) |
|---|---|---|
| `bd` / Dolt tracker | `br` + `bv` — `BEADS_DIR="$(ao beads dir)" br <cmd>` for tracking, `bv --robot-insights` for triage | The old tracker was a single-host server with no offline lane. `br` is offline and git-JSONL-backed. Your old `.beads/` is preserved but non-authoritative. |
| Runtime hooks (the 2.x hook bundle) | Hookless skills + the `ao` CLI + the installed local pre-push gate (`scripts/install-pre-push-gate.sh`) | Hooks were deleted (ADR-0002) after an A/B showed the injected-context delta was zero. `--with-hooks` remains as an opt-in for anyone who still wants them. The gate — not a hook — is what enforces the bar before a push. |
| `agentopsd` daemon · `ao schedule` · `ao overnight` · `ao cron` | An external substrate you choose: **NTM** (a local tmux swarm) + `ao mcp serve` (MCP tool surface) + `ao agent` (managed agents) | AgentOps ships no always-on runtime (ADR-0009 — "delete, not deprecate"). Out-of-session scheduling belongs to whatever substrate you run. The `ao cron` shim was removed at `b242136ac` under ADR-0012. |
| `ao rpi` · `ao evolve` command surface | The in-session operating loop + the `/rpi` skill (one turn over the loop) | `ao rpi` was deleted outright at `f61c5f0e7` (3.2); `ao evolve` earlier (#724). No build tag brings the verbs back — what `AGENTOPS_LEGACY=1 make build` restores is the archived RPI/factory machinery in the next row (`ao loop`, `ao orchestrate`, `ao operator`, …), not these verbs. |
| `ao recall` · `ao memory ingest-claude` | `cass` (search your past agent sessions) + `cm` (procedural memory) | Removed at `9d5be0b9e` (3.2). AgentOps consumes these tools instead of shipping its own memory store. Session-log → provenance mining stays native (ADR-0010); only recall/memory moved out. |
| Corpus / flywheel commands (`corpus`, `curate`, `defrag`, `harvest`, `mind`, `refinery`) | `make build-flywheel` to restore them; consume knowledge via `cass` + `cm` | Archived behind `//go:build flywheel` (ADR-0012), not deleted — the code stays buildable because its revival conditions require it. A default (`spine`) build simply omits them. |
| RPI / factory commands (`autodev`, `codex`, `loop*`, `orchestrate*`, `operator*`, `tick`, `turn_verify`, `harness`) | The operating loop + an external substrate; restore the old commands with `AGENTOPS_LEGACY=1 make build` | Archived behind `//go:build legacy` (ADR-0012). Same reasoning as the flywheel set: focus the default surface, keep the satellites buildable. |
| In-repo Gas City substrate (`runtime=gc`, `city.toml`, `packs/agentops`) | The external Gas City project, if you want a managed workspace | The in-repo bridge was pruned (`ag-124p`, #679); ADR-0009 re-narrated the substrate to NTM + MCP + managed-agents. `runtime=gc` is no longer a selectable mode. An AgentOps pack for Gas City is planned, not shipped (see below). |
| PR-per-change flow | **push-to-main** + the local pre-push gate | Retired `ag-qidx` (2026-06-07). Branch protection is off; run the gate locally (`ao gate check --fast --scope head`) and push the coherent arc straight to `main`. `validate.yml` is a tag/PR/manual backstop, not routine authority. |
| `acfs` skill | The individual wrapper skills — `ntm`, `cass`, `beads-br`, `beads-bv`, `agent-mail`, `dcg` — plus the upstream ACFS installer directly | The umbrella skill was cut 2026-07-02 at `aae1a3af9` (skill catalog 70→63). The wrappers cover each tool; the installer bundles the whole stack (see next section). |
| Mount Olympus daemon | Parked — the in-repo pawl gate is the gate | Olympus was a high-assurance multi-tenant take that was parked, not shipped. The routine release authority is the local cockpit / pawl gate in this repo. |

## The stack we recommend (and use ourselves)

AgentOps consumes an open-source toolchain rather than reinventing it. All of the following are by Jeffrey Emanuel ([github.com/Dicklesworthstone](https://github.com/Dicklesworthstone)). The one-command way to get the whole set is ACFS; you can also install any piece on its own.

| Tool | What it does | Where it plugs into your workflow |
|---|---|---|
| **ACFS** — [agentic_coding_flywheel_setup](https://github.com/Dicklesworthstone/agentic_coding_flywheel_setup) | One command turns a fresh box into a full multi-agent dev environment, bundling every tool below | The fastest way to stand up the stack AgentOps expects; the environment we target for native support |
| **br + bv** — [beads_rust](https://github.com/Dicklesworthstone/beads_rust) | Local-first issue tracker (SQLite + git JSONL) with graph-aware triage | Your tracker — already the canonical one in this repo (`br` to track, `bv` to triage) |
| **NTM** — [ntm](https://github.com/Dicklesworthstone/ntm) | Named tmux manager for driving agent swarms | The out-of-session substrate: run whole AgentOps loops as background lanes |
| **MCP Agent Mail** — [mcp_agent_mail](https://github.com/Dicklesworthstone/mcp_agent_mail) | Inboxes, threads, and advisory file leases for a fleet of agents | Multi-lane coordination so parallel agents don't collide on the same files (the `agent-mail` skill wraps it) |
| **cass** — [coding_agent_session_search](https://github.com/Dicklesworthstone/coding_agent_session_search) | Searches your past agent sessions across harnesses | Find the prompt/decision that worked before; replaces `ao recall` |
| **cm** — [cass_memory_system](https://github.com/Dicklesworthstone/cass_memory_system) | Procedural memory — durable playbooks and mistake-guards | Carry lessons between sessions; replaces `ao memory` |
| **ubs** — [ultimate_bug_scanner](https://github.com/Dicklesworthstone/ultimate_bug_scanner) | Deterministic 1000+-pattern static bug scanner | A cheap deterministic producer feeding the validation gate before you ship |
| **dcg** — destructive-command guard | Blocks `rm -rf`, `git reset --hard`, `DROP DATABASE`, and similar footguns | A safety rail on every agent shell; ships with ACFS (`agentic_coding_flywheel_setup`) |

## Where this is headed

We support the ACFS environment today and track deeper native support as open work. Two harness projects are planned transition targets — an AgentOps **pack for Gas City** ([github.com/gastownhall/gascity](https://github.com/gastownhall/gascity)) and one for **omnigent** ([github.com/omnigent-ai/omnigent](https://github.com/omnigent-ai/omnigent)) — so their users get the same validation gate and provenance record without leaving their workspace. These are tracked as `age-trim-fat-migration-map-4cpk.6` (Gas City pack), `age-trim-fat-migration-map-4cpk.7` (omnigent pack), and `age-trim-fat-migration-map-4cpk.8` (deeper ACFS native support). To be explicit: these are **planned, not shipped** — each is blocked on upstream-contract research before any code lands. Nothing above claims a pack exists yet.

---

**See also:** [MIGRATION-3.0.md](MIGRATION-3.0.md) (what the 3.0 narrowing removed, in detail) · [UPGRADING.md](UPGRADING.md) (version-keyed breaking changes and migration steps) · [dependencies.md](dependencies.md) (the full required-vs-optional dependency classification).
