# DevOps for Vibe-Coding

**Version:** 1.2.0
**Date:** 2026-01-31
**Status:** Foundation document for strategic pivot

---

## Tagline

**Primary:** The missing DevOps layer for coding agents. Flow, feedback, and memory that compounds between sessions.

**Secondary:** Validation Built In, Not Bolted On

**Category:** DevOps for coding agents

**Legacy (SEO/blog only):** DevOps for Vibe-Coding

---

## Elevator Pitch (30 seconds)

> 12-Factor AgentOps is DevOps for coding agents. It shifts validation left, but it also does the two things most tooling still misses: durable repo memory and loop closure. Pre-mortem before you implement. Vibe check before you commit. Findings and learnings written back into the environment so the next session starts smarter.

### One-Liner (10 seconds)

> The missing DevOps layer for coding agents: validation, memory, and loop closure.

### Tweet-Length (280 chars)

> DevOps gave us reliable infrastructure. 12-Factor AgentOps gives us reliable coding agents. Validation built in, not bolted on. Durable memory on disk. Findings that compile into better future work.

---

## The Three Gaps

Most coding-agent tooling is strong at prompt construction and agent routing. The failure mode comes after that. AgentOps treats three gaps as a lifecycle contract (see [docs/context-lifecycle.md](../context-lifecycle.md) for the full treatment):

1. **Judgment validation** — the agent ships without the risk context that would challenge its choices. `/pre-mortem` before implementation, `/vibe` before commit, `/council` for multi-judge review.
2. **Durable learning** — solved problems recur because nothing extracts, scores, and retrieves the lesson. The `.agents/` ledger, `ao lookup`, finding registry, and `/retro` keep learnings alive across sessions.
3. **Loop closure** — completed work does not produce better next work. `/post-mortem` harvests learnings and next-work items, the finding compiler promotes repeat failures into preventive constraints, and `GOALS.md` + `/evolve` turn findings into measurable improvements.

These three gaps are not separate features. They form a single feedback loop:

```
/pre-mortem → Implement → /vibe → Commit → /post-mortem → extract → curate → retrieve → next session
   (gap 1)                (gap 1)           (gap 3)        (gap 2)   (gap 2)   (gap 2)    (gap 3)
```

## Key Differentiators

### 1. Validation Built In, Not Bolted On

**Traditional workflow:**
```
Write code → Ship → CI catches problems → Fix → Repeat
```

**Shift-left workflow:**
```
/pre-mortem → Implement → /vibe → Commit → Knowledge compounds
```

The validation loop happens before code ships, not after. But validation alone is not the whole story. The larger system also extracts what was learned, compiles repeat failures into preventive artifacts, and feeds better context into the next cycle. See gaps 2 and 3 above.

### 2. Coding Agent Specific

We focus on **coding agents**—AI assistants that write, modify, and review code:

- Claude Code running in terminal/IDE
- AI pair programming sessions
- Code generation with validation workflows
- Agents using Read, Edit, Write, Bash for development

We are NOT:
- A framework for customer service chatbots
- A platform for RAG-based Q&A systems
- An SDK for multi-modal agents
- A solution for general autonomous production agents

### 3. DevOps Principles, New Context

We apply proven operational discipline to a new domain:

| DevOps Principle | Coding Agent Application |
|------------------|--------------------------|
| Infrastructure as Code | Prompts and workflows as versioned artifacts |
| Shift-Left Testing | /pre-mortem before implementation |
| Continuous Integration | /vibe checks before every commit |
| Post-Mortems | /retro to extract and compound learnings |
| Observability | Knowledge flywheel tracks what works |

### 4. Single-Session Excellence

AgentOps optimizes the single coding session while refusing to waste what it learned:

- Context management (40% rule)
- Validation gates within the session
- Knowledge extraction, retrieval, and compiled prevention
- Human-AI collaboration patterns

For multi-session orchestration, see Olympus (Temporal-based workflows).

### 5. Knowledge That Compounds

The Knowledge Flywheel:
```
Session -> extract -> curate -> retrieve -> apply -> reinforce
    ^__________________________________________________________|
```

Every session makes the next one better because the environment changes, not because the agent remembers. This is the moat.

---

## What We Are

- **DevOps principles applied to coding agents** — The same operational discipline that made infrastructure reliable
- **Validation-first workflow** — Shift-left, not shift-blame
- **Knowledge flywheel that compounds** — Learnings persist, get curated, and harden into future prevention
- **Single-session orchestration** — Excellence where it matters most
- **Framework, not SDK** — Patterns and practices, not lock-in

## What We Are NOT

- **General production agent framework** — For that, see [12-Factor Agents](https://github.com/humanlayer/12-factor-agents) by Dex Horthy
- **Just another automation tool** — We're about validation, not execution speed
- **Competing with Agent SDKs** — We're complementary (use LangChain, CrewAI, etc. for the runtime)
- **Model-specific** — Works with Claude, could work with others
- **Multi-session orchestration** — For that, see Olympus

---

## Target Audience

### Primary: Developers Using Coding Agents

- Engineers using Claude Code, GitHub Copilot, or similar
- Teams adopting AI-assisted development
- Developers who want "vibe coding" without the "hope and pray"

### Secondary: Engineering Managers

- Teams scaling AI-assisted development
- Leaders concerned about code quality with AI
- Organizations building coding agent workflows

### Tertiary: Platform/DevOps Engineers

- Building internal developer platforms with AI
- Integrating coding agents into CI/CD
- Establishing validation patterns for AI-generated code

---

## Competitive Positioning

| Solution | Focus | Relationship to Us |
|----------|-------|-------------------|
| **12-Factor Agents** (Dex Horthy) | General autonomous agents | Complementary—we're coding-specific |
| **Agent SDKs** (LangChain, CrewAI) | Runtime infrastructure | We sit above—validation patterns, not execution |
| **Olympus** (mt-olympus.io) | Multi-session orchestration | Complementary—we're single-session |
| **CI/CD tools** | Post-merge validation | We shift-left—validation before commit |
| **Linters/Formatters** | Syntax validation | We're semantic—does the code do what you intended? |

### Our Unique Position

```
                    General ←—→ Coding-Specific
                         │
Multi-Session           │   Olympus
       ↑                │
       │                │
       │           ┌────┴────┐
       │           │ AgentOps │ ← WE ARE HERE
       │           │(Shift-L)│
       │           └────┬────┘
       │                │
Single-Session          │   Agent SDKs
       ↓                │
                        │
              Execution ←—→ Validation
```

---

## Core Message Framework

### When Asked "What is 12-Factor AgentOps?"

> It's DevOps for coding agents. We apply the same operational principles that made infrastructure reliable, then add the missing memory and loop-closure layers agents need because they forget everything between sessions. Instead of shipping AI-generated code and hoping CI catches problems, you validate before commit and write the learning back into the repo.

### When Asked "How is it different from X?"

| X | Response |
|---|----------|
| Regular coding | Same principles, but with AI-specific patterns for context management and validation |
| Other agent frameworks | We're coding-specific and validation-focused, not general autonomous agents |
| CI/CD | We shift validation left—into the workflow, before you push |
| Copilot/Claude Code | We're complementary—the operational layer around your coding agent |

### When Asked "Why should I care?"

> Because AI-generated code still needs validation, and validation alone still leaves you paying for the same lesson twice. AgentOps shifts validation left, captures what failed, and turns useful findings into better future context and gates.

---

## Messaging Hierarchy (Council-Validated 2026-03-30)

Different surfaces need different messaging density. Match the message to the audience:

| Surface | Message | Source |
|---------|---------|--------|
| **GitHub description** (< 100 chars) | Agent memory that compounds. Zero infrastructure. | Hybrid A+C |
| **README hero** | Your agents forget everything between sessions. AgentOps fixes that. | A (emotional) |
| **README subtitle** | Learnings, patterns, and decisions live as plain files in `.agents/` | C (mechanical) |
| **Social / Twitter** | Mistakes happen once. `.agents/` makes sure of it. | A+C (outcome) |
| **HN / technical blog** | Grep replaces RAG: permanent agent memory with plain files | C (provocative) |
| **Conference talk** | Your agents are temps. Your repo remembers everything. | A (emotional) |
| **Landing page (managers)** | Institutional memory for AI agents. Zero infrastructure. | A+C (balanced) |
| **Competitive comparison** | Agent knowledge as files. Diffable. Greppable. No vendor lock-in. | C (mechanical) |

**Category claim:** "Repo-native agent memory" — agent knowledge managed like code (version-controlled, reviewed, promoted, decayed), not stuffed into a vector database or a proprietary cloud store.

**Strategic position:** Validation drives adoption. The flywheel drives retention. Loop closure drives lock-in. The flywheel is pillar 2 of 3, not the headline.

## Key Phrases Reference

### Use These

- "DevOps for Vibe-Coding"
- "The Three Gaps" (judgment validation, durable learning, loop closure)
- "Shift-left validation for coding agents"
- "Validation built in, not bolted on"
- "Catch it before you ship it"
- "Knowledge that compounds"
- "Repo-native agent memory"
- "Your agents are temps. Your repo remembers everything."
- "Mistakes happen once. `.agents/` makes sure of it."
- "Grep replaces RAG"
- "The 40% rule" (context budget)
- "Pre-mortem before implement"
- "Vibe check before commit"
- "Findings compile into prevention"

### Avoid These

- "Operational principles for reliable AI agents" (old, too general)
- "Production-grade agents" (implies general agents)
- "AI-assisted development" (too generic, no validation emphasis)
- "Autonomous agents" (not our focus)
- "Just use Claude better" (undersells the framework)
- "Compiler for agent experience" (metaphor breaks under scrutiny — council-killed)
- "Institutional memory" without "zero infrastructure" (triggers enterprise anxiety alone)

---

## The Three Core Skills

The shift-left workflow expressed as skills:

### 1. /pre-mortem — Simulate Failures Before Implementing

> "What could go wrong with this plan?"

Run BEFORE implementing. Identifies risks, missing requirements, edge cases. The validation starts before code exists.

### 2. /vibe — Validate Before You Commit

> "Does this code do what you intended?"

The semantic vibe check. Not just syntax—does the implementation match the intent? Run BEFORE every commit.

### 3. /retro — Extract Learnings to Compound Knowledge

> "What did we learn that makes the next session better?"

Closes the loop. Extracts learnings, feeds the flywheel. Every session makes the next one better.

**Supporting skills:** /research (understand before acting), /plan (think before implementing), /crank (execute with validation gates)

---

## Success Criteria

This positioning is successful when:

1. **Users can state what we do in one sentence** — "DevOps for vibe-coding" or equivalent
2. **No confusion with general agent frameworks** — Clear we're coding-specific
3. **All three gaps are understood** — Validation, learning, and loop closure carry equal weight
4. **Knowledge flywheel is understood** — Sessions compound, not isolated

---

## Appendix: Related Work

### The Vibe Coding Book

Steve Yegge and Gene Kim's "Vibe Coding" popularized the term. We embrace it and add the operational rigor that makes it sustainable.

### 12-Factor App

Heroku's original 12-factor methodology for SaaS apps. We adapt the philosophy (operational principles) to the new domain (coding agents).

### 12-Factor Agents

Dex Horthy's framework for general autonomous agents. Complementary work—we cite them for users who need general agent patterns.

### DevOps Handbook

The operational discipline that made infrastructure reliable. We're applying the same shift-left philosophy to a new domain.

---

*This document is the foundation for all messaging updates. Reference it when updating READMEs, articles, plugin metadata, and skill descriptions.*
