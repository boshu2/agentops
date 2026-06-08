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

## Permissions

All installers are user-space installers. They must not require `sudo`.

If a write fails:

1. Confirm the target home directory is writable.
2. Re-run the installer from a normal user shell.
3. Avoid changing ownership recursively; repair only the reported file or
   directory.

## Recover

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
```

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
