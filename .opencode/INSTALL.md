# Installing AgentOps for OpenCode

AgentOps 3.3 uses one canonical checkout and source symlinks.

## Recommended install

```bash
brew tap boshu2/agentops https://github.com/boshu2/homebrew-agentops
brew install agentops
git clone https://github.com/boshu2/agentops.git ~/.local/share/agentops
cd ~/.local/share/agentops
ao skills link --dest ~/.config/opencode/skills --skill test --skill refactor --dry-run
ao skills link --dest ~/.config/opencode/skills --skill test --skill refactor
```

OpenCode documents `~/.config/opencode/skills` and the portable
`~/.agents/skills` as global [skill discovery roots](https://opencode.ai/docs/skills).
The explicit destination above chooses its dedicated root. Use your configured
discovery root if different.
`--dest` selects one explicit directory; automatic `ao skills link` fan-out does
not detect OpenCode. Repeat `--skill` for selected catalog names, or omit the
selectors for an explicit full install. Linking preserves real files/directories
and foreign or wrong links, reporting them as conflicts. It does not remove
unselected skills or prove that OpenCode loaded them. See the
[host/install mapping](../docs/contracts/multi-runtime-tier-charter.md#host-and-install-surface-mapping).

## Optional OpenCode plugin

If you want the OpenCode-specific plugin hooks from this repo:

```bash
mkdir -p ~/.config/opencode/plugins
ln -s ~/.local/share/agentops/.opencode/plugins/agentops.js \
  ~/.config/opencode/plugins/agentops.js
```

Install plugin dependencies once:

```bash
cd ~/.local/share/agentops/.opencode && bun install && cd -
```

If the plugin destination already exists, inspect its owner before changing it;
do not force-replace a user-owned entry. Optional hooks need separate native
activation and behavior checks.

## Update

```bash
cd ~/.local/share/agentops
git pull --ff-only
ao skills link --dest ~/.config/opencode/skills --skill test --skill refactor
```

## Migration

Keep the same destination and selection, then start a fresh OpenCode session
and confirm selected loading. The old `scripts/install-opencode.sh` curl installer
was removed in 3.3. See [migration](../docs/MIGRATION.md) and
[failed/partial-upgrade recovery](../docs/install-day2-ops.md#recover).
