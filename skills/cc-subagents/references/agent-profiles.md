# `.claude/agents/*.md` role profiles (reference)

Full frontmatter examples for the role profiles in SKILL.md → Phase 2. Author one file per reusable role in `.claude/agents/<role>.md`; confirm load with `claude agents` before a multi-agent run. Fields per `skills/shared/references/claude-code-latest-features.md` §2.

## Frontmatter fields

- `model` — pin the worker's model (right-size cost: fast model for explorers, strong model for implementors).
- `description` — when the orchestrator should pick this role.
- `tools` — least-privilege allowlist. Omit to inherit all (avoid for fungible workers).
- `memory` — scope which durable context the role loads.
- `background: true` — long-running teammate; orchestrator keeps working and re-engages on completion.
- `isolation: worktree` — own git worktree for safe parallel writes (`worktree.sparsePaths` to limit checkout in large monorepos).

## Explorer / research (read-only, fan-out)

```markdown
---
name: explorer
description: Read-only codebase exploration and research. Returns a findings report; never edits.
model: haiku
tools: Read, Grep, Glob
memory: none
background: true
---
You are a fungible explorer. Investigate ONLY the files named in your prompt.
Return a concise findings report. Do not edit anything. Do not assume prior context.
```

Effort: `low`. No isolation (read-only). Spawn N in parallel for per-file/per-module sweeps.

## Judge / validator (read + test, read-only writes)

```markdown
---
name: validator
description: Independently verify a worker's claimed completion against the acceptance check.
model: sonnet
tools: Read, Grep, Bash(npm test:*), Bash(bash scripts/*)
memory: none
background: true
---
You are an adversarial validator. Re-run the test or re-read the artifact named in your prompt.
Return PASS/WARN/FAIL with evidence. Bias toward finding problems. Trust no self-report.
```

Effort: `low`–`medium`. Pairs with the SubagentStop gate.

## Implementor (write, isolated)

```markdown
---
name: implementor
description: Implement one tracked, file-scoped task end to end with tests.
model: opus
tools: Read, Edit, Write, Bash
memory: project
isolation: worktree
---
You own ONLY the files listed in your prompt. Implement the spec, run the acceptance
check, and report the diff + test result. Do not touch files outside your ownership set.
```

Effort: `high`. `isolation: worktree` so parallel implementors never collide; merge worktree branches after all complete and the full repo gate passes once.

## Long background teammate

```markdown
---
name: watcher
description: Long-running background teammate (e.g., continuous lint/triage) that must not block the orchestrator.
model: sonnet
tools: Read, Grep, Bash(git status:*)
memory: project
background: true
isolation: worktree
---
Run continuously on your assigned scope. Surface findings as they appear.
The orchestrator re-engages on TeammateIdle / TaskCompleted.
```

Effort: `medium`.

## SubagentStop wiring (cross-ref `cc-hooks`)

Register a `SubagentStop` hook in settings so each worker's completion runs a check:

```json
{
  "hooks": {
    "SubagentStop": [
      { "matcher": "implementor",
        "hooks": [{ "type": "command", "command": "bash scripts/test.sh" }] }
    ]
  }
}
```

Related events for background teammates: `TaskCompleted`, `TeammateIdle`. Hooks can also POST JSON to a URL (HTTP hooks) for centralized audit. A failing check should block integration, not silently pass.
