# Hooks

AgentOps ships a set of runtime hooks that wire skills, the `ao` CLI, and the knowledge flywheel into your coding agent. This page is an orientation: what each lifecycle event does, how to install or disable hooks, and where to go for deeper detail.

For the comprehensive technical reference — including CASS wiring, token budgets, environment variables, and runtime-specific install paths — see [`cli/docs/HOOKS.md`](https://github.com/boshu2/agentops/blob/main/cli/docs/HOOKS.md).

## Source of truth

- Hook manifest: [`hooks/hooks.json`](https://github.com/boshu2/agentops/blob/main/hooks/hooks.json) (validated against [`schemas/hooks-manifest.v1.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/hooks-manifest.v1.schema.json))
- Hook scripts: [`hooks/*.sh`](https://github.com/boshu2/agentops/tree/main/hooks) (shell scripts invoked by the runtime)
- Shared helpers: [`lib/hook-helpers.sh`](https://github.com/boshu2/agentops/blob/main/lib/hook-helpers.sh)
- Runtime contract: [`contracts/hook-runtime-contract.md`](contracts/hook-runtime-contract.md)

When `hooks.json` disagrees with this page, trust `hooks.json`.

## Lifecycle events

AgentOps currently ships a full Claude runtime manifest across the supported hook event surface. Hooks are for event-timed gates, lifecycle bookkeeping, and small JIT nudges; broad knowledge should be loaded with `ao lookup`, factory briefings, or explicit `/inject` flows instead of automatic startup injection.

| Event | Purpose | Representative scripts |
|-------|---------|-----------------------|
| `SessionStart` | Prepare runtime state, consume handoffs, stage factory briefing state | `session-start.sh` |
| `SessionEnd` | Compile session signal, maintain the knowledge pool | `session-end-maintenance.sh`, `compile-session-defrag.sh` |
| `Stop` | Preserve handoff/team state and close the flywheel for the turn | `stop-team-guard.sh`, `stop-auto-handoff.sh`, `ao-flywheel-close.sh` |
| `UserPromptSubmit` | Route explicit factory intake, watch context, and capture quality signals | `factory-router.sh`, `context-guard.sh`, `quality-signals.sh` |
| `PreToolUse` | Gate risky tool calls and inject compact file-scoped guidance | `pre-mortem-gate.sh`, `dangerous-git-guard.sh`, `commit-review-gate.sh`, `standards-injector.sh`, `holdout-isolation-gate.sh` |
| `PostToolUse` | Quality gate after edits | `go-complexity-precommit.sh` |
| `TaskCompleted` | Validate task closure metadata and structural checks | `task-validation-gate.sh` |
| `PreCompact` | Snapshot branch, changed files, and ratchet state before compaction | `precompact-snapshot.sh` |
| `SubagentStop` | Capture worker output for later recovery | `subagent-stop.sh` |
| `WorktreeCreate` | Initialize isolated worktree state | `worktree-setup.sh` |
| `WorktreeRemove` | Archive worktree-local state before deletion | `worktree-cleanup.sh` |
| `ConfigChange` | Audit or block high-risk runtime configuration changes | `config-change-monitor.sh` |

Codex uses the same hook scripts where its native event map can support them. Codex keeps startup lean as well: `hooks/codex-hooks.json` intentionally omits `ao-inject.sh`.

### Codex/Claude PreToolUse output parity

Codex CLI 0.128.0 accepts the same `PreToolUse` stdin schema as Claude Code (`tool_input.command`, `tool_input.file_path`, etc.), but only honors a subset of the response shape. Current AgentOps hooks emit only the portable subset.

| Output field | Claude Code | Codex CLI 0.128.0 |
|---|---|---|
| `decision: block` + `reason` | yes | yes |
| `hookSpecificOutput.additionalContext` | yes | yes |
| `hookSpecificOutput.updatedInput` (rewrite) | yes | NO (silently dropped, hook logged "PreToolUse Failed") |
| Exit code 2 (block) | yes | yes |

Transparent command rewriting via `updatedInput` is not available on Codex; opt-in nudges in `~/.codex/instructions.md` are the only path. The `scripts/test-hooks-output.sh` CI lint enforces this allow-list against every stdin-consuming hook.

## Install and uninstall

### Claude Code

```bash
ao hooks install       # writes ~/.claude/settings.json hook entries
ao hooks show          # prints current effective config
ao hooks test          # smoke-tests the hook wiring
ao hooks uninstall     # removes ao hook entries (other entries are preserved)
```

### Codex (v0.115.0+)

Codex installs hookless by default. Use `scripts/install-codex-plugin.sh --with-hooks`
or `scripts/install-codex.sh --with-hooks` only when you intentionally want the
native hook manifest in `~/.codex/hooks.json`.

### Codex (older)

No native hook support. Use the explicit fallback: `ao codex start` at the beginning of a session and `ao codex stop` at the end. See [`architecture/codex-hookless-lifecycle.md`](architecture/codex-hookless-lifecycle.md).

## Customizing behavior

Most hooks read environment variables rather than checking in-tree config. See [`ENV-VARS.md`](ENV-VARS.md) for the full list. Common knobs:

| Variable | Effect |
|----------|--------|
| `AGENTOPS_STARTUP_CONTEXT_MODE` | `factory` (default) stages goal-scoped briefing state; `manual` skips prompt-time factory intake |
| `AGENTOPS_STANDARDS_FULL_INJECT` | Set to `1` only for legacy debugging when you need full standards references injected by `standards-injector.sh` |
| `AGENTOPS_GITIGNORE_AUTO` | Legacy no-op for repo-root `.agents/`; local agent state must remain git-ignored |
| `AGENTOPS_HOOKS_DISABLED` | Set to `1` to short-circuit all hooks without uninstalling them |
| `AGENTOPS_QUIET` | Set to `1` to suppress non-error hook output |

To disable a single hook, either edit the runtime settings manually or delete the entry from your merged settings file (`ao hooks install` preserves unrelated entries on re-run). Do not edit `hooks/hooks.json` directly in an installed copy — edit the repo source and reinstall.

## Common failure modes

| Symptom | Likely cause | Fix |
|---------|-------------|-----|
| Hook runs but nothing happens | Agent runtime does not emit the event | Check [runtime contract](contracts/hook-runtime-contract.md) |
| Hook times out | Script exceeded `timeout` in manifest | Increase the timeout in `hooks.json` or make the script cheaper |
| `ao` not found | CLI not on `PATH` for the runtime's shell | `brew install agentops` or add `~/.local/bin` to PATH |
| Commit blocked by `commit-review-gate` | Uncommitted or unsigned review trail | Run `/council validate` or review manually |
| `pre-mortem-gate` fires on every skill call | Expected — it is the shift-left gate | Set `AGENTOPS_PREMORTEM_MODE=advisory` during exploration |

See [`troubleshooting.md`](troubleshooting.md) for more.

## Adding a new hook

1. Add the script to `hooks/<new-hook>.sh`. Source `lib/hook-helpers.sh` for logging, timeouts, and exit-code conventions.
2. Register it in `hooks/hooks.json` against the appropriate event (and matcher, if relevant).
3. Run `cd cli && make sync-hooks` so the embedded copy in `cli/embedded/` stays in sync. CI fails if you skip this.
4. Add an integration test under `tests/` — the shape should follow existing examples for the same event.
5. Update this page and [`cli/docs/HOOKS.md`](https://github.com/boshu2/agentops/blob/main/cli/docs/HOOKS.md) if the hook introduces a user-visible contract.

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for the full review gate.
