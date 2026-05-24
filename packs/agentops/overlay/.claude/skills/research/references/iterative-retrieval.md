# Iterative Retrieval Pattern

> Progressive context refinement for subagents. Solves "I don't know what I need to know."

## Problem

When spawning research or explore agents, the initial query often misses critical context because:
- The agent doesn't know the codebase's naming conventions
- Related features use unexpected terminology
- Key context lives in files the agent wouldn't think to search

Flat keyword search returns either too much noise or misses relevant files.

## Solution: 4-Phase Iterative Loop

### Phase 1: DISPATCH — Broad keyword search
```
Search for: <topic>
Use 3-5 keyword variants:
  - Exact term: "<topic>"
  - Synonyms: "<synonym1>", "<synonym2>"
  - Implementation terms: "<likely-function-name>", "<likely-file-pattern>"
```

### Phase 2: EVALUATE — Score relevance (0-1)
For each result, assign a relevance score:

| Score | Meaning | Action |
|-------|---------|--------|
| 0.8-1.0 | Directly implements target feature | Read fully, extract details |
| 0.5-0.7 | Contains related patterns or interfaces | Skim for cross-references |
| 0.2-0.4 | Tangentially related | Note for later if gaps remain |
| 0.0-0.2 | Not relevant | Discard |

### Phase 3: REFINE — Extract new keywords
From high-relevance files (0.5+), extract:
- Function/class names referenced but not yet searched
- Import paths pointing to unexplored modules
- Config keys or env vars mentioned
- Error messages or log strings (grep targets)

Add these as new search terms.

### Phase 4: LOOP — Repeat max 3 cycles
```
Cycle 1: Broad search → find core files → extract new terms
Cycle 2: Targeted search with extracted terms → find related files → more terms
Cycle 3: Fill remaining gaps → verify completeness
```

**Stop early if:**
- No new high-relevance results in a cycle
- All critical questions answered
- Context budget reached

## Integration with /research

In Step 3 (Launch Explore Agent), add iterative retrieval to the exploration prompt:

```
Use iterative retrieval:
1. Start with broad keyword search for "<topic>"
2. Score each result 0-1 for relevance
3. From files scoring 0.5+, extract new search terms
4. Search with new terms (max 3 cycles)
5. Report: files found per cycle, relevance scores, final coverage
```

## Integration with /swarm

When spawning parallel workers that need codebase context:

```
Before implementation, run 1-2 retrieval cycles to gather context:
- Search for files related to your task
- Read the highest-relevance files (0.7+)
- Note patterns and conventions from those files
- Then implement following those patterns
```

This prevents workers from reinventing patterns that already exist in the codebase.

## Example: Researching "authentication"

**Cycle 1:**
- Search: "auth", "authentication", "login", "session"
- Hits: `auth/middleware.go` (0.9), `auth/token.go` (0.8), `config/auth.go` (0.6), `README.md` (0.2)
- New terms from hits: `ValidateToken`, `SessionStore`, `JWT_SECRET`

**Cycle 2:**
- Search: "ValidateToken", "SessionStore", "JWT_SECRET"
- Hits: `store/session.go` (0.9), `config/env.go` (0.7), `test/auth_test.go` (0.8)
- New terms: `RefreshToken`, `store.NewRedisStore`

**Cycle 3:**
- Search: "RefreshToken", "RedisStore"
- Hits: `auth/refresh.go` (0.9), `store/redis.go` (0.8)
- No new high-relevance terms → STOP

**Result:** Complete auth system map in 3 cycles vs flat search that would miss `store/` and `config/env.go`.

## Anti-Patterns

| Anti-Pattern | Why It Fails | Fix |
|-------------|-------------|-----|
| Searching entire repo with no scope | Context overload, slow | Always scope to directories |
| Only 1 keyword | Misses synonym usage | Start with 3-5 variants |
| No relevance scoring | Reads everything equally | Score and prioritize |
| >3 cycles | Diminishing returns | Stop at 3, report gaps |
| Ignoring low-relevance files | Sometimes tangential files have key context | Note them, revisit if gaps remain |
