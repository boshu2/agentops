---
name: agent-native
description: Make an out-of-session Claude/Codex background session AgentOps-native — via skills + the ao CLI + CI, not hooks.
skill_api_version: 1
practices:
- continuous-delivery
- ddd
hexagonal_role: supporting
consumes:
- standards
- converter
- validation
produces:
- docs/contracts/agent-runtime-profile.md
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude: [HISTORY]
  intel_scope: topic
metadata:
  tier: meta
  dependencies: [standards, converter]
  stability: experimental
output_contract: "An AgentOps-native Agent definition (skills + ao tool surface) graded by the same CI gate as interactive work"
---

# /agent-native — Make NTM Background Agents AgentOps-Native (Hookless)

Run Claude and Codex agents in the background under **NTM** — long-lived tmux sessions coordinated by mcp-agent-mail — and keep them under the same AgentOps guardrails as an interactive session. The old reflex ("port the ~50 marketplace hooks into the new runtime" or "ship work through Anthropic Managed Agents first") is **wrong for AgentOps 3.0**. This skill is the hookless, NTM-first reframe.

## Overview

**AgentOps 3.0 is hookless.** Guardrails come from three things, never hooks:

1. **Skills** — `skills/<name>/SKILL.md` progressive-disclosure contracts (standards, behavioral-discipline, council, validation, trace, provenance).
2. **The `ao` CLI** — the deterministic tool surface (`ao session bootstrap`, `ao inject`, `ao corpus inject --query`, `ao validate`, `ao goals measure`) plus the `standards` skill loaded into the agent's instructions.
3. **CI as the authoritative gate** — `.github/workflows/validate.yml` runs the standards/scenario checks as CI jobs, NOT as a PreToolUse hook.

So an out-of-session agent becomes AgentOps-native by: **(a)** starting a Claude/Codex session profile with the right AgentOps skills loaded, **(b)** giving that session `ao` and MCP tools so it can `ao session bootstrap` / `ao inject` / `ao validate` itself, **(c)** coordinating claims, file reservations, and handoff through mcp-agent-mail, and **(d)** running the same CI-style validation gate on its outputs before the work is accepted. The Agent SDK's own hooks remain an optional thin adapter for teams wanting in-loop interception — never the primary mechanism.

> **Mechanism status.** `ao agent bundle` and `ao mcp serve` exist, but the active direction is NTM background-agent profiles/rosters rather than hosted Managed Agents. The background session starts with `ao session bootstrap`, pulls context with `ao inject` / `ao corpus inject`, follows skills, coordinates through mcp-agent-mail, and validates through `ao validate` / CI.

This is an **extension of two existing skills**, not a rewrite:
- [standards](../standards/SKILL.md) — gains an Agent-runtime profile: how the standards/behavioral-discipline checklists get loaded by a non-interactive Claude and enforced via CI rather than `/vibe`.
- [converter](../converter/SKILL.md) + the `skills/` ↔ `skills-codex/` parity machinery — reused as-is to keep the bundle dual-runtime.

## ⚠️ Critical Constraints

- **This is a reframe of the retired "port hooks" idea, NOT a hook revival.** **Why:** hooks are runtime-coupled and fork the guardrail surface; skills + `ao` + CI are the portable 3.0 waist that works in any runtime.
- **Single source of truth — no skill fork.** The NTM background session loads the *same* `skills/` files an interactive session uses. **Why:** a forked guardrail set drifts and defeats the corpus moat.
- **Cloud Managed Agents are out of scope for the default path.** Never bundle holdout `target`/`ground_truth`/PII into any hosted/cloud agent definition or MCP tool response. **Why:** anything sent to a non-ZDR cloud agent leaves the boundary permanently. For holdout-touching work see [eval-outcomes](../eval-outcomes/SKILL.md).
- **CI is the gate, not the adapter.** The optional SDK hook adapter is convenience, never the enforcement boundary. **Why:** a bypassed in-loop hook must not mean unvalidated work merges; CI is unconditional.

## Workflow

### Phase 1: Bundle skills into a background-session profile

```bash
ao agent bundle --runtime codex-ntm > codex-background-agent.json
# Claude NTM profiles use the same contract; the dedicated --runtime value
# lands with the background-agent roster work.
```

Stitches the selected AgentOps skills (default: `session-bootstrap`, `standards`, `behavioral-discipline`, `validation`, `provenance`) into an NTM session profile — runtime, instructions, skill list, mailbox identity, working-directory policy, and an MCP/`ao` tool descriptor. `codex-ntm` is the checked-in runtime today; a Claude NTM profile is the same shape and is part of the background-agent roster work.

**Checkpoint:** the profile carries the skills + the `ao`/MCP tool descriptor, names its mcp-agent-mail identity, and contains no holdout values.

### Phase 2: Expose `ao` as a tool

Run a thin MCP server (`ao mcp serve`) — or use the local shell-tool spec — exposing `session_bootstrap`, `inject`, `corpus_inject`, `validate`, and provenance helpers so the NTM background session can orient and self-check.

**Checkpoint:** the agent can call `ao session bootstrap` + `ao inject` itself before doing work.

### Phase 3: Gate the output via CI

A reusable workflow (`agent-output-validate.yml`) runs `ao validate` + the standards/scenario gates against whatever the agent produced (PR branch or artifact bundle) — the **same** authoritative gate as interactive work. Green CI is the merge gate.

**Checkpoint:** the agent's output passed the identical CI gate; nothing merges red.

### Optional: SDK hook adapter

For Agent SDK users who *want* in-loop interception, a documented `PreToolUse`/`Stop` adapter shells out to `ao validate` (with the `standards` checklist loaded). **Clearly optional — the default path is CI, never hooks.** Reference samples (TypeScript + Python, wired into no runtime by default): [references/sdk-hook-adapter.md](references/sdk-hook-adapter.md).

## Output Specification

**Format:** a JSON background-session profile plus a validated PR/artifact. **Path:** profiles are written outside tracked runtime state unless explicitly committed as fixtures/contracts; the runtime profile contract is written to `docs/contracts/agent-runtime-profile.md` (the frontmatter `produces` path). **Structure:** runtime, model/CLI invocation, instructions (stitched skills), `skills` array, mailbox identity, worktree policy, and `ao`/MCP descriptor; the output is accepted only on a green CI run.

## Quality Rubric

- [ ] Background-session profile loads the *same* `skills/` files as interactive sessions (no fork).
- [ ] `ao` is callable by the agent (MCP/shell-tool); it can self-bootstrap + self-validate.
- [ ] mcp-agent-mail identity is declared so reservations and handoff are traceable.
- [ ] Outputs pass the same CI gate as interactive work (CI is the boundary, not a hook).
- [ ] No holdout `target`/`ground_truth`/PII in the Agent definition or tool responses.

## Examples

```bash
# Bundle profiles, serve the ao tool surface, and let CI gate the output
ao agent bundle --runtime codex-ntm > codex-background-agent.json
ao mcp serve &   # exposes session_bootstrap/inject/validate/provenance helpers as MCP tools
# NTM starts the Claude/Codex sessions; mcp-agent-mail coordinates assignments.
```

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Tempted to port the hooks | Old runtime-coupled reflex | Don't — bundle skills + expose `ao` + gate via CI. Hooks are the optional adapter only |
| Agent can't orient | `ao` not exposed as a tool | Run `ao mcp serve` (or the shell-tool spec) so the session can `ao session bootstrap` |
| Agent collides with another worker | No mcp-agent-mail reservation | Reserve files/threads through mcp-agent-mail before editing |
| Unvalidated work merged | Relied on the optional in-loop adapter | CI (`agent-output-validate.yml`) is the gate — never the adapter |

## See Also

- [standards](../standards/SKILL.md) — the checklists the agent loads + CI enforces
- [converter](../converter/SKILL.md) — keeps the bundle dual-runtime (skills ↔ skills-codex)
- [eval-outcomes](../eval-outcomes/SKILL.md) — holdout-safe grading for cloud/out-of-session agents
- [swarm](../swarm/SKILL.md) — the in-session/NTM multi-agent backends that start Claude/Codex skill sessions (`ao agent bundle` produces the profile an NTM background session uses)
- [skill-auditor](../skill-auditor/SKILL.md) — audit this skill before declaring stable
