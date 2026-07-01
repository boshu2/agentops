# Demo: /retro Session

A session showing learning extraction at the end of work.

---

## Before

- You've been working for a while
- Solved some problems, discovered patterns
- About to end the session or hit context limits

---

## Session

```
You: /retro

Claude: Reviewing this session to extract learnings...

**Session Summary:**

This session focused on debugging authentication timeouts.

**Problems Solved:**
1. JWT tokens expiring mid-request due to clock skew
   - Solution: Added 30-second buffer to expiry check
   - File: src/auth/jwt.ts:42

2. Redis connection pool exhaustion under load
   - Solution: Increased pool size from 10 to 50
   - File: config/redis.ts:8

**Patterns Discovered:**
- Always check token expiry with buffer time
- Connection pool size should be 5x expected concurrent users
- Log connection pool metrics for capacity planning

**Mistakes to Avoid:**
- Don't trust client-provided timestamps
- Don't cache tokens longer than their remaining TTL

**Writing to:** .agents/learnings/2025-01-15-auth-debugging.md

[Write] .agents/learnings/2025-01-15-auth-debugging.md

✓ Learnings saved.
```

---

## After

```
.agents/
├── research/
└── learnings/
    └── 2025-01-15-auth-debugging.md   # <- New file
```

---

## The Learnings File

```markdown
# Auth Debugging Session - 2025-01-15

## Problems Solved

### JWT Token Expiry Race Condition
- **Symptom**: Intermittent 401 errors under load
- **Root Cause**: Clock skew between servers
- **Solution**: Added 30-second buffer (src/auth/jwt.ts:42)

### Redis Pool Exhaustion
- **Symptom**: Connection timeouts after 10 concurrent requests
- **Root Cause**: Pool size too small
- **Solution**: Increased to 50 (config/redis.ts:8)

## Patterns

- Token expiry checks need buffer time
- Pool size = 5x concurrent users
- Monitor pool metrics

## Don't Repeat

- Never trust client timestamps
- Don't cache tokens past their TTL
```

---

## What You Learned

1. `/post-mortem` captures session knowledge before it's lost
2. Learnings are structured for future reference
3. Problems, patterns, and mistakes are all valuable
4. Run `/post-mortem` before context limits or session end
