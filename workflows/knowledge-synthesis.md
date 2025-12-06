---
name: knowledge-synthesis
description: Extract and synthesize knowledge from multiple sources
estimated_time: 30-60 minutes
phases: 3
---

# Knowledge Synthesis Workflow

**Purpose:** Combine knowledge from code, docs, history, and patterns

**When to use:**
- Onboarding (understand new codebase)
- Documentation (create comprehensive guides)
- Pattern extraction (learn from multiple implementations)
- Architecture review (understand system design)

**Token budget:** 40-60k tokens (single session)

---

## Workflow Phases

```
Phase 1: Gather (15-20% context)
   ↓
Phase 2: Synthesize (10-15% context)
   ↓
Phase 3: Document (5-10% context)
```

---

## Phase 1: Gather

**Goal:** Collect knowledge from multiple sources

**Commands:**
```bash
/research-multi "[topic]"
# Launches 3 agents in parallel:
# - code-explorer: Code structure
# - doc-explorer: Documentation
# - history-explorer: Git history
```

**Output:** Combined research from 3 perspectives

---

## Phase 2: Synthesize

**Goal:** Connect insights, identify patterns

**Activities:**
- Cross-reference findings
- Extract common patterns
- Identify best practices
- Note gaps or inconsistencies

**Output:** Synthesized understanding

---

## Phase 3: Document

**Goal:** Capture knowledge for future use

**Commands:**
```bash
/learn [topic]
# Creates pattern documentation
# Updates knowledge base
```

**Output:** Documentation + pattern catalog

---

## Example: Understand Auth System

```bash
Read CLAUDE.md
/research-multi "authentication and authorization system"

# Agent 1 (Code):
# - auth/ directory structure
# - JWT validation in middleware
# - Session management in handlers

# Agent 2 (Docs):
# - docs/auth-design.md explains OAuth2 flow
# - examples/auth-example/ shows usage

# Agent 3 (History):
# - Added in commit abc123 (2024-03)
# - Refactored in commit def456 (2024-08)
# - Pattern: Standard OAuth2 + custom claims

# Synthesis:
# Authentication: OAuth2 with JWT
# Authorization: Role-based (admin, user, guest)
# Pattern: Middleware-based validation
# Best practice: Separate auth concerns from business logic

/learn auth-system-knowledge
# Documented for future reference
```

---

**Start knowledge synthesis with:** `Read CLAUDE.md` + `/research-multi`
