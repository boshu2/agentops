---
name: athena
description: 'Active knowledge intelligence. Runs Mine → Grow → Defrag cycle. Mine extracts signal from git/.agents/code. Grow validates existing learnings against current reality, synthesizes cross-domain insights, traces provenance chains, and identifies knowledge gaps. Defrag cleans up. Triggers: "athena", "knowledge cycle", "mine and grow", "knowledge defrag", "clean flywheel", "grow knowledge".'
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

**Assign Initial Confidence.** For every new learning candidate extracted, assign a
confidence score based on evidence strength:

| Evidence | Score | Rationale |
|----------|-------|-----------|
| Single session observation | 0.3 | Anecdotal — seen once, may not generalize |
| Explicit user correction or post-mortem finding | 0.5 | Demonstrated — user-validated signal |
| Pattern observed in 2+ sessions | 0.6 | Repeated — likely real, not coincidence |
| Validated across multiple sessions or projects | 0.7 | Strong — safe to auto-apply |
| Battle-tested, never contradicted | 0.9 | Near-certain — always apply |

Also assign a **scope** tag: `project:<name>` (project-specific), `language:<lang>`
(language convention), or `global` (universal pattern). Default to `project:<current>`
unless the pattern is clearly language- or tool-universal.

Write the confidence and scope into the learning frontmatter:

```yaml
---
title: "Learning title"
confidence: 0.3
scope: project:agentops
observed_in:
  - session: "YYYY-MM-DD"
    context: "Brief description of observation"
---
```

### Step 2 — Grow: LLM-Driven Synthesis

This is the reasoning phase. Perform each sub-step using tool calls.

**Flywheel Health Diagnostic:** Compute σ (consistency), ρ (velocity), δ (decay) and report escape velocity status. See `references/flywheel-diagnostics.md` for measurement commands and remediation actions.

**2a. Validate Top Learnings and Adjust Confidence**

Select the 5 most recent files from `.agents/learnings/`. For each:
1. Read the learning file (including its `confidence` and `scope` frontmatter)
2. If it references a function or file path, use Read to verify the code still exists
3. Classify as: **validated** (matches), **stale** (changed), or **contradicted** (opposite)
4. **Adjust confidence** based on validation result:
   - Validated and still accurate: **+0.1** (cap at 0.9)
   - Stale but partially true: **no change** (mark for review)
   - Contradicted by current code: **-0.2** (floor at 0.1, flag for removal)
   - Pattern validated in a new project: **+0.15**
   - Not referenced in 30+ days: **-0.05** (time decay)
   - Not referenced in 90+ days: **-0.1** (time decay)

Update the learning file frontmatter with the new confidence score.

**Auto-Promotion Rule:** After confidence adjustment, check if the learning's
confidence is **> 0.7**. If so, and it is not already in MEMORY.md, promote it:
1. Add the learning's key insight to the relevant MEMORY.md topic file
2. Log: `"Promoted '<title>' to MEMORY.md (confidence: <score>)"`
3. If the same pattern appears in 2+ projects with confidence >= 0.8, promote its
   scope from `project:<name>` to `global`

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

## Scheduling / Auto-Trigger

Lightweight defrag (prune + dedup, no mining) runs automatically at session end
via the `athena-session-defrag.sh` hook. This keeps the knowledge store clean
without requiring manual `$athena` invocations. The hook:

- Fires on every `SessionEnd` event after `session-end-maintenance.sh`
- Skips silently if the `ao` CLI is not available
- Runs only `ao defrag --prune --dedup` (no `--oscillation-sweep` or mining)
- Has a 20-second timeout to avoid blocking session teardown

For a full Mine → Grow → Defrag cycle, invoke `$athena` manually.

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

## Reference Documents

- [references/confidence-scoring.md](references/confidence-scoring.md)
- [references/flywheel-diagnostics.md](references/flywheel-diagnostics.md)

## Local Resources

### scripts/

- `scripts/validate.sh`


