# Dependencies

> The complete tool-dependency declaration for AgentOps. This is the **canonical detail** — the README "Requirements" section and the [shared skill fallback table](https://github.com/boshu2/agentops/blob/main/skills/shared/SKILL.md) summarize it.

AgentOps is designed to degrade gracefully. Almost everything is optional: skills check for a tool before using it and fall back when it is absent (the contract is in [`skills/shared/SKILL.md`](https://github.com/boshu2/agentops/blob/main/skills/shared/SKILL.md) "CLI Availability Pattern"). Only an agent runtime plus `git` is genuinely required to get value.

## Classification

| Tool | Class | Purpose | Required? | Fallback if absent |
|------|-------|---------|-----------|--------------------|
| **agent runtime** (`claude` / `codex` / `opencode`) | REQUIRED | The coding harness AgentOps sits on top of. At least one is needed — AgentOps adds bookkeeping, gates, and a corpus to it. | **Required** (one of) | None — AgentOps has nothing to drive without a runtime. The installer warns and points at the install links. |
| `git` | REQUIRED | Version control; `.agents/` state lives next to your code, worktrees isolate parallel work, provenance ties artifacts to commits. | **Required** | None — the SDLC control plane assumes a git repo. |
| `ao` | REQUIRED (recommended) | The AgentOps CLI: repo-native bookkeeping, retrieval (`ao inject`/`ao lookup`), health (`ao doctor`), session bootstrap/profile support, provenance capture, and validation gates. | Strongly recommended | Skills still guide the workflow, but the knowledge flywheel and gates that need the binary are unavailable. Write learnings to `.agents/learnings/` by hand. |
| `bd` (beads) | TRACKING | Dependency-aware, git-native issue tracker. The mandatory task-tracking surface (no markdown TODOs). | Required for tracked work | Use the harness task list / plain markdown. Note: "install bd for persistent issue tracking." |
| Dolt (bd backend) | TRACKING | Version-controlled SQL backend `bd` syncs through; enables cross-host issue sharing and native history. | Optional (bd ships a backend) | bd works against its default local store; Dolt server mode is only needed for shared/remote trackers. |
| NTM background-agent substrate (`ntm` + `mcp-agent-mail` + MCP) | ORCHESTRATION | Keeps Claude and Codex agents ready in tmux sessions. NTM supervises panes/processes; mcp-agent-mail coordinates inboxes, reservations, and handoffs; MCP (`ao mcp serve`) exposes tools. Workers run skill-guided sessions, not `ao rpi`/`ao evolve` wrappers. | Optional (out-of-session only) | Start a normal interactive session and use the relevant skills yourself. |
| `gh` (GitHub CLI) | PR / CI | Open and manage PRs, query CI status, drive the ship/merge flow. | Optional | Open PRs through the web UI; skip automated PR/merge steps. |
| `go` | BUILD-FROM-SOURCE | Toolchain to build `cli/bin/ao` from source (`go 1.26`, per `cli/go.mod`). | Optional | Install a prebuilt `ao` via Homebrew, the install script, or release binaries — no Go needed. |
| `jq` | UTILITY | Parse `--json` output from `ao`, `bd`, and `gh` in scripts and dispatch loops. | Optional | Read JSON manually or use non-JSON output modes; some script automations are unavailable. |
| `rg` (ripgrep) | UTILITY | Fast code/corpus search used by research and several scripts. | Optional | Falls back to `grep`/`git grep`; slower but functional. |
| `curl` | UTILITY | Fetch the installer and release assets during install. | Required only for curl-pipe install | Download release binaries manually or install via Homebrew. |
| `openssl` | UTILITY | Hashing/randomness in some scripts (e.g. `openssl dgst`); paired with the sha256 tools. | Optional | `sha256sum`/`shasum` cover the hashing path; most flows do not need openssl. |
| `sha256sum` / `shasum -a 256` | UTILITY | Verify download integrity and compute content hashes (codex artifact hashes, install checksums). | Optional | Either tool satisfies the need; scripts detect whichever is present. |
| `tmux` | UTILITY | Session multiplexing for long-running Claude/Codex background agents under NTM. | Optional | Background swarms are unavailable; use interactive skill sessions instead. |
| `cass` | UTILITY | Session-history search (`ao search` upstream backend). | Optional | Skip transcript search. Note: "install cass for session history." |
| `awk`, `bash` | UTILITY | Shell plumbing the scripts and `ao doctor` checks rely on. | Effectively always present | POSIX baseline; present on every supported platform. |

## Notes

- **Health check.** `ao doctor` probes the tools it depends on (`ao`, `awk`, `bash`, `bd`, `cass`, `git`, `tmux`) and reports what is missing without failing the workflow.
- **Install helpers.** `scripts/install.sh` and `scripts/install-bd.sh` detect package managers (`brew`, `apt`/`apt-get`, `dnf`/`yum`, `pacman`, `zypper`) and runtimes (`claude`, `codex`, `opencode`) and adapt; nothing here is hard-required beyond `curl` for the curl-pipe path.
- **Out-of-session vs in-session.** The only orchestration dependency is the NTM background-agent substrate, and it is strictly out-of-session and optional. In-session, an agent runtime + `git` (+ `ao`/`bd` recommended) is the whole stack. Background agents are just long-lived Claude/Codex skill sessions supervised by NTM and coordinated through mcp-agent-mail. See [docs/3.0.md](https://github.com/boshu2/agentops/blob/main/docs/3.0.md).
- **Graceful degradation is a contract, not a courtesy.** Every skill that shells out to an optional tool must check availability first and inform the user what was skipped — see the [shared fallback table](https://github.com/boshu2/agentops/blob/main/skills/shared/SKILL.md).
