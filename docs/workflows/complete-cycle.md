---
name: complete-cycle
description: Full Discovery → Implementation → Validation → Learn workflow
estimated_time: 2-4 hours (or multi-session)
phases: 5
---

# Complete Cycle Workflow

**Purpose:** Execute a full development cycle from research to learning

**When to use:**
- New features (complete implementation needed)
- Complex changes (multiple files, dependencies)
- Architectural changes (requires research and planning)
- Learning opportunities (extract patterns at end)

**Token budget:** 80-160k tokens across 2-4 sessions (40% per session)

---

## Workflow Phases

```
Phase 1: Discovery (20-30% context)
   ↓
Phase 2: Scope + risk (20-30% context)
   ↓
Phase 3: Implementation (20-40% context)
   ↓
Phase 4: Validate (5-10% context)
   ↓
Phase 5: Learn (5-10% context)
```

**Key principle:** Fresh context per phase (prevents degradation)

---

## How the cycle runs under `ao rpi`

The five conceptual phases above map onto the three executable RPI phases
the CLI actually drives: **Discovery → Implementation → Validation**. Run the
full phased cycle with:

```bash
ao rpi phased <goal>      # Discovery → Implementation → Validation
```

(`cli/cmd/ao/rpi_phased.go`, `Use: "phased <goal>"`.)

**Fresh context per phase is enforced, not aspirational.** Each phase runs in
its own context window and, before finishing, writes a bounded summary to
`.agents/rpi/phase-N-summary.md` (max ~500 tokens — the
`PhaseSummaryInstruction` contract in `cli/internal/rpi/phased_context.go`).
The next phase reads that file as its carry-forward, so Implementation never
inherits Discovery's raw working set — only the distilled summary. This is the
mechanism behind "fresh context per phase (prevents degradation)."

**Multi-cycle loops and live sessions:**
- Bounded multi-cycle execution (gate / landing / kill-switch) runs through
  `cli/internal/daemon/rpi_executor.go`.
- Recurring or scheduled runs submit `RPIRunJobSpec` / `RPIPhaseJobSpec`
  (`cli/internal/daemon/rpi_jobs.go`) to the daemon; job lifecycle is
  Submit → Claim → Heartbeat → Complete/Fail.
- Live phased sessions surface through the Gas City bridge
  (`cli/cmd/ao/gc_bridge.go`, `gc_events.go`, `rpi_phased_gc.go`).

> Deprecated RPI files still exist on disk but are **not** the current path —
> do not script against `rpi_loop_supervisor.go`, `rpi_parallel.go`,
> `rpi_c2_events.go`, or `rpi_phased_tmux.go`.

---

## Closing the loop (Knowledge Flywheel)

Phase 5 (Learn) is where the cycle feeds itself. Learnings written to
`.agents/learnings/` are read back by `ao inject` / `ao lookup`, which append a
`CitationEvent` to `.agents/ao/citations.jsonl`. Promotion flows
`.agents/learnings/ → .warmind/pool/staged/ → .warmind/learnings/` (team canon)
by tier: Gold (≥0.8, auto after 24h), Silver (≥0.5 + 1 external citation),
Bronze (3 external citations).

```bash
ao flywheel close-loop    # promote + decay + contradict + emit FlywheelMetrics
ao flywheel status        # escape-velocity check
```

The compounding equation (`dK/dt = I − δK + σ·ρ·K − B`) and the full
`FlywheelMetrics` model are documented in `docs/the-science.md` — that is their
canonical home; cite it rather than restating the form here.

---

## Phase 1: Research

**Goal:** Understand the system deeply before planning

**Commands:**
```bash
Read CLAUDE.md                    # Load repository primer context
/research "[topic]"               # Or /research-multi for 3x speedup
/bundle-save [topic]-research     # Compress findings
```

**Agents used:**
- code-explorer: Map code structure
- doc-explorer: Find relevant documentation
- history-explorer: Mine git history

**Output:** Research bundle (500-1k tokens)

**Success criteria:**
✅ System understood
✅ Similar implementations found
✅ Constraints identified
✅ Approach recommended

**Token budget:** 40-60k tokens (20-30%)

---

## Phase 2: Plan

**Goal:** Specify EVERY change with file:line precision

**Commands:**
```bash
# New session (fresh context)
Read CLAUDE.md                    # Reload primer context if needed
/bundle-load [topic]-research     # Load research (1k tokens)
/plan [topic]-research            # Create detailed plan
/bundle-save [topic]-plan         # Compress plan
```

**Agents used:**
- spec-architect: Design detailed specifications
- validation-planner: Create test strategy
- risk-assessor: Identify and mitigate risks

**Output:** Plan bundle (1-2k tokens)

**Success criteria:**
✅ All files specified
✅ Exact changes detailed (file:line, before/after)
✅ Test strategy defined
✅ Implementation order clear
✅ Risks assessed and mitigated

**Token budget:** 40-60k tokens (20-30%)

---

## Phase 3: Implement

**Goal:** Execute approved plan mechanically

**Commands:**
```bash
# New session (fresh context)
Read CLAUDE.md                    # Reload primer context if needed
/bundle-load [topic]-plan         # Load plan (1.5k tokens)
/implement [topic]-plan           # Execute changes

# If context approaches 40% mid-implementation:
# Auto-checkpoint: [topic]-implementation-progress.md

# Resume in next session:
/implement --resume [topic]-implementation-progress
```

**Agents used:**
- change-executor: Apply changes mechanically
- test-generator: Create test cases
- continuous-validator: Validate continuously

**Output:** Working implementation + commit

**Success criteria:**
✅ All changes applied
✅ All validation passed
✅ Build succeeds
✅ Tests pass
✅ Ready to commit

**Token budget:** 40-80k tokens (20-40%), may span 2 sessions

---

## Phase 4: Validate

**Goal:** Comprehensive quality check before deployment

**Commands:**
```bash
/vibe recent                      # Use --deep for more judges (or --quick for fast inline check)
```

**Agents used:**
- continuous-validator: Run full validation suite

**Output:** Validation report

**Success criteria:**
✅ Syntax checks pass
✅ All tests pass
✅ Coverage meets threshold
✅ No security vulnerabilities
✅ Performance acceptable

**Token budget:** 10-20k tokens (5-10%)

---

## Phase 5: Learn

**Goal:** Extract reusable patterns for institutional memory

**Commands:**
```bash
/retro --quick [topic]            # Extract patterns
/bundle-save [topic]-learning     # Share learnings
```

**Agents used:**
- (Learning analysis - pattern extraction)

**Output:** Learning bundle + pattern catalog update

**Success criteria:**
✅ Pattern documented
✅ Evidence included
✅ Discoverable (tags, index)
✅ Reusable (generalizes)

**Token budget:** 10-20k tokens (5-10%)

---

## Multi-Session Strategy

**Session 1: Research**
```bash
Read CLAUDE.md
/research "[topic]"
/bundle-save [topic]-research
# Context: 40-60k (20-30%)
```

**Session 2: Plan**
```bash
Read CLAUDE.md
/bundle-load [topic]-research
/plan [topic]-research
/bundle-save [topic]-plan
# Context: 40-60k (20-30%)
```

**Session 3: Implement (Part 1)**
```bash
Read CLAUDE.md
/bundle-load [topic]-plan
/implement [topic]-plan
# Context approaches 40% → auto-checkpoint
# Saved: [topic]-implementation-progress.md
```

**Session 4: Implement (Part 2) + Validate + Learn**
```bash
/implement --resume [topic]-implementation-progress
# Complete implementation
/vibe recent
/retro --quick [topic]
# Context: 40-70k total (20-35%)
```

**Total:** 4 sessions, ~160-200k tokens, sustained quality

---

## When to Use vs Alternatives

### Use Complete Cycle when:
✅ Change is complex (multiple files, dependencies)
✅ Risk is significant (critical system, production)
✅ Learning is valuable (new pattern to extract)
✅ Time is available (2-4 hours or multi-day)

### Use Quick Fix instead when:
- Change is simple (1-2 files, low risk)
- Time is critical (need fix NOW)
- Pattern is well-known (done 10+ times)

### Use Debug Cycle instead when:
- Problem needs investigation first
- Root cause unknown
- Fix approach unclear

---

## Success Metrics

**Complete cycle succeeds when:**
✅ Research → comprehensive understanding
✅ Plan → detailed specification
✅ Implement → working solution
✅ Validate → all checks pass
✅ Learn → pattern extracted

**Quality indicators:**
- Research was thorough (found constraints early)
- Plan was complete (implementation felt mechanical)
- Implementation was clean (few surprises)
- Validation passed first time (no rework)
- Learning is reusable (generalizes to other problems)

---

## Example: Complete Cycle for Redis Caching

**Session 1: Research**
```bash
Read CLAUDE.md
/research "Redis connection pool exhaustion under burst traffic"
# Agent explores:
# - config/redis.yaml (current settings)
# - app/cache.go (how Redis is used)
# - Similar implementations (other services with pooling)
# - Git history (past connection issues)
#
# Findings:
# - Pool size: 10 (too small for burst)
# - Similar: auth-service increased to 100 (worked)
# - Pattern: Add health checks + circuit breaker
#
/bundle-save redis-pooling-research
# Compressed: 60k → 1.2k tokens
```

**Session 2: Plan**
```bash
Read CLAUDE.md
/bundle-load redis-pooling-research
/plan redis-pooling-research
# Agent specifies:
# 1. config/redis.yaml:15 - pool_size: 10 → 100
# 2. app/cache.go:34 - Add pool initialization
# 3. app/cache.go:89 - Add health check endpoint
# 4. tests/cache_test.go:1-50 - Add pool tests
# 5. Validation: go test ./app/... && load test
#
/bundle-save redis-pooling-plan
# Compressed: 50k → 1.5k tokens
```

**Session 3: Implement + Validate**
```bash
Read CLAUDE.md
/bundle-load redis-pooling-plan
/implement redis-pooling-plan
# Agent executes:
# ✅ Edit config/redis.yaml:15
# ✅ Edit app/cache.go:34
# ✅ Edit app/cache.go:89
# ✅ Create tests/cache_test.go
# ✅ Validate: All tests pass
#
/vibe recent
# ✅ Syntax: PASSED
# ✅ Tests: 48/48 passed, 89% coverage
# ✅ Security: No issues
# ✅ Performance: P95 500ms → 50ms (10x improvement!)
#
# Commit with full context/solution/learning/impact
```

**Session 4: Learn**
```bash
/retro --quick redis-pooling-implementation
# Agent extracts:
# Pattern: Redis Connection Pooling for Burst Traffic
# - When: High-traffic services with burst patterns
# - Solution: Increase pool size, health checks, circuit breaker
# - Evidence: 10x latency improvement, handled 3x traffic
#
/bundle-save redis-pooling-learning
# Pattern added to: .agentops/patterns/implementation/
```

**Total time:** 3 hours across 4 sessions
**Result:** Problem solved + Pattern extracted + Institutional memory improved

---

## Related Workflows

- **quick-fix:** Simplified version (skip research/planning)
- **debug-cycle:** Research-focused version (root cause investigation)
- **knowledge-synthesis:** Learning-focused version (extract from multiple sources)

---

**Start complete cycle with:** Read `CLAUDE.md` to load primer context → describe your task
