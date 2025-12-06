---
description: Phase 2 - Specify exact changes from research (Middle Loop)
---

# /plan - Phase 2: Specify Changes

**Purpose:** Convert research into file:line implementation plan. This is Middle Loop work.

**Why this matters:** Planning IS the work. If the plan is precise, implementation is mechanical. "If I'm shouting at Claude, the plan was bad." A well-structured plan catches architectural mistakes early when they're cheap to fix.

**Loop:** Middle Loop (hrs-days decisions), preparing for Inner Loop execution

**Token budget:** 40-60k tokens (20-30% of context)

**Input:** research.md (from `/research` or bundle)

**Output:** plan.md bundle with file:line specs → fresh session → `/implement`

**Gate:** Human approval required before implementation

---

## Opus 4.5 Behavioral Standards

<avoid_overengineering>
Design the simplest solution that meets requirements. Do not add features, abstractions, or "improvements" beyond what was researched. The right complexity is the minimum needed for the current task.
</avoid_overengineering>

<explicit_decisions>
Document each architectural decision with rationale. Future sessions need to understand why choices were made, not just what was chosen. Include "Why not X?" sections for rejected alternatives.
</explicit_decisions>

<quantified_scope>
Express work in concrete terms: number of files, specific functions, estimated lines changed. Vague plans lead to vague implementations.
</quantified_scope>

---

## FAAFO Alignment

| Dimension | How Planning Delivers |
|-----------|----------------------|
| **Fast** | Prevents rework during implementation |
| **Ambitious** | Breaks complex work into executable steps |
| **Autonomous** | Creates self-contained execution instructions |
| **Fun** | Reduces frustration from unclear direction |
| **Optionality** | Preserves rollback paths (recovery options) |

---

## Three Loops Context

```
┌─────────────────────────────────────────────────────────┐
│              OUTER LOOP (Weeks-Months)                   │
│   ┌─────────────────────────────────────────────────┐   │
│   │          MIDDLE LOOP (Hours-Days)                │   │
│   │   ┌─────────────────────────────────────────┐   │   │
│   │   │       INNER LOOP (Sec-Min)              │   │   │
│   │   │   [Implementation happens here]          │   │   │
│   │   └─────────────────────────────────────────┘   │   │
│   │   [PLANNING HAPPENS HERE] ← YOU ARE HERE        │   │
│   └─────────────────────────────────────────────────┘   │
│   [Research happened here]                              │
└─────────────────────────────────────────────────────────┘
```

**Why Middle Loop:** Planning spans hours-days, bridges research (Outer) to implementation (Inner).

---

## When to Use

**Use /plan when:**
- Research complete (from `/research` or bundle)
- Approach selected
- Constraints understood
- Fresh context window

**Don't use if:**
- Still deciding approaches (need `/research`)
- Simple single-file change (just do it)
- Same session as research (context polluted)

---

## PDC Framework for Planning

### Prevent (Before Planning)

| Prevention | Action |
|------------|--------|
| **Context pollution** | Fresh session, load only research bundle |
| **Scope creep** | Stick to research recommendation |
| **Vague specifications** | Force file:line precision |
| **Missing rollback** | Require rollback procedure |

**Pre-Planning Checklist:**
- [ ] Fresh context window (<20% used)?
- [ ] Research bundle loaded?
- [ ] Approach from research confirmed?
- [ ] Token budget allocated (40-60k max)?
- [ ] **Tracer tests from research completed?** (CRITICAL for infra work)

### Detect (During Planning)

| Detection | Watch For |
|-----------|-----------|
| **Instruction drift** | Plan expanding beyond research scope |
| **Vague specs** | "Update config" instead of file:line |
| **Missing validation** | No way to verify changes work |
| **Missing rollback** | No way to undo if broken |

**Mid-Planning Checks:**
- "Is this change in research scope?"
- "Can I specify exact file:line?"
- "How will we verify this works?"
- "How do we undo if needed?"

### Correct (After Issues)

| Issue | Correction |
|-------|------------|
| **Scope expanded** | Split into multiple plans |
| **Can't specify precisely** | Need more research |
| **No validation possible** | Add test strategy to plan |
| **Rollback unclear** | Document explicit procedure |

---

## How It Works

### Step 1: Load Research

**Entry Methods:**

**Method 1: Automatic (from /research)**
- Research completes → you approve → I start planning
- Research already in context
- No manual loading needed

**Method 2: Manual (fresh session)**
```bash
/bundle-load [research-name]
/plan [research-name].md
```

### Step 2: Create Precise Specifications

**I will:**

1. **Analyze research** (5-10k tokens)
   - Review recommendation
   - Understand constraints
   - Identify integration points

2. **Specify file changes** (15-25k tokens)
   - Every file to CREATE: full template
   - Every file to MODIFY: exact file:line:change
   - Every file to DELETE: rationale + backup
   - Implementation ORDER (sequence matters)

3. **Define validation** (5-10k tokens)
   - How to verify each change
   - Test commands
   - Success criteria
   - Failure indicators

4. **Document rollback** (5-10k tokens)
   - How to undo every change
   - Order of rollback
   - Data preservation
   - Recovery verification

5. **Generate progress files** (1-2k tokens)
   - Create `feature-list.json` at project root
   - Create `claude-progress.json` at project root
   - Enables session continuity for long-running projects

### Step 3: Output Plan Bundle

```markdown
# [Topic] Implementation Plan

**Type:** Plan
**Created:** [Date]
**Depends On:** [research bundle name]
**Loop:** Middle (bridges research to implementation)
**Tags:** [relevant tags]

---

## Overview
[Summary of what will change and why]

## Approach Selected
[From research, with brief rationale]

## Tracer Test Results

**BEFORE creating this plan, the following were validated:**

| Tracer Test | Result | What We Learned |
|-------------|--------|-----------------|
| [Test from research] | PASS/FAIL | [Findings that inform the plan] |

**If tracer tests were NOT run:** ⚠️ HIGH RISK - Plan based on unvalidated assumptions

## PDC Strategy

### Prevent
- [ ] [Pre-implementation check]

### Detect
- [ ] [Validation during implementation]

### Correct
- [ ] [Rollback step if needed]

---

## Files to Create

### 1. `path/to/new-file.yaml`

**Purpose:** [Why this file]
**Sync-wave:** [If ArgoCD]

\```yaml
[Full file content - no placeholders, no "TODO"]
\```

**Validation:** [How to verify file is correct]

---

## Files to Modify

### 1. `path/to/existing-file.yaml:15-20`

**Purpose:** [Why this change]

**Before:**
\```yaml
[Exact current content]
\```

**After:**
\```yaml
[Exact new content]
\```

**Reason:** [Why this specific change]
**Validation:** [How to verify change is correct]

---

## Files to Delete

### 1. `path/to/deprecated-file.yaml`

**Reason:** [Why no longer needed]
**Dependencies:** [What depends on this - should be none]
**Backup:** [If data preservation needed]

---

## Implementation Order

**CRITICAL: Sequence matters. Do not reorder.**

| Step | Action | Validation | Rollback |
|------|--------|------------|----------|
| 1 | Create X | `make lint` | Delete X |
| 2 | Create Y | `make lint` | Delete Y |
| 3 | Modify Z | `make test` | Revert Z |
| 4 | Full test | `make ci-all` | Revert all |

---

## Validation Strategy

### Syntax Validation
\```bash
[Command]
# Expected: [Output]
\```

### Functional Validation
\```bash
[Command]
# Expected: [Output]
\```

---

## Rollback Procedure

**Time to rollback:** [X minutes]

### Full Rollback
\```bash
# Step 1: [Command]
# Step 2: [Command]
# Verify: [Command]
\```

---

## Failure Pattern Risks

| Pattern | Risk | Prevention in Plan |
|---------|------|-------------------|
| Tests Passing Lie | [H/M/L] | [Explicit validation commands] |
| Instruction Drift | [H/M/L] | [Precise file:line specs] |
| Bridge Torching | [H/M/L] | [API compatibility check] |

---

## Risk Assessment

### High Risk
- **What:** [Could break badly]
- **Mitigation:** [In this plan]
- **Detection:** [How we'll know]
- **Recovery:** [Rollback step]

---

## Approval Checklist

**Human must verify before /implement:**

- [ ] Every file specified precisely (file:line)
- [ ] All templates complete (no placeholders)
- [ ] Validation commands provided
- [ ] Rollback procedure complete
- [ ] Implementation order is correct
- [ ] Risks identified and mitigated
- [ ] **Tracer tests completed** (for infrastructure/integration work)

---

## Progress Files Generated

\```
feature-list.json     # Feature tracking for this plan
claude-progress.json  # Session continuity state
\```

**Load with:** `/bundle-load [plan-name]` then `/session-start`

---

## Next Step

Once approved: `/implement [plan-name].md`
```

### Step 4: Approval Gate

**Human MUST approve before implementation.**

**If approved:** Plan is contract. Implementation follows exactly.

**If changes needed:** Iterate plan. Don't implement until approved.

**If rejected:** Return to research or abandon.

---

## Plan Quality Standards

### GOOD: Precise, Executable

```markdown
File: apps/redis/kustomization.yaml:12-15
Change: Add redis-values overlay

Before:
resources:
  - base/

After:
resources:
  - base/
  - overlays/redis-values.yaml

Reason: Enables site-specific Redis configuration
Validation: `helm template | grep redis-values`
```

### BAD: Vague, Unexecutable

```markdown
File: kustomization.yaml
Change: Add Redis config
Reason: For caching
```

---

## Failure Pattern Prevention

**Planning prevents these patterns:**

| Pattern | How Plan Prevents |
|---------|------------------|
| **Tests Passing Lie** | Explicit validation commands |
| **Instruction Drift** | Precise file:line specs |
| **Debug Loop Spiral** | Clear success criteria |
| **Eldritch Horror** | Modular, reviewable changes |
| **Bridge Torching** | API compatibility in validation |

---

## Integration with RPI Flow

```
RESEARCH (Outer Loop)
    │
    ↓ research.md bundle
    │
    ↓ [Fresh Session - Context Reset]
    │
PLAN (Middle Loop) ← YOU ARE HERE
    │
    ↓ plan.md bundle (approved!)
    │
    ↓ [Fresh Session - Context Reset]
    │
IMPLEMENT (Inner Loop)
    │
    ↓ Code changes + commit
```

---

## Command Options

```bash
# Default planning (40-60k tokens)
/plan [research].md

# Quick planning - outline only (20-30k tokens)
/plan --quick [research].md
# Good for: Well-understood changes

# Detailed planning - exhaustive (55-70k tokens)
/plan --detailed [research].md
# Good for: Complex, risky changes

# Risk-focused planning
/plan --risk-focus [research].md
# Emphasizes: Rollback, failure scenarios, mitigations
```

---

## After Planning

**If approved:**
1. `/bundle-save [plan-name]`
2. Start fresh session
3. `/bundle-load [plan-name]`
4. `/implement [plan-name].md`

**If changes needed:**
1. Discuss changes
2. I update plan
3. Re-review
4. Once approved → save and implement

---

## Best Practices

### Do
- Be precise (file:line or it's not a plan)
- Include validation for every change
- Document rollback completely
- Get explicit approval
- Consider failure patterns

### Don't
- Use vague descriptions
- Skip validation strategy
- Assume rollback is obvious
- Implement without approval
- Expand scope beyond research

---

## Emergency Procedures

**If planning goes wrong:**

1. **Can't specify precisely**
   - Need more research
   - `/research --extend` on unclear area

2. **Scope too large**
   - Split into multiple plans
   - Implement incrementally

3. **Context degraded**
   - Save current plan state
   - Fresh session
   - Continue planning

**Universal:** If you can't write file:line specs, you don't understand the problem well enough. Return to research.

---

**Ready?** Load your research and I'll create a precise implementation plan.
