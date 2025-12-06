---
description: Run vibe-check analysis and display results
allowed-tools: Bash(npx*), Read
argument-hint: "[--since 'time'] [--format json|markdown]"
---

# /vibe-check - Catch Spirals, Make Vibe Coding Fun

Analyze your git history to see how your session actually went.

## Usage

```bash
/vibe-check                        # Basic analysis
/vibe-check --since "2 hours ago"  # Check this session
/vibe-check --since "1 week ago"   # Last week
/vibe-check --format markdown      # Markdown output
```

## Execution

```bash
npx @boshu2/vibe-check $ARGUMENTS
```

## The 5 Metrics

| Metric | Target | What It Catches |
|--------|--------|-----------------|
| Iteration Velocity | >3/hour | Are you shipping or stuck? |
| Rework Ratio | <50% | Building or fixing the same thing? |
| Trust Pass Rate | >80% | Does code stick on first try? |
| Debug Spiral Duration | <30m | How long before you reset? |
| Flow Efficiency | >75% | Productive time vs thrashing? |

## Why It Matters

- **Catch spirals early** - If you're stuck >30min, step back
- **Honest feedback** - Git doesn't lie about what actually happened
- **Make it fun** - Gamify your sessions, chase ELITE rating

## Next Steps

- Rework Ratio high? → Validate assumptions before coding
- Debug spiral detected? → Break into smaller steps
- Trust Pass Rate low? → Use tracer tests first
- All ELITE? → Keep vibing
