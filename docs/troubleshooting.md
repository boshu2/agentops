# Troubleshooting

Common issues and quick fixes for AgentOps.

---

## Hooks aren't running

Hooks require configuration in Claude Code's settings file.

**Diagnosis:**

```bash
ao doctor
```

Look for the "Hooks installed" check. If it shows `✗`, hooks are not configured.

**Fixes:**

1. Verify hooks are configured in `~/.claude/settings.json`:
   ```json
   {
     "hooks": {
       "PostToolUse": [...],
       "UserPromptSubmit": [...]
     }
   }
   ```
   The `ao doctor` check counts all hooks across event types. If it reports "no hooks configured", hooks are missing from settings.json entirely.

2. Check that hooks are not disabled via environment variable:
   ```bash
   echo $AGENTOPS_HOOKS_DISABLED
   ```
   If set to `1`, all hooks are bypassed. Unset it:
   ```bash
   unset AGENTOPS_HOOKS_DISABLED
   ```

3. Verify hook scripts exist and are executable:
   ```bash
   ls -la hooks/
   ```
   All `.sh` files in the hooks directory should have execute permissions.

---

## Skills not showing up

Skills must be installed as a Claude Code plugin.

**Diagnosis:**

```bash
claude plugin list
claude plugin marketplace list
ao doctor
```

The `ao doctor` "Plugin" check scans the `skills/` directory for subdirectories containing a `SKILL.md` file. If it reports "no skills found" or "skills directory not found", the plugin is not installed correctly.

**Fixes:**

1. Install or reinstall the AgentOps skills:
   ```bash
   claude plugin marketplace add boshu2/agentops
   claude plugin install agentops@agentops-marketplace
   ```

2. Update existing skills:
   ```bash
   claude plugin marketplace update agentops-marketplace
   claude plugin update agentops
   ```

3. If updates seem stale, clear the cache and reinstall:
   ```bash
   # The skills cache lives here:
   ls ~/.claude/plugins/marketplaces/agentops-marketplace/
   # Pull latest directly if marketplace update lags:
   cd ~/.claude/plugins/marketplaces/agentops-marketplace/ && git pull
   ```

4. Verify the plugin loads:
   ```bash
   claude --plugin ./
   ```

5. If skills load but automation hooks are missing, install hooks from repo root:
   ```bash
   ao init --hooks
   ao hooks test
   ```

For Codex/Cursor/OpenCode, use the `npx skills@latest add boshu2/agentops --all -g` install path instead of Claude plugin commands.

### `npx skills` update issues (Codex/Cursor/OpenCode)

**Symptoms:**

- Running `npx update` installs an unrelated npm package and does not update skills.
- `npx skills@latest update` reports failed skills without actionable detail.

**Fixes:**

1. Use the correct updater command:
   ```bash
   npx skills@latest update
   ```
2. If specific skills still fail, reinstall each failed skill directly:
   ```bash
   npx skills@latest add boshu2/agentops -g -a codex -s <skill-name> -y
   ```
3. Re-run update to verify a clean state:
   ```bash
   npx skills@latest update
   ```

If reinstalling one-by-one works but bulk update previously failed, the local skills lock state was stale; per-skill reinstall refreshes it.

---

## Push blocked by vibe gate

The push gate hook blocks `git push` unless a recent `/vibe` check has passed. This enforces quality validation before code reaches the remote.

**Why it exists:** The vibe gate prevents untested or unreviewed code from being pushed. It is part of the AgentOps quality enforcement workflow.

**Quick bypass (use sparingly):**

```bash
AGENTOPS_HOOKS_DISABLED=1 git push
```

**Proper resolution:**

1. Run `/vibe` on your changes:
   ```
   /vibe
   ```

2. Address any findings until you get a PASS verdict.

3. Push normally:
   ```bash
   git push
   ```

---

## Worker tried to commit

This is expected behavior in the **lead-only commit** pattern used by `/crank` and `/swarm`.

**How it works:**

- Workers write files but NEVER run `git add`, `git commit`, or `git push`.
- The team lead validates all worker output, then commits once per wave.
- This prevents merge conflicts when multiple workers run in parallel.

**If a worker accidentally committed:**

1. The lead should review the commit before pushing.
2. Amend or squash if needed to maintain clean history.

**For workers:** If you are a worker agent, your only job is to write files. The lead handles all git operations.

---

## Phantom command error

If you see errors for commands like `bd mol`, `gt convoy`, or `bd cook`, these are **planned future features** that do not exist yet.

**How to identify:** Look for `FUTURE` markers in skill documentation. These indicate commands or features that are designed but not yet implemented.

**What to do:**

- Do not retry the command. It will not work.
- Check the skill's `SKILL.md` for current supported commands.
- Use `bd --help` or `gt --help` to see available subcommands.

---

## ao doctor shows failures

`ao doctor` runs 9 health checks. Here is how to fix each one.

### Required checks (failures make the result UNHEALTHY)

| Check | What it verifies | How to fix |
|-------|-----------------|------------|
| **ao CLI** | The `ao` binary is running and reports its version. | Reinstall via Homebrew, or build from `cli/` (see `cli/README.md`). |
| **Knowledge Base** | The `.agents/ao/` directory exists in the current working directory. | Run `ao init` from your project root, or verify you are in the correct directory. |
| **Plugin** | The `skills/` directory exists and contains at least one subdirectory with a `SKILL.md` file. | See [Skills not showing up](#skills-not-showing-up) above. |

### Optional checks (warnings, result stays HEALTHY)

| Check | What it verifies | How to fix |
|-------|-----------------|------------|
| **CLI Dependencies** | `gt` and `bd` are on your PATH (nice-to-have for multi-repo ops + beads issue tracking). | Install missing tools (e.g., `brew install gastown`, `brew install beads`). |
| **Hook Coverage** | Claude Code hooks are configured (checks `~/.claude/hooks.json` first, then `~/.claude/settings.json`). | From your repo root: run `ao init --hooks` (or `ao init --hooks --full`). Hooks-only: `ao hooks install`. |
| **Knowledge Freshness** | At least one recent session exists under `.agents/ao/sessions/`. | After a session, run `ao forge transcript <path>` to ingest it. |
| **Search Index** | A non-empty `.agents/ao/index.jsonl` exists for faster searches. | Run `ao search --rebuild-index`. |
| **Flywheel Health** | At least one learning exists under `.agents/ao/learnings/` (or legacy `.agents/learnings/`). | Run `/retro` or `/forge` to extract learnings; empty is normal early on. |
| **Codex CLI** | The `codex` binary is on your PATH (optional, used for `--mixed` validation modes). | Install Codex CLI and ensure it is on PATH. |

### Reading the output

```
ao doctor
─────────
 ✓ ao CLI              vX.Y.Z
 ! Hook Coverage       No hooks found — run 'ao hooks install'
 ✓ Knowledge Base      .agents/ao initialized
 ✓ Plugin              skills found
 ! Codex CLI           not found (optional — needed for --mixed council)

 7/9 checks passed, 2 warnings
```

- `✓` = pass
- `!` = warning (optional component missing or degraded)
- `✗` = failure (required component missing or broken)

Use `ao doctor --json` for machine-readable output.

---

## Getting help

- **New to AgentOps?** Run `/quickstart` for an interactive onboarding walkthrough.
- **Run diagnostics:** `ao doctor` checks your installation health.
- **Report issues:** [github.com/boshu2/agentops/issues](https://github.com/boshu2/agentops/issues)
- **Full workflow guide:** Run `/using-agentops` for the complete RPI workflow reference.
