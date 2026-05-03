# Phase 2 Summary: Implementation

- **Epic:** soc-ocq8
- **Objective:** Fix ao eval baseline A/B default run ID collision
- **Status:** DONE
- **Timestamp:** 2026-05-03T08:46:56-04:00
- **Files changed:**
  - `cli/internal/eval/baseline_ab.go`
  - `cli/internal/eval/baseline_ab_test.go`
  - `docs/contracts/eval-baseline-ab.md`
- **Implementation:** `RunBaselineAB` now derives a shared default base run ID when `--run-id` is omitted, then assigns distinct `-skill-on` and `-skill-off` IDs to the two legs. Explicit `RunID` input is preserved as the suffix base.
- **Tests added:** `TestRunBaselineABDefaultRunIDsAreDistinct`
- **Tracker:** `soc-ocq8` closed after focused tests, CLI smoke, and fast pre-push gate passed.
