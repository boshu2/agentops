# Confidence Scoring for Learnings

> Add weighted confidence to every learning, enabling automatic promotion and project scoping.

## Problem

Current learnings are binary (exists or doesn't). This makes it impossible to distinguish:
- A tentative pattern observed once (0.3) from a near-certain behavior confirmed across 5 sessions (0.9)
- A project-specific convention from a universal best practice
- A pattern that helped from one that was tried but not validated

## Confidence Scale

| Score | Meaning | Enforcement |
|-------|---------|-------------|
| 0.3 | Tentative — observed once, not validated | Suggest only, never auto-apply |
| 0.5 | Moderate — observed 2-3 times, some validation | Include in context, flag as "likely" |
| 0.7 | Strong — validated across multiple sessions | Auto-apply unless contradicted |
| 0.9 | Near-certain — confirmed, battle-tested | Always apply |

## Scoring Rules

### Initial Score
- Extracted from single session observation: **0.3**
- Extracted from explicit user correction: **0.5** (user-validated signal)
- Extracted from successful post-mortem finding: **0.5**
- Extracted from pattern seen in 2+ sessions: **0.6**

### Score Updates
- User confirms pattern works: **+0.1** (cap at 0.9)
- User contradicts/corrects pattern: **-0.2** (floor at 0.1, then mark for review)
- Pattern causes a bug or revert: **-0.3** (floor at 0.1)
- Pattern validated in new project: **+0.15**

### Decay
- Learnings not referenced in 30 days: **-0.05**
- Learnings not referenced in 90 days: **-0.1**
- Floor: 0.1 (never auto-delete, just deprioritize)

## Project Scoping

### Scope Assignment
Every learning gets a scope tag:

| Scope | When | Example |
|-------|------|---------|
| `project:<name>` | Pattern specific to project conventions | "This repo uses kebab-case for skill dirs" |
| `language:<lang>` | Language-specific pattern | "Go tests must use table-driven style" |
| `global` | Universal pattern | "Always validate input at system boundaries" |

### Auto-Promotion
When the same pattern (matched by semantic similarity, not exact text) appears in **2+ projects** with confidence >= 0.8:
1. Promote scope from `project:<name>` to `global`
2. Log promotion: `"Promoted learning '<title>' from project scope to global (seen in: <project1>, <project2>)"`
3. Archive project-scoped duplicates

### Isolation
When loading learnings for a session:
1. Load all `global` scope learnings
2. Load `project:<current-project>` learnings
3. Load `language:<detected-languages>` learnings
4. DO NOT load learnings from other projects (prevents cross-contamination)

## Integration with Compile

### Mine Phase
When extracting learnings, assign initial confidence score and scope:

```yaml
# In learning frontmatter
---
title: "Go tests use table-driven style in this repo"
confidence: 0.3
scope: project:agentops
observed_in:
  - session: "2026-03-21"
    context: "Observed in cli/internal/ test files"
---
```

### Grow Phase
During Grow, update confidence based on:
- Cross-reference with other learnings (similar patterns = boost)
- Check against current codebase (still true? = boost; contradicted? = decay)
- Check against recent session outcomes

### Defrag Phase
During Defrag, merge learnings with overlapping content:
- Keep the one with highest confidence
- Sum observation counts
- Expand scope if both project-scoped in different projects

## Integration with Forge

When `/forge` extracts learnings from transcripts:
1. Check if similar learning already exists (semantic match)
2. If yes: update confidence (+0.1) and add observation
3. If no: create new learning at 0.3 confidence

## Frontmatter Schema

```yaml
---
title: "<learning title>"
confidence: 0.5          # 0.1-0.9
scope: "project:agentops" # project:<name> | language:<lang> | global
observed_in:
  - session: "2026-03-21"
    context: "Brief description of observation"
promoted_from: null       # null or previous scope
last_referenced: "2026-03-21"
---
```
