# age-wp1 — auditable verdict fixture + sandbox-exec integration

**Bead:** age-wp1  
**Date:** 2026-06-17

## Problem

The "first VALID corpus A/B verdict" rested on gitignored `.agents/evals/` artifacts.
Seatbelt isolation tests asserted profile strings only — zero live `sandbox-exec` runs.

## Fix

1. Tracked redacted scorecard fixture:
   `evals/scenarios/fixtures/scenario-ab-valid-redacted.scorecard.json`
2. Darwin integration test `TestSandboxExec_Integration_DeniesCorpusRead` runs
   real `sandbox-exec` and asserts corpus deny + non-corpus allow.

## Proof

```bash
go test ./cli/cmd/ao/ -run 'ScenarioABValidRedacted|SandboxExec_Integration' -count=1
```

**Commit:** 0bf8b8a2b (fix) + e445d6ac8 (regen, land on origin/main)
