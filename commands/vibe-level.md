---
description: Determine appropriate Vibe Level for a task
allowed-tools: Read
argument-hint: "[task description]"
---

# /vibe-level - How Much Should You Trust AI Here?

Quick classification to set expectations before you start.

## Usage

```bash
/vibe-level "implement new API endpoint"
/vibe-level "fix typo in README"
/vibe-level "redesign authentication system"
```

## The Levels

| Level | Trust | Verify | Examples |
|-------|-------|--------|----------|
| 5 | 95% | Final only | Formatting, linting |
| 4 | 80% | Spot check | Boilerplate, copy edits |
| 3 | 60% | Key outputs | Features, CRUD |
| 2 | 40% | Every change | Integrations, APIs |
| 1 | 20% | Every line | Architecture, security |
| 0 | 0% | N/A | Research, novel problems |

## Classification

For task: **$ARGUMENTS**

Ask yourself:
1. **Reversibility** - Can I easily undo this?
2. **Blast Radius** - What breaks if it's wrong?
3. **Complexity** - How much context is needed?

## Output

- Recommended level (0-5)
- How much to verify
- Whether to use tracer tests (Level 1-2)

## The Point

Set the right expectations. L5 = let it rip. L1 = check every line. Knowing which is which prevents both over-checking and under-checking.
