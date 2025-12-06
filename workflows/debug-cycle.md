---
name: debug-cycle
description: Systematic debugging from symptoms to root cause to fix
estimated_time: 1-3 hours
phases: 4
---

# Debug Cycle Workflow

**Purpose:** Systematic investigation and resolution of bugs or issues

**When to use:**
- Production incidents (unknown root cause)
- Intermittent failures (hard to reproduce)
- Performance degradation (needs profiling)
- Integration issues (multiple components)

**Token budget:** 60-120k tokens across 1-3 sessions

---

## Workflow Phases

```
Phase 1: Isolate (10-20% context)
   ↓
Phase 2: Locate (15-25% context)
   ↓
Phase 3: Fix (15-25% context)
   ↓
Phase 4: Verify & Learn (10-15% context)
```

---

## Phase 1: Isolate

**Goal:** Reproduce the issue and isolate the component

**Commands:**
```bash
Read CLAUDE.md
/research "isolate [symptom]"
```

**Activities:**
- Reproduce the issue
- Identify affected components
- Gather logs, metrics, traces
- Narrow down scope

**Output:** Isolation report (which component/function)

---

## Phase 2: Locate

**Goal:** Find exact root cause

**Commands:**
```bash
/research "locate root cause in [component]"
# Use history-explorer agent for git blame, past fixes
```

**Activities:**
- Read relevant code
- Trace execution path
- Check git history for related changes
- Identify the exact bug location

**Output:** Root cause identification (file:line)

---

## Phase 3: Fix

**Goal:** Implement fix

**Commands:**
```bash
/plan [issue]-root-cause
/implement [issue]-plan
```

**Activities:**
- Design fix (may be simple or complex)
- Implement fix
- Add tests to prevent regression
- Validate fix resolves issue

**Output:** Working fix + regression test

---

## Phase 4: Verify & Learn

**Goal:** Ensure fix works and extract learning

**Commands:**
```bash
/validate
/learn [issue]-fix
```

**Activities:**
- Full validation
- Deploy to staging/production
- Monitor for recurrence
- Extract debugging pattern

**Output:** Validated fix + debugging pattern

---

## Example: Debug Auth Failures

```bash
# Phase 1: Isolate
Read CLAUDE.md
/research "isolate intermittent auth failures"
# Finding: Failures at 5pm daily, connection timeouts
# Component: Redis connection pool

# Phase 2: Locate
/research "locate root cause in Redis pool"
# Root cause: config/redis.yaml:15 - pool_size: 10 (too small)

# Phase 3: Fix
/plan redis-pool-fix
/implement redis-pool-fix
# Fix: Increase pool size to 100, add health checks

# Phase 4: Verify
/validate
# ✅ Load test: No failures at 3x traffic
/learn redis-pool-debugging
# Pattern: Intermittent failures + time correlation = resource exhaustion
```

---

**Start debug cycle with:** `Read CLAUDE.md` → describe symptoms
