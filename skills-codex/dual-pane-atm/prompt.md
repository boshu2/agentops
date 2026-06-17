# Codex Execution Profile -- dual-pane-atm

Repeatable Opus (Claude) + Codex dual-pane ATM collaboration for bounded CEP-style build/duel work.

## Steps

1. Read `../../skills/dual-pane-atm/SKILL.md` and identify the work-split pattern, spawn checklist, and synthesis artifact path.
2. Load only the source `references/*` files needed for that path.
3. Confirm live ATM/Agent Mail syntax with local `--help`, repo docs, or the source skill's evidence before spawning panes.
4. Execute with Codex-native tools: local shell, `atm`, `am`, repo scripts, and AgentOps binaries as directed by the source skill.
5. Capture machine-checkable evidence: spawn commands, pane IDs, reserves/releases, exit codes, and synthesis file paths.
6. If the source skill is still being upgraded by the Claude lane, do not rewrite it. Report the missing source-side contract and keep this Codex wrapper intact.

## Guardrails

- Do not use Claude Code, `claude -p`, or Claude-only tools as the executor from Codex.
- Do not invent command flags. Verify with `--help` or checked-in references.
- Do not broaden scope beyond the requested dual-pane session.
- Do not land source files into `~/dev/agentops`; staged generation belongs under `$HOME/acfs` until the orchestrator lands the batch.
- Keep backstage/operator terminology out of client-facing artifacts.
