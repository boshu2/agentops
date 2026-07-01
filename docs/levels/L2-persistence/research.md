---
description: Explore codebase and save findings to .agents/research/
---

# /research (L2)

Investigates code and saves findings to `.agents/research/`. Use when you need persistent research that survives session clears.

## Usage

```
/research "your topic"
```

## What's Different from L1

At L2, research output persists:
- Findings saved to `.agents/research/YYYY-MM-DD-topic.md`
- Can be loaded in future sessions
- Builds institutional memory

## Steps

1. Claude searches for relevant files
2. Claude reads and analyzes code
3. Claude writes findings to `.agents/research/`
4. Summary shown in conversation

## Output

```
.agents/research/2025-01-15-authentication.md
```

## Example

```
You: /research "how does caching work"

Claude: I'll explore the caching system and save findings.

[Searches, reads files...]

**Findings saved to:** .agents/research/2025-01-15-caching.md

Summary:
- Redis-based cache in src/cache/
- TTL of 5 minutes for API responses
- Cache invalidation on writes
```

## Next

- `/post-mortem` to extract learnings from your session
- Future sessions: load research with "read .agents/research/..."
