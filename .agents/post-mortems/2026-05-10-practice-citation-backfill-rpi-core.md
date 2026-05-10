---
id: post-mortem-2026-05-10-practice-citation-backfill-rpi-core
type: post-mortem
date: 2026-05-10
mode: quick
epic_id: soc-hdot
plan_ref: .agents/plans/2026-05-10-practice-citation-backfill-rpi-core.md
phase2_ref: .agents/rpi/phase-2-summary-2026-05-10-crank.md
pre_mortem_ref: .agents/council/2026-05-10-pre-mortem-practice-citation-backfill.md
vibe_ref: .agents/council/2026-05-10-vibe-practice-citation-backfill.md
---

## Council Verdict: PASS

Inline (--quick) retrospective on epic soc-hdot — practice-citation backfill pass 1.

## Plan vs delivered

| Surface | Planned | Delivered | Delta |
|---|---|---|---|
| SKILL.md frontmatter edits | 13 | 13 | — |
| Validator/gate run | 1 | 1 | — |
| Schema property fix | 0 | 1 (`practices` in skill-frontmatter.v1.schema.json) | +1 (in-flight infrastructure correction; required to pass Wave 2 gate) |
| Codex twin parity markers | 0 (regen artifacts not enumerated as plan items) | 14 (manifest + 13 markers, all auto-generated via regen-codex-hashes.sh) | +14 (regen output; not new editorial work) |

Single deliberate scope addition: the schema fix. Pre-mortem missed this dependency. The Wave-2 gate caught it; fixed in the same cycle.

## Closure integrity

- 15 beads closed (1 epic + 14 children). All resolve on commit 235e5c20.
- No phantom beads; titles are descriptive.
- No orphans.
- Single-wave epic, no multi-wave regression possible.

## Prediction accuracy (vs pre-mortem)

| Prediction | Outcome | Score |
|---|---|---|
| Slug correctness PASS | Confirmed by validator | HIT |
| Frontmatter placement PASS | Confirmed | HIT |
| Validator first-200-line scan PASS | Confirmed | HIT |
| YAML strict-parser risk LOW | **schema.json (`additionalProperties: false`) AND codex-bundle allowlist BOTH blocked** | **MISS** |
| Wave 1 file overlap PASS | Confirmed | HIT |
| Pre-push gate PASS | Failed first try, passed after schema fix + codex revert | PARTIAL HIT |

One material MISS. The pre-mortem checked `heal.sh` (lenient, presence-only) and concluded strict-parser risk was low. Two strict consumers were not audited: the JSON Schema with `additionalProperties: false`, and the codex-bundle allowlist regex (`name|description` only). Both caught my changes at Wave 2.

## Learnings extracted

1. **Repo-specific** (`.agents/learnings/2026-05-10-codex-frontmatter-is-strict-name-description.md`): codex twin frontmatter is strictly name+description; new fields stay Claude-side only. Use `regen-codex-hashes.sh` to record source drift without touching codex content.

2. **Cross-cutting** (`~/.agents/learnings/2026-05-10-new-frontmatter-key-needs-schema-and-allowlist-audit.md`): adding a new top-level YAML/frontmatter key requires auditing ALL strict consumers (`additionalProperties: false` in schemas, allowlist regexes in validators) — lenient consumers are not representative.

## Test pyramid assessment

| Issue | Planned levels | Actual | Gaps | Action |
|---|---|---|---|---|
| All 14 child issues | L1 (validator) | L1 (validator ran clean, exit 0) | None | — |

No test-pyramid gaps. Practice-citation validator IS the L1 test surface for this class of change.

## What went well

- The pre-mortem's mandatory list (slug correctness, placement, validator scan) was complete for what it covered. All HIT.
- The Wave-2 acceptance gate caught the schema + allowlist issue before push had a chance to red CI.
- regen-codex-hashes.sh exists and worked correctly on both edit-and-revert cycles.
- The 13 edits + 1 schema fix + revert + retry-gate all landed in a single commit (235e5c20), green local gate.

## What went poorly

- Pre-mortem audit-coverage gap (see learnings).
- One unnecessary round-trip through the codex bundle validator and 13 unnecessary Edit calls on codex twins, then 11 Edit-equivalents to revert (the user got hit with ~26 hook side-effects per Edit call). Future passes should pre-check the codex frontmatter contract before editing.

## Flywheel: Next Cycle

Based on this post-mortem, the highest-priority follow-up is:

> **Backfill practices: declarations into next 10-15 primitives (pass 2)** (task, medium)
> Continue the bounded practice-citation backfill. Target: drop missing count from 741 toward 730.

Ready to run:
```
/rpi "Backfill practices: declarations into next 10-15 primitives (pass 2 of N) — see plan template at .agents/plans/2026-05-10-practice-citation-backfill-rpi-core.md"
```

Or see all 2 harvested items in `.agents/rpi/next-work.jsonl`.

## Mode

`--quick` — single-agent inline review. Sweep skipped per flag.
