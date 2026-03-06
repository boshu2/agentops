---
name: retro
description: 'Quick-capture a learning. For full retrospectives, use $post-mortem. Trigger phrases: "quick learning", "capture lesson", "retro quick".'
---


# Retro Skill

**YOU MUST EXECUTE THIS WORKFLOW. Do not just describe it.**

Quick-capture a learning to the knowledge flywheel. For comprehensive retrospectives with backlog processing, activation, and retirement, use `$post-mortem`.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--quick "text"` | off | Quick-capture a single learning directly to `.agents/learnings/` without running a full retrospective. |

## Quick Mode

Given `$retro --quick "insight text"` or `$retro "insight text"`:

### Quick Step 1: Generate Slug

Create a slug from the content: first meaningful words, lowercase, hyphens, max 50 chars.

### Quick Step 2: Write Learning Directly

**Write to:** `.agents/learnings/YYYY-MM-DD-quick-<slug>.md`

```markdown
---
type: learning
source: retro-quick
date: YYYY-MM-DD
---

# Learning: <Short Title>

**Category**: <auto-classify: debugging|architecture|process|testing|security>
**Confidence**: medium

## What We Learned

<user's insight text>

## Source

Quick capture via `$retro --quick`
```

This skips the pool pipeline — writes directly to learnings, not `.agents/knowledge/pending/`.

### Quick Step 3: Confirm

```
Learned: <one-line summary>
Saved to: .agents/learnings/YYYY-MM-DD-quick-<slug>.md

For comprehensive knowledge extraction, use `$post-mortem`.
```

**Done.** Return immediately after confirmation.

---

## Full Retrospective

For comprehensive knowledge extraction with backlog processing and activation, use:

```
$post-mortem <target>
```

The `$post-mortem` skill includes all retro functionality plus:
- Council validation of completed work
- Backlog deduplication and scoring
- Auto-promotion to MEMORY.md
- Stale learning retirement
- Harvest next work items

---

## Examples

### Quick Capture

**User says:** `$retro --quick "macOS cp alias prompts on overwrite — use /bin/cp to bypass"`

**What happens:**
1. Agent generates slug: `macos-cp-alias-overwrite`
2. Agent writes learning to `.agents/learnings/2026-03-03-quick-macos-cp-alias-overwrite.md`
3. Agent confirms: `Learned: macOS cp alias prompts — use /bin/cp`

**Result:** Learning captured in 5 seconds, indexed for future sessions.

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Learning too generic | Surface-level capture | Be specific: "auth tokens expire after 1h" not "learned about auth" |
| Duplicate learnings | Same insight captured twice | Check existing learnings with grep before writing |
| Need full retrospective | Quick capture isn't enough | Use `$post-mortem` for comprehensive extraction + processing |

## Local Resources

### scripts/

- `scripts/validate.sh`


