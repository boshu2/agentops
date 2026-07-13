# Plan Compliance Checklist

Use this checklist to mechanically verify implementation against plan.

## How to Use

1. Read the plan file
2. Extract each TODO/deliverable
3. For each item, fill in the table

## Checklist Template

| # | Plan Item | Expected File | File Exists? | Implementation Matches? | Evidence |
|---|-----------|---------------|--------------|------------------------|----------|
| 1 | <copy from plan> | <path> | yes/no | yes/partial/no | file:line |
| 2 | ... | ... | ... | ... | ... |

## Verification Rules

### "File Exists?" Column
- `yes` = file exists at expected path
- `no` = file missing (GAP)

### "Implementation Matches?" Column
- `yes` = code does what plan says
- `partial` = some of it, not all
- `no` = code does something different

### "Evidence" Column
- Must include file:line reference
- If no match, explain what's there instead

## Common Gaps

| Gap Type | Example | Action |
|----------|---------|--------|
| Missing file | Expected `auth.py`, not found | Create follow-up issue |
| Partial impl | Tests exist but don't cover edge cases | Document gap |
| Scope change | Plan said X, we built Y instead | Document rationale |

## Command-Surface Parity Addendum (CLI repos)

When plan scope includes `cli/cmd/ao/*.go` changes, add this parity table:

| Command File | Tested Run-path Evidence | Intentionally Uncovered? | Follow-up Issue |
|---|---|---|---|
| cli/cmd/ao/<command>.go | TestName / file:line | yes/no | ag-xxxx (if yes) |

Rules:
- Every modified command file must have either tested run-path evidence or an intentional-uncovered entry.
- "Intentionally uncovered" requires a concrete follow-up issue ID.
- Do not close plan-compliance as PASS when command-surface rows are missing.

## Example

From a real plan:

| # | Plan Item | Expected File | File Exists? | Implementation Matches? | Evidence |
|---|-----------|---------------|--------------|------------------------|----------|
| 1 | Create toolchain-validate.sh | scripts/toolchain-validate.sh | yes | yes | scripts/toolchain-validate.sh:1-375 |
| 2 | Support --json flag | scripts/toolchain-validate.sh | yes | yes | scripts/toolchain-validate.sh:26,339 |
| 3 | Add unit tests | tests/scripts/test-toolchain-validate.sh | yes | partial | Missing exit code tests |
