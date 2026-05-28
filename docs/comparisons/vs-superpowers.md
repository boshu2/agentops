---
title: "AgentOps vs Superpowers — Detailed Comparison"
description: "How AgentOps compares to Superpowers for AI coding agents. Superpowers enforces strict TDD. AgentOps adds cross-session memory and multi-model validation councils."
permalink: /comparisons/agentops-vs-superpowers
last_reviewed: 2026-05-07
---

# AgentOps vs Superpowers

> **Superpowers** is a popular coding agent plugin known for disciplined TDD workflows and autonomous operation (29K+ GitHub stars). It now ships via the official Anthropic marketplace (`claude-plugins-official`), and supports Kimi Code CLI, OpenClaw, and Mistral Vibe. The plugin is a lightweight shim that auto-updates skills on session start; skills live in a dedicated repo (obra/superpowers-skills).
>
> *Comparison updated 2026-05-07. See [Superpowers repo](https://github.com/obra/superpowers) for current features.*

---

## At a Glance

| Aspect | Superpowers | AgentOps |
|--------|-------------|----------|
| **Philosophy** | "Disciplined senior engineer" | "Operational layer for coding agents; technically a context compiler" |
| **Core strength** | TDD, planning, autonomous hours | Cross-session memory, learning |
| **GitHub stars** | 29,000+ | Growing |
| **Primary use** | Greenfield development | Ongoing codebase work |

### Three-Layer Comparison

| Product Layer | Superpowers | AgentOps |
|---------------|------------:|----------|
| **Context Compiler** (L1) | Session-local planning context (no cross-session retrieval) | Decay-ranked retrieval of prior learnings + phase-scoped assembly |
| **Validation Gates** (L2) | TDD as implicit gate (tests pass or fail) | TDD + multi-model councils with independent judges and pre-build validation |
| **Knowledge Flywheel** (L3) | No persistent learning between sessions | Extract → score → promote → inject; autonomous overnight compounding |

---

## What Superpowers Does Well

### 1. TDD Enforcement
Superpowers enforces true red/green TDD. Write tests first, implement second. No shortcuts.

```
Superpowers TDD Flow:
  Write failing test → Run test (red) → Implement → Run test (green) → Refactor
```

### 2. Planning Mode
The `/superpowers:brainstorm` and `/superpowers:write-plan` commands create structured implementation plans. Claude asks intelligent questions to flesh out details.

### 3. Autonomous Operation
Superpowers can work autonomously for hours without drifting. It spawns subagents for context gathering and uses test suites to validate its work.

### 4. YAGNI/DRY Principles
Built-in enforcement of software engineering best practices. No over-engineering, no repetition.

---

## Where Superpowers Falls Short

### No Cross-Session Memory

```
┌─────────────────────────────────────────────────────────────────┐
│                     SUPERPOWERS                                 │
│                                                                 │
│  Session 1: Debug auth bug        [learned: token refresh]      │
│  Session 2: Debug auth bug        [learned: token refresh]      │
│  Session 3: Debug auth bug        [learned: token refresh]      │
│                                   ↑                             │
│                              Same learning, every time          │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                      AGENTOPS                                   │
│                                                                 │
│  Session 1: Debug auth bug        [learned: token refresh]      │
│                                          ↓ (stored)             │
│  Session 2: Auth issue?           "I remember this pattern"     │
│                                          ↓ (reinforced)         │
│  Session 3: Auth?                 *instant recall*              │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Superpowers resets every session.** Your agent debugs the same issues repeatedly with no memory of past solutions.

**The mechanism is a wiki.** AgentOps's `.agents/` directory is markdown in your repo, version-controlled with your code, that agents read and contribute to. Wikis are a 25-year-old idea — that is the deflationary positioning. The novel part is not the format; it is making one for agents to traverse, and making the discipline of maintenance mechanical so it actually happens. Superpowers does not implement a corpus of this kind; learnings live in the session and end with it.

### No Failure Prevention

Superpowers validates *after* implementation (TDD). AgentOps adds *pre-implementation* validation:

```
Superpowers:
  Plan → Implement → Test (catch failures here)

AgentOps:
  Plan → Pre-Mortem (catch failures here) → Implement → Test → Post-Mortem
```

The `/pre-mortem` skill simulates 10 failure scenarios *before* you write code. Cheaper to catch bad designs early.

### Limited Validation Scope

Superpowers relies on your test suite. If tests pass, it's "done."

<!-- agentops:claim:AOP-CLAIM-COMP-SUPERPOWERS -->
AgentOps `/vibe` validates 8 aspects beyond tests:
- Semantic correctness (does code match spec?)
- Security vulnerabilities
- Code quality / smells
- Architecture violations
- Complexity metrics
- Performance issues
- AI "slop" detection
- Accessibility

---

## Feature Comparison

| Feature | Superpowers | AgentOps | Winner |
|---------|:-----------:|:--------:|:------:|
| TDD enforcement | ✅ Strict | ✅ Supported | Superpowers |
| Planning workflow | ✅ Excellent | ✅ `/plan` + `/pre-mortem` | Tie |
| Subagent spawning | ✅ Built-in | ✅ 20 expert agents | Tie |
| Autonomous work | ✅ Hours | ✅ `/crank` loop | Tie |
| YAGNI/DRY | ✅ Enforced | ⚠️ Via standards | Superpowers |
| **Cross-session memory** | ❌ None | ✅ Git-persisted | **AgentOps** |
| **Knowledge compounding** | ❌ No | ✅ Escape velocity | **AgentOps** |
| **Pre-mortem** | ❌ No | ✅ 10 failure modes | **AgentOps** |
| **8-aspect validation** | ❌ Tests only | ✅ Semantic + security + ... | **AgentOps** |
| **Scientific foundation** | ❌ Best practices | ✅ Peer-reviewed | **AgentOps** |

---

## Workflow Comparison

### Superpowers Workflow

```
/superpowers:brainstorm  →  Refine requirements interactively
         ↓
/superpowers:write-plan  →  Create implementation plan
         ↓
/superpowers:execute-plan → Execute in batches with TDD
         ↓
       Done
```

### AgentOps Workflow

```
/research     →  Explore codebase + inject prior knowledge
     ↓
/plan         →  Break into tracked issues
     ↓
/pre-mortem   →  Simulate 10 failure modes (BEFORE building)
     ↓
/crank        →  Implement → validate → commit loop
     ↓
/post-mortem  →  Validate + extract learnings (FOR NEXT TIME)
```

**Key difference:** AgentOps has gates *before* and *after* implementation, and learnings persist to future sessions.

---

## When to Choose Superpowers

- You're doing **greenfield development** (no prior context to leverage)
- You want **strict TDD enforcement** as the primary quality gate
- Your sessions are **independent** (different features each time)
- You prefer a **battle-tested** solution (29K stars, marketplace approved)

## When to Choose AgentOps

- You work on the **same codebase** over many sessions
- You want your agent to **remember past bugs and solutions**
- You want **failure prevention** before building, not just testing after
- You want **semantic validation** beyond just "tests pass"
- You value **compounding knowledge** over time

---

## Can They Work Together?

**Partially.** Both have planning workflows, so you'd need to pick one. But:

- Use Superpowers for TDD enforcement during implementation
- Use AgentOps for cross-session knowledge capture

The overlap is in planning. The differentiation is in memory.

---

## The Bottom Line

| Dimension | Superpowers | AgentOps |
|-----------|-------------|----------|
| **Optimizes** | Quality within session | Learning across sessions |
| **Quality gate** | Tests | 8-aspect semantic validation |
| **Failure catch** | After implementation | Before and after |
| **Knowledge** | Ephemeral | Compounds |

**Superpowers makes Claude a disciplined engineer *today*.**
**AgentOps makes Claude a domain expert *over time*.**

---

<div align="center">

[← Back to Comparisons](README.md) · [vs. Claude-Flow →](vs-claude-flow.md)

</div>
