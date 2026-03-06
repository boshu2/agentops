---
name: athena
description: '>'
---


# Athena — Active Knowledge Intelligence

Run the Mine → Grow → Defrag cycle to keep the knowledge flywheel healthy.

## What This Skill Does

The flywheel captures learnings reactively (via `$retro`, `$post-mortem`). Athena closes
the loop by actively mining for unextracted signal, validating existing learnings against
current code, synthesizing cross-domain insights, and cleaning up stale or duplicate
artifacts.

**When to use:** Before an evolve cycle, after a burst of development, or weekly.
Athena is non-destructive — it proposes changes without modifying existing learnings.

**Output:** `.agents/athena/YYYY-MM-DD-report.md`

## Execution Steps

### Step 1 — Mine: Extract Signal

Run mechanical extraction. Mine scans git history, `.agents/research/`, and code
complexity hotspots for patterns never captured as learnings.

```bash
ao mine --since 26h                    # default: all sources, last 26h
ao mine --since 7d --sources git,agents  # wider window, specific sources
```

Read `.agents/mine/latest.json` and extract: co-change clusters (files changing
together), orphaned research (unreferenced `.agents/research/` files), and complexity
hotspots (high-CC functions with recent edits).

**Fallback (no ao CLI):** Use `git log --since="7 days ago" --name-only` to find
recurring file groups. List `.agents/research/*.md` and check references in learnings.

### Step 2 — Grow: LLM-Driven Synthesis

This is the reasoning phase. Perform each sub-step using tool calls.

**2a. Validate Top Learnings**

Select the 5 most recent files from `.agents/learnings/`. For each:
1. Read the learning file
2. If it references a function or file path, use Read to verify the code still exists
3. Classify as: **validated** (matches), **stale** (changed), or **contradicted** (opposite)

**2b. Rescue Orphaned Research**

For each orphaned research file from mine output: read it, summarize the key insight
in 2-3 sentences, and propose as a new learning candidate with title and category.

**2c. Cross-Domain Synthesis**

Group mine findings by theme (e.g., "testing patterns", "CLI conventions"). For themes
with 2+ findings, write a synthesized pattern candidate capturing the common principle.

**2d. Gap Identification**

Compare mine output topics against existing learnings. Topics with no corresponding
learning are knowledge gaps. List each with: topic, evidence, suggested learning title.

### Step 3 — Defrag: Mechanical Cleanup

Run cleanup to find stale, duplicate, and oscillating artifacts.

```bash
ao defrag --prune --dedup --oscillation-sweep
```

Read `.agents/defrag/latest.json` and note: orphaned learnings (unreferenced, >30 days
old), near-duplicate pairs (>80% content similarity), and oscillating goals (alternating
improved/fail for 3+ cycles).

**Fallback:** `find .agents/learnings -name "*.md" -mtime +30` for stale files.
Check `.agents/evolve/cycle-history.jsonl` for alternating result patterns.

### Step 4 — Report

```bash
mkdir -p .agents/athena
```

Write `.agents/athena/YYYY-MM-DD-report.md`:

```markdown
# Athena Report — YYYY-MM-DD

## New Learnings Proposed
- [title]: [summary] (source: [research file or synthesis])

## Validations
- Validated: N | Stale: N (list files) | Contradicted: N (list with explanation)

## Knowledge Gaps
- [topic]: [evidence] → suggested learning: "[title]"

## Defrag Summary
- Orphaned: N | Duplicates: N | Oscillating goals: N

## Recommendations
1. [Actionable next step]
```

If `bd` is available, create issues for knowledge gaps:

```bash
bd add "[Knowledge Gap] <topic>" --label knowledge --label athena
```

Report findings to the user: proposed learnings, validation results, gaps, and
defrag actions recommended.

## Examples

**User says:** `$athena` — Full Mine → Grow → Defrag cycle, report in `.agents/athena/`.

**User says:** `$athena --since 7d` — Mines with a wider window (7 days).

**Pre-evolve warmup:** Run `$athena` before `$evolve` for a fresh, validated knowledge base.

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| `ao mine` not found | ao CLI not in PATH | Use manual fallback in Step 1 |
| No orphaned research | All research already referenced | Skip 2b, proceed to synthesis |
| Empty mine output | No recent activity | Widen `--since` window |
| Oscillation sweep empty | No oscillating goals | Healthy state — no action needed |

## Local Resources

### scripts/

- `scripts/validate.sh`


