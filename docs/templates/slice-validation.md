# Experiment and validation worksheet

Use this worksheet to record evidence from one bounded experiment and its
independent Validate review. The current machine contracts are
[`subject-manifest.v1`](https://github.com/boshu2/agentops/blob/main/schemas/subject-manifest.v1.schema.json) and
[`verdict.v2`](https://github.com/boshu2/agentops/blob/main/schemas/verdict.v2.schema.json).

## Pinned inputs

- Intent reference or snapshot: `<caller source or content-addressed path>`
- Acceptance digest: `<sha256>`
- Author context ID: `<nonempty identity>`
- Subject locator: `<directory or explicit root>`

## Bounded experiment

- RED evidence: `<first acceptance check and factual result>`
- GREEN evidence: `<same acceptance check and factual result>`
- Refactor evidence: `<check that remained green>`

## Runtime-derived subject facts

- Subject-manifest digest: `<sha256>`
- Actual changed paths: `<complete list>`
- Changed-path coverage complete: `true | false`
- Other factual evidence: `<ids and references>`
- Checks not run: `<explicit list>`

## Independent Validate review

- Validator context ID: `<distinct nonempty identity>`
- Freshness source: `runtime | caller`
- Freshness attester: `<identity>`
- Validation result: `<PASS | FAIL | NOT_PROVEN>`
- Criterion results: `<PASS | FAIL | NOT_PROVEN with evidence>`
- Checked: `<claims and surfaces>`
- Not checked: `<claims and surfaces>`
- Verdict artifact (optional): `<content-addressed path when requested>`

Any subject edit invalidates the review. A proven out-of-scope path is FAIL;
incomplete changed-path coverage is NOT_PROVEN. Validate returns one fresh result and stops
without repair, retry, Git, closure, release, or delivery.
Persist `verdict.v2` only when the caller requests machine-readable evidence or
a declared downstream consumer requires it.
