# Hooks for Automatic Knowledge Flywheel

The ao CLI integrates with runtime hooks to automate the CASS (Contextual Agent Session Search) knowledge flywheel.

## Runtime Modes

| Runtime | Hook Mechanism | Install |
|---------|---------------|---------|
| **Claude Code** | Full hook manifest via `~/.claude/settings.json` | `ao hooks install` |
| **Codex** | Skill-driven lifecycle (`ao codex start/stop`) | `scripts/install-codex-plugin.sh` |
| **Manual** | Explicit commands (`ao inject`, `ao forge`) | N/A |

See `docs/contracts/hook-runtime-contract.md` for the full event mapping.

The rest of this document covers the **Claude Code** hook configuration.

## Quick Start

```bash
# Install ao hooks to Claude Code (full 12-event coverage by default)
ao hooks install

# Verify installation
ao hooks test

# View current configuration
ao hooks show
```

## What Gets Automated

### SessionStart Hook

When you start a Claude Code session, behavior depends on `AGENTOPS_STARTUP_CONTEXT_MODE`:

**`manual` (default):** MEMORY.md is auto-loaded by Claude Code. The hook emits only a pointer to on-demand retrieval commands (`ao search`, `ao lookup`). No `ao extract` or `ao inject` runs. This is the lightest startup path.

**`lean`:** Runs `ao extract` + `ao lookup` with a reduced token budget (400 tokens when MEMORY.md is fresh). Provides automatic knowledge retrieval alongside MEMORY.md. Use `AGENTOPS_STARTUP_LEGACY_INJECT=1` to force this mode.

**`legacy`:** Runs `ao extract` + `ao lookup` with full token budget (800 tokens). Pre-notebook behavior for backward compatibility.

In `lean`/`legacy` modes, injection is weighted by:
- **Freshness**: More recent = higher score
- **Utility**: Learnings that led to successful outcomes score higher
- **Maturity**: Established learnings weighted over provisional ones

### SessionEnd Hook

When a Claude Code session ends:

1. **Learning extraction** via `ao forge transcript --last-session`
2. **Notebook update** via `ao notebook update` (updates Claude Code MEMORY.md)
3. **Repo-root sync** via `ao memory sync` (opt-in, set `AGENTOPS_MEMORY_SYNC=1`)
4. **Maturity maintenance** applies maturity scoring and eviction

### Stop Hook

When your session stops:

1. **Flywheel close** via `ao flywheel close-loop`

### CPU Safety Guardrails

`ao hooks install --full` now installs bounded hook commands by default:

- Inline `ao` hook commands include `AGENTOPS_HOOKS_DISABLED` guard checks.
- All inline `ao` hook commands have explicit per-hook `timeout` values.
- SessionEnd heavy maintenance is serialized with a cross-process lock (`session-end-heavy.lock`).
- Session-end `ao batch-feedback` runs with bounded defaults:
  - `--days ${AGENTOPS_BATCH_FEEDBACK_DAYS:-2}`
  - `--max-sessions ${AGENTOPS_BATCH_FEEDBACK_MAX_SESSIONS:-3}`
  - `--max-runtime ${AGENTOPS_BATCH_FEEDBACK_MAX_RUNTIME:-8s}`
  - `--reward ${AGENTOPS_BATCH_FEEDBACK_REWARD:-0.70}`

Override those defaults via environment variables when needed.

## The Knowledge Flywheel Equation

```
dK/dt = I(t) - δ·K + σ·ρ·K - B(K, K_crit)

Where:
- I(t) = injection rate (new learnings per session)
- δ = decay rate (0.17/week, literature default)
- σ = selection coefficient (which learnings get used)
- ρ = reproduction rate (how often patterns spawn variants)
- K_crit = critical mass for self-sustaining growth
```

**Escape velocity**: When σ·ρ > δ, knowledge compounds rather than decays.

## Commands

### `ao hooks init`

Generate hooks configuration without installing.

```bash
# Output as JSON (for manual editing)
ao hooks init

# Output as shell commands (for debugging)
ao hooks init --format shell
```

### `ao hooks install`

Install ao hooks to `~/.claude/settings.json`.

```bash
# Install full coverage (creates backup automatically)
ao hooks install

# Preview changes without modifying
ao hooks install --dry-run

# Overwrite existing ao hooks explicitly
ao hooks install --force

# Optional: install lightweight mode (SessionStart + SessionEnd + Stop)
ao hooks install --minimal
```

### `ao hooks show`

Display current hooks configuration (Claude Code runtime).

```bash
ao hooks show
```

### `ao hooks test`

Verify hooks are working correctly.

```bash
# Full test
ao hooks test

# Skip actual command execution
ao hooks test --dry-run
```

## Manual Configuration

If you prefer manual setup, add this to `~/.claude/settings.json`.
Note: this is a minimal example. `ao hooks install` is recommended for full coverage.

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "",
        "command": ["bash", "-c", "${CLAUDE_PLUGIN_ROOT}/hooks/session-start.sh"]
      }
    ],
    "SessionEnd": [
      {
        "matcher": "",
        "command": ["bash", "-c", "${CLAUDE_PLUGIN_ROOT}/hooks/session-end-maintenance.sh"]
      }
    ],
    "Stop": [
      {
        "matcher": "",
        "command": ["bash", "-c", "${CLAUDE_PLUGIN_ROOT}/hooks/ao-flywheel-close.sh"]
      }
    ]
  }
}
```

## Customization

### Startup Context Mode

Control what happens at session start via environment variable:

```bash
# Default — extract + lookup with reduced budget (lean retrieval alongside MEMORY.md)
AGENTOPS_STARTUP_CONTEXT_MODE=lean claude

# MEMORY.md auto-loaded, no extract/lookup (lightest)
AGENTOPS_STARTUP_CONTEXT_MODE=manual claude

# Full extract + lookup (pre-notebook backward compatibility)
AGENTOPS_STARTUP_CONTEXT_MODE=legacy claude
```

### On-Demand Knowledge Retrieval

In `manual` mode, use CLI commands for on-demand knowledge retrieval:

```bash
ao search "authentication"     # Search knowledge by keyword
ao lookup --query "auth flow"  # Relevance-ranked lookup
```

## Troubleshooting

### ao not found in PATH

Ensure the ao binary is in your PATH:

```bash
# Check where ao is installed
which ao

# Add to PATH in ~/.zshrc or ~/.bashrc
export PATH="$HOME/go/bin:$PATH"
```

### No knowledge being retrieved

1. Check if `.agents/learnings/` exists and has content:
   ```bash
   ls -la .agents/learnings/
   ```

2. Verify lookup works manually:
   ```bash
   ao lookup --query "test" --verbose
   ```

3. Check for parse errors:
   ```bash
   ao lookup --query "test" 2>&1 | head -20
   ```

### Hooks not running

1. Verify hooks are in settings:
   ```bash
   ao hooks show
   ```

2. Check Claude Code recognizes them:
   ```bash
   cat ~/.claude/settings.json | jq '.hooks'
   ```

3. Test hooks manually:
   ```bash
   ao hooks init --format shell | bash
   ```

## The Science

The knowledge flywheel is based on research in:

- **Knowledge decay**: Darr et al. (2002) - 17%/week depreciation rate
- **Memory Reinforcement Learning**: Lewis et al. (2023) - utility-weighted retrieval
- **Two-Phase Retrieval**: freshness + learned utility scoring

For deep dive: see `docs/the-science.md` in the repository.

## Related Commands

| Command | Purpose |
|---------|---------|
| `ao lookup` | On-demand knowledge retrieval |
| `ao forge transcript` | Extract learnings from transcripts |
| `ao task-sync` | Sync Claude Code tasks to CASS |
| `ao feedback-loop` | Update utility scores |
| `ao metrics report` | View flywheel health |
