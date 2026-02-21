# Claude Code Hooks for Automatic Knowledge Flywheel

The ao CLI integrates with Claude Code's hooks system to automate the CASS (Contextual Agent Session Search) knowledge flywheel.

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

When you start a Claude Code session:

1. **Confidence decay** is applied to stale learnings (10%/week decay rate)
2. **Knowledge injection** delivers relevant context:
   - Recent learnings from `.agents/learnings/`
   - Active patterns from `.agents/patterns/`
   - Recent session summaries

The injection is weighted by:
- **Freshness**: More recent = higher score
- **Utility**: Learnings that led to successful outcomes score higher
- **Maturity**: Established learnings weighted over provisional ones

### Stop Hook

When your session ends:

1. **Learning extraction** from the completed session transcript
2. **Task sync** promotes completed tasks to higher maturity levels
3. **Feedback loop** updates utility scores based on session outcome

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

# Optional: install lightweight mode only (SessionStart + Stop)
ao hooks install --minimal
```

### `ao hooks show`

Display current Claude Code hooks configuration.

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
Note: this is a minimal example. `ao hooks install --full` is recommended for full 12-event coverage.

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "",
        "command": ["bash", "-c", "ao inject --apply-decay --max-tokens 1500 2>/dev/null || true"]
      }
    ],
    "Stop": [
      {
        "matcher": "",
        "command": ["bash", "-c", "ao forge transcript --last-session --quiet --queue 2>/dev/null; ao task-sync --promote 2>/dev/null || true"]
      }
    ]
  }
}
```

## Customization

### Token Budget

Adjust the SessionStart injection budget:

```json
{
  "command": ["bash", "-c", "ao inject --apply-decay --max-tokens 2000 2>/dev/null || true"]
}
```

### Context Query

Filter injected knowledge by topic:

```json
{
  "command": ["bash", "-c", "ao inject --apply-decay --context 'kubernetes' 2>/dev/null || true"]
}
```

### Disable Citation Tracking

Skip recording which learnings were retrieved:

```json
{
  "command": ["bash", "-c", "ao inject --apply-decay --no-cite 2>/dev/null || true"]
}
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

### No knowledge being injected

1. Check if `.agents/learnings/` exists and has content:
   ```bash
   ls -la .agents/learnings/
   ```

2. Verify inject works manually:
   ```bash
   ao inject --verbose
   ```

3. Check for parse errors:
   ```bash
   ao inject 2>&1 | head -20
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
| `ao inject` | Manually inject knowledge |
| `ao forge transcript` | Extract learnings from transcripts |
| `ao task-sync` | Sync Claude Code tasks to CASS |
| `ao feedback-loop` | Update utility scores |
| `ao metrics report` | View flywheel health |
