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

AgentOps currently uses six lifecycle events. Every event dispatches one or more scripts in `hooks/`.

| Event | Purpose | Representative scripts |
|-------|---------|-----------------------|
| `SessionStart` | Seed the session with recent learnings, pointers to MEMORY.md | `session-start.sh`, `ao-inject.sh` |
| `SessionEnd` | Compile session signal, maintain the knowledge pool | `session-end-maintenance.sh`, `compile-session-defrag.sh` |
| `Stop` | Close the flywheel for the turn | `ao-flywheel-close.sh` |
| `UserPromptSubmit` | Route the prompt, nudge discipline, echo intent | `factory-router.sh`, `prompt-nudge.sh`, `intent-echo.sh`, `quality-signals.sh` |
| `PreToolUse` | Gate risky tool calls (commits, edits, reads in isolation) | `pre-mortem-gate.sh`, `commit-review-gate.sh`, `holdout-isolation-gate.sh`, `go-test-precommit.sh` |
| `PostToolUse` | Quality and loop-detection after edits | `write-time-quality.sh`, `go-complexity-precommit.sh`, `go-vet-post-edit.sh`, `research-loop-detector.sh`, `context-monitor.sh` |

A `TaskCompleted` block also exists in the manifest (for `task-validation-gate.sh`); it is runtime-dependent and may be a no-op on agents that do not emit that event.

## Install and uninstall

### Claude Code

```bash
ao hooks install       # writes ~/.claude/settings.json hook entries
ao hooks show          # prints current effective config
ao hooks test          # smoke-tests the hook wiring
ao hooks uninstall     # removes ao hook entries (other entries are preserved)
```

### Codex (v0.115.0+)

Use `scripts/install-codex-plugin.sh` or `scripts/install-codex.sh` to install the native hook manifest to `~/.codex/hooks.json`.

### Codex (older)

No native hook support. Use the explicit fallback: `ao codex start` at the beginning of a session and `ao codex stop` at the end. See [`architecture/codex-hookless-lifecycle.md`](architecture/codex-hookless-lifecycle.md).

## Customizing behavior

Most hooks read environment variables rather than checking in-tree config. See [`ENV-VARS.md`](ENV-VARS.md) for the full list. Common knobs:

| Variable | Effect |
|----------|--------|
| `AGENTOPS_STARTUP_CONTEXT_MODE` | `manual` (default), `lean`, `legacy` — controls how much context `SessionStart` injects |
| `AGENTOPS_GITIGNORE_AUTO` | Set to `0` to commit `.agents/` artifacts to the repo |
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
