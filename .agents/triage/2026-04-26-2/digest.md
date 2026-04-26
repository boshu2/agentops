# Triage Digest — 2026-04-26 (run: triage/2026-04-26-2)

Branch `triage/2026-04-26` already existed on origin; used collision-safe name `triage/2026-04-26-2`.

## Items Rewritten (3)

| # | Title | Probe evidence |
|---|-------|---------------|
| 1 | Update SKILL-TIERS.md diagram terminology | `skills/SKILL-TIERS.md:81` diagram reads `(council + knowledge)`; line 233 reads `Council + knowledge lifecycle` — old `(council + retro)` text absent |
| 2 | Backfill next-work queue rows to schema v1.3 and add drift validation | `scripts/check-next-work-schema-rows.sh` PASS 66 row(s) conform to v1.3 schema enums — legacy severity/source/type values absent |
| 3 | Sweep skills-codex/ DAG bodies for Skill() to $skill notation | `grep -rn 'Skill()' skills-codex/*/SKILL.md` returned empty — only match was in converter script (line 154: comment about rewriting) |

**Items left (probe ambiguous or not done):** 50 remaining unconsumed

Notable non-stale:
- `skills/crank/SKILL.md` is 660 lines (248-line limit item is live)
- `--fast-path` flag still present across 5+ files (rename item is live)
- 750 `os.Chdir`/`os.Getwd` occurrences (refactor items live)
- 6 orphan research files still unlinked in `.agents/learnings/`
- Items with `target_repo=nami` skipped — cannot probe external repo

## Packets Rewritten (0)

No overnight dir — `.agents/overnight/latest/` does not exist. Section 3 skipped.

## Schema Violations Open

All three validators pass:
```
PASS: 66 row(s) conform to v1.3 schema enums
next-work contract parity validation passed.
```

One pre-existing failure (NOT caused by this triage run):
```
✗ test-runtime-cursor-smoke.sh failed
FAILED - 1 errors, 0 warnings
```
Confirmed pre-existing: same failure on `origin/main` before any rewrites.

## Queue-Flood Warning

Not triggered (3 rewrites << 30-item cap).

## Commit SHA

See git log after push.
