---
id: competitive-intelligence-feature-driver
type: learning
created_at: 2026-02-16
category: process
confidence: high
---

# Learning: Competitive Intelligence as Feature Driver

## What We Learned

A single `/council --deep` competitive analysis of a rival plugin (Compound Engineering Plugin) produced 3 specific, shippable improvements in one session. The pattern: council judges with product perspectives compare feature matrices → gap analysis reveals approachability deficit → gaps map directly to plan issues. Zero guesswork.

## Why It Matters

Most feature work starts from internal "what should we build?" brainstorming. Starting from "what do they have that we don't?" is faster, more focused, and produces work users actually want. The competitor had already validated the ideas with their users.

## Source

ag-tnp0 session: competitive analysis → 6-issue epic → shipped in 35 min

---

# Learning: CI Gate as Post-Mortem Prophecy

**ID**: L2
**Category**: testing
**Confidence**: high

## What We Learned

The tech-debt judge's #1 finding was "hardcoded counts will drift" — and CI failed within 60 seconds of push for exactly that reason. The existing `validate-doc-release.sh` gate caught README.md badge (36 vs 37) and docs/SKILLS.md + docs/ARCHITECTURE.md stale counts. The fix was trivial: `scripts/sync-skill-counts.sh` updated all 5 drift locations automatically.

## Why It Matters

This is the ideal validation loop: (1) judge predicts the failure mode, (2) CI mechanically catches it, (3) sync script auto-fixes it. The lesson isn't "we should have updated README" — it's that the CI gate worked exactly as designed and the tech-debt judge's warning was vindicated immediately.

## Source

ag-tnp0: CI failure on commit eb0f075, fixed by 68e420f

---

# Learning: Steal, Don't Invent

**ID**: L3
**Category**: architecture
**Confidence**: high

## What We Learned

The approachability problem didn't require novel solutions. The competitor had already solved it: (1) plain-English command names, (2) show 5 commands not 26, (3) quick knowledge capture. We adapted all three to our architecture with zero new infrastructure — just enriched description fields, reorganized existing tables, and added one 190-line skill.

## Why It Matters

The bias toward "building something new" wastes time when proven patterns exist. The simplest implementation (description field overloading) avoided schema changes, file migrations, and new dependencies. Ship the minimum change that closes the gap.

## Source

ag-tnp0: 262 insertions across 13 files, all markdown/YAML, no Go code changes

---

# Learning: Pre-Mortem Catches What Vibe Can't

**ID**: L4
**Category**: process
**Confidence**: high

## What We Learned

Pre-mortem caught 7 issues including `ao pool add` (a command that doesn't exist) and a line count contradiction (100 vs 200). These are plan-level bugs that vibe (code review) would never catch because the code would implement the wrong spec correctly. Pre-mortem validates the spec; vibe validates the implementation.

## Why It Matters

Plan-level bugs are the most expensive to fix — they produce correct code that does the wrong thing. This is the 7th consecutive epic where pre-mortem had positive ROI, proving it's not a ceremony but a genuine error-catching mechanism.

## Source

ag-tnp0 pre-mortem: 7 judges, 7 fixes applied before any code was written

---

# Learning: Full-Catalog Stale-on-Creation Anti-Pattern

**ID**: L5
**Category**: debugging
**Confidence**: high

## What We Learned

When creating a new reference file by extracting content from another file (quickstart Step 6 → full-catalog.md), the extracted content can be stale at creation time if the source was already outdated or if new items were added in the same wave. full-catalog.md was created from the old 24-skill table but the epic also added /learn, so it was immediately wrong.

## Why It Matters

Any "move content to new file" operation should include a completeness check as part of the task's acceptance criteria. The vibe caught it, but ideally the worker's validation metadata should have included a count assertion.

## Source

ag-tnp0 Issue 2: full-catalog.md created with stale content, caught in vibe --quick

---

# Learning: sync-skill-counts.sh Already Existed

**ID**: L6
**Category**: process
**Confidence**: high

## What We Learned

The CI gate AND the fix script both already existed (`tests/docs/validate-doc-release.sh` + `scripts/sync-skill-counts.sh`). The tech-debt judge recommended "add 3 validation scripts" not realizing the infra was already in place. The real gap was that crank workers don't run `sync-skill-counts.sh` after modifying skill counts — it should be part of the wave acceptance check for any wave that adds/removes skills.

## Why It Matters

Don't build what you already have. Check existing tooling before recommending new scripts. The fix was one command, not 90 minutes of new script writing.

## Source

ag-tnp0 CI failure: `scripts/sync-skill-counts.sh` fixed all 5 drift locations in one run
