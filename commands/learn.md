---
description: Extract reusable patterns from completed work for institutional memory
---

# /learn - Pattern Extraction & Learning

**Purpose:** Extract reusable patterns from work for future agents and developers.

**Why this matters:** Institutional memory compounds over time. Every problem solved is an opportunity to improve the system. Patterns prevent solving the same problem twice and share solutions across teams.

**When to use:**
- After completing implementation (work is done)
- After solving complex problem (capture solution)
- After discovering efficient pattern (share with team)
- Periodic retrospectives (extract learnings from recent work)

**Token budget:** 10-20k tokens (5-10% of context window)

**Output:** Pattern files in `.agents/learnings/` + INDEX.md update

**Note:** Output is repository-local. Each repo has its own learnings that are loaded at session start.

---

## Opus 4.5 Behavioral Standards

<pattern_specificity>
Extract patterns with concrete examples and file:line references. Generic patterns are not useful. Include before/after code, metrics, and specific implementation steps.
</pattern_specificity>

<avoid_duplication>
Before creating a new pattern, search existing patterns. Update existing patterns with new evidence rather than creating duplicates.
</avoid_duplication>

---

## The Learning Philosophy

**"Institutional memory compounds over time."**

Every problem solved is an opportunity to improve the system:
- Extract the pattern that worked
- Document why it worked
- Make it reusable for next time
- Prevent solving the same problem twice

---

## Quick Start

```bash
# After completing work
/learn [topic]

# Example
/learn redis-caching
# → Scans recent work for patterns
# → Shows candidates for extraction
# → Prompts for approval
# → Saves to .agents/learnings/patterns/
```

---

## Implementation (Guided Automation)

### Step 1: Analyze Recent Work

**I will scan for pattern indicators:**

```
Sources checked:
├─ Recent git commits (last 10)
├─ Modified files in current session
├─ Any bundles created today
└─ Conversation history (solutions discussed)

Pattern indicators:
├─ "Problem:" / "Solution:" discussions
├─ Code with before/after changes
├─ "What worked:" / "What didn't:" notes
├─ Metrics showing improvement
└─ Repeated approaches across files
```

### Step 2: Present Pattern Candidates

**I will show what I found:**

```
Scanning recent work for patterns...

Found 3 pattern candidates:

1. ✨ Connection Pooling Strategy (NEW)
   Category: implementation
   Problem: Redis connection exhaustion under load
   Solution: Increase pool size + circuit breaker
   Evidence: P95 latency 500ms → 50ms
   Files: config/redis.yaml:15, app/cache.go:34

2. ⚠️  Retry with Backoff (SIMILAR to existing)
   Category: implementation
   Similar to: patterns/implementation/exponential-backoff.md
   Recommend: Update existing pattern with new evidence

3. 🔄 Health Check Pattern (EXISTS)
   Category: debugging
   Already in: patterns/debugging/health-checks.md
   Recommend: Skip (no new information)

Extract patterns? [1,2,all,none]
```

### Step 3: Generate Pattern File

**For each approved pattern, I will:**

1. Create pattern file from template
2. Fill in all sections with extracted information
3. Add to appropriate category directory
4. Update INDEX.md with new entry

**Generated file example:**

```markdown
# Pattern: Redis Connection Pooling

**Category:** Implementation
**Domain:** Redis, Caching, Performance
**Difficulty:** Moderate
**Created:** 2025-11-23
**Author:** Session extraction
**Validated:** 1 time

## Problem Statement
Redis connections exhaust under burst traffic, causing 500ms+ latency
spikes and intermittent failures during peak hours (5pm daily).

## Solution Summary
Increase connection pool size to handle burst, add circuit breaker
to prevent cascade failures, implement health checks for early detection.

## When to Use
- High-traffic services using Redis
- Burst traffic patterns (predictable peaks)
- Connection limits approaching maximum
- Latency-sensitive applications

## When NOT to Use
- Low-traffic services (pool overhead not worth it)
- Memory-constrained environments (pools consume RAM)
- Single-connection architectures

## Implementation Steps
1. **Increase pool size**
   - File: `config/redis.yaml:15`
   - Change: `pool_size: 10` → `pool_size: 100`

2. **Add circuit breaker**
   - File: `app/cache.go:34`
   - Add: Circuit breaker wrapper around Redis calls

3. **Implement health check**
   - File: `app/cache.go:89`
   - Add: `/health/redis` endpoint that verifies connectivity

## Evidence
| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| P95 latency | 500ms | 50ms | 10x faster |
| Connection errors | 5%/day | 0.05%/day | 99% reduction |
| Peak traffic handled | 1x | 3x | 3x capacity |

## Gotchas
- **Pool size too high:** Exhausts memory. Calculate: pool_size * connection_memory < available_ram
- **Health check false positives:** Check actual connectivity, not just pool existence

## Related
- [Circuit Breaker](../architecture/circuit-breaker.md) - Combine with pooling
- [Exponential Backoff](exponential-backoff.md) - For retry logic

## Tags
#pattern #implementation #redis #caching #performance #connection-pooling
```

### Step 4: Update Catalog

**I will update `.agents/learnings/INDEX.md`:**

```markdown
### Implementation Patterns (1)
- [Redis Connection Pooling](implementation/redis-connection-pooling.md) - Handle burst traffic with pool + circuit breaker

## Recently Added
- 2025-11-23: Redis Connection Pooling (implementation)
```

### Step 5: Confirmation

```
✅ Pattern extracted successfully

📁 Created: .agents/learnings/patterns/implementation/redis-connection-pooling.md
📊 Updated: .agents/learnings/INDEX.md

Pattern can be found via:
  - Direct: cat .agents/learnings/patterns/implementation/redis-connection-pooling.md
  - Search: grep -r "connection pool" .agents/learnings/
```

---

## Pattern Categories

### Implementation Patterns
*How to solve specific problems*
- Save to: `.agents/learnings/patterns/implementation/`
- Examples: caching strategies, API integrations, data transformations

### Debugging Patterns
*How to diagnose issues*
- Save to: `.agents/learnings/patterns/debugging/`
- Examples: log analysis, performance profiling, error tracing

### Architecture Patterns
*How to structure systems*
- Save to: `.agents/learnings/patterns/architecture/`
- Examples: microservice boundaries, event sourcing, CQRS

### Automation Patterns
*What to automate and how*
- Save to: `.agents/learnings/patterns/automation/`
- Examples: CI/CD workflows, deployment scripts, monitoring

### Anti-Patterns
*What NOT to do*
- Save to: `.agents/learnings/anti-patterns/`
- Examples: common mistakes, failed approaches, gotchas

---

## Command Options

### Basic Learning
```bash
/learn [topic]
# Scans recent work, extracts patterns for topic
```

### Retrospective Learning
```bash
/learn --retrospective --since "2025-11-01"
# Scans last N days of commits for patterns
# Good for: Monthly retrospectives
```

### Failure Learning
```bash
/learn --failure "[what-went-wrong]"
# Documents what didn't work and why
# Creates anti-pattern entry
```

### Comparative Learning
```bash
/learn --compare "[approach-A]" "[approach-B]"
# Documents trade-offs between approaches
# Creates decision framework
```

### Update Existing
```bash
/learn --update [pattern-name]
# Adds new evidence to existing pattern
# Updates validation count
```

---

## Validation Rules

**Pattern is extractable when:**
- [ ] Problem is clear and reproducible
- [ ] Solution is generalizable (not one-off)
- [ ] Evidence exists (metrics, outcomes)
- [ ] Not already documented (or adds new info)

**Pattern is NOT extractable when:**
- One-time fix with no reuse potential
- Solution is too specific to context
- No evidence of effectiveness
- Duplicate of existing pattern (use `--update` instead)

---

## Integration with Laws

**Law #1: Extract Learnings**
- `/learn` is the primary mechanism
- Every session should consider: "What did I learn?"

**Law #2: Improve Self or System**
- Patterns improve the system
- Future agents benefit from past learnings

**Law #3: Document Context**
- Pattern template captures full context
- Why, what, how, evidence all documented

---

## Examples

### Example 1: After Implementation

```bash
# Just finished implementing Redis caching
/learn redis-caching

# Output:
Scanning recent work...

Found 2 pattern candidates:
1. ✨ Connection Pooling (NEW) - Extract? [y/n]
2. ✨ Cache Invalidation (NEW) - Extract? [y/n]

> y, y

✅ Created: patterns/implementation/redis-connection-pooling.md
✅ Created: patterns/implementation/cache-invalidation-pubsub.md
✅ Updated: patterns/INDEX.md
```

### Example 2: After Debugging Session

```bash
# Just debugged intermittent auth failures
/learn auth-debugging

# Output:
Found 1 pattern candidate:
1. ✨ Diagnosing Connection Pool Exhaustion (NEW)
   Symptoms: Intermittent failures, clustered timing
   Root cause: Pool exhaustion during burst
   Diagnostic steps: Check metrics → correlate timing → trace pools

Extract? [y/n]
> y

✅ Created: patterns/debugging/connection-pool-exhaustion.md
```

### Example 3: Retrospective

```bash
# Monthly learning extraction
/learn --retrospective --since "2025-11-01"

# Output:
Scanning 45 commits from 2025-11-01...

Found 5 pattern candidates:
1. ✨ Kustomize Base+Overlay (NEW)
2. ⚠️  GitOps Sync Waves (SIMILAR to existing)
3. ✨ Bundle Compression (NEW)
4. 🔄 YAML Validation (EXISTS)
5. ✨ Context Engineering 40% Rule (NEW)

Extract 1, 3, 5? Update 2? Skip 4? [y/n]
```

---

## Pattern Template Location

Full template available at:
```
.agents/learnings/PATTERN-TEMPLATE.md
```

Use for manual pattern creation or reference.

---

## Related Commands

- `/implement` - Where patterns get applied (learning comes after)
- `/bundle-save` - Save learning bundles for sharing
- `/bundle-prune` - Extract patterns before archiving bundles
- `/maintain` - Weekly pattern catalog review
- `/session-end` - Often triggers learning extraction
- `/retro` - Retrospective identifies patterns to extract

---

## Troubleshooting

### No Patterns Found
```
No pattern candidates found in recent work.

Try:
- Specify topic: /learn [specific-topic]
- Expand timeframe: /learn --since "2025-11-01"
- Check git log: Recent commits may not have pattern-worthy changes
```

### Similar Pattern Exists
```
Similar pattern found: patterns/implementation/existing-pattern.md

Options:
- Update existing: /learn --update existing-pattern
- Create new anyway: /learn --force [topic]
- Skip: Pattern already documented
```

---

**Ready to extract patterns? Run `/learn [topic]` after completing work.**
