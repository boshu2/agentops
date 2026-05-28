# 2026-05-19 — Coherent-arc PR rule validation (the rule dogfooded itself)

> **TL;DR.** The "1 PR per coherent arc" rule replacing "1 PR per scenario" (shipped 2026-05-19 as PR #348, `soc-1lp1`) cut the projected PR count for the next ~6 hours of follow-on drift work from ~12 down to 6. The rule's own first full-gate run is what surfaced the drift it then helped land. Two consecutive self-corrections inside the session each tightened the discipline rather than weakened it. This is the durable record of that result.

## Context

PR #348 replaced the workflow default "one scenario per PR (carve-out: type=chore #trivial)" with "one PR per coherent arc — a closable bead or small-epic slice with a single rollback semantic."

The session that immediately followed (`/evolve` cycles 1-8, see `.agents/learnings/2026-05-19-*.md`) was the first real-world exercise of the rule. The arc structure:

| PR | Arc | Coherent because... |
|---|---|---|
| #348 (`soc-1lp1`) | Workflow rule + ship-loop SKILL twins | doc-change ship-or-revert as one |
| #349 (`soc-j026`) | 6 pre-existing shellcheck warnings in 6 files | uniform-mechanical, same rollback |
| #350 (`soc-nmhp`) | Eval-canary path filter (first attempt) | 1 logic site |
| #351 (`soc-g3ei`) | CHANGELOG backfill for session | docs regen |
| #352 (`soc-98o8`) | Eval-canary path filter (corrected fix) | 1 logic site (self-correction) |
| #353 (`soc-ruvk`) | 3 more pre-existing shellcheck warnings | same shape as #349 |

## Falsifiable measurement

**Claim:** For sessions producing 5+ atomic fixes of similar shape, the coherent-arc rule cuts PR count by 50-70% vs the old "1 scenario per PR" default, without sacrificing atomic-revert granularity.

**Today's data point:**
- Under old rule (projected): ~12 PRs (6 from #349 + 3 from #353 + the rest as-is = 12+).
- Under new rule (actual): 6 PRs.
- Cut: 50%.

**The rule fails** when fixes share a surface but have **independent rollbacks**. Both #350 and #353 touched `scripts/`, but reverting one is unrelated to reverting the other — correctly kept as separate arcs.

## Why this matters across sessions

The previous default ("1 scenario per PR") was derived from `.agents/council/sdlc-shape-2026-05-17/DUEL.md` (Claude Opus 4.7 vs Codex gpt-5.5 duel). It optimized for **review legibility** at the cost of **operator throughput**. The 2026-05-18 8-PR merge-arc made the cost visible: `gh-merge-chain.sh` deadlocked on all-BEHIND state (F3), every successor PR ate an update-branch tax, the operator burned ~3-5x tokens per arc on rebase coordination.

The coherent-arc rule moves the unit-of-PR from "Gherkin scenario" to "rollback unit." The doctrine altitudes (BC architecture, RPI orchestration, Gherkin acceptance) all still apply — what changed is the PR-cut boundary, downstream of acceptance.

## Decision rule for future arcs

When deciding "1 PR or split":

1. **Same single rollback semantic?** Bundle as one PR with N commits.
2. **Different surfaces, different reviewers?** Split.
3. **Pre-existing drift caught by gate?** Always atomic side-quest PR (anti-pattern #2 still wins).
4. **Same surface but different intent?** Split.
5. **In doubt?** Split. Bundling is riskier because of squash-merge irreversibility.

## Co-evolved learnings (today)

Three local learnings (in `.agents/learnings/`) supported this validation:

- **`2026-05-19-default-true-flags-are-path-filter-footguns.md`** — codebase-specific: `HAS_<surface>=1` defaults outside `FAST_MODE` block are stale and dangerous when used as path filters. PR #350 hit this footgun.
- **`2026-05-19-self-verify-before-claiming-fix-lands.md`** — discipline: re-run the targeted gate at post-merge HEAD before claiming a fix lands. PRs #350 and #352 both demonstrated the cost of skipping this. Encoded as anti-pattern #7 in `skills/ship-loop/references/anti-patterns.md`.
- **`2026-05-19-coherent-arc-rule-dogfood-result.md`** — the local origin of this durable doc.

Encoded into ship-loop SKILL.md (Claude + Codex twins) anti-pattern #7 and #8.

## See also

- `CLAUDE.md ## Workflow` — canonical statement of the rule
- `AGENTS.md ## Workflow` — same, user-facing surface
- `skills/ship-loop/SKILL.md` — the lane skill with the discipline encoded
- `skills/ship-loop/references/anti-patterns.md` — anti-patterns #7 and #8 codify today's self-corrections
- `scripts/pre-push-gate.sh` — has the `HAS_<surface>` footgun warning comment at line 387
- PR #346 (ship.sh wrapper, mechanical fix for anti-pattern #1) — the prerequisite mechanical layer
- PRs #348-#353 — the session that produced this learning
