## Quick Mode

Given `/post-mortem --quick "insight text"`:

### Quick Step 1: Generate Slug

Create a slug from the content: first meaningful words, lowercase, hyphens, max 50 chars.

### Quick Step 2: Write Learning Directly

**Write to:** `.agents/learnings/YYYY-MM-DD-quick-<slug>.md`

```markdown
---
type: learning
source: post-mortem-quick
date: YYYY-MM-DD
maturity: provisional
utility: 0.5
---

# Learning: <Short Title>

**Category**: <auto-classify: debugging|architecture|process|testing|security>
**Confidence**: medium

## What We Learned

<user's insight text>

## Source

Quick capture via `/post-mortem --quick`
```

This skips the full pipeline — writes directly to learnings, no council or backlog processing.

### Quick Step 3: Confirm

```
Learned: <one-line summary>
Saved to: .agents/learnings/YYYY-MM-DD-quick-<slug>.md

For deeper reflection, use `/post-mortem` without --quick.
```

**Done.** Return immediately after confirmation.
