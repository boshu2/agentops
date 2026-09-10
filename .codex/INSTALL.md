# Installing AgentOps for Codex

Use native Codex and the shell with zero mandatory AgentOps skills:

```bash
go install github.com/boshu2/agentops/cli/cmd/ao@latest
ao quick-start
ao demo
```

Give Codex a concrete outcome and the repository's acceptance checks. A fresh
context reviews the completed change. No RPI invocation, skill bundle, hook or
`ao init` is required.

## Optional guidance

From a source checkout, link only the skills you want to make discoverable:

```bash
git clone https://github.com/boshu2/agentops.git ~/.local/share/agentops
cd ~/.local/share/agentops
ao skills link --skill test --skill refactor --dry-run
ao skills link --skill test --skill refactor
```

The command links canonical sources into `~/.agents/skills` and detected
runtime roots, including `~/.codex/skills`. Restart Codex if it snapshots its
inventory at startup. It does not remove existing skills or replace foreign
entries. No-selector `ao skills link` still links the full catalog.

Managed plugins and npx full bundles remain supported optional installations.
Choose one source for a skill to avoid duplicate discovery. Updating a source
checkout updates its existing links; repeat the same selectors to restore them.

See [installation and updates](../docs/install-day2-ops.md) for Homebrew,
building development features from source, full bundles and removal, and
[migration](../docs/MIGRATION.md) for legacy installations.
