---
title: "AgentOps vs SDD (Spec-Driven Development) — Detailed Comparison"
description: "How AgentOps compares to Spec-Driven Development tools (cc-sdd, GitHub Spec Kit). SDD captures what you decided. AgentOps captures what you learned."
permalink: /comparisons/agentops-vs-sdd
---

# AgentOps vs SDD (Spec-Driven Development)

> **Spec-Driven Development** has gone mainstream in 2026. Major tools include **cc-sdd v2.0** (8 agents, 13 languages), **GitHub Spec Kit v0.5.0** (now industry standard), and **AWS Kiro** (spec-driven IDE). Tessl is pushing "spec-as-source" for infrastructure-as-code.

---

## At a Glance

| Aspect | SDD Tools | AgentOps |
|--------|-----------|----------|
| **Philosophy** | "Spec is the source of truth" | "Operational layer for coding agents; technically a context compiler" |
| **Core strength** | Structured requirements, cross-platform | Cross-session memory, learning |
| **Primary tools** | cc-sdd, spec-kit, SDD_Flow | AgentOps plugin + CLI |
| **Primary use** | Spec-first development | Ongoing codebase work |

---

## The SDD Landscape

### cc-sdd
NPM package (`npx cc-sdd@latest`) that installs unified SDD workflow across 7+ AI coding agents:
- Claude Code, Cursor, Gemini CLI, Codex CLI, GitHub Copilot, Qwen Code, Windsurf

### GitHub Spec Kit
GitHub's official SDD implementation:
- CLI-based workspace setup
- Slash commands for spec management
- Integration with GitHub ecosystem

### SDD_Flow
Comprehensive framework with:
- Hybrid Waterfall-Agile methodology
- Documentation templates
- Prompt library

---

## What SDD Does Well

### 1. Spec as Source of Truth
The specification becomes an executable contract:

```
SDD Workflow:
  Requirements → Spec (markdown) → Human Review → Implementation → Validation
                      ↑
                 Central artifact
```

### 2. Cross-Platform Compatibility
cc-sdd works with 7+ different AI coding agents. Write your workflow once, use it everywhere.

### 3. Structured Requirements
Formalizes the "vibe coding" chaos into structured documents:
- Requirements spec
- Design spec
- Task breakdown
- Implementation plan

### 4. Human-in-the-Loop Gates
Built-in approval points where humans review specs before implementation proceeds.

---

## Where SDD Falls Short

### Specs Are Static, Not Learning

```
┌─────────────────────────────────────────────────────────────────┐
│                        SDD                                      │
│                                                                 │
│  Session 1: Write spec → Implement → Done                       │
│                 ↓                                               │
│            (spec saved)                                         │
│                                                                 │
│  Session 2: Write spec → Implement → Done                       │
│                 ↓                                               │
│            (different spec, no learning from Session 1)         │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                      AGENTOPS                                   │
│                                                                 │
│  Session 1: Plan → Implement → Extract learnings                │
│                                      ↓                          │
│                              (patterns saved)                   │
│                                                                 │
│  Session 2: Inject learnings → Plan → Implement → More learnings│
│                  ↑                                    ↓         │
│                  └────────────── compounds ──────────┘          │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**SDD captures *what* you planned. AgentOps captures *what you learned*.**

### No Failure Prevention

SDD validates specs against implementation. It doesn't simulate failures *before* building.

```
SDD:
  Spec → Implement → Check against spec

AgentOps:
  Plan → Pre-Mortem (10 failure modes) → Implement → Validate → Extract
```

### Limited Validation Depth

SDD asks: "Does implementation match spec?"

AgentOps `/vibe` asks 8 questions:
1. Does code match spec? (semantic)
2. Is it secure?
3. Is it quality code?
4. Does it follow architecture?
5. Is complexity manageable?
6. Will it perform well?
7. Is it AI slop?
8. Is it accessible?

---

## Feature Comparison

| Feature | SDD Tools | AgentOps | Winner |
|---------|:---------:|:--------:|:------:|
| Spec-first workflow | ✅ Core focus | ✅ Via `/plan` | SDD |
| Cross-platform | ✅ 7+ agents | ⚠️ 4 supported runtime paths | SDD |
| Structured templates | ✅ Comprehensive | ⚠️ Via standards | SDD |
| Human approval gates | ✅ Built-in | ✅ 4 gates | Tie |
| **Cross-session memory** | ❌ Specs only | ✅ Learnings + patterns | **AgentOps** |
| **Knowledge compounding** | ❌ No | ✅ Escape velocity | **AgentOps** |
| **Pre-mortem simulation** | ❌ No | ✅ 10 failure modes | **AgentOps** |
| **8-aspect validation** | ❌ Spec match only | ✅ Semantic + security + ... | **AgentOps** |
| **Scientific foundation** | ❌ Methodology | ✅ Peer-reviewed | **AgentOps** |

---

## Workflow Comparison

### SDD Workflow (cc-sdd)

```
/sdd:requirements    →  Analyze and document requirements
         ↓
/sdd:design          →  Create design specification
         ↓
/sdd:tasks           →  Break into implementation tasks
         ↓
/sdd:implement       →  Execute tasks
         ↓
       Done           (specs archived, no learning extracted)
```

### AgentOps Workflow

```
/research     →  Explore codebase + inject prior knowledge
     ↓
/plan         →  Break into tracked issues (spec-like)
     ↓
/pre-mortem   →  Simulate 10 failure modes
     ↓
/crank        →  Implement → validate → commit
     ↓
/post-mortem  →  Validate + extract learnings (FOR NEXT TIME)
```

**Key difference:** AgentOps extracts *learnings* (patterns, decisions, failures), not just specs.

---

## What Gets Captured

### SDD Captures

```
project/
├── specs/
│   ├── requirements.md      # What we need
│   ├── design.md            # How we'll build it
│   └── tasks.md             # What to implement
└── src/
    └── ...
```

### AgentOps Captures

```
.agents/
├── learnings/     # "Token refresh bugs usually stem from..."
├── patterns/      # "Here's how we handle retries in this codebase"
├── research/      # Deep exploration outputs
├── specs/         # Validated specifications
├── retros/        # What worked, what didn't
└── pre-mortems/   # Failure simulations
```

**SDD:** Documents
**AgentOps:** Documents + patterns + learnings + retrospectives

---

## The Three Levels of SDD

Martin Fowler identifies three levels:

| Level | Description | SDD Tools | AgentOps |
|-------|-------------|:---------:|:--------:|
| **Spec-first** | Write spec before code | ✅ | ✅ |
| **Spec-anchored** | Keep spec after completion | ✅ | ✅ |
| **Spec-as-source** | Spec is the only source humans edit | ✅ | ❌ |

AgentOps doesn't aim for "spec-as-source" — it captures *learnings*, not just specs.

---

## When to Choose SDD Tools

- You want **spec-first development** as the core methodology
- You work across **multiple AI coding agents** (not just Claude)
- **Documentation** is your primary deliverable
- You want **structured templates** for requirements/design
- Your sessions are **independent** (no need for cross-session learning)

## When to Choose AgentOps

- You work on the **same codebase** repeatedly
- You want to capture **learnings**, not just specs
- You want **failure prevention** before building
- You want **semantic validation** beyond spec matching
- You value **compounding knowledge** over time

---

## Can They Work Together?

**Yes, naturally:**

```
┌─────────────────────────────────────────────────────────────────┐
│                    SDD + AGENTOPS                               │
│                                                                 │
│  SDD handles:                                                   │
│    └── Requirements → Design → Tasks (structured specs)         │
│                                                                 │
│  AgentOps handles:                                              │
│    └── Pre-mortem (failure simulation)                          │
│    └── /vibe (8-aspect validation)                              │
│    └── /post-mortem (learning extraction)                       │
│    └── Cross-session memory                                     │
│                                                                 │
│  Combined flow:                                                 │
│    SDD specs → AgentOps pre-mortem → Implement → AgentOps vibe  │
│                                                       ↓         │
│                                              Extract learnings  │
│                                                       ↓         │
│                                              Next session       │
└─────────────────────────────────────────────────────────────────┘
```

- **SDD** provides the structured specification methodology
- **AgentOps** provides the learning and validation layer

---

## The Bottom Line

| Dimension | SDD Tools | AgentOps |
|-----------|-----------|----------|
| **Central artifact** | Specification | Knowledge |
| **What persists** | Documents | Learnings + patterns |
| **Validation** | Spec match | 8-aspect semantic |
| **Learning** | None | Compounds over time |
| **Cross-platform** | 7+ agents | 4 supported runtime paths |

**SDD captures *what you decided*.**
**AgentOps captures *what you learned*.**

**Best approach:** Use SDD for specs, AgentOps for learning.

---

<div align="center">

[← vs. Claude-Flow](vs-claude-flow.md) · [Back to Comparisons](README.md) · [vs. GSD →](vs-gsd.md)

</div>
