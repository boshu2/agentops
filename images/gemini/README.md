# Gemini / Antigravity compatibility image

This directory is a generated distribution bundle for Gemini and Google
Antigravity. The canonical skill source remains `skills/<slug>/`; bundled
`images/gemini/skills/<slug>/` copies must stay byte-identical.

New installations should use one checkout plus source links:

```bash
cd ~/.local/share/agentops
ao skills link
```

That links each canonical skill into `~/.agents/skills` and
`~/.gemini/skills` when the runtime is installed. The AGY plugin wrapper,
optional Agent Mail MCP configuration, hooks, agents, and rules remain
migration-only compatibility for this release and do not participate in the
core RPI verdict sequence.

`plugin.json` and the bundled skill set are generated from canonical metadata.
Verify JSON validity, complete inventory, byte identity, and optional AGY package
validation with:

```bash
bash images/gemini/verify.sh
```
