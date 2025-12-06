# Post-Work Retrospective Workflow

**Purpose:** Systematic retrospective after completing significant work

**Composes:** `/retro` -> `/learn` -> improvement creation

**Failure Patterns Prevented:** Institutional memory loss, repeated mistakes, ecosystem gaps

---

## Overview

This workflow captures learnings and identifies improvements after work completes.

```
Run /retro (failure pattern analysis)
    ↓
Identify Learnings (patterns, anti-patterns)
    ↓
Run /learn for each pattern
    ↓
Create Improvements (skills, workflows, agents)
    ↓
Save Retrospective Bundle
```

---

## When to Use

- After completing significant implementation
- After debugging session (especially if it took >1 hour)
- After deployment (success or failure)
- End of day/session with meaningful work
- After hitting unexpected obstacles
- When something "just worked" surprisingly well

## When NOT to Use

- Trivial changes with no learnings
- Routine operations that went as expected
- When context window is exhausted (defer to next session)
- Before work is actually complete

---

## Process

### Step 1: Run /retro

Invoke retrospective command with context:

```markdown
/retro [topic]

# Example:
/retro dify-deployment

# Retro should analyze:
- Vibe-coding failure patterns hit
- .claude/ ecosystem usage
- Time spent vs expected
- What worked well
- What didn't work
- Unexpected discoveries
```

### Step 2: Vibe-Coding Failure Pattern Analysis

Check session against the 12 failure patterns:

```markdown
## Failure Pattern Audit

| # | Pattern | Hit? | Evidence | Impact |
|---|---------|------|----------|--------|
| 1 | Tests Passing Lie | YES | Tracer bullet caught what CI missed | 2h saved |
| 2 | Overfitting to Stack Overflow | NO | | |
| 3 | Copy-Pasta Blindspot | YES | Copied from wrong EDB version | 4h lost |
| 4 | Debug Loop Spiral | YES | 3 iterations on image pull | 1h lost |
| 5 | Eldritch Code Horror | NO | | |
| 6 | LLM Latency Blindspot | NO | | |
| 7 | Local Environment Mismatch | YES | Docker docs != OpenShift | 2h lost |
| 8 | Moving Target Syndrome | NO | | |
| 9 | External Dependency Assumption | YES | Assumed signature policy | 1h lost |
| 10 | Context Window Amnesia | NO | Used bundles effectively | |
| 11 | Completion Impulse | YES | Pushed before validation | 30m lost |
| 12 | AI Hallucination Cascade | NO | | |

**Patterns Hit:** 5/12
**Total Time Lost:** ~8.5 hours
**Patterns That Would Have Helped:** cluster-reality-check (3,7,9), tracer-bullet (1,4)
```

### Step 3: .claude/ Ecosystem Audit

Analyze tool usage during session:

```markdown
## .claude/ Ecosystem Audit

### Skills Used
| Skill | Times Used | Effectiveness | Notes |
|-------|------------|---------------|-------|
| tracer-bullet | 3 | HIGH | Caught 2 critical issues |
| validate | 5 | HIGH | Standard workflow |
| context7-lookup | 2 | MEDIUM | Found docs but outdated |

### Skills That Should Have Been Used
| Skill | When | Why Not Used | Impact |
|-------|------|--------------|--------|
| cluster-reality-check | Before planning | Didn't exist | 4h lost |
| divergence-check | During research | Didn't exist | 2h lost |

### Skills Missing from Ecosystem
| Need | Description | Would Prevent |
|------|-------------|---------------|
| cluster-reality-check | Validate APIs exist | Pattern 3, 9 |
| divergence-check | Compare upstream to local | Pattern 7 |
| phase-gate | Validate before proceeding | Pattern 4, 11 |

### Workflows Used
| Workflow | Used? | Effectiveness |
|----------|-------|---------------|
| /research | YES | Incomplete - missed reality check |
| /plan | YES | Good - phases clear |
| /implement | YES | Poor - no gates between phases |

### Workflows That Should Have Been Used
| Workflow | Why Not Used | Impact |
|----------|--------------|--------|
| infrastructure-deployment | Didn't exist | No validation gates |

### Agents Used
| Agent | Times | Result |
|-------|-------|--------|
| applications-create-app | 1 | Partial success |

### Commands Used
| Command | Times | Notes |
|---------|-------|-------|
| /research | 1 | Good depth |
| /plan | 1 | Good structure |
| /bundle-save | 2 | Context preserved |
```

### Step 4: Identify Learnings

Extract patterns (what to repeat) and anti-patterns (what to avoid):

```markdown
## Patterns Identified (Do This Again)

### Pattern 1: Tracer Bullet First
**What:** Deploy minimal resource before full implementation
**Why it worked:** Caught admission webhook rejection before writing full spec
**When to apply:** Any operator-based deployment
**Evidence:** Session commit abc123

### Pattern 2: Bundle Context Across Sessions
**What:** Save research as bundle, load in implementation session
**Why it worked:** 0% context collapse over 3 sessions
**When to apply:** Any multi-session work
**Evidence:** bundle-dify-research.md used successfully

## Anti-Patterns Identified (Don't Do This)

### Anti-Pattern 1: Trust Upstream Docs Blindly
**What happened:** Copied EDB v1.24 examples, we have v1.23
**Impact:** 4 hours debugging imageCatalogRef that doesn't exist
**Prevention:** Always check installed version before using docs
**Evidence:** Session commit def456 (reverted)

### Anti-Pattern 2: Skip Validation Between Phases
**What happened:** Deployed Phase 3 before Phase 2 was ready
**Impact:** Cascading failures, hard to diagnose
**Prevention:** Phase gate after every phase
**Evidence:** 3 rollback commits in session
```

### Step 5: Run /learn for Each Pattern

Extract reusable patterns to pattern library:

```markdown
# For each pattern worth capturing:

/learn pattern="tracer-bullet-first" \
  context="Infrastructure deployments with operators" \
  what="Deploy minimal resource before full implementation" \
  why="Catches admission webhook rejections early" \
  when="Any deployment using CRDs with admission webhooks" \
  evidence="Dify deployment 2025-11-27, saved 2h"

/learn pattern="version-check-before-docs" \
  context="Using external documentation" \
  what="Check installed version before trusting docs" \
  why="Docs may be for newer version with features you don't have" \
  when="Any use of external/upstream documentation" \
  evidence="EDB imageCatalogRef failure, cost 4h" \
  --failure  # Mark as anti-pattern
```

### Step 6: Create Improvements

Based on gaps identified, create issues or drafts for:

```markdown
## Improvements to Create

### Skills Needed
1. **cluster-reality-check**
   - Purpose: Validate APIs/images/operators exist
   - Priority: P0 (would have saved 4h today)
   - Spec: [draft spec]

2. **divergence-check**
   - Purpose: Compare upstream docs to local reality
   - Priority: P1 (would have saved 2h today)
   - Spec: [draft spec]

### Workflow Updates
1. **infrastructure-deployment**
   - Add: Reality check in Phase R
   - Add: Phase gates in Phase I
   - Priority: P0

### Agent Updates
1. **applications-create-app**
   - Add: Call cluster-reality-check
   - Add: Call tracer-bullet
   - Priority: P1

### Documentation Needed
1. **Vibe-coding failure pattern reference**
   - Document all 12 patterns
   - Include prevention strategies
   - Priority: P2
```

### Step 7: Save Retrospective Bundle

```markdown
/bundle-save retro-[topic]-[date]

# Bundle should include:
- Failure pattern audit
- Ecosystem audit
- Patterns identified
- Anti-patterns identified
- Improvements proposed
- Time analysis
- Key learnings summary
```

---

## Output Template

```markdown
# Retrospective: [Topic]

**Date:** YYYY-MM-DD
**Duration:** [Time spent on work]
**Outcome:** [Success/Partial/Failure]

## Executive Summary

[2-3 sentence summary of what happened and key learning]

## Failure Pattern Analysis

**Patterns Hit:** N/12
**Time Lost:** X hours
**Key Pattern:** [Most impactful pattern hit]

| Pattern | Hit | Impact |
|---------|-----|--------|
| ... | ... | ... |

## .claude/ Ecosystem Audit

**Skills Used:** N
**Skills Missing:** N
**Workflows Used:** N
**Most Effective:** [skill/workflow]
**Biggest Gap:** [what was missing]

## Patterns Extracted

### [Pattern 1 Name]
- **What:** ...
- **Why:** ...
- **When:** ...

## Anti-Patterns Documented

### [Anti-Pattern 1 Name]
- **What happened:** ...
- **Prevention:** ...

## Improvements Proposed

| Type | Name | Priority | Impact |
|------|------|----------|--------|
| Skill | cluster-reality-check | P0 | 4h/deployment |
| Workflow | infrastructure-deployment | P0 | 2h/deployment |

## Time Analysis

| Phase | Expected | Actual | Variance |
|-------|----------|--------|----------|
| Research | 1h | 2h | +1h (reality check missing) |
| Planning | 30m | 45m | +15m |
| Implementation | 2h | 6h | +4h (no phase gates) |
| Total | 3.5h | 8.75h | +5.25h (150% over) |

## Key Learnings

1. [Learning 1]
2. [Learning 2]
3. [Learning 3]

## Next Actions

- [ ] Create cluster-reality-check skill
- [ ] Update infrastructure-deployment workflow
- [ ] Document tracer-bullet pattern
```

---

## Integration

### With /retro Command

```markdown
# /retro invokes this workflow

/retro [topic]
→ Runs failure pattern analysis
→ Runs ecosystem audit
→ Prompts for pattern extraction
→ Creates improvement items
→ Saves bundle
```

### With /learn Command

```markdown
# Each pattern identified gets captured

/learn pattern="name" context="..." what="..." why="..." when="..."

# Patterns stored in:
# - Knowledge graph (Memory MCP)
# - Pattern library (docs/patterns/)
# - Retrospective bundle
```

### With infrastructure-deployment Workflow

```markdown
# Phase V of infrastructure-deployment uses this

infrastructure-deployment:
  ...
  Phase V:
    - Full validation
    - Rollback test
    - post-work-retro  # This workflow
    - Documentation
```

---

## Time Budget

| Step | Time | Notes |
|------|------|-------|
| Run /retro | 5 min | Failure pattern analysis |
| Ecosystem audit | 5 min | Tool usage review |
| Identify patterns | 10 min | Extract learnings |
| Run /learn | 5 min | Capture patterns |
| Create improvements | 10 min | File issues/drafts |
| Save bundle | 2 min | Preserve context |
| **Total** | **~40 min** | |

---

## Success Criteria

Post-work retrospective is successful when:

- [ ] Failure pattern audit complete
- [ ] Ecosystem audit complete
- [ ] At least 1 pattern extracted (if work was meaningful)
- [ ] At least 1 anti-pattern documented (if failures occurred)
- [ ] Improvement items created for gaps found
- [ ] Retrospective bundle saved
- [ ] Time analysis documented
- [ ] Key learnings articulated

---

## Quick Reference

```bash
# After completing significant work:

1. /retro [topic]
   # Analyze session for patterns

2. Review failure patterns
   # Which of 12 did we hit?

3. Audit .claude/ usage
   # What tools helped? What was missing?

4. Extract patterns
   /learn pattern="name" ...

5. Create improvements
   # File issues for skills/workflows needed

6. Save bundle
   /bundle-save retro-[topic]-[date]
```

---

## Anti-Patterns in Retrospectives

### Skipping When Things Went Well

```markdown
# BAD: "Everything worked, no retro needed"
```

**Why bad:** Miss opportunity to document WHY it worked.

**Should:** Capture successful patterns for reuse.

### Too Shallow

```markdown
# BAD: "It failed because the image didn't pull"
```

**Why bad:** Doesn't capture root cause or prevention.

**Should:** "It failed because we trusted upstream docs without version check. Prevention: Always check installed version first."

### No Improvements Created

```markdown
# BAD: "We should have a skill for this"
# (But never create it)
```

**Why bad:** Same problem will recur.

**Should:** Create issue, draft spec, or implement immediately.

### Blame-Focused

```markdown
# BAD: "The docs were wrong"
```

**Why bad:** Doesn't lead to systemic improvement.

**Should:** "We lacked a divergence check between docs and reality. Created skill to prevent this."

---

**Remember:** A retrospective that doesn't lead to improvement is just complaining. Every retro should produce at least one concrete action.
