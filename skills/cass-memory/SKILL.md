---
name: cass-memory
user-invocable: false
skill_api_version: 1
hexagonal_role: supporting
metadata:
  tier: execution
description: "Use when starting non-trivial work, mining lessons, or preventing repeated mistakes with cm procedural memory."
practices:
- pragmatic-programmer
---
# cass-memory — CASS Memory System (cm)

> **Core Capability:** Transforms scattered agent sessions into persistent, cross-agent procedural memory. A pattern discovered in Cursor **automatically** helps Claude Code on the next session.

`cm` is an upstream (Dicklesworthstone) tool and is **self-describing** — discover its command surface from `cm --help` (and per-subcommand `--help`), not from this skill. Full catalog snapshot: [COMMANDS.md](references/COMMANDS.md). This skill carries only the AgentOps operating doctrine: the session protocol, feedback discipline, and boundaries.

Architecture in one line: episodic memory (cass session logs) → working memory (diary summaries) → procedural memory (playbook rules with confidence tracking and decay). Full model: [ARCHITECTURE.md](references/ARCHITECTURE.md).

## When to Use

- Starting any non-trivial task: pull prior rules and history first
- After a mistake or rabbit hole: check whether a rule already warned about it
- When a rule helped or hurt: record feedback so confidence tracking works

## THE EXACT PROMPT — Session Start

```
Before starting this task, run:

cm context "<task description>" --json

Read the output carefully:
- relevantBullets: Rules from playbook scored by relevance
- antiPatterns: Things that have caused problems before
- historySnippets: Past sessions (yours and other agents')
- suggestedCassQueries: Deeper investigation if needed

Reference rule IDs when following them (e.g., "Following b-8f3a2c...")
```

`cm context "<task>" --json` is THE ONE COMMAND — everything else is optional. Budget flags (`--limit`, `--min-score`, `--no-history`) exist when context is tight; see `cm context --help`.

## Agent Protocol

```
1. START:    cm context "<task>" --json
2. WORK:     Reference rule IDs when following them
3. FEEDBACK: Leave inline comments when rules help/hurt
4. END:      Just finish. Learning happens automatically.
```

**You do NOT need to:**
- Run `cm reflect` (automation handles this)
- Run `cm mark` manually (use inline comments)
- Manually add rules to the playbook

## Feedback Discipline

```bash
# When a rule helped / caused problems
cm mark b-8f3a2c --helpful
cm mark b-xyz789 --harmful --reason "Caused regression"

# Or leave inline comments (parsed during reflection)
// [cass: helpful b-8f3a2c] - this saved me from a rabbit hole
// [cass: harmful b-x7k9p1] - wrong for our use case
```

Why feedback matters: rules aren't immortal. Confidence halves every 90 days without revalidation, one harmful mark counts 4x a helpful one, and repeatedly-harmful rules are inverted into explicit anti-pattern warnings rather than deleted. Skipping feedback starves the decay model.

## Trauma Guard

`cm guard --install` / `--git` / `--status` installs hooks that block known-dangerous commands; `cm trauma add` / `cm trauma scan` manage the pattern set. Doctrine, scope, and pattern design: [TRAUMA-GUARD.md](references/TRAUMA-GUARD.md).

## Safety Boundaries

- **LAW 0:** never configure `cm` reflection to shell out to `claude -p` (e.g. `provider: cli`) — that path is forbidden on this fleet. Use a compliant provider, or rely on deterministic reflection (cm degrades gracefully without an LLM).
- Do not hand-edit the playbook to add rules; the reflect/curate pipeline owns it. Feedback marks are the only manual write you need.
- `cm doctor --json` first when anything misbehaves; `cm doctor --fix` for a corrupt playbook. If `cass` is missing, cm still works playbook-only (no history).

## References

| Topic | Reference |
|-------|-----------|
| Full command reference | [COMMANDS.md](references/COMMANDS.md) |
| Cognitive architecture | [ARCHITECTURE.md](references/ARCHITECTURE.md) |
| Trauma guard system | [TRAUMA-GUARD.md](references/TRAUMA-GUARD.md) |
| MCP server integration | [MCP-SERVER.md](references/MCP-SERVER.md) |
| Onboarding workflow | [ONBOARDING.md](references/ONBOARDING.md) |
