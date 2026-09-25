# Installing AgentOps for OpenCode

OpenCode gets the skills through the external Skills installer.

## Install

With Node.js installed, run from your project directory:

```bash
npx skills@latest add boshu2/agentops -a opencode
```

Project scope writes `.agents/skills`. Add `-g` for a user-level install; the
installer then writes the portable `~/.agents/skills`, which OpenCode documents
as a global [skill discovery root](https://opencode.ai/docs/skills). In a script,
add `-y` (and `--skill test refactor` for a subset). Don't also link a checkout
into the same root, or every skill appears twice. Installation
does not prove that OpenCode loaded the skills; see the
[host/install mapping](../docs/contracts/multi-runtime-tier-charter.md#host-and-install-surface-mapping).

## Optional OpenCode plugin

If you want the OpenCode-specific plugin hooks from this repo, link them from
an AgentOps checkout:

```bash
git clone https://github.com/boshu2/agentops.git ~/.local/share/agentops
mkdir -p ~/.config/opencode/plugins
ln -s ~/.local/share/agentops/.opencode/plugins/agentops.js \
  ~/.config/opencode/plugins/agentops.js
```

The plugin uses Node built-ins only. If the plugin destination already exists,
inspect its owner before changing it; do not force-replace a user-owned entry.
Optional hooks need separate native activation and behavior checks.

## Update

```bash
npx skills@latest update
```

Contributors who link a checkout instead use
`ao skills link --dest ~/.config/opencode/skills` from that checkout, then
`git pull --ff-only` and the same command to update.

## Migration

Start a fresh OpenCode session and confirm selected loading. The old
`scripts/install-opencode.sh` curl installer was deleted. See
[migration](../docs/MIGRATION.md) and
[failed/partial-upgrade recovery](../docs/install-day2-ops.md#recover).
