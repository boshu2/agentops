---
id: pre-mortem-2026-05-10-practice-citation-backfill
type: pre-mortem
date: 2026-05-10
mode: quick
plan_ref: .agents/plans/2026-05-10-practice-citation-backfill-rpi-core.md
scope_mode: hold
---

## Council Verdict: PASS

Inline (--quick) structured review of the bounded practice-citation backfill plan.

## Verdict table

| Dimension | Result | Note |
|---|---|---|
| Slug correctness | PASS | All 38 unique slug citations exist in the 45-slug PRACTICE.md catalog |
| Frontmatter placement assumption | PASS | All 13 target files have `description:` on line 3, single-line scalar |
| Validator first-200-line scan | PASS | Frontmatters end well under line 200 in all 13 files |
| YAML strict-parser risk | PASS (low) | `heal.sh` validates required-keys presence only, not a closed allowlist; `user-invocable:`, `compatibility:`, `allowed-tools:` already coexist as additional top-level keys, so adding `practices:` follows precedent |
| Wave 1 file overlap | PASS | 13 distinct paths, write-only |
| Pre-push gate (Wave 2) | PASS | practice-citations job is advisory; declared-13/missing-741 is acceptable progress, not a regression |
| Worker scope discipline | PASS | Each issue body lists exactly one SKILL.md path |
| Test pyramid coverage | PASS | Validator IS the L1 test surface; no new tests needed |
| Temporal interrogation | N/A | No deferred-bead input |
| Council FAIL patterns | None match | Not a refactor; no boundary-crossing edits; no schema change |
| Mandatory-for-epics enforcement | N/A | Epic has 14 child issues (>3), but pre-mortem is being run as part of /rpi --auto |

## Risks not blocking

- **Future-pass slug drift:** if a subsequent session edits PRACTICE.md and renames a slug, today's declarations become invalid. Mitigation: PRACTICE.md L208 says "Add new slugs by appending — never rename silently." Enforced by convention, not yet by a hook.
- **Mapping-quality subjectivity:** the 13 mappings are defensible but not the only reasonable assignment. Future evolution may add/remove practices per primitive. Validator only enforces slug validity, not mapping aptness. Acceptable for pass 1.

## Decision gate

Plan is mechanically verifiable, bounded (13 files, ~30 chars added per file), and validated by the existing advisory CI gate. **Proceed to /crank.**

## Step 4.5 — Reusable findings extracted

None — this is mechanical backfill, no new general lessons surfaced.
