# Installing AgentOps for Codex

AgentOps 4 uses one canonical checkout and source symlinks. Do not use the
old curl installer or a Codex plugin cache.

## Installation

```bash
brew tap boshu2/agentops https://github.com/boshu2/homebrew-agentops
brew install agentops
git clone https://github.com/boshu2/agentops.git ~/.local/share/agentops
cd ~/.local/share/agentops
ao skills link
```

`ao skills link` creates source links under `~/.agents/skills` and every
detected runtime skill root, including `~/.codex/skills`.

## Verification

Restart Codex and confirm AgentOps skills are visible (for example `/plan` or
`/quickstart`).

## Update

```bash
cd ~/.local/share/agentops
git pull --ff-only
ao skills link
```

## Migration from 3.x

See [docs/MIGRATION.md](../docs/MIGRATION.md). Remove any old
`~/.codex/plugins/cache/agentops-marketplace` install before linking.
