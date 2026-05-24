# Environment Variables

All optional. AgentOps works out of the box with no configuration.

## Council / Validation

These control `/council`, `/vibe`, `/pre-mortem`, and `/post-mortem` behavior.

| Variable | Default | Description |
|----------|---------|-------------|
| `COUNCIL_TIMEOUT` | `120` | Maximum time (seconds) for each judge to complete one round. If a judge times out, the council proceeds with remaining judges and notes it in the report. |
| `COUNCIL_CLAUDE_MODEL` | `sonnet` | Claude model for judges. Use `opus` for high-stakes reviews (security audits, architecture decisions). Overrides `--profile` flag. |
| `COUNCIL_CODEX_MODEL` | (user's Codex default) | Override Codex model for `--mixed` mode. When unset, `codex exec` uses whatever model the user has configured as their default. |
| `COUNCIL_EXPLORER_MODEL` | `sonnet` | Model for explorer sub-agents spawned by `--explorers=N`. Explorers do parallel deep-dive research before judges assess. |
| `COUNCIL_EXPLORER_TIMEOUT` | `60` | Maximum time (seconds) for each explorer sub-agent. Shorter than judge timeout since explorers do focused searches. |
| `COUNCIL_R2_TIMEOUT` | `90` | Maximum time (seconds) for debate round 2 (`--debate`). Shorter than R1 since judges already have their R1 analysis in context. |

### Model Profiles

The `--profile` flag sets `COUNCIL_CLAUDE_MODEL`, judge count, and timeout as a bundle:

| Profile | Model | Judges | Timeout | Use case |
|---------|-------|--------|---------|----------|
| `thorough` | opus | 3 | 120s | Security audits, architecture decisions |
| `balanced` | sonnet | 2 | 120s | Default — general validation |
| `fast` | haiku | 2 | 60s | Quick checks, mid-implementation sanity |

Explicit env vars override profiles: `COUNCIL_CLAUDE_MODEL=opus` beats `--profile=fast`.

## MemRL Policy

These control deterministic MemRL policy evaluation in retry/escalation paths.

| Variable | Default | Description |
|----------|---------|-------------|
| `MEMRL_MODE` | `off` | MemRL policy mode: `off` (strict legacy parity), `observe` (evaluate + audit without enforcement), `enforce` (evaluate + enforce `retry|escalate` decision). |

## CLI / RPI Toolchain

These control AO CLI configuration loading and RPI control-plane command customization.

| Variable | Default | Description |
|----------|---------|-------------|
| `AGENTOPS_CONFIG` | unset | Explicit config file path for AO CLI. When set, this path is used instead of the default project config location (`.agentops/config.yaml`). |
| `AGENTOPS_RPI_WORKTREE_MODE` | `auto` | Worktree policy for phased runs: `auto`, `always`, `never`. |
| `AGENTOPS_RPI_RUNTIME` | `auto` | Legacy alias for runtime mode (`auto`, `direct`, `stream`). |
| `AGENTOPS_RPI_RUNTIME_MODE` | `auto` | Preferred runtime mode variable (`auto`, `direct`, `stream`). Overrides `AGENTOPS_RPI_RUNTIME` when both are set. |
| `AGENTOPS_RPI_RUNTIME_COMMAND` | `claude` | Runtime command used for phase prompt execution. |
| `AGENTOPS_RPI_AO_COMMAND` | `ao` | `ao` command used for ratchet/checkpoint operations in RPI control plane. |
| `AGENTOPS_RPI_BD_COMMAND` | `bd` | `bd` command used for epic and child issue queries in RPI control plane. |
| `AGENTOPS_RPI_TMUX_COMMAND` | `tmux` | `tmux` command used for status liveness probes in RPI control plane. |
| `AGENTOPS_DREAM_REPORT_DIR` | `.agents/overnight/latest` | Default output directory for private overnight Dream reports. |
| `AGENTOPS_DREAM_RUN_TIMEOUT` | `8h` | Default maximum duration for a Dream run. |
| `AGENTOPS_DREAM_KEEP_AWAKE` | `true` | Default keep-awake behavior for Dream runs. Accepts `true/false`, `1/0`, `yes/no`, `on/off`. |
| `RPI_RUN_ID` | (unset) | When set, `ao lookup --for=<skill>` uses this as the context artifact directory name instead of generating an `adhoc-<timestamp>` ID. Automatically set by the `/rpi` orchestrator during phased runs. |

## Hooks

These control the optional hook system installed via `ao init --hooks`. Each hook checks `AGENTOPS_HOOKS_DISABLED` first (global kill switch), then its own variable.

| Variable | Default | Description |
|----------|---------|-------------|
| `AGENTOPS_HOOKS_DISABLED` | `0` | Set to `1` to disable **all** hooks at once. Global kill switch. Useful for debugging or when hooks interfere with a specific workflow. |
| `AGENTOPS_SESSION_START_DISABLED` | `0` | Set to `1` to disable the session-start hook entirely. |
| `AGENTOPS_STARTUP_CONTEXT_MODE` | `factory` | Controls SessionStart behavior: `factory` (default — silently stage factory goal/briefing state when a goal is recoverable, and mark missing-goal sessions for prompt-time intake) or `manual` (baseline startup maintenance only). Legacy values `lean` and `legacy` are treated as `manual` for compatibility. |
| `AGENTOPS_STARTUP_LEGACY_INJECT` | (unset) | Compatibility alias that forces `manual` startup mode regardless of `AGENTOPS_STARTUP_CONTEXT_MODE`. Use only as a rollback for older startup expectations that relied on hidden hook-managed startup context. |
| `AGENTOPS_FACTORY_ROUTER_DISABLED` | `0` | Set to `1` to disable the `UserPromptSubmit` factory intake hook that captures the first substantive prompt into `.agents/ao/` state when startup had no goal. |
| `AGENTOPS_NEW_USER_WELCOME_DISABLED` | `0` | Set to `1` to disable the one-time `UserPromptSubmit` welcome that appears when AgentOps detects a brand-new repo with no existing `.agents/` state. |
| `AGENTOPS_MEMORY_SYNC` | `0` | Set to `1` to enable repo-root MEMORY.md sync at SessionEnd. Writes session history with managed block markers for cross-runtime access (Codex, OpenCode). |
| `AGENTOPS_EVICTION_DISABLED` | `0` | Set to `1` to disable knowledge eviction. Eviction removes stale learnings that have decayed below the retention threshold. Disable if you want to keep all learnings indefinitely. |
| `AGENTOPS_GITIGNORE_AUTO` | legacy | Legacy compatibility knob. Repo-root `.agents/` is always treated as local/private runtime state and must not be tracked. |
| `AGENTOPS_AUTO_PRUNE` | `1` | Set to `0` to disable automatic `.agents/` directory pruning at session end. Pruning removes stale lock files and empty directories to keep the workspace clean. |

### Usage Examples

```bash
# Repo-root .agents/ stays git-ignored; there is no commit opt-out.

# Use opus for a critical security review
COUNCIL_CLAUDE_MODEL=opus claude
# Then: /council --deep --preset=security-audit validate src/auth/

# Fast iteration — haiku judges, shorter timeout
COUNCIL_CLAUDE_MODEL=haiku COUNCIL_TIMEOUT=60 claude

# Keep all knowledge forever (disable eviction)
AGENTOPS_EVICTION_DISABLED=1 claude

# Disable all hooks temporarily
AGENTOPS_HOOKS_DISABLED=1 claude
```

### Precedence

For council model selection:
1. `COUNCIL_CLAUDE_MODEL` env var (highest priority)
2. `--profile=<name>` flag
3. Explicit `--count`/`--deep`/`--mixed` flags
4. Skill defaults (sonnet, 2 judges, 120s)

For hooks:
1. `AGENTOPS_HOOKS_DISABLED=1` disables everything (highest priority)
2. Individual `AGENTOPS_*_DISABLED=1` disables one hook
3. Default: all hooks enabled
