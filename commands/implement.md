---
description: Phase 3 - Execute approved plan with validation (Inner Loop)
---

# /implement - Phase 3: Execute Plan

**Purpose:** Execute approved plan exactly, validating at each step. This is Inner Loop work.

**Why this matters:** Trust the plan. Execute mechanically. Don't redesign. "If I'm shouting at Claude, the plan was bad" — but we're past planning now. Implementation is where plans meet reality; validation catches deviations early.

**Loop:** Inner Loop (sec-min execution), with Middle Loop validation checkpoints

**Token budget:** 40-80k tokens (20-40% of context)

**Input:** Approved plan.md (from `/plan` or bundle)

**Output:** Code changes + git commit + validation results

**Gate:** Final review before git push

---

## Opus 4.5 Behavioral Standards

<default_to_action>
Execute the plan. Do not re-research or re-plan unless blocked. If the plan says "modify auth.ts:45", open that file and make the change.
</default_to_action>

<avoid_overengineering>
Implement exactly what the plan specifies. Do not add error handling for impossible scenarios, create abstractions for one-time operations, or "improve" surrounding code. The plan defines scope.
</avoid_overengineering>

<validate_before_proceeding>
After each task, run the specified acceptance check. Do not proceed to task N+1 until task N passes validation. If validation fails, fix before continuing.
</validate_before_proceeding>

<communication_style>
After completing each task, briefly summarize what changed and the validation result. Keep updates concise - focus on outcomes, not narration.
</communication_style>

---

## FAAFO Alignment

| Dimension | How Implementation Delivers |
|-----------|----------------------------|
| **Fast** | Mechanical execution, no redesign delays |
| **Ambitious** | Complex plans become simple steps |
| **Autonomous** | Self-contained execution instructions |
| **Fun** | Satisfying completion of planned work |
| **Optionality** | Rollback preserves recovery options |

---

## Three Loops Context

```
┌─────────────────────────────────────────────────────────┐
│              OUTER LOOP (Weeks-Months)                   │
│   ┌─────────────────────────────────────────────────┐   │
│   │          MIDDLE LOOP (Hours-Days)                │   │
│   │   ┌─────────────────────────────────────────┐   │   │
│   │   │       INNER LOOP (Sec-Min)              │   │   │
│   │   │   [IMPLEMENTATION HERE] ← YOU ARE HERE  │   │   │
│   │   └─────────────────────────────────────────┘   │   │
│   │   [Planning happened here]                      │   │
│   └─────────────────────────────────────────────────┘   │
│   [Research happened here]                              │
└─────────────────────────────────────────────────────────┘
```

**Why Inner Loop:** Implementation is rapid execution—seconds to minutes per change—with immediate feedback.

---

## When to Use

**Use /implement when:**
- Plan is APPROVED (human signed off)
- All file:line specifications clear
- Validation strategy defined
- Rollback procedure documented
- Fresh context window

**Don't use if:**
- Plan unclear → return to `/plan`
- Plan not approved → get approval first
- Still deciding approach → return to `/research`
- Same session as planning → start fresh

---

## The Prime Directive

**Trust the plan. Execute mechanically.**

During implementation:

**Do:**
- Follow plan specifications exactly
- Validate after each change
- Stop on validation failure
- Execute in specified order
- Document what changed

**Do not:**
- Optimize "while you're here"
- Refactor unrelated code
- Add features beyond plan
- Reorder steps
- Skip validation
- "Improve" the plan mid-execution

**If you discover improvements:** Note them for next iteration. Complete the approved plan first.

---

## PDC Framework for Implementation

### Prevent (Before Implementation)

| Prevention | Action |
|------------|--------|
| **Wrong plan loaded** | Verify plan is approved, recent |
| **Dirty workspace** | `git status` clean |
| **Context pollution** | Fresh session |
| **Missing dependencies** | Check prerequisites |

**Pre-Implementation Checklist:**
- [ ] Plan bundle loaded and approved?
- [ ] Fresh context window?
- [ ] `git status` clean (no uncommitted changes)?
- [ ] All prerequisites satisfied?
- [ ] **Tracer tests passed?** (check plan's Tracer Test Results section)

### Detect (During Implementation)

| Detection | Watch For |
|-----------|-----------|
| **Tests Passing Lie** | Run tests yourself, don't trust claims |
| **Instruction Drift** | Deviating from plan specs |
| **Debug Loop Spiral** | >3 attempts to fix same issue |
| **Context Amnesia** | Forgetting plan details |

**After EVERY change:**
```bash
# Run validation from plan
[validation command]
# Expected: [expected output]
# If FAIL: STOP. Don't continue.
```

### Correct (When Issues Found)

| Issue | Correction |
|-------|------------|
| **Validation fails** | STOP. Report error. Don't continue. |
| **Plan unclear** | Return to planning phase |
| **Unexpected state** | Assess, rollback if needed |
| **Context degraded** | Save progress, fresh session |

---

## How It Works

### Step 1: Load Approved Plan

```bash
/bundle-load [plan-name]
/implement [plan-name].md
```

**Verify before starting:**
- Plan is approved (not draft)
- Plan is recent (not stale)
- All prerequisites met
- **Tracer tests completed** (if plan has "Tracer Test Results" section, it should show PASS)

### Step 2: Execute in Sequence

**I will follow plan EXACTLY:**

```
FOR EACH step in plan.implementation_order:

    1. Execute the change
       - Create file OR modify file:line OR delete file
       - Use EXACT content from plan

    2. Run validation from plan
       - [validation command]
       - Expected: [from plan]

    3. IF validation PASSES:
       - Continue to next step

    4. IF validation FAILS:
       - STOP IMMEDIATELY
       - Report error
       - Do NOT continue
       - Await instructions
```

### Step 3: Validate After Each Change

**This is non-negotiable:**

```
Step 1: Create apps/redis/kustomization.yaml
        ↓ Run: make lint
        PASS → Continue

Step 2: Create apps/redis/values.yaml
        ↓ Run: make lint
        PASS → Continue

Step 3: Modify apps/api/kustomization.yaml:18
        ↓ Run: make test
        PASS → Continue

Step 4: Full validation
        ↓ Run: make ci-all
        FAIL → STOP HERE

        ERROR: Validation failed at Step 4
        Output: [error details]

        Options:
        1. Fix and retry step 4
        2. Rollback to step 3
        3. Rollback completely
        4. Return to planning
```

### Step 4: Create Commit

**After all validations pass:**

```bash
git add [files from plan]
git commit -m "[type](scope): [description]

Context: [from research/plan]
Solution: [what was implemented]
Testing: [validation results]
Impact: [what changed]

Co-Authored-By: Claude <noreply@anthropic.com>"
```

### Step 5: Report Results

```markdown
# Implementation Results

## Files Created
- path/to/file1.yaml (X lines)
- path/to/file2.yaml (Y lines)

## Files Modified
- path/to/file3.yaml:15 (change description)

## Files Deleted
- path/to/deprecated.yaml (reason)

## Validation Results
- Step 1: make lint - PASSED
- Step 2: make lint - PASSED
- Step 3: make test - PASSED
- Step 4: make ci-all - PASSED

## Git Commit
commit abc123
feat(infrastructure): implement feature
[full commit message]

## Ready to Push
git push

## Post-Push Monitoring
- Watch for: [from plan]
- Rollback if: [from plan]
```

---

## Failure Pattern Defense

**Implementation is where failures materialize. Watch for:**

### Inner Loop Patterns (HIGH RISK)

| Pattern | Symptoms | Defense |
|---------|----------|---------|
| **Tests Passing Lie** | "All tests pass" but didn't run | RUN TESTS YOURSELF |
| **Context Amnesia** | Forgets plan details | Re-read plan section |
| **Instruction Drift** | Adding features beyond plan | STICK TO PLAN |
| **Debug Loop Spiral** | >3 fix attempts | STOP, reassess |

### Middle Loop Patterns (Can emerge)

| Pattern | Symptoms | Defense |
|---------|----------|---------|
| **Eldritch Horror** | Code getting complex | Trust plan's modularity |
| **Agent Collision** | Wrong files modified | Verify file paths |

### Emergency Responses

**Tests Passing Lie detected:**
```bash
# AI claims: "Tests pass!"
# YOU verify:
make test
# If output differs from claim → STOP, investigate
```

**Instruction Drift detected:**
```
AI: "While I'm here, I'll also improve..."
YOU: "STOP. Only implement what's in the plan."
```

**Debug Loop Spiral detected:**
```
After 3 failed attempts at same fix:
1. STOP trying
2. Save current state
3. Return to planning
4. Plan wasn't specific enough
```

---

## Universal Emergency Procedures

**When something goes wrong during implementation:**

```
1. STOP all AI activity immediately

2. ASSESS the situation:
   - What step were we on?
   - What failed?
   - What's the current state?

3. CHECK version control:
   - git status
   - git diff
   - What's committed vs uncommitted?

4. DECIDE: Fix-forward or rollback?
   - Minor issue → fix and continue
   - Major issue → rollback per plan
   - Unknown issue → rollback, return to planning

5. DOCUMENT what happened:
   - Add to learnings
   - Improve future plans

6. CONTINUE or ABORT:
   - If fixed → continue from current step
   - If rolled back → reassess plan
```

---

## Integration with RPI Flow

```
RESEARCH (Outer Loop)
    │
    ↓ research.md bundle
    │
PLAN (Middle Loop)
    │
    ↓ plan.md bundle (approved!)
    │
    ↓ [Fresh Session - Context Reset]
    │
IMPLEMENT (Inner Loop) ← YOU ARE HERE
    │
    ↓ Execute step by step
    ↓ Validate after each
    ↓ Create commit
    │
    ↓ Code changes + commit
    │
git push (after human review)
```

---

## Command Options

```bash
# Default implementation (40-80k tokens)
/implement [plan].md

# Dry-run - show what would change (30-40k tokens)
/implement --dry-run [plan].md
# Good for: Final verification before execution

# Staged - pause after each step (60-80k tokens)
/implement --staged [plan].md
# Good for: High-risk changes, manual verification

# Fast - syntax validation only (30-50k tokens)
/implement --fast [plan].md
# Warning: Less safety, only for low-risk changes
```

---

## After Implementation

### If Successful

1. **Review results** - All validations passed?
2. **Review commit** - Message clear?
3. **Push when ready** - `git push`
4. **Monitor** - Watch for issues (per plan)

### If Failed

1. **Don't panic** - Rollback procedure exists
2. **Assess failure** - Which step? Why?
3. **Decide action:**
   - Retry step (if minor issue)
   - Rollback (if major issue)
   - Return to planning (if plan was wrong)

### Post-Implementation Learning

**Capture for institutional memory:**

```markdown
## Implementation Learnings

**What worked well:**
- [Learning]

**What was harder than expected:**
- [Learning]

**What should be in future plans:**
- [Learning]

**Failure patterns encountered:**
- [Pattern]: [How it manifested]
```

---

## Best Practices

### Do
- Trust the approved plan
- Execute in specified order
- Validate after EVERY change
- Stop on validation failure
- Document what changed
- Review before pushing

### Don't
- Redesign during implementation
- Add improvements beyond plan
- Skip validation steps
- Continue after failure
- Push without review

---

## Validation Checkpoint Flow

**At each step:**
```
┌─────────────────────────────────────────┐
│ EXECUTE CHANGE                          │
└─────────────────┬───────────────────────┘
                  │
                  ↓
┌─────────────────────────────────────────┐
│ RUN VALIDATION (from plan)              │
└─────────────────┬───────────────────────┘
                  │
        ┌─────────┴─────────┐
        ↓                   ↓
   ┌────────┐          ┌────────┐
   │  PASS  │          │  FAIL  │
   └────┬───┘          └────┬───┘
        │                   │
        ↓                   ↓
   Continue to         STOP IMMEDIATELY
   next step           Report error
                       Await instructions
```

---

## Success Criteria

Implementation is successful when:

- All files created per plan
- All files modified per plan
- All files deleted per plan
- All validations pass
- Test results match expectations
- Rollback procedure verified
- Changes committed with clear message
- Ready to git push

---

**Ready?** Load your approved plan and I'll execute it precisely.
