# Complexity Estimation Heuristics

## Extract-Method CC Reduction

**Rule of thumb:** Each extract-method refactoring reduces the source function's CC by 3-5 points.

| Source CC | Extractions Needed | Expected Result |
|-----------|-------------------|-----------------|
| 15-20     | 1-2               | CC 10-15        |
| 20-25     | 3-4               | CC 10-15        |
| 25-30     | 4-6               | CC 10-15        |
| 30-40     | 6-10              | CC 10-15        |

**Do NOT assume 50% reduction per extraction.** Each extraction only removes the branching contained within the extracted block. A 35 CC function with 8 if/switch arms needs ~8 extractions, not 4.

## Evidence

- ag-atu: Plan estimated CC 35 → ~10 with 4 extractions. Pre-mortem caught the error. Actual: 8 extractions needed to reach CC 12.
- General: CC tracks decision points (if, switch, for, &&, ||). Extract-method moves decision points, not removes them.

## When to Flag

- Plan claims CC reduction >50% with <4 extractions → likely overestimate
- Target CC <10 from CC >25 → will need 6+ extractions minimum
- Any plan that doesn't count extractions individually → underspecified
