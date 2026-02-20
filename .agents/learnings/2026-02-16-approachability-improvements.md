---
id: competitive-analysis-pipeline
type: learning
created_at: 2026-02-16
category: process
confidence: high
---

# Learning: Competitive Analysis Pipeline

## What We Learned

Competitive analysis via `/council --deep` directly feeds actionable implementation backlogs. Running a structured multi-model review of a competitor's feature set produces a prioritized gap analysis that maps 1:1 to plan issues.

## Why It Matters

Evidence-based roadmap planning replaces feature guessing. The competitor analysis for ag-tnp0 identified the exact three interventions (aliases, tiering, /learn) that shipped.

## Source

ag-tnp0: Compound Engineering Plugin competitive analysis → 6-issue epic

---

# Learning: Description Field Overloading for Aliases

**ID**: L2
**Category**: architecture
**Confidence**: high

## What We Learned

Adding verb aliases to existing `description:` frontmatter fields is sufficient for Claude Code skill discovery. No new infrastructure (alias registry, mapping files) needed. Claude Code matches skills against descriptions, so enriching descriptions = enabling natural language discovery.

## Why It Matters

Avoids schema changes and migration overhead. The simplest solution that works.

## Source

ag-tnp0 Issue 1: 7 skills aliased via description enrichment only

---

# Learning: File Boundaries Table Prevents Parallel Conflicts

**ID**: L3
**Category**: process
**Confidence**: high

## What We Learned

Including an explicit "File Boundaries" table in plans (Issue → Files Modified → Files Created) enables zero-conflict parallel execution. ag-tnp0 Wave 1 had 4 parallel workers with zero file conflicts because the plan proved disjointness.

## Why It Matters

Pre-mortem spec-completeness judges can validate file disjointness mechanically. Workers don't need to coordinate if boundaries are proven at plan time.

## Source

ag-tnp0 plan, Wave 1: 4 workers, 0 conflicts

---

# Learning: CLI Flag Verification Pattern

**ID**: L4
**Category**: debugging
**Confidence**: high

## What We Learned

Always run `<cmd> --help` during planning for any CLI invoked in worker tasks. Assumed flags cause task failures. This is the 4th occurrence: `codex -q` (ag-uj4), `codex review -m/-o/-s` (ag-3b7), `ao ratchet --format` (ag-oke), `bd edit --body` (ag-tnp0).

## Why It Matters

Workers copy flags exactly from plans. Unverified flags propagate to every worker in the wave. Pre-mortem judges lack CLI context to catch this.

## Source

ag-tnp0: bd edit --body doesn't exist, had to skip issue body update

---

# Learning: Pre-Mortem Graduation (7/7 ROI)

**ID**: L5
**Category**: process
**Confidence**: high

## What We Learned

7 consecutive epics with positive pre-mortem ROI (ag-oke through ag-tnp0). Pre-mortem is now mandatory for 3+ issue epics via crank hook enforcement.

## Why It Matters

Each pre-mortem costs ~2 min but catches real implementation bugs (wrong CLI commands, contradictory specs, missing conformance checks). ROI is consistently positive.

## Source

ag-tnp0 pre-mortem: caught ao pool command error, line count contradiction, 5 other findings
