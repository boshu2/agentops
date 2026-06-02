---
name: agent-native
description: 'Make an out-of-session agent AgentOps-native via skills + ao CLI + CI, not hooks.'
---

# $agent-native — Make NTM Background Agents AgentOps-Native (Codex Native)

> **Quick Ref:** Run Claude/Codex background sessions under NTM with the same AgentOps guardrails — hooklessly. Guardrails = skills + the `ao` CLI + CI, never ported hooks. Bundle skills into the session profile, expose `ao` as a callable tool, coordinate through mcp-agent-mail, and gate the output through the SAME CI as interactive work.

## Codex/NTM path

Codex background agents run as NTM tmux pane sessions coordinated by mcp-agent-mail. A Codex NTM session becomes AgentOps-native by loading AgentOps skills, calling `ao session bootstrap` / `ao inject` / `ao validate` directly (no cloud Managed Agents API required — Codex shells out), reserving files through agent-mail, and gating outputs through CI (`agent-output-validate.yml`). `ao` does not wrap `gc`, and background agents do not run deprecated `ao rpi`/`ao evolve` wrappers.

## Instructions

Load and follow the skill instructions from the sibling `SKILL.md` — OR read `skills/agent-native/SKILL.md` in the host repo for the canonical specification. Honor the Critical Constraints: this is a reframe of "port hooks", NOT a hook revival; no skill fork (load the same `skills/` files); no holdout/PII in any background-agent profile or tool response; CI is the enforcement boundary, not the optional in-loop adapter.
