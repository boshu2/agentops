# Claude compatibility image

This directory declares the generated AgentOps skill inventory for Claude Code.
The canonical source remains `skills/<slug>/`; no skill implementation is owned
here.

Claude Code users install the AgentOps plugin (see the README Quickstart).
Contributors working from a checkout can run `ao skills link` instead, which
links each canonical skill into `~/.agents/skills` and `~/.claude/skills`.

`manifest.json` is generated from canonical skill metadata. `verify.sh` checks
that every declared skill exists and that the plugin manifest has
the release version:

```bash
bash images/claude/verify.sh
```
