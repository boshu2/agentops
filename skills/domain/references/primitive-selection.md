---
name: Primitive Selection
kind: concept
status: draft
see-also: [primitive, anti-pattern, slice]
---
# Primitive Selection

Which behavior/enforcement Primitive to reach for. `primitive.md` enumerates the
nouns; this entry is the decision rule for *when to use which*. Four behavior
primitives, each owning one axis.

## Definition

| Primitive | What it is | Axis it owns | How it's invoked |
|---|---|---|---|
| **Skill** (`skills/<name>/SKILL.md`) | Instructions the agent reads and follows | WHO decides — the agent's judgment | `/skill` or agent choice; **stochastic** (may not be followed exactly) |
| **CLI subcommand** (`ao <cmd>`) | Deterministic Go logic | WHAT runs deterministically | explicitly, by agent / human / skill / CI; **testable, gateable** |
| **Hook** (`hooks/<name>.sh`, in `hooks/hooks.json`) | Code the harness fires on a lifecycle event | WHEN it fires — automatically | auto, on PreToolUse / SessionStart / Stop…; **local, bypassable** (`AGENTOPS_HOOKS_DISABLED=1`), **Claude-Code-only** |
| **CI gate** (`.github/workflows/`) | A check at the merge boundary | WHERE enforcement is authoritative | auto, on push / PR; **unbypassable, every runtime** |

## The core relationship

**The CLI subcommand is the reusable deterministic core. Hooks and CI gates are
two _trigger surfaces_ that call it:**

```
ao <cmd>   ── the deterministic logic (written ONCE)
  ├── a HOOK calls it    → local, instant, advisory, bypassable, Claude-only
  └── a CI GATE calls it → merge boundary, authoritative, unbypassable, all runtimes
```

You rarely choose "hook *or* CLI." You write the **CLI**, then choose *where it
fires*: a CI gate for enforcement, a hook only for instant local feedback.

## When to use

1. Needs judgment / reasoning / orchestration? → **Skill**.
2. Deterministic + repeatable + codeable? → **CLI subcommand** (the default; the core).
3. Must be *enforced*? → run that CLI as a **CI gate** (authoritative).
4. Want instant *local* feedback before CI? → *also* wire a **Hook** that calls the
   same CLI — never rely on it for enforcement.

**AgentOps 3.0 is hookless-first:** CI is the authoritative gate. Hooks are
advisory, local, and runtime-coupled (Claude Code only — the product must serve
Claude / Codex / Gemini), so enforcement lives in CLI + CI, not hooks.

## Not in this family

`Bead` (unit of work / tracking) and `schema` / contract (data shape) are different
layers — do not select among them with this rule.

## Anti-pattern

- **Deterministic logic as skill prose.** If it is repeatable and can be coded it
  belongs in a CLI subcommand; a skill that *describes* a mechanical step the agent
  must perform by hand will be skipped (skills are stochastic). Put the mechanism in
  `ao`; let the skill *call* it.
- **Enforcement in a hook.** A hook gate is bypassable (`AGENTOPS_HOOKS_DISABLED=1`)
  and Claude-Code-only. Authoritative enforcement is a CI gate. Use a hook only as a
  fast local mirror of a CI gate, never as the gate of record.
