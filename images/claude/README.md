# Claude compatibility image

This directory declares the generated AgentOps skill inventory for Claude Code.
The canonical source remains `skills/<slug>/`; no skill implementation is owned
here.

New installations should use one checkout plus source links:

```bash
cd ~/.local/share/agentops
ao skills link
```

That links each canonical skill into `~/.agents/skills` and
`~/.claude/skills` when Claude is installed. The 3.x marketplace plugin remains
migration-only compatibility for this release.

`manifest.json` is generated from canonical skill metadata. `verify.sh` checks
that every declared skill exists and that the compatibility plugin manifest has
the release version:

```bash
bash images/claude/verify.sh
```
