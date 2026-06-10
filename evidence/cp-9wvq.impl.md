# cp-9wvq — Skill-count single-source-of-truth (kill the 5-place hardcode)

## Root cause (confirmed)

The skill count is enforced by two scripts the doc-release gate runs:
- `tests/docs/validate-skill-count.sh` — compares the disk skill-dir count vs
  SKILL-TIERS.md table rows vs every doc count header.
- `scripts/sync-skill-counts.sh --check` — same, plus a fail-closed ASCII-diagram
  guard.

`TOTAL` already derives from ONE place: the set of `skills/*/` directories (this
is also what `generate-skill-catalog.sh` counts into `catalog.json.skill_count`,
which equals `skills | length` — already consistent). The friction was **not**
the number itself — it was that **SKILL-TIERS.md table rows are hand-maintained**.
Adding a skill dir made `rows != directories`, and `sync-skill-counts.sh`
**fail-closes on that mismatch before it can patch any doc**. So every skill
add/remove required a manual SKILL-TIERS row edit *before* sync would run — the
exact thing that stuck the 3.1 release (compounded by a stray `.pyc` masking the
true 166). A second, separate hardcode class: two ASCII box-drawing diagrams
hold the count inside alignment-sensitive borders and were deliberately left as
a manual "re-pad by hand", so they drifted on every churn too.

## SSOT mechanism (what landed)

The set of skill directories is the SSOT. The count is now derived + propagated
in one flow with **zero manual doc edits**:

1. **`scripts/ensure-skill-tiers-rows.sh` (new)** — for any skill dir lacking a
   SKILL-TIERS.md row, auto-renders a placeholder row into the user-facing
   "Factory-Built Operator And Pack Skills" table (description pulled from the
   skill's own `SKILL.md` frontmatter; tier defaults to `execution`). Curated
   rows are never touched. After it runs, `rows == directories` holds by
   construction, so the fail-closed equality check can no longer block on a
   forgotten row. A maintainer may later re-tier/re-word/move a row to Internal —
   editorial polish, not a release blocker. Has a `--check` mode.

2. **`scripts/sync-skill-counts.sh` (modified)** — now calls `ensure-skill-tiers-rows.sh`
   first (so the count derives from disk before any equality check), then patches
   all doc count surfaces as before. The ASCII-diagram guard is extended:
   **same-digit-width drift is auto-patched** (alignment preserved — this is the
   166↔167 case that broke 3.1); only a **digit-width change** (e.g. 99→100) still
   fails for a human re-pad. This closes the count-drift class for the common case.

3. The gate (`validate-skill-count.sh` / `sync --check`) verifies docs == the
   derived count, in both directions — a hand-typed wrong number is still CAUGHT.

Net: the worst hardcode (manual-SKILL-TIERS-edit-before-sync) is killed; all 9
doc count surfaces (incl. both ASCII diagrams) propagate from the disk SSOT.

## Red-green acceptance test (new, wired into CI)

`tests/docs/test-skill-count-ssot.sh` — plants a fixture skill dir, asserts the
gate FAILS with zero doc edits (RED), runs sync, asserts the gate PASSES with
zero hand-edits and the auto-row is present (GREEN), then asserts a deliberately
wrong hand-typed count is still CAUGHT (NEGATIVE). Self-restoring via git/snapshot;
refuses to run on a dirty relevant tree. Wired into `.github/workflows/validate.yml`
right after the doc-release gate step.

### Verbatim run

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

### Full doc-release gate (clean tree, baseline 166)

```
PASS: Link validation
PASS: All skill counts consistent (total=166, user-facing=158, internal=8)
PASS: Skill count validation
PASS: Skill count sync check
PASS: CLI skills map validation
PASS: release message freeze intact
PASS: Release message freeze validation
PASS: doc-release gate succeeded
```

`bash -n` clean on all three scripts; `shellcheck -S warning` clean; sync is
idempotent on a clean tree ("DONE: All counts already in sync.").

## Follow-up (routed)

- ASCII-diagram **digit-width** changes (e.g. crossing 99→100, 999→1000) still
  require a manual box-drawing re-pad by design — the guard fails loudly with the
  instruction. A future bead could auto-re-pad the box widths, but that is a
  rare, alignment-sensitive change and out of scope here.
- Auto-rendered rows default to tier `execution` / user-facing. If a new skill is
  Internal or a different tier, a maintainer edits the placeholder row post-hoc;
  the gate never blocks on it. Optionally, a future change could read a `tier:` /
  `metadata.internal:` field from frontmatter when present.

## Files

- `scripts/ensure-skill-tiers-rows.sh` (new)
- `scripts/sync-skill-counts.sh` (modified — ensure-rows hook + same-width diagram auto-patch)
- `tests/docs/test-skill-count-ssot.sh` (new)
- `.github/workflows/validate.yml` (modified — CI step)
