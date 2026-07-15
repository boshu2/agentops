# Candidate and validation worksheet

Use this worksheet for one bounded CandidatePacket and its independent Validate
review. The machine contracts are
[`candidate-packet.v1`](../../schemas/candidate-packet.v1.schema.json),
[`subject-manifest.v1`](../../schemas/subject-manifest.v1.schema.json), and
[`verdict.v2`](../../schemas/verdict.v2.schema.json).

## Pinned inputs

- PlanPacket digest: `<sha256>`
- Acceptance digest: `<sha256>`
- Author context ID: `<nonempty identity>`
- Subject locator: `<directory or explicit root>`

## Bounded experiment

- RED evidence: `<first acceptance check and factual result>`
- GREEN evidence: `<same acceptance check and factual result>`
- Refactor evidence: `<check that remained green>`

## Candidate facts

- Subject-manifest digest: `<sha256>`
- Actual changed paths: `<complete list>`
- Changed-path coverage complete: `true | false`
- Other factual evidence: `<ids and references>`
- Checks not run: `<explicit list>`

## Independent Validate review

- Validator context ID: `<distinct nonempty identity>`
- Freshness source: `runtime | caller`
- Freshness attester: `<identity>`
- Criterion results: `<PASS | FAIL | NOT_PROVEN with evidence>`
- Checked: `<claims and surfaces>`
- Not checked: `<claims and surfaces>`
- Verdict artifact: `<content-addressed path>`

Any subject edit invalidates the review. A proven out-of-scope path is FAIL;
incomplete changed-path coverage is NOT_PROVEN. Validate persists one verdict
and stops without repair, retry, Git, closure, release, or delivery.
