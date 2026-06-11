# Skill corpus token/efficiency audit — 2026-06-10

Read-only audit of `~/dev/agentops/skills/` (166 skills with SKILL.md). Goal: cut slop / token waste, boost efficiency.

## Headline verdict

**There is no prose-slop problem.** The corpus is already disciplined:
- Descriptions: median **125 chars**, max **194**, total **~19.1K chars (~4.8K tokens)** across 166 skills — the always-on router tax, and it's lean.
- Filler/hedge phrases ("leverage the power", "in this guide we", "seamlessly", etc.): **1 hit** across the entire corpus.

The real token waste is **structural**, in two places:

## Lever 1 — oversized on-invoke bodies (move detail → references/)

14 skills exceed the 400-line modularization threshold. The whole body loads **every time the skill fires**; the tail is command-catalog detail that belongs in `references/` (loaded on demand). These are dense, not padded — but a ~150-line kernel + on-demand reference beats a 700-line body on every invoke.

| Lines | Skill | Body shape | Fix |
|------:|-------|-----------|-----|
| 757 | system-performance-remediation | runbook: kill hierarchy + copy-paste command catalog | split catalog → `references/COMMANDS.md`, keep kernel |
| 656 | ntm | command matrix + tier tables (already has references/) | thin body to selection-kernel; push command detail down |
| 524 | cass | **has in-body Table of Contents** — doc-sized smell | clearest split candidate; TOC = it's a doc, not a kernel |
| 503 | evolve | loop spec + checkpoints | move checkpoint detail to references/ |
| 484 | vibing-with-ntm | operator cards (already has OPERATOR-CARDS.md ref) | dedup body vs the 1288-line reference |
| 458 | vibe | embeds multi-language standards | references/ already carry 6 *-standards.md; body should point, not repeat |
| 454–408 | refactor, goals, scaffold, bug-hunt, test, codebase-pattern-extraction, gh-triage-ru, research | mixed | per-skill kernel/reference split |

**No-references-dir + big body** (nowhere to offload — these need a references/ created):
- rust-ub-risk-audit (325), rust-sqlite-cli-architecture (304)

## Lever 2 — overlap clusters (always-on router tax + mis-routing)

Skill *count* itself is the always-on cost (every description loads every session). Near-duplicate families inflate the list and create trigger ambiguity (the agent picks the wrong sibling):

| Cluster | Members | Action |
|---------|--------:|--------|
| beads/bead | 6 (beads, beads-br, beads-bv, beads-workflow, bead-completion-audit, bead-tracker-migration) | audit for merge; sharpen trigger boundaries so router doesn't guess |
| codebase | 6 (audit, archaeology, pattern-extraction, report, briefing-report, risk-audit) | heavy semantic overlap — consolidate or cross-link |
| agy | 6 | confirm all are live (newer cluster) |
| cc | 5 (cc-hooks, cc-cron-ticks, cc-loop-driver, cc-subagents, cc-worktree-isolation) | likely all justified; verify |
| codex | 4 · pr | 3 · gh | 3 · rust | 6 | review for redundancy |

## What NOT to do

- Don't rewrite descriptions — they're already tight; churn with no gain.
- Don't chase "slop phrases" — the corpus is clean.
- Don't bulk-delete skills without confirming live use; the always-on cost per skill is ~30 tokens, so consolidation value is in *clarity/routing*, not raw tokens.

## Recommended sequencing

1. **Highest ROI:** modularize cass (has a TOC), system-performance-remediation, ntm — biggest on-invoke bodies, clearest catalog/kernel split.
2. Dedup vibe / vibing-with-ntm bodies against their existing references (they already pay for references but repeat content in the body).
3. Trigger-boundary pass on the beads(6) + codebase(6) clusters.

Each is a closable bead. Auditor is read-only — these are filing recommendations, not applied changes.
