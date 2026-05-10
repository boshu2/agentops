---
id: plan-2026-05-10-practice-citation-backfill-rpi-core
type: plan
date: 2026-05-10
goal: Backfill `practices:` frontmatter declarations into 13 RPI-core/foundational skill SKILL.md files
complexity: standard
detail_level: minimal
research_ref: .agents/research/2026-05-10-practice-citation-backfill-mapping.md
test_levels: [L1]
applied_findings: [finding-2026-05-07-ci-parity-as-wave-acceptance, finding-2026-05-07-worker-file-list-overscope]
---

# Plan: Practice-Citation Backfill — RPI-Core (Pass 1 of N)

## Context

`scripts/validate-practice-citations.sh` (added in commit 254138fa) walks 754 primitives and reports zero have a `practices:` declaration. This pass adds declarations to 13 RPI-core skills, validating the gate's machinery before scaling to the remaining 741 primitives in future sessions. The validator stays in **report-only / advisory** mode — no promotion to `--strict` until 0 missing.

## Applied findings

- `finding-2026-05-07-ci-parity-as-wave-acceptance`: each issue's acceptance includes the validator run; the wave isn't complete until the local gate (`scripts/pre-push-gate.sh --fast`) is also green.
- `finding-2026-05-07-worker-file-list-overscope`: each issue's file list is exactly **one** SKILL.md path. No worker may touch unrelated skills, scripts, or docs.

## Baseline audit (numbers)

| Quantity | Value |
|---|---|
| Primitives currently missing `practices:` | 754 |
| Target primitives this pass | 13 |
| Expected missing after this pass | 741 |
| Slug catalog size | 45 |
| Largest target SKILL.md (LOC) | 712 (implement) — frontmatter <30 LOC, validator scans first 200 |
| All target frontmatters under 200 lines | ✓ |

## Files to modify (one per issue, plus a no-edit validation issue)

| # | File | LOC | `practices:` line to add |
|---|---|---:|---|
| 1 | `skills/rpi/SKILL.md` | 186 | `practices: [continuous-delivery, dora-metrics, agile-manifesto, pragmatic-programmer]` |
| 2 | `skills/discovery/SKILL.md` | 240 | `practices: [adr, lean-startup, mythical-man-month]` |
| 3 | `skills/crank/SKILL.md` | 688 | `practices: [continuous-delivery, xp, agile-manifesto]` |
| 4 | `skills/validation/SKILL.md` | 248 | `practices: [llm-eval-harness, dora-metrics, sre]` |
| 5 | `skills/plan/SKILL.md` | 260 | `practices: [adr, agile-manifesto, pragmatic-programmer]` |
| 6 | `skills/implement/SKILL.md` | 712 | `practices: [tdd, refactoring, code-complete]` |
| 7 | `skills/vibe/SKILL.md` | 573 | `practices: [ai-assisted-dev, llm-eval-harness, code-complete, pragmatic-programmer]` |
| 8 | `skills/pr-prep/SKILL.md` | 258 | `practices: [continuous-integration, continuous-delivery, gitops]` |
| 9 | `skills/pr-validate/SKILL.md` | 197 | `practices: [continuous-integration, code-complete, pragmatic-programmer]` |
| 10 | `skills/post-mortem/SKILL.md` | 631 | `practices: [dora-metrics, sre, lean-startup]` |
| 11 | `skills/domain/SKILL.md` | 88 | `practices: [ddd-bounded-context, wiki-knowledge-surface, pragmatic-programmer]` |
| 12 | `skills/flywheel/SKILL.md` | 298 | `practices: [wiki-knowledge-surface, lean-startup, dora-metrics]` |
| 13 | `skills/handoff/SKILL.md` | 348 | `practices: [adr, wiki-knowledge-surface, code-complete]` |
| 14 | _(no file edit; runs validator + pre-push gate)_ | — | — |

## Placement contract (applies to issues 1–13)

For each target file:
1. Read the existing frontmatter.
2. Locate the `description:` line in the YAML frontmatter (typically line 3).
3. Insert the `practices: [...]` line **immediately after** `description:`.
4. Do not modify any other field.
5. Save the file.

The `practices:` key is a new top-level YAML field. No existing SKILL.md parses YAML strictly enough to fail on an unknown top-level key (verified in research artifact).

## Boundaries

- **In scope:** the 13 SKILL.md frontmatter edits + running the validator + running the local pre-push gate.
- **Out of scope:** any other SKILL.md file; hook scripts; CLI command files; eval JSONs; PRACTICE.md itself; the validator script; CI workflow YAML; any docs.

## Issues

### Wave 1 (parallel — 13 independent file edits)

Each issue body shape (filled per file):

> **Title:** `Add practices: declaration to skills/<name>/SKILL.md`
> **Description:** Add the line `practices: [<slugs>]` immediately after the `description:` line in the frontmatter. No other changes.
> **Files:** `skills/<name>/SKILL.md` (exactly one)
> **Acceptance:** After edit, `grep -nE '^practices:' skills/<name>/SKILL.md` returns one match with the exact slug list above.

### Wave 2 (depends on all Wave 1)

> **Title:** `Run practice-citation validator + local pre-push gate`
> **Description:** Run `bash scripts/validate-practice-citations.sh` and confirm: with-practices = 13, missing = 741, invalid = 0. Then run `bash scripts/pre-push-gate.sh --fast` and confirm clean.
> **Files:** (none modified)
> **Acceptance:** Both commands exit 0; validator report shows the expected 13/741/0 split.

## Wave Structure

```
Wave 1: 13 frontmatter edits — fully parallel, zero file overlap.
Wave 2: 1 validation run — depends on all of Wave 1.
```

## File dependency matrix

| Task | File | Access | Notes |
|---|---|---|---|
| 1 | skills/rpi/SKILL.md | write | |
| 2 | skills/discovery/SKILL.md | write | |
| 3 | skills/crank/SKILL.md | write | |
| 4 | skills/validation/SKILL.md | write | |
| 5 | skills/plan/SKILL.md | write | |
| 6 | skills/implement/SKILL.md | write | |
| 7 | skills/vibe/SKILL.md | write | |
| 8 | skills/pr-prep/SKILL.md | write | |
| 9 | skills/pr-validate/SKILL.md | write | |
| 10 | skills/post-mortem/SKILL.md | write | |
| 11 | skills/domain/SKILL.md | write | |
| 12 | skills/flywheel/SKILL.md | write | |
| 13 | skills/handoff/SKILL.md | write | |
| 14 | PRACTICE.md | read | validator parses slug catalog |
| 14 | (1..13).SKILL.md | read | validator re-scans |

Zero `write` overlap across Wave 1. Wave 2 is read-only.

## Test levels

L1 — the validator IS the test surface. No new tests needed.

## Verification (Wave 2 issue)

```bash
bash scripts/validate-practice-citations.sh | tee /tmp/practice-report.txt
grep -E "with practices field: 13" /tmp/practice-report.txt
grep -E "missing practices field: 741" /tmp/practice-report.txt
grep -E "invalid slug citations: 0" /tmp/practice-report.txt
bash scripts/pre-push-gate.sh --fast
```

## Planning Rules Compliance

| Rule | Status | Justification |
|---|---|---|
| PR-001 mechanical enforcement | ✓ | `scripts/validate-practice-citations.sh` enforces; no judgment-call in this loop |
| PR-002 external validation | ✓ | Validator runs the slug catalog parse independently from this plan's slug list |
| PR-003 feedback loop | ✓ | Wave 2 reports concrete counts (13/741/0) before declaring success |
| PR-004 separation | ✓ | Each issue touches exactly one SKILL.md; no cross-file edits |
| PR-005 process gates | ✓ | Pre-push gate runs in Wave 2; CI advisory job runs on push |
| PR-006 cross-layer consistency | N/A | No cross-layer surfaces touched (frontmatter only; no docs/PRACTICE.md/CI changes) |
| PR-007 phased rollout | ✓ | This is pass 1-of-N; 741 remaining backfilled in subsequent sessions; gate stays advisory until 0 missing |

## Post-Merge Cleanup

None — frontmatter is the artifact.

## Next Steps after this plan ships

- Subsequent sessions: backfill 10-15 more primitives per pass.
- When validator report shows `missing = 0` and `invalid = 0`, flip `practice-citations` CI job from advisory to required (single-line YAML edit + AGENTS.md SLA-table promotion).
