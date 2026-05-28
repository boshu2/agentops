# Swarm Troubleshooting

## Worktree isolation did not engage
Cause: `isolation: worktree` was specified but the Task result has no `worktreePath` — worker changes land in the main tree.
Solution: Verify agent definitions include `isolation: worktree`. If the runtime does not support declarative isolation, fall back to manual `git worktree add` (see Worktree Isolation section). For overlapping-file waves, abort and switch to serial execution.

## Workers produce file conflicts
Cause: Multiple workers editing the same file in parallel.
Solution: Use worktree isolation (`--worktrees`) for multi-epic dispatch. For single-epic waves, use wave decomposition to group workers by file scope. Homogeneous waves (all Go, all docs) prevent conflicts.

## Team creation fails
Cause: Stale team from prior session not cleaned up.
Solution: Run `rm -rf ~/.claude/teams/<team-name>` then retry.

## Codex agents unavailable
Cause: `codex` CLI not installed or API key not configured.
Solution: Run `which codex` to verify installation. Check `~/.codex/config.toml` for API credentials.

## Workers timeout or hang
Cause: Worker task too large or blocked on external dependency.
Solution: Break tasks into smaller units. Add timeout metadata to worker tasks.

## gc backend detected but workers unresponsive
Cause: gc controller is running but worker sessions are idle or not accepting nudges.
Solution: Run `gc status --json` to check session states. Use `gc session peek <alias> --lines 50` to inspect last activity. If a session is stuck, restart it via gc pool commands. Verify `scale_check = "bd ready --count"` returns pending work.

## Tasks assigned but workers never spawn
Cause: Backend selection failed or spawning API unavailable.
Solution: Check which spawn backend was selected (look for "Using: <backend>" message). Verify Codex CLI (`which codex`) or native team API availability.
