# Hook Runtime Contract

> Canonical reference for AgentOps hook behavior across runtimes.
> Consumed by: `ao hook install`, `ao hook validate`, install scripts, pre-push gate.

## Runtime Detection

| Runtime | Detection | Hook Surface |
|---------|-----------|-------------|
| Claude Code | `CLAUDE_PLUGIN_ROOT` env or `~/.claude/settings.json` exists | Full hook manifest (`hooks.json`) |
| Codex (native hooks) | `CODEX_HOME` env plus `~/.codex/version.json` / `config.toml` / `hooks.json` indicate native hooks are supported and enabled | Native hook manifest via `scripts/install-codex-plugin.sh` (`SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `Stop`) |
| Codex (hookless fallback) | `CODEX_HOME` env or `~/.codex/config.toml` exists but native hooks are unavailable or not configured | Explicit lifecycle (`ao codex start/stop`) |
| Manual | Neither detected | Explicit `ao inject` / `ao forge` commands |

## Event Mapping

| Capability | Claude/OpenCode hook-capable | Codex native hooks (v0.115.0+) | Codex hookless fallback |
|-----------|------------------------------|------------------------------|------------------------|
| `SessionStart` | Native `SessionStart` hook | Native `SessionStart` hook | `ao codex start` / `ao codex ensure-start` |
| `UserPromptSubmit` | Native `UserPromptSubmit` hook | Native `UserPromptSubmit` hook | Skill/runtime preambles |
| `PreToolUse` | Native `PreToolUse` hook | Native `PreToolUse` hook (currently Bash-only) | No runtime hook; validation moves inline or to pre-commit gates |
| `PostToolUse` | Native `PostToolUse` hook | Native `PostToolUse` hook (currently Bash-only) | No runtime hook; quality checks move inline or post-edit |
| `Stop` | Native `Stop` hook | Native `Stop` hook | `ao codex stop` / `ao codex ensure-stop` |
| `SessionEnd` | Native `SessionEnd` hook | No native `SessionEnd` event; transcript-driven closeout still uses `ao codex stop` / `ao codex ensure-stop` | `ao codex stop` / `ao codex ensure-stop` |
| `PermissionRequest` | Native `PermissionRequest` hook | Not wired in the current AgentOps Codex manifest | Approval handled explicitly by the runtime/workflow |
| `TaskCompleted` | Native `TaskCompleted` hook | No native event; task validation remains skill-driven | Skill epilogue validation |

## Install Behavior by Runtime

### Claude Code

```bash
ao hook install  # Merges hooks.json into ~/.claude/settings.json
```

- Full event manifest from `hooks/hooks.json`
- `${CLAUDE_PLUGIN_ROOT}` path resolution
- All events wired automatically
- `CLAUDE.md` remains the startup surface; hooks prepare factory state silently and keep only explicit safety nudges operator-facing

### Codex (v0.115.0+ — native hooks)

```bash
# Plugin install (handled by install-codex-plugin.sh):
# 1. Enable plugin in ~/.codex/config.toml
# 2. Stage skills-codex/ bundle
# 3. Install native hook manifest for the current Codex event set
```

- Native hook manifest installed from `hooks/codex-hooks.json`
- Current AgentOps Codex-native surface: `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, and `Stop`
- Transcript-driven closeout still uses `ao codex stop` / `ao codex ensure-stop` because Codex does not expose a native `SessionEnd` event today

### Codex (pre-v0.115.0 — hookless fallback)

```bash
# Legacy plugin install for older Codex versions:
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
