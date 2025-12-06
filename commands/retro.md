---
description: Post-work retrospective with vibe-coding analysis and .claude/ audit
---

# /retro - Session Retrospective

**Purpose:** Capture learnings, audit tool usage, identify improvements after completing work

**When to use:**
- After completing significant work (implementation, debugging, deployment)
- End of session before context is lost
- After hitting failures or blockers
- Periodic team retrospectives

**Token budget:** 15-25k tokens (8-12% of context)

**Output:** Retrospective bundle with actionable improvements

**Output location:** `.agents/learnings/retros/[date]-[topic].md`

**Integration:** After retro, run `/learn` to extract patterns to `.agents/learnings/`

---

## The Retrospective Philosophy

**"Every session is a learning opportunity."**

Retrospectives serve three purposes:
1. **Capture learnings** before context is lost
2. **Audit tool usage** to improve our .claude/ ecosystem
3. **Identify patterns** (both failure and success) for future work

---

## Quick Start

```bash
# After completing work
/retro

# With specific focus
/retro --focus "infrastructure deployment"

# For failure analysis
/retro --failure "what went wrong"
```

---

## Retrospective Framework

### Section 1: Work Summary

**What was accomplished:**
- [ ] Tasks completed
- [ ] Deliverables produced
- [ ] Time invested

**What was blocked:**
- [ ] Blockers encountered
- [ ] Time lost to blockers
- [ ] Root causes identified

### Section 2: Vibe-Coding Failure Pattern Analysis

**For each of the 12 patterns, assess if it was hit:**

| # | Pattern | Hit? | Impact | How/Prevention |
|---|---------|------|--------|----------------|
| 1 | Tests Passing Lie | Y/N | Hours lost | |
| 2 | Context Amnesia | Y/N | Hours lost | |
| 3 | Instruction Drift | Y/N | Hours lost | |
| 4 | Debug Loop Spiral | Y/N | Hours lost | |
| 5 | Eldritch Code Horror | Y/N | Hours lost | |
| 6 | Agent Workspace Collision | Y/N | Hours lost | |
| 7 | Memory Tattoo Decay | Y/N | Hours lost | |
| 8 | Multi-Agent Deadlock | Y/N | Hours lost | |
| 9 | Bridge Torching | Y/N | Hours lost | |
| 10 | Repository Deletion | Y/N | Hours lost | |
| 11 | Process Gridlock | Y/N | Hours lost | |
| 12 | Stewnami | Y/N | Hours lost | |

**Total time lost to failure patterns:** X hours

**Patterns to address:** List patterns with systemic prevention needed

### Section 3: .claude/ Ecosystem Audit

**Commands Used:**

| Command | Used? | Helpful? | Issues | Improvement Needed |
|---------|-------|----------|--------|-------------------|
| /research | Y/N | 1-5 | | |
| /plan | Y/N | 1-5 | | |
| /implement | Y/N | 1-5 | | |
| /learn | Y/N | 1-5 | | |
| /bundle-* | Y/N | 1-5 | | |
| [others] | Y/N | 1-5 | | |

**Commands that SHOULD have been used but weren't:**

| Command | Why Not Used | Impact |
|---------|--------------|--------|
| | | |

**Commands that SHOULD EXIST but don't:**

| Proposed Command | What It Would Do | Why Needed |
|------------------|------------------|------------|
| | | |

**Agents Used:**

| Agent | Used? | Helpful? | Issues | Improvement Needed |
|-------|-------|----------|--------|-------------------|
| [agent-name] | Y/N | 1-5 | | |

**Agents that SHOULD have been used but weren't:**

| Agent | Why Not Used | Impact |
|-------|--------------|--------|
| | | |

**Agents that SHOULD EXIST but don't:**

| Proposed Agent | What It Would Do | Why Needed |
|----------------|------------------|------------|
| | | |

**Skills Used:**

| Skill | Used? | Helpful? | Issues | Improvement Needed |
|-------|-------|----------|--------|-------------------|
| [skill-name] | Y/N | 1-5 | | |

**Skills that SHOULD EXIST but don't:**

| Proposed Skill | What It Would Do | Why Needed |
|----------------|------------------|------------|
| | | |

**Workflows Used:**

| Workflow | Used? | Helpful? | Issues | Improvement Needed |
|----------|-------|----------|--------|-------------------|
| [workflow-name] | Y/N | 1-5 | | |

**Workflows that SHOULD EXIST but don't:**

| Proposed Workflow | What It Would Orchestrate | Why Needed |
|-------------------|---------------------------|------------|
| | | |

### Section 4: What Worked Well

**Practices that saved time:**
1.
2.
3.

**Tools that helped:**
1.
2.
3.

### Section 5: What Could Be Improved

**Practices that cost time:**
1.
2.
3.

**Missing tools/capabilities:**
1.
2.
3.

### Section 6: Actionable Improvements

**Immediate (this session):**
- [ ] Action item 1
- [ ] Action item 2

**Short-term (this week):**
- [ ] Create skill: [name]
- [ ] Update command: [name]
- [ ] Document pattern: [name]

**Medium-term (this month):**
- [ ] Create workflow: [name]
- [ ] Create agent: [name]
- [ ] Refactor: [what]

### Section 7: Learnings for /learn

**Patterns to extract:**
1. Pattern name: [description]
2. Pattern name: [description]

**Anti-patterns to document:**
1. What not to do: [description]
2. What not to do: [description]

---

## Output Template

**Save retrospective to:** `.agents/learnings/retros/YYYY-MM-DD-[topic].md`

```markdown
# Retrospective: [Work Title]

**Date:** YYYY-MM-DD
**Duration:** X hours
**Type:** Implementation/Debugging/Deployment/Research

---

## Summary

**Accomplished:** [1-2 sentences]
**Blocked by:** [1-2 sentences]
**Key learning:** [1-2 sentences]

---

## Vibe-Coding Analysis

### Failure Patterns Hit

| Pattern | Impact | Root Cause | Prevention |
|---------|--------|------------|------------|
| [pattern] | Xh | [why] | [how to prevent] |

**Total time lost:** X hours (Y% of total)

### Patterns Avoided

| Pattern | How Avoided |
|---------|-------------|
| [pattern] | [what we did right] |

---

## .claude/ Ecosystem Audit

### What We Used

| Type | Name | Rating | Notes |
|------|------|--------|-------|
| Command | /research | 4/5 | Worked well for docs |
| Command | /plan | 3/5 | Missing validation phase |
| Skill | [name] | X/5 | |

### Gaps Identified

| Type | Proposed | Purpose | Priority |
|------|----------|---------|----------|
| Skill | cluster-reality-check | Validate APIs against cluster | P0 |
| Workflow | infra-deployment | Tracer bullet + phases | P1 |
| Agent | assumption-validator | Check research assumptions | P2 |

---

## Actionable Items

### Create

- [ ] **Skill:** [name] - [purpose]
- [ ] **Workflow:** [name] - [purpose]
- [ ] **Agent:** [name] - [purpose]

### Update

- [ ] **Command:** [name] - [change needed]
- [ ] **Documentation:** [file] - [update needed]

### Document

- [ ] **Pattern:** [name] - via `/learn`
- [ ] **Anti-pattern:** [name] - via `/learn --failure`

---

## For Future Work

**Do more of:**
1.
2.

**Do less of:**
1.
2.

**Try next time:**
1.
2.

---

**Next step:** `/learn` to extract patterns, then create identified .claude/ improvements
```

**After completion, prompt:**

```
📝 Retrospective saved to .agents/learnings/retros/

Extract learnings? Run: /learn [topic]
- Patterns discovered → .agents/learnings/patterns/
- Anti-patterns hit → .agents/learnings/anti-patterns/
- Key decisions → .agents/learnings/decisions/
```

---

## Cline Integration

For Cline workflows, add this section:

### Cline Workflow Analysis

**Cline workflows used:**

| Workflow | Used? | Helpful? | Issues |
|----------|-------|----------|--------|
| [workflow-name] | Y/N | 1-5 | |

**Cline workflows that SHOULD EXIST:**

| Proposed | What It Would Do | Why Needed |
|----------|------------------|------------|
| | | |

**Cline/Claude coordination issues:**

| Issue | Impact | Resolution |
|-------|--------|------------|
| | | |

---

## Command Options

```bash
# Full retrospective
/retro

# Focus on specific area
/retro --focus "deployment"

# Failure-focused
/retro --failure "what went wrong"

# Quick retro (summary only)
/retro --quick

# Team retro (multiple sessions)
/retro --team --since "2025-11-01"
```

---

## Integration with Other Commands

**After /retro:**

1. **Extract learnings:** `/learn [topic]`
2. **Save retrospective:** `/bundle-save retro-[topic]-[date]`
3. **Create improvements:** Implement identified skills/workflows/agents

**Retro should trigger:**
- Pattern extraction via `/learn`
- Skill/workflow/agent creation if gaps identified
- Documentation updates if processes need clarification

---

## Best Practices

### Do
- Run retro while context is fresh
- Be honest about failure patterns hit
- Identify specific .claude/ improvements
- Follow up on action items

### Don't
- Skip retro after failures (that's when it's most valuable)
- Blame without identifying systemic improvements
- Create action items without owners
- Ignore recurring patterns

---

**Ready?** Run `/retro` after completing significant work.
