# Codex compatibility image

This directory declares the generated AgentOps skill inventory for Codex. The
canonical source is `skills/<slug>/`, and it is also what Codex loads; no skill
implementation is owned here and no second copy of the skills is generated.

Codex users install the AgentOps Codex plugin (see the README Quickstart). Its
manifest, `.codex-plugin/plugin.json`, ships `./skills`. Contributors working
from a checkout can run `ao skills link` instead, which links each canonical
skill into `~/.agents/skills` and `~/.codex/skills`.

`manifest.json` is generated from canonical skill metadata. `verify.sh` checks
that every declared skill exists, that the plugin manifest ships `./skills`,
and that the tree passes the Codex loader checks:

```bash
bash images/codex/verify.sh
```

What Codex reads from a skill, and how to keep an explicit-only skill explicit
there, is in [the Codex skill contract](../../docs/contracts/codex-skill-api.md).
