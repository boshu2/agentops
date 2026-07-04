# Dependencies

> The complete tool-dependency declaration for AgentOps. This is the **canonical detail** — the README "Requirements" section and the [shared skill fallback table](https://github.com/boshu2/agentops/blob/main/skills/shared/SKILL.md) summarize it.

AgentOps is designed to degrade gracefully. Almost everything is optional: skills check for a tool before using it and fall back when it is absent (the contract is in [`skills/shared/SKILL.md`](https://github.com/boshu2/agentops/blob/main/skills/shared/SKILL.md) "CLI Availability Pattern"). Only an agent runtime plus `git` is genuinely required to get value.

## Classification

| Tool | Class | Purpose | Required? | Fallback if absent |
|------|-------|---------|-----------|--------------------|
| **agent runtime** (`claude` / `codex` / `opencode`) | REQUIRED | The coding harness AgentOps sits on top of. At least one is needed — AgentOps adds bookkeeping, gates, and a corpus to it. | **Required** (one of) | None — AgentOps has nothing to drive without a runtime. The installer warns and points at the install links. |
| `git` | REQUIRED | Version control; `.agents/` state lives next to your code, worktrees isolate parallel work, provenance ties artifacts to commits. | **Required** | None — the SDLC control plane assumes a git repo. |
| `ao` | REQUIRED (recommended) | The AgentOps CLI: repo-native bookkeeping, retrieval (`ao inject`/`ao lookup`), health (`ao doctor`), the operating loop, validation gates. | Strongly recommended | Skills still guide the workflow, but the knowledge flywheel, gates, and loops that need the binary are unavailable. Write learnings to `.agents/learnings/` by hand. |
| `br` + `bv` (beads_rust) | TRACKING | Offline, git-JSONL-backed issue tracking (`_beads/issues.jsonl`) plus graph-aware triage. The mandatory task-tracking surface for non-trivial work. | Required for tracked work in this repo | Use the harness task list / plain markdown only for trivial or untracked work. Install/use `br` for persistent issue tracking. |
| legacy `bd` / Dolt tracker | HISTORICAL | Retired AgentOps tracker backend retained only for migration records and historical runbooks. | Not required | Do not use for current AgentOps work; route through `BEADS_DIR="$(ao beads dir)" br ...`. |
| out-of-session substrate (`ntm` / `ao agent` / MCP) | ORCHESTRATION | Runs whole operating-loop sessions out of session — an **NTM** tmux swarm, **managed-agents** via `ao agent`, or the **MCP** tool surface (`ao mcp serve`). AgentOps owns none of it; it adopts a substrate, the way it adopts `br`. | Optional (out-of-session only) | Run the loop in-session yourself (`/rpi`, `/evolve`). The substrate adds only always-on orchestration. |
| `mcp_agent_mail` | ORCHESTRATION | Multi-agent coordination — inboxes and advisory file leases so parallel lanes don't collide on the same files (the `agent-mail` skill wraps it). | Optional (multi-lane only) | Single-lane work needs no coordination bus. |
| `gh` (GitHub CLI) | PR / CI | Open and manage PRs, query CI status, drive the ship/merge flow. | Optional | Open PRs through the web UI; skip automated PR/merge steps. |
| `go` | BUILD-FROM-SOURCE | Toolchain to build `cli/bin/ao` from source (`go 1.26`, per `cli/go.mod`). | Optional | Install a prebuilt `ao` via Homebrew, the install script, or release binaries — no Go needed. |
| `jq` | UTILITY | Parse `--json` output from `ao`, `br`, and `gh` in scripts and dispatch loops. | Optional | Read JSON manually or use non-JSON output modes; some script automations are unavailable. |
| `rg` (ripgrep) | UTILITY | Fast code/corpus search used by research and several scripts. | Optional | Falls back to `grep`/`git grep`; slower but functional. |
| `curl` | UTILITY | Fetch the installer and release assets during install. | Required only for curl-pipe install | Download release binaries manually or install via Homebrew. |
| `openssl` | UTILITY | Hashing/randomness in some scripts (e.g. `openssl dgst`); paired with the sha256 tools. | Optional | `sha256sum`/`shasum` cover the hashing path; most flows do not need openssl. |
| `sha256sum` / `shasum -a 256` | UTILITY | Verify download integrity and compute content hashes (codex artifact hashes, install checksums). | Optional | Either tool satisfies the need; scripts detect whichever is present. |
| `tmux` | UTILITY | Session multiplexing for streamed/long-running RPI runs and an NTM agent swarm's sessions. | Optional | RPI runs in non-tmux modes; a managed-agents substrate needs no local tmux. |
| `cass` | UTILITY | Session-history search (`ao search` upstream backend). | Optional | Skip transcript search. Note: "install cass for session history." |
| `cm` (cass_memory_system) | UTILITY | Procedural memory for agents — durable playbooks and mistake-guards; pairs with `cass`. | Optional | Lessons stay in `.agents/` markdown, carried by hand between sessions. |
| `ubs` (ultimate_bug_scanner) | UTILITY | Deterministic static bug scanning that feeds the review/validation gate before you ship. | Optional | Skill-driven review only; no deterministic pre-ship bug scan. |
| `ACFS` (agentic_coding_flywheel_setup) | BOOTSTRAP | One-command environment that installs the recommended toolchain above (`br`/`bv`, `ntm`, `cass`, `cm`, `ubs`, agent mail, `dcg`). | Optional | Install each tool individually. |
| `awk`, `bash` | UTILITY | Shell plumbing the scripts and `ao doctor` checks rely on. | Effectively always present | POSIX baseline; present on every supported platform. |

## Notes

- **Health check.** `ao doctor` probes the tools it depends on (`ao`, `awk`, `bash`, `br`, `cass`, `git`, `tmux`) and reports what is missing without failing the workflow.
- **Install helpers.** `scripts/install.sh` detects package managers (`brew`, `apt`/`apt-get`, `dnf`/`yum`, `pacman`, `zypper`) and runtimes (`claude`, `codex`, `opencode`) and adapt; nothing here is hard-required beyond `curl` for the curl-pipe path.
- **Out-of-session vs in-session.** The only orchestration dependency is an out-of-session substrate (NTM / managed-agents / MCP), and it is strictly out-of-session and optional. In-session, an agent runtime + `git` (+ `ao`/`br` recommended) is the whole stack. See [docs/3.0.md](https://github.com/boshu2/agentops/blob/main/docs/3.0.md).
- **Graceful degradation is a contract, not a courtesy.** Every skill that shells out to an optional tool must check availability first and inform the user what was skipped — see the [shared fallback table](https://github.com/boshu2/agentops/blob/main/skills/shared/SKILL.md).
- **Migrating off a retired surface** (bd, hooks, the daemon, the removed `rpi`/`evolve` verbs, `ao recall`)? The living map is [MIGRATION.md](MIGRATION.md).
