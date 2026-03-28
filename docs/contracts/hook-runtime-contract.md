# Hook Runtime Contract

> Canonical reference for AgentOps hook behavior across runtimes.
> Consumed by: `ao hook install`, `ao hook validate`, install scripts, pre-push gate.

## Runtime Detection

| Runtime | Detection | Hook Surface |
|---------|-----------|-------------|
| Claude Code | `CLAUDE_PLUGIN_ROOT` env or `~/.claude/settings.json` exists | Full hook manifest (`hooks.json`) |
| Codex | `CODEX_HOME` env or `~/.codex/config.toml` exists | Skill-driven lifecycle (`ao codex start/stop`) |
| Manual | Neither detected | Explicit `ao inject` / `ao forge` commands |

## Event Mapping

### Supported (native equivalent exists)

| Claude Event | Codex Equivalent | Mechanism |
|-------------|-----------------|-----------|
| `SessionStart` | `ao codex start` / `ao codex ensure-start` | Explicit command in skill preamble or user invocation |
| `SessionEnd` | `ao codex stop` / `ao codex ensure-stop` | Explicit command in skill epilogue or user invocation |
| `Stop` | `ao codex stop --close-loop` | Explicit flywheel close via command |

### Adapted (behavior preserved, different mechanism)

| Claude Event | Codex Adaptation | Notes |
|-------------|-----------------|-------|
| `UserPromptSubmit` | Skill preamble checks | Prompt nudges and intent echo run as inline skill logic, not hook callbacks |
| `TaskCompleted` | Skill epilogue validation | Task validation gates run inline after task completion |

### Unsupported (no Codex equivalent)

| Claude Event | Codex Status | Fallback |
|-------------|-------------|----------|
| `PreToolUse` | No tool-intercept surface | Validation moves to skill-level checks or pre-commit gates |
| `PostToolUse` | No tool-intercept surface | Quality checks move to skill-level or post-edit inline checks |

## Install Behavior by Runtime

### Claude Code

```bash
ao hook install  # Merges hooks.json into ~/.claude/settings.json
```

- Full event manifest from `hooks/hooks.json`
- `${CLAUDE_PLUGIN_ROOT}` path resolution
- All events wired automatically

### Codex

```bash
# Plugin install (handled by install-codex-plugin.sh):
# 1. Enable plugin in ~/.codex/config.toml
# 2. Stage skills-codex/ bundle
# 3. No hook manifest — lifecycle is skill-driven
```

- Skills invoke `ao codex ensure-start` at entry
- Skills invoke `ao codex ensure-stop` at exit
- Pre/post tool checks embedded in skill workflow steps
- Flywheel close-loop via explicit `ao codex stop`

### Manual

```bash
ao inject          # Manual startup context
ao forge transcript  # Manual transcript extraction
ao flywheel close-loop  # Manual maintenance
```

## Validation

```bash
# Verify runtime detection
ao hook validate --runtime auto

# Verify Claude hooks installed
ao hook validate --runtime claude

# Verify Codex lifecycle commands work
ao codex status
```

## Contract Rules

1. **No silent parity claims.** If an event has no Codex equivalent, document it as unsupported rather than mapping to a no-op.
2. **Fail open.** Missing hook support must not break the skill workflow — degrade gracefully to manual commands.
3. **Single source of truth.** This contract is the canonical reference. Hook manifests, install scripts, and docs must align with this mapping.
4. **Sync requirement.** Changes to this contract require updates to:
   - `hooks/hooks.json` (Claude manifest)
   - `scripts/install-codex-plugin.sh` (Codex installer)
   - `cli/cmd/ao/hooks.go` (CLI tooling)
   - `docs/architecture/codex-hookless-lifecycle.md` (architecture docs)
