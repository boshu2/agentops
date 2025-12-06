---
name: continuous-improvement
description: Ongoing system optimization and pattern refinement
estimated_time: Ongoing (periodic)
phases: 4
---

# Continuous Improvement Workflow

**Purpose:** Systematically improve system quality over time

**When to use:**
- Weekly/monthly reviews (extract learnings)
- After major milestones (retrospective)
- Pattern refinement (improve reusable patterns)
- Technical debt reduction (systematic cleanup)

**Token budget:** 30-50k tokens per cycle

---

## Workflow Phases

```
Phase 1: Review (10-15% context)
   ↓
Phase 2: Identify (5-10% context)
   ↓
Phase 3: Prioritize (5-10% context)
   ↓
Phase 4: Improve (10-15% context)
```

---

## Phase 1: Review

**Goal:** Assess recent work

**Commands:**
```bash
/learn --retrospective --since "30 days ago"
```

**Activities:**
- Review git commits
- Analyze patterns used
- Identify repeated problems
- Note what worked well

**Output:** Review summary

---

## Phase 2: Identify

**Goal:** Find improvement opportunities

**Activities:**
- Code quality issues
- Documentation gaps
- Process inefficiencies
- Pattern opportunities

**Output:** Improvement candidates list

---

## Phase 3: Prioritize

**Goal:** Rank by impact and effort

**Matrix:**
```
High Impact, Low Effort → DO FIRST
High Impact, High Effort → PLAN
Low Impact, Low Effort → MAYBE
Low Impact, High Effort → SKIP
```

**Output:** Prioritized improvement backlog

---

## Phase 4: Improve

**Goal:** Execute improvements

**Commands:**
```bash
# For each improvement:
/plan [improvement]
/implement [improvement]-plan
/validate
/learn [improvement]
```

**Output:** Improved system + new patterns

---

## Example: Monthly Improvement Cycle

```bash
# Phase 1: Review
/learn --retrospective --since "2025-10-01"
# Findings:
# - 15 commits reviewed
# - 3 patterns emerged (used 5+ times each)
# - 2 repeated issues (test failures, linting)

# Phase 2: Identify
# Opportunities:
# 1. Extract 3 patterns to pattern catalog
# 2. Fix recurring test flakiness
# 3. Add pre-commit hook for linting

# Phase 3: Prioritize
# Priority 1: Pre-commit hook (high impact, low effort)
# Priority 2: Fix test flakiness (high impact, medium effort)
# Priority 3: Extract patterns (medium impact, low effort)

# Phase 4: Improve
/plan pre-commit-hook
/implement pre-commit-hook-plan
/validate
/learn pre-commit-improvement
# Result: 90% reduction in linting issues
```

---

**Start continuous improvement with:** `/learn --retrospective`
