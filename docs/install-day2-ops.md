# Install And Day-2 Operations

AgentOps 3.1 ships three installable image paths. Pick the runtime the operator
uses, then run the matching one-liner.

## Install

### Claude Code

```bash
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-claude.sh | bash
```

Equivalent marketplace commands:

```bash
claude plugin marketplace add boshu2/agentops
claude plugin install agentops@agentops-marketplace
```

### Codex CLI

```bash
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash
```

The Codex installer refreshes the native plugin cache, enables the plugin, and
archives stale raw skill mirrors when they overlap the AgentOps-managed set.

### Gemini / Antigravity

```bash
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-agy.sh | bash
```

The Gemini/AGY installer downloads the AgentOps bundle, runs
`images/gemini/verify.sh`, runs `agy plugin validate`, installs the
`agentops-core-gemini` plugin, and enables it.

## Update

Re-run the same installer used for the original runtime:

```bash
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-claude.sh | bash -s -- --update
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-agy.sh | bash
```

For a pinned release, set `AGENTOPS_INSTALL_REF` or pass `--ref` where the
runtime installer supports it.

## Backup

- Claude Code: the marketplace plugin is reinstallable from source; preserve any
  local Claude settings before changing plugin configuration.
- Codex CLI: the installer writes `~/.codex/.agentops-codex-install.json` and
  archives overlapping `~/.codex/skills` or `~/.agents/skills` entries into
  timestamped backup directories.
- Gemini/AGY: use the AGY plugin manager to list the installed plugin before
  reinstalling, then keep any exported Antigravity workspace settings with the
  project backup.

## Uninstall

A clean, documented exit. Two categories, stated up front so you know which is
which before you remove anything:

- **AgentOps-owned artifacts** — the plugin/skill installs each runtime installer
  wrote. Safe to remove; the steps below remove exactly these.
- **User-owned data** — anything in your own repos. AgentOps never removes it,
  and the uninstall deliberately leaves it in place (see "What is kept").

### Per-runtime plugin/skill removal

Remove the runtime(s) you installed:

**Claude Code**

```bash
claude plugin uninstall agentops@agentops-marketplace
claude plugin marketplace remove agentops-marketplace
```

**Codex CLI**

The Codex installer writes the native plugin cache and one enable entry. Remove
both, plus the install manifest:

```bash
rm -rf ~/.codex/plugins/cache/agentops-marketplace   # cached plugin bundle
rm -f  ~/.codex/.agentops-codex-install.json          # install manifest + backup pointers
# then delete the AgentOps plugin's enable entry from ~/.codex/config.toml (edit by hand)
```

If the installer archived overlapping raw skills into a timestamped backup
directory, its path is recorded in `~/.codex/.agentops-codex-install.json`;
restore or discard that backup as you prefer.

**Gemini / Antigravity (AGY)**

```bash
agy plugin disable agentops-core-gemini
agy plugin uninstall agentops-core-gemini
```

**OpenCode**

The OpenCode installer symlinks a plugin and a skills dir; remove both symlinks
(they are links, so removing them never touches the repo they point at):

```bash
rm -f ~/.config/opencode/plugins/agentops.js
rm -f ~/.config/opencode/skills/agentops
```

**Clone-linked skills (`ao skills link` / the generic `scripts/install.sh` and
npx skill paths)**

If you followed a repo clone with `ao skills link` (the "track main" path), run
its inverse from inside the clone. It removes exactly the symlinks link minted —
those pointing into this repo's `skills/` tree — across every runtime, and leaves
every foreign skill and real directory (e.g. the jsm corpus) untouched:

```bash
ao skills unlink --dry-run   # rehearse: show what would be removed
ao skills unlink             # remove AgentOps-owned links from every installed runtime
```

### CLI binary

If you installed the `ao` CLI via Homebrew:

```bash
brew uninstall agentops
```

For a source checkout (`scripts/install.sh --dev`), remove the checkout directory
itself; nothing was installed outside it except the clone-linked skills handled
by `ao skills unlink` above.

### What is kept (by design)

Uninstall stops at the AgentOps-owned artifacts above. It deliberately does not
touch your data — this is the whole portability pitch, that AgentOps rides on top
of your work without owning it:

- **`.agents/` in your repos is YOUR data**, not an AgentOps artifact — the local
  knowledge corpus, provenance, and runtime state. It is never removed. Delete it
  yourself only if you want to discard that history.
- **Quick-start artifacts are your files.** The `CLAUDE.md` block the quick-start
  appended and the generated `GOALS.md` are checked into your repo and owned by
  you. Edit or delete them by hand if you no longer want them; the uninstall
  leaves them alone.

## Permissions

All installers are user-space installers. They must not require `sudo`.

If a write fails:

1. Confirm the target home directory is writable.
2. Re-run the installer from a normal user shell.
3. Avoid changing ownership recursively; repair only the reported file or
   directory.

## Recover

Start every recovery with the CLI self-check when `ao` is installed. It reports
which pieces are healthy before you reinstall anything:

```bash
ao doctor          # health check: prints what is wired and what is broken
ao version         # confirm the installed CLI version
```

If `ao doctor` reports a broken or missing plugin install, re-run the matching
runtime installer below — the installers are idempotent and safe to re-run.

### Claude Code

```bash
claude plugin marketplace update agentops-marketplace
claude plugin update agentops
```

If the marketplace state is corrupt, remove the stale marketplace/plugin through
Claude Code's plugin manager, then re-run `scripts/install-claude.sh`.

### Codex CLI

```bash
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash
codex --version
ao codex image-health --json
```

`ao codex image-health` is the read-only Codex image doctor. It aggregates
`images/codex/verify.sh`, Codex parity/override/generated-artifact gates, the
RPI contract, lifecycle guards, and headless-runtime checks without running
`ao codex start`, `ao codex stop`, `ao codex ensure-start`, or
`ao codex ensure-stop`.

If stale raw skills shadow the plugin, inspect the backup path recorded in
`~/.codex/.agentops-codex-install.json`, then re-run the installer.

### Gemini / Antigravity

```bash
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-agy.sh | bash -s -- --validate-only
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-agy.sh | bash
```

If `agy plugin validate` fails, run `bash images/gemini/verify.sh` from a source
checkout to distinguish AgentOps bundle drift from a local AGY runtime problem.

## Escalate

Escalate with the runtime, command, OS, and exact installer output. Include:

- `ao version` and `ao doctor` if the CLI is installed.
- `claude plugin list` for Claude Code issues.
- `codex --version` and `~/.codex/.agentops-codex-install.json` for Codex issues.
- `agy plugin list` and `agy plugin validate <plugin-dir>` for Gemini/AGY issues.
