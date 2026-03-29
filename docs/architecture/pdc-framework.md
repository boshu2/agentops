# PDC Framework: Prevent, Detect, Correct

**Purpose:** Systematic approach to handling issues across all workflow phases. PDC transforms reactive debugging into proactive quality control.

**Audience:** L2+ practitioners who need deeper understanding of defensive workflows.

---

## Overview

PDC (Prevent, Detect, Correct) is a three-phase framework for maintaining quality through the Research-Plan-Implement cycle.

```
┌──────────────────────────────────────────────────────────┐
│                    PREVENT                               │
│         Stop problems before they enter the system       │
├──────────────────────────────────────────────────────────┤
│                    DETECT                                │
│         Catch problems as soon as they occur             │
├──────────────────────────────────────────────────────────┤
│                    CORRECT                               │
│         Fix problems and prevent recurrence              │
└──────────────────────────────────────────────────────────┘
```

**Core insight:** Prevention is cheapest. Correction is most expensive. Invest early to reduce late-phase work.

---

## Prevent: Stop Issues Before They Start

Prevention creates conditions where problems cannot occur. This happens BEFORE work begins.

### Prevention Strategies

| Strategy | Example |
|----------|---------|
| **Fresh Context** | New session for /plan, new session for /implement |
| **Pre-flight Checks** | `git status` clean, dependencies installed |
| **Scope Boundaries** | "Only modify auth.ts, nothing else" |
| **Tracer Tests** | 5-line script to test API before building integration |
| **Clear Specifications** | "apps/config.yaml:15" not "update config" |

### Prevention Checklists

**Research Phase:**
- [ ] Problem statement written before research begins
- [ ] Token budget allocated (don't exceed 30% context)

**Planning Phase:**
- [ ] Research bundle loaded (not recreated from memory)
- [ ] Fresh context window (< 20% used)
- [ ] Approach confirmed from research (don't redesign)

**Implementation Phase:**
- [ ] Plan is approved (human sign-off)
- [ ] Workspace clean (`git status` shows no uncommitted changes)
- [ ] Rollback procedure documented

### Prevention Anti-Patterns

| Anti-Pattern | Prevention |
|--------------|------------|
| "I'll just wing it" | Write scope before starting |
| "Same session is fine" | Fresh session per phase |
| "I remember the plan" | Load the bundle, don't recreate |
| "Let's also improve..." | Stick to approved changes only |

---

## Detect: Catch Issues Early

Detection identifies problems as close to their origin as possible. The sooner you detect, the cheaper the fix.

### Detection Strategies

| Strategy | Example |
|----------|---------|
| **Validation After Every Change** | Run `make lint` after each file |
| **Test Commands, Not Claims** | Run the actual test, don't rely on memory |
| **Pattern Recognition** | "3+ attempts = Debug Spiral" |
| **Drift Detection** | "Is this change in the plan?" |
| **Context Monitoring** | Stop at 60% context, bundle state |

### Detection Signals by Loop

**Inner Loop (sec-min):**

| Signal | Response |
|--------|----------|
| Test failure | Stop, investigate, don't continue |
| 3+ fix attempts | Stop, reassess approach (Debug Spiral) |
| "While I'm here..." | Return to plan, no extras (Instruction Drift) |

**Middle Loop (hrs-days):**

| Signal | Response |
|--------|----------|
| Plan expanding | Split into multiple plans |
| Can't specify file:line | Return to research phase |
| Context > 60% | Bundle state, fresh session |

### The Tests Passing Lie

The most dangerous detection failure: AI claims tests pass but didn't run them.

```bash
# AI claims: "All tests pass!"
# YOU verify by running:
make test

# If output differs from claim:
# 1. STOP immediately
# 2. Investigate the discrepancy
# 3. Do not trust further claims in this session
```

---

## Correct: Fix Issues and Learn

Correction has two parts: fix the symptom and address the cause.

### Correction Strategies

| Strategy | Example |
|----------|---------|
| **Stop First** | On test failure: stop, don't continue |
| **Assess State** | `git status`, `git diff`, what's committed? |
| **Fix Forward or Rollback** | Minor: fix. Major: rollback. |
| **Document Learning** | Add to learnings, update patterns |
| **Verify Recovery** | Run validation again |

### Rollback Procedure

```bash
# 1. Stop all AI activity
# 2. Check git state
git status && git diff

# 3. Identify last known good state
git log --oneline -5

# 4. Execute rollback per plan
# (specific commands from plan.md)

# 5. Verify recovery
[validation command from plan]

# 6. Document incident
```

---

## PDC by Workflow Phase

| Phase | Prevent | Detect | Correct |
|-------|---------|--------|---------|
| **Research** | Clear problem statement, token budget | Scope creep, rabbit holes | Narrow focus, split research |
| **Planning** | Load bundle, confirm approach | Vague specs, scope expansion | Force file:line precision |
| **Implementation** | Approved plan, clean workspace | Test failures, instruction drift | Rollback per plan |

---

## Integration with Failure Patterns

| Pattern | Prevention | Detection | Correction |
|---------|------------|-----------|------------|
| **Tests Passing Lie** | Explicit test commands in plan | Run tests yourself | Rerun all tests, fresh session |
| **Debug Spiral** | Clear success criteria | 3+ attempt threshold | Stop, reassess, return to plan |
| **Instruction Drift** | Precise file:line specs | "Is this in the plan?" | Remove additions, stick to plan |
| **Context Amnesia** | Bundles preserve state | Re-read plan when uncertain | Load bundle, don't recreate |
| **Eldritch Horror** | Modular plan structure | Complexity growing beyond spec | Simplify, split, or redesign |

---

## Quick Reference

**Prevention:** Fresh context, bundle loaded, git status clean, scope defined

**Detection:** Validate after each change, stay within plan, context < 60%, no 3+ failures

**Correction:** STOP, assess (git status/diff), fix or rollback, verify, document

---

## Summary

PDC transforms reactive debugging into proactive quality control:

- **Prevent:** Invest in setup to eliminate entire classes of problems
- **Detect:** Catch issues early when fixes are cheap
- **Correct:** Fix the symptom and the cause, then document

The framework applies at every scale: individual changes, workflow phases, and project lifecycles.
