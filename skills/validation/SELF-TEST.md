# Validation Skill Self-Test

## Trigger Cases

- User says: `/validation soc-1234` (validate an epic with full close-out).
  - Expected: load `validation`, run the vibe → lifecycle → post-mortem → retro → forge DAG by strict delegation, and roll every acceptance criterion into a verdict.

- User says: `/validation` (validate recent work, no epic).
  - Expected: load `validation` and run the DAG against the recent changes.

- User says: "validate this work before we close the bead" or `/rpi --quality` reaches the validation phase.
  - Expected: load `validation`; `--strict-surfaces` is passed automatically by `/rpi --quality`, making the four closure surfaces blocking.

## Non-Trigger Cases

- User asks for a quick lint or readiness check on uncommitted code only.
  - Expected: route to `/vibe` directly — `validation` orchestrates the full phase, not a single quick check.

- User asks to decompose a goal or estimate scope.
  - Expected: route to `/plan`, not `validation`.

## Behavior Checks

These map to the four scenarios in [references/validation.feature](references/validation.feature):

- Every acceptance criterion maps to a passing test: each Given/When/Then from the intent maps to a passing test, and an unmapped or failing criterion blocks the verdict.
- Strict delegation across the DAG: `validation` delegates to `/vibe`, the lifecycle skills, `/post-mortem`, `/retro`, and `/forge` as separate `Skill(...)` invocations, and does not compress or skip those steps.
- The verdict is proof, not an activity log: `validation` produces `verdict.json` capturing the per-criterion verdict; an activity log alone never closes a bead.
- Surface failures block under strict mode: with `--strict-surfaces` set, any of the four closure surfaces failing makes the verdict FAIL, not WARN.

## Validation Commands

Run from the repo root:

```bash
bash skills/heal-skill/scripts/heal.sh --strict skills/validation
bash scripts/validate-skill-frontmatter.sh --strict
```

For JSM-style export readiness, run:

```bash
scripts/check-jsm-export.sh --json skills/validation
```

## Failure Cases

- Judges spawned via `Agent()` in place of `/vibe`, or post-mortem/forge inlined: reject the compression and re-run via separate `Skill(...)` invocations per the strict delegation contract.
- An acceptance criterion has no mapped passing test: block the verdict and either add the test or mark the criterion explicitly cancelled in bead metadata.
- Completion marker emitted without `verdict.json`: re-run — the per-criterion verdict artifact is the exit signal, not a stdout summary.
