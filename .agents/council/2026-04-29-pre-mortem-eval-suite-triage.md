---
id: pre-mortem-2026-04-29-eval-suite-triage
type: pre-mortem
date: 2026-04-29
source: "[[.agents/plans/2026-04-29-eval-suite-triage]]"
prediction_ids:
  - pm-20260429-001
  - pm-20260429-002
  - pm-20260429-003
---

# Pre-Mortem: Eval Suite Triage

## Council Verdict: WARN

| ID | Judge | Finding | Severity | Prediction |
|----|-------|---------|----------|------------|
| pm-20260429-001 | Missing-Requirements | The plan can still become taxonomy churn if ag-v29.1 adds labels without an audit command that fails on uncategorized suites. | significant | Workers add metadata to a few suites and leave most cases ambiguous, so "eval" remains overloaded. |
| pm-20260429-002 | Feasibility | Deleting artifact-only canaries without proving replacement coverage can remove useful tripwires. | significant | A future refactor removes exact-string checks and an architectural boundary regresses unnoticed. |
| pm-20260429-003 | Scope | Fast/full lane work can sprawl into CI policy, pre-push, baselines, and suite rewrites at once. | moderate | A worker changes runner behavior and CI wiring together, producing hard-to-debug local/remote mismatch. |

## Pseudocode Fixes

Finding: pm-20260429-001 — evidence-kind taxonomy must be mechanically enforced.

Severity: significant

Fix (pseudocode):

```go
func missingEvidenceKinds(suites []CoverageSuite) []string {
    missing := []string{}
    for _, suite := range suites {
        if len(suite.EvidenceKinds) == 0 {
            missing = append(missing, suite.ID)
        }
    }
    sort.Strings(missing)
    return missing
}
```

Affected files: `cli/internal/eval/coverage.go`, `cli/internal/eval/coverage_test.go`, `schemas/eval-suite.v1.schema.json`.

Finding: pm-20260429-002 — culling needs replacement proof.

Severity: significant

Fix (pseudocode):

```bash
if [[ "$action" == "delete" || "$action" == "merge" ]]; then
  test -n "$replacement_command" || die "replacement_command is required"
  test -n "$replacement_scope" || die "replacement_scope is required"
fi
```

Affected files: `tests/scripts/test-eval-suite-triage.sh` or the chosen audit script.

## Shared Findings

- The plan is directionally correct because it names what "eval" means instead of only adding more suites.
- The first implementation slice should be ag-v29.1 only. Do not combine taxonomy with suite deletion.
- Baseline policy and artifact culling should not happen in the same PR; otherwise warnings and missing checks become hard to interpret.
- Live runtime work belongs late and must stay opt-in.

## Known Risks Applied

- `f-2026-04-27-004` — Applied to duplicated validators and mirrored gate wrappers.
- `f-2026-04-14-002` — Applied to baseline and closure proof paths.
- `f-2026-04-14-001` — Applied to `ao eval` command and script changes.
- `f-2026-04-25-001` — Applied to final closeout and disposition proof.

## Concerns Raised

- The plan introduces an enum-like evidence-kind field. It must include validation, fallback behavior for old suites during migration, and tests for invalid values.
- `process` and `quality` domains already appear in coverage output but are not in `DefaultCoverageDomains`; the plan should decide whether they are required domains or supporting domains.
- Baseline file placement under `.agents/evals/baselines/` is semantically awkward because `.agents` is often local/generated. The plan marks relocation as Ask First, which is appropriate.

## Recommendation

Proceed with WARN. Start with ag-v29.1 only: add evidence-kind schema/docs/coverage reporting and an uncategorized-suite audit. Re-run pre-mortem before deleting or merging suites.

## Decision Gate

Decision: PROCEED WITH WARN.
