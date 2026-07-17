# Installing AgentOps for OpenCode

AgentOps 3.3 uses one canonical checkout and source symlinks.

## Recommended install

```bash
brew tap boshu2/agentops https://github.com/boshu2/homebrew-agentops
brew install agentops
git clone https://github.com/boshu2/agentops.git ~/.local/share/agentops
cd ~/.local/share/agentops
ao skills link
```

`ao skills link` fans skills into detected runtimes, including OpenCode when
its config root is present.

## Optional OpenCode plugin

If you want the OpenCode-specific plugin hooks from this repo:

```bash
mkdir -p ~/.config/opencode/plugins
ln -sf ~/.local/share/agentops/.opencode/plugins/agentops.js \
  ~/.config/opencode/plugins/agentops.js
```

Install plugin dependencies once:

```bash
cd ~/.local/share/agentops/.opencode && bun install && cd -
```

## Update

```bash
cd ~/.local/share/agentops
git pull --ff-only
ao skills link
```

## Migration

The old `scripts/install-opencode.sh` curl installer was removed in 3.3. See
[docs/MIGRATION.md](../docs/MIGRATION.md).
