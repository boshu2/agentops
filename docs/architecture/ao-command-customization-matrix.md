# AO Command Customization Matrix

This matrix tracks external command dependencies in the AO CLI and how each command group is customized.

> The tracker command is `br` (beads_rust), invoked as `BEADS_DIR=$PWD/_beads br <cmd>`; `bd`/Dolt is retired (2026-06-11). The `rpi` rows describe the load-bearing-legacy RPI lane, not the live operating loop.

Audit source:
- `scripts/audit-cli-command-deps.sh`

Customization tiers:
- `Tier A` (runtime-customizable): command path can be configured via `rpi.*_command` settings and matching env vars.
- `Tier B` (fixed system tools): command path stays fixed for safety/contract stability.
- `Tier C` (no external process): no `exec.Command*`/`exec.LookPath` dependency on the runtime path.

## Current Matrix

| Command Group | External Dependencies | Tier | Notes |
|---|---|---|---|
| `rpi phased` | `runtime`, `br`, `ao` | Tier A (`runtime`, `br`, `ao`) + Tier B (`git`, `bash`, `ps`) | Runtime + control-plane commands routed through shared RPI toolchain resolver. |
| `rpi loop --supervisor` | `git`, `bash`, `br` | Tier A (`br`) + Tier B (`git`, `bash`) | Landing/sync uses configurable `br` tracker command. |
| `rpi status` | `tmux` | Tier A (`tmux`) | Tmux liveness probe uses shared RPI toolchain resolver. |
| `rpi cancel` | `ps` | Tier B | Process tree inspection remains fixed for portability. |
| `rpi cleanup` | `git` | Tier B | Cleanup lifecycle remains fixed to git contracts. |
| `internal/rpi/worktree` | `git` | Tier B | Detached-head/worktree safety remains fixed to git contracts. |
| `context` | `tmux` | Tier B | Not yet migrated to shared customization layer. |
| `worktree` | `git`, `tmux` | Tier B | Not yet migrated to shared customization layer. |
| `search` | `cass`, `rg`, `grep` | Tier B | Brokers to upstream `cass` for session history and keeps fixed repo-local fallback helpers. |
| `goals`/`ratchet` | `bash`, `git`, `br` | Tier B | Candidate for follow-up after RPI path is stable. |
| `plans` | `br` | Tier B | Candidate for follow-up after RPI path is stable. |
| `quick-start` | `br` | Tier B | Candidate for follow-up after RPI path is stable. |
| `hooks` | `ao` | Tier B | Candidate for follow-up after RPI path is stable. |
| Other AO command groups | none on runtime path | Tier C | No external process invocation in steady-state execution path. |

## Policy Defaults

Runtime-focused customization defaults:
- configurable: `runtime`, `ao`, `br`, `tmux` for RPI control plane.
- fixed: `git`, `bash`, `ps` unless a future adapter contract is introduced.

Configuration sources (highest to lowest):
1. command flags (where exposed)
2. environment variables
3. config file (`--config` override or default project/home lookup)
4. built-in defaults
