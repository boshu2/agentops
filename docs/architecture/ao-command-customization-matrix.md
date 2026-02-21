# AO Command Customization Matrix

This matrix tracks external command dependencies in the AO CLI and how each command group is customized.

Audit source:
- `scripts/audit-cli-command-deps.sh`

Customization tiers:
- `Tier A` (runtime-customizable): command path can be configured via `rpi.*_command` settings and matching env vars.
- `Tier B` (fixed system tools): command path stays fixed for safety/contract stability.
- `Tier C` (no external process): no `exec.Command*`/`exec.LookPath` dependency on the runtime path.

## Current Matrix

| Command Group | External Dependencies | Tier | Notes |
|---|---|---|---|
| `rpi phased` | `runtime`, `bd`, `ao` | Tier A (`runtime`, `bd`, `ao`) + Tier B (`git`, `bash`, `ps`) | Runtime + control-plane commands routed through shared RPI toolchain resolver. |
| `rpi loop --supervisor` | `git`, `bash`, `bd` | Tier A (`bd`) + Tier B (`git`, `bash`) | Landing/sync uses configurable `bd` command. |
| `rpi status` | `tmux` | Tier A (`tmux`) | Tmux liveness probe uses shared RPI toolchain resolver. |
| `rpi cancel` | `ps` | Tier B | Process tree inspection remains fixed for portability. |
| `rpi cleanup` | `git` | Tier B | Cleanup lifecycle remains fixed to git contracts. |
| `internal/rpi/worktree` | `git` | Tier B | Detached-head/worktree safety remains fixed to git contracts. |
| `context` | `tmux` | Tier B | Not yet migrated to shared customization layer. |
| `worktree` | `git`, `tmux` | Tier B | Not yet migrated to shared customization layer. |
| `search` | `rg`, `grep` | Tier B | Candidate for a follow-up customization wave. |
| `goals`/`ratchet` | `bash`, `git`, `bd` | Tier B | Candidate for follow-up after RPI path is stable. |
| `plans` | `bd` | Tier B | Candidate for follow-up after RPI path is stable. |
| `quick-start` | `bd` | Tier B | Candidate for follow-up after RPI path is stable. |
| `hooks` | `ao` | Tier B | Candidate for follow-up after RPI path is stable. |
| Other AO command groups | none on runtime path | Tier C | No external process invocation in steady-state execution path. |

## Policy Defaults

Runtime-focused customization defaults:
- configurable: `runtime`, `ao`, `bd`, `tmux` for RPI control plane.
- fixed: `git`, `bash`, `ps` unless a future adapter contract is introduced.

Configuration sources (highest to lowest):
1. command flags (where exposed)
2. environment variables
3. config file (`--config` override or default project/home lookup)
4. built-in defaults
