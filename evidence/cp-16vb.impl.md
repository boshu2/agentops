# cp-16vb — land cp-9wvq skill-count SSOT fix in agentops MAIN

**Date:** 2026-06-10
**Outcome:** MERGED to `main` via clean fast-forward. Doc-release gate + SSOT red-green test both GREEN. No CI triggered (by design). Ready to convene (A7) — NOT closed.

## Merge

- Branch `fix/cp-9wvq-skill-count-ssot` was exactly **1 ahead, 0 behind** `origin/main`
  (`c98836977` = v3.1.0 release commit). SSOT commit `0626f7a19` sat directly on top → pure
  fast-forward, no rebase needed. The SSOT diff (validate.yml, scripts/, tests/, evidence/) is
  disjoint from the v3.1 release docs.
- Re-verified `origin/main` unchanged at `c98836977` immediately before push.
- **Merge command:** `git push origin 0626f7a19:main` → `ok main` (fast-forward).
- **main HEAD now:** `0626f7a19 fix(docs): derive skill count from disk SSOT, kill manual-edit doc-gate block (cp-9wvq)`
- Main branch is **not** GitHub-protected; recent history (cp-fhq3) lands via direct push to main —
  matched that convention.

## Verification (run on the merged tree = origin/main tree, worktree HEAD `0626f7a19`)

`git show origin/main:scripts/ensure-skill-tiers-rows.sh | head -3` →
```
#!/usr/bin/env bash
# ensure-skill-tiers-rows.sh — auto-render SKILL-TIERS.md rows from the disk SSOT.
#
```

### Doc-release stabilization gate — `tests/docs/validate-doc-release.sh` → EXIT 0
```
=== Link validation ===
3127 links checked, 0 broken (50 allowlisted, 1 mkdocs-generated)
PASS: Link validation
=== Skill count validation ===
  Skill directories: 166 | Codex: 166 | Overrides: 27 | Table total: 166
PASS: All skill counts consistent (total=166, user-facing=158, internal=8)
PASS: Skill count validation
=== Skill count sync check ===
OK: every skill directory has a SKILL-TIERS.md row (166 skills)
DONE: All counts already in sync.
PASS: Skill count sync check
=== CLI skills map validation ===     PASS
=== Release message freeze validation === PASS
PASS: doc-release gate succeeded
```

### SSOT red-green acceptance — `tests/docs/test-skill-count-ssot.sh` → EXIT 0
```
=== Baseline: doc-release skill-count gate is GREEN ===
PASS: baseline green
=== RED: add a skill dir, run gate with ZERO doc edits (must FAIL) ===
PASS: gate correctly fails before sync (proves the count is enforced)
=== GREEN: run sync (auto-row + patch), gate must PASS with ZERO hand-edits ===
PASS: gate green after sync, zero manual doc edits, auto-row present
=== NEGATIVE: a hand-typed WRONG count must still be CAUGHT ===
PASS: wrong hand-typed count is caught
PASS: skill-count SSOT red-green acceptance complete
```
The `add-a-skill -> doc-gate-green-with-zero-hand-edits` path reproduces against the landed-main tree.

## CI

No GitHub Actions run was triggered by the push to `main` — and that is **by design**, not a miss.
`validate.yml` triggers only on `push: tags: v*`, `pull_request: [main]`, `workflow_dispatch`, and
`merge_group` (header comment: *"Local validation is the release authority for routine direct-main
work... so push-to-main does not consume Actions quota or become a serialization bottleneck."*).
The most recent Validate FAIL run (`27284023423`) is the **prior** `c98836977` commit on the
`v3.1.0` branch — unrelated to this merge. Local gates are the release authority here and both passed.

## Status

Landed + locally validated. **NOT closed** — A7: convene codex+claude (or note ready-to-convene,
since this was a trivial clean fast-forward of an already-validated+closed cp-9wvq branch).
