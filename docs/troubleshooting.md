# Troubleshooting

Common issues and quick fixes for AgentOps.

---

## "Where are the hooks?"

**AgentOps 3.0 ships zero hooks.** There is no `hooks/` directory, no
`hooks.json`, and no `ao hooks` command — nothing auto-injects orientation or
gates your tool calls at session start. If you came from an older version
expecting hooks to "run", that behavior is gone by design (the hookless-first
teardown). The workflow is now guided by **skills + the `ao` CLI**, and **CI is
the authoritative gate** (see `.github/workflows/validate.yml`).

**What replaces the old auto-injected context:**

```bash
ao session bootstrap                 # the universal init prompt / orientation report
ao inject "<topic>"                  # pull decay-ranked prior context on demand
```

**Diagnosis (check your install, not hooks):**

```bash
ao doctor
```

This reports CLI, knowledge-base, plugin, and freshness health. None of these
are hooks — there are none to install.

**If you want your own gates:** AgentOps deliberately ships none, but you can
author opt-in hooks yourself. Use the `hooks-authoring` skill to add a bounded
gate (block a dangerous op, bootstrap a session, run a parity check) for your
runtime — Claude reads `~/.claude/settings.json`; other harnesses use their own
config. These are yours to own; AgentOps neither installs nor requires them.

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

5. AgentOps 3.0 ships **zero hooks** — there is nothing to install. The workflow
   is guided by skills plus the `ao` CLI, and **CI is the authoritative gate**.
   If you want a bounded gate of your own (block a dangerous op, bootstrap a
   session, run a parity check), author it with the `hooks-authoring` skill.

---

## `bd` says a column is missing or RPI falls back to tasklist mode

If `bd ready --json` fails with an error such as:

```text
column "crystallizes" could not be found in any table in scope
```

you likely have a beads CLI / beads DB schema mismatch.

**Diagnosis:**

```bash
bd version
bd upgrade status --json
bd status --json
bd migrate --inspect
```

If the JSON version/status probes or the human-readable migration inspection
show the database state is newer than the installed `bd` version, the local CLI
is too old for the repo's tracker data.

**Fixes:**

1. Upgrade beads to the matching or newer version:
   ```bash
   brew upgrade beads
   ```
2. Re-run tracker probes:
   ```bash
   bd ready --json
   bd list --type epic --status open --json
   ```
3. If you cannot repair beads immediately, Codex phased RPI now degrades
   honestly to tasklist mode instead of silently assuming beads is healthy. That
   fallback is for continuity, not a substitute for repairing the tracker.

For Codex, use `curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash`. The installer enables plugins and suppresses the unstable-plugins warning in `~/.codex/config.toml`. On Linux, install system `bubblewrap` as well so Codex does not warn that it is using the vendored fallback. For OpenCode, use `curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-opencode.sh | bash`. For other agents, use the platform-specific scripts in `scripts/`.

```bash
sudo apt-get install -y bubblewrap
```


**Symptoms:**

- Running `npx update` installs an unrelated npm package and does not update skills.
- `bash <(curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install.sh)` reports failed skills without actionable detail.

**Fixes:**

1. Use the correct updater command:
   ```bash
   bash <(curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install.sh)
   ```
2. If specific skills still fail, reinstall each failed skill directly:
   ```bash
   bash <(curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install.sh)
   ```
3. Re-run update to verify a clean state:
   ```bash
   bash <(curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install.sh)
   ```

If reinstalling one-by-one works but bulk update previously failed, the local skills lock state was stale; per-skill reinstall refreshes it.

---

## Skills show up twice in Codex

This usually means Codex is seeing AgentOps skills from more than one location.
For native-plugin installs, the active source of truth is the plugin cache under
`~/.codex/plugins/cache/.../skills-codex`. Stale copies in `~/.codex/skills` or
`~/.agents/skills` can still create duplicates if your local Codex build scans
more than one of those locations.

**Diagnosis:**

```bash
ao doctor
```

If the "Plugin" check warns about duplicate installs, inspect the active homes:

```bash
find ~/.codex/plugins/cache/agentops-marketplace/agentops/local/skills-codex -maxdepth 1 -mindepth 1 -type d | sort
find ~/.codex/skills -maxdepth 1 -mindepth 1 -type d | sort
find ~/.agents/skills -maxdepth 1 -mindepth 1 -type d | sort
```

**Fix:**

1. Reinstall so the native plugin cache is refreshed and stale raw mirrors are archived:
   ```bash
   curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash
   ```
2. If duplicates persist, archive the stale `~/.agents/skills` copy:
   ```bash
   mv ~/.agents/skills ~/.agents/skills.backup.$(date +%Y%m%d-%H%M%S)
   ```
3. If duplicates still persist, archive the stale `~/.codex/skills` copy:
   ```bash
   mv ~/.codex/skills ~/.codex/skills.backup.$(date +%Y%m%d-%H%M%S)
   ```
4. If duplicates still persist after that, remove the compatibility plugin cache:
   ```bash
   rm -rf ~/.codex/plugins/cache/agentops-marketplace/agentops/local
   ```
5. Validate the runtime in a fresh session:
   ```bash
   bash scripts/validate-codex-cli-skills.sh
   ```
6. Restart Codex so interactive sessions reload the current skill list.
7. Re-run `ao doctor` to confirm the warning is gone.

Keep the native plugin cache as the source of truth for native-plugin installs.
Only restore `~/.agents/skills` or `~/.codex/skills` if you intentionally want
raw-skill mode for a specific Codex build.

---

## CI rejects a push that skipped quality validation

AgentOps 3.0 has **no push gate hook** — `git push` itself is never blocked
locally. Quality enforcement happens in **CI** (`.github/workflows/validate.yml`),
which is the authoritative gate: a push that hasn't been validated will fail
its checks on the PR, not before it leaves your machine.

**Why it works this way:** there is no local hook to bypass, so the gate cannot
be silently skipped. The contract is "CI must be green to merge", and the cheap
way to predict that locally is to run `/vibe` before you push.

**Proper resolution:**

1. Run `/vibe` on your changes before pushing:
   ```
   /vibe
   ```

2. Address any findings until you get a PASS verdict.

3. Push, then let CI confirm:
   ```bash
   git push
   gh pr checks   # confirm the validation job is green
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

If you see an error for a command that is documented as planned, it does not exist yet. Designed-but-unbuilt commands are tracked in [ROADMAP.md](ROADMAP.md).

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
| **Knowledge Freshness** | At least one recent session exists under `.agents/ao/sessions/`. | After a session, run `ao forge transcript <path>` to ingest it. |
| **Search Index** | A non-empty `.agents/ao/index.jsonl` exists for faster repo-local searches. | Run `ao store rebuild`. |
| **Flywheel Health** | At least one learning exists under `.agents/ao/learnings/` (or legacy `.agents/learnings/`). | Run `/retro` or `/forge` to extract learnings; empty is normal early on. |
| **Codex CLI** | The `codex` binary is on your PATH (optional, used for `--mixed` validation modes). | Install Codex CLI and ensure it is on PATH. |

### Reading the output

```
ao doctor
─────────
 ✓ ao CLI              vX.Y.Z
 ✓ Knowledge Base      .agents/ao initialized
 ✓ Plugin              skills found
 ! Codex CLI           not found (optional — needed for --mixed council)

 7/8 checks passed, 1 warning
```

- `✓` = pass
- `!` = warning (optional component missing or degraded)
- `✗` = failure (required component missing or broken)

Use `ao doctor --json` for machine-readable output.

---

## Pre-mortem gate blocks `/crank`

The pre-mortem gate denies ambiguous state by default (as of 2.37.2). If `/crank` exits immediately with a pre-mortem error, it is telling you there is no pre-mortem artifact or the artifact is stale for the current epic.

**Fixes:**

1. Run `/pre-mortem` against the epic before invoking `/crank`.
2. For exploratory runs where a pre-mortem is not worth the cost:
   ```bash
   AGENTOPS_PREMORTEM_MODE=advisory /crank ...
   ```
   This downgrades the gate to a warning.

## Go tests fail in CI after a change

AgentOps 3.0 ships no pre-commit hook that runs Go tests for you — verify
locally before pushing. Run the per-tool checks for the surfaces you touched:

```bash
cd cli && make test          # or: go build ./... && go vet ./... && go test ./...
```

If tests fail, common causes:

- Tests that depend on network (`go test -short` typically skips these).
- A package import that fails to compile — fix compilation first, tests second.

CI runs the omnibus validation on push; if you skip the local check, the
failure surfaces on the PR instead.

## Context window compacted and lost work

If a session compacts and drops critical context, re-seed it with the corpus
primitives rather than relying on an auto-snapshot hook (there is none in 3.0):

```bash
ao session bootstrap                 # re-orient
ao inject "<topic>"                  # pull back the relevant prior context
```

You can also manually re-seed the session from `MEMORY.md`.

## Getting help

- **New to AgentOps?** Run `/quickstart` for an interactive onboarding walkthrough.
- **Run diagnostics:** `ao doctor` checks your installation health.
- **Report issues:** [github.com/boshu2/agentops/issues](https://github.com/boshu2/agentops/issues)
- **Full workflow guide:** Run `/using-agentops` for the complete RPI workflow reference.
