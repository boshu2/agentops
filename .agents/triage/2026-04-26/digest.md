# Triage Digest — 2026-04-26

## Summary

- Items rewritten (stale): **9**
- Packets rewritten (stale): **0** (no .agents/overnight/latest/morning-packets/ directory)
- Schema violations open: **0** (PASS: 66 rows conform to v1.3 schema enums)
- Queue-flood: **not triggered** (cap: 30, actual: 9)
- bd: unavailable (not a blocking condition per protocol)

## Items Rewritten

9 next-work items marked `consumed=true, claim_status=consumed, consumed_by=stale-audit-2026-04-26`.

| # | Title | Probe Evidence |
|---|-------|---------------|
| 1 | Add integration merge checklist script | `scripts/post-merge-check.sh` exists |
| 2 | Create resolveProjectDir migration tracker | `docs/migration-trackers/resolve-project-dir.md` exists |
| 3 | Implement sections.include allowlist semantics | `applyContextFilter` in `inject_context.go:245-263` implements allowlist mode |
| 4 | Add intel_scope and section-name enum validation | `validIntelScopes` and `validateIntelScope` in `inject_context.go`; section-name validation at lines 173, 178 |
| 5 | Document RPI_RUN_ID env var contract | `docs/ENV-VARS.md:55` documents format, setter, and fallback |
| 6 | Fix go-test-precommit.sh to use stdin JSON pattern | `hooks/go-test-precommit.sh:12` uses `INPUT=$(cat)` pattern |
| 7 | Fix validate-learning-coherence.sh frontmatter-only file | Script exits 0: "14 files checked, 0 failures, 0 warnings" |
| 8 | Add no-jq fallback tests to lib-hook-helpers.bats | `tests/hooks/lib-hook-helpers.bats:996,1034` cover both fallback paths |
| 9 | Write AgentOps philosophy doc from validated flywheel thesis | `docs/philosophy.md` exists |

## Packets Rewritten

0 — no `.agents/overnight/latest/morning-packets/` directory exists. Dream section was a no-op.

## Schema Violations Open

None. Schema check output:
```
PASS: 66 row(s) in .agents/rpi/next-work.jsonl conform to v1.3 schema enums
```

## Items Left in Queue (Not Probed as Done)

Items deliberately left unconsumed (probe said "not done" or was ambiguous):

- **Replace os.Chdir with path-based helpers** — 437 os.Chdir calls remain in cli/
- **Refactor production code to accept projectDir** — 125 os.Getwd() calls in production code
- **Decompose skills/crank/SKILL.md to under 248-line limit** — currently 660 lines
- **Rename --fast-path to --quick-gates** — `--fast-path` flag still present in rpi_phased.go
- **Runtime hook for RPI phase enforcement** — `hooks/rpi-phase-enforcement.sh` does not exist
- **Add cmd/ao to coverage baseline** — not tracked in coverage-ratchet.sh
- **Expand audit-assertion-density.sh to cover all test files** — script only covers `*coverage*_test.go`
- All 9 orphan-rescue items — files exist in `.agents/research/` but still unreferenced in learnings
- Process-improvement items (footguns, templates, gates) — no tractable single-file probe available

## Baseline

| Metric | Before | After |
|--------|--------|-------|
| Unconsumed items | 62 | 53 |
| Morning packets | 0 | 0 |
| Schema violations | 0 | 0 |

## Commit SHA

_Pending commit._
