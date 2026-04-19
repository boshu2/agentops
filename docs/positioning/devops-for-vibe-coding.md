# DevOps for Vibe-Coding

**Version:** 1.2.0
**Date:** 2026-01-31
**Status:** Supporting bridge narrative, not the primary category page

---

## Tagline

**Primary:** The operational layer for coding agents.

**Secondary:** Bookkeeping, validation, primitives, and flows that compound between sessions.

**Category:** Operational layer for coding agents

**Legacy (SEO/blog only):** DevOps for Vibe-Coding

---

## Elevator Pitch (30 seconds)

> AgentOps is the operational layer for coding agents. DevOps is the lineage,
> not the category. It gives coding agents bookkeeping, validation,
> primitives, and flows so work starts with repo context instead of a blank
> prompt, gets challenged before shipping, and compounds between sessions.

### One-Liner (10 seconds)

> Bookkeeping and validation for coding agents that compounds between sessions.

### Tweet-Length (280 chars)

> Coding agents do not do their own bookkeeping. AgentOps is the operational
> layer for coding agents: bookkeeping, validation, primitives, and flows that
> help every session start where the last one left off.

---

## Internal Proof Contract

Most coding-agent tooling is strong at prompt construction and agent routing.
The failure mode comes after that. Internally, AgentOps proves the product
through a three-gap lifecycle contract (see
[docs/context-lifecycle.md](../context-lifecycle.md) for the full treatment):

1. **Validation** (internal: judgment validation) — the agent ships without the risk context that would challenge its choices. `/pre-mortem` before implementation, `/vibe` before commit, `/council` for multi-judge review.
2. **Bookkeeping** (internal: durable learning) — solved problems recur because nothing extracts, scores, and retrieves the lesson. The `.agents/` ledger, `ao lookup`, finding registry, and `/retro` keep learnings alive across sessions.
3. **Closure** (internal: loop closure) — completed work does not produce better next work. `/post-mortem` harvests learnings and next-work items, the finding compiler promotes repeat failures into preventive constraints, and `GOALS.md` + `/evolve` turn findings into measurable improvements.

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

### 2. Repo-Native Bookkeeping

AgentOps does not stop at chat history or "memory" as a vague promise. It
writes research, findings, learnings, handoffs, ratchet traces, and next-work
artifacts into the repo-native environment:

- `.agents/` as the working ledger
- retrieval and injection at startup and task boundaries
- curation controls for freshness, contradiction, and promotion
- flywheel closure so each session leaves better context behind

That is the mechanism behind the compounding claim.

### 3. Coding Agent Specific

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

### 4. DevOps Principles, New Context

We apply proven operational discipline to a new domain:

| DevOps Principle | Coding Agent Application |
|------------------|--------------------------|
| Infrastructure as Code | Prompts and workflows as versioned artifacts |
| Shift-Left Testing | /pre-mortem before implementation |
| Continuous Integration | /vibe checks before every commit |
| Post-Mortems | /retro to extract and compound learnings |
| Observability | Knowledge flywheel tracks what works |

DevOps is therefore lineage and supporting doctrine, not the primary category label.

### 5. Primitives and Flows

Skills are the building blocks. Flows are the named compositions of those skills:

- Pull one primitive: `/council validate this PR`
- Compose several manually: `/research` -> `/plan` -> `/council validate`
- Run the full lane: `/rpi "add retry backoff"`
- Automate toward goals: `/evolve`

This matters because AgentOps is not one monolithic workflow. It gives
operators reusable surfaces they can enter and exit based on intent.

### 6. Knowledge That Compounds

The Knowledge Flywheel:
```
Session -> extract -> curate -> retrieve -> apply -> reinforce
    ^__________________________________________________________|
```

Every session makes the next one better because the environment changes, not because the agent remembers. This is the moat.

---

## What We Are

- **Operational layer for coding agents** — a repo-native layer around the models you already use
- **DevOps principles applied to coding agents** — the lineage and operational discipline behind the design
- **Validation-first workflow** — Shift-left, not shift-blame
- **Repo-native bookkeeping that compounds** — Learnings persist, get curated, and harden into future prevention
- **Primitives and flows** — One-off actions and repeatable lanes on the same surface
- **Framework, not SDK** — Patterns and practices, not lock-in

## What We Are NOT

- **General production agent framework** — For that, see [12-Factor Agents](https://github.com/humanlayer/12-factor-agents) by Dex Horthy
- **Just another automation tool** — We're about validation, not execution speed
- **Competing with Agent SDKs** — We're complementary (use LangChain, CrewAI, etc. for the runtime)
- **Model-specific** — Works with Claude, could work with others

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
| **CI/CD tools** | Post-merge validation | We shift-left—validation before commit |
| **Linters/Formatters** | Syntax validation | We're semantic—does the code do what you intended? |

### Our Unique Position

```
                    General ←—→ Coding-Specific
                         │
                         │
       ┌────────────┐    │
       │  AgentOps  │ ← WE ARE HERE
       │ (Shift-L)  │    │
       └────────────┘    │
                         │
                         │   Agent SDKs
                         │
                         │
              Execution ←—→ Validation
```

---

## Core Message Framework

### When Asked "What is 12-Factor AgentOps?"

> Publicly, AgentOps is the operational layer for coding agents. It gives
> coding agents bookkeeping, validation, primitives, and flows so every
> session starts where the last one left off. Under the hood, DevOps and
> 12-factor ideas explain why the lifecycle/control plane looks the way it
> does.

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
| **GitHub description** (< 100 chars) | The operational layer for coding agents. | Hybrid A+C |
| **README hero** | Coding agents don't do their own bookkeeping. | A (emotional) |
| **README subtitle** | Bookkeeping, validation, primitives, and flows that compound between sessions. | C (mechanical) |
| **Social / Twitter** | Mistakes happen once. `.agents/` makes sure of it. | A+C (outcome) |
| **HN / technical blog** | Coding agents don't do their own bookkeeping. AgentOps makes that repo-native. | C (provocative) |
| **Conference talk** | Your agents are temps. Your repo remembers everything. | A (emotional) |
| **Landing page (managers)** | Bookkeeping and validation for coding agents. Zero infrastructure. | A+C (balanced) |
| **Competitive comparison** | Repo-native bookkeeping and validation. Diffable. Greppable. No vendor lock-in. | C (mechanical) |

**Category claim:** "Operational layer for coding agents" — the repo-native layer that gives agents bookkeeping, validation, primitives, and flows.

**Strategic position:** Validation drives adoption. Bookkeeping and the flywheel drive retention. Closure hardens the moat. The flywheel is real, but it is not the headline.

## Key Phrases Reference

### Use These

- "The operational layer for coding agents"
- "Bookkeeping, validation, primitives, and flows"
- "The Three Gaps" (as internal proof, not the public headline)
- "Shift-left validation for coding agents"
- "Validation built in, not bolted on"
- "Catch it before you ship it"
- "Knowledge that compounds"
- "Repo-native bookkeeping"
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

1. **Users can state what we do in one sentence** — "The operational layer for coding agents"
2. **No confusion with general agent frameworks** — Clear we're coding-specific
3. **Public and internal layers do not blur** — Public story first, three-gap proof second
4. **Knowledge flywheel is understood** — Sessions compound, not isolated

---

## Appendix: Related Work

### The Vibe Coding Book

Steve Yegge and Gene Kim's "Vibe Coding" popularized the term. We embrace it and add the operational rigor that makes it sustainable.

### 12-Factor App

Heroku's original 12-factor methodology for SaaS apps. We adapt the philosophy as supporting doctrine for the lifecycle/control-plane contract, not as the product definition.

### 12-Factor Agents

Dex Horthy's framework for general autonomous agents. Complementary work—we cite them for users who need general agent patterns, while AgentOps stays focused on the coding-agent lifecycle and three-gap contract.

### DevOps Handbook

The operational discipline that made infrastructure reliable. We're applying the same shift-left philosophy to a new domain.

---

*This document is a supporting bridge narrative. Reference it when writing for
DevOps- or vibe-coding-native audiences, not as the primary category source of
truth.*
