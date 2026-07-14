# Mortem Naming Compatibility Contract

`premortem` and `postmortem` are the canonical skill slugs and Brownian
Ratchet values. Writers emit those values. The exported Go identifiers remain
`StepPreMortem` and `StepPostMortem` for source compatibility.

Legacy ratchet inputs and explicit skill requests using `pre-mortem`,
`post-mortem`, `pre_mortem`, or `post_mortem` are permanent read aliases. An
old skill request follows exactly one historical `merged-into` edge to the
canonical live tree; no executable legacy skill tree remains.

## Execution-packet direct cut

Every supported execution-packet schema version uses only
`premortem_verdict` and `artifacts.premortem_path`. The verdict is binary:
`PASS` or `FAIL`. Removed packet keys are rejected as unknown properties; there
is no packet alias reader, writer mode, normalization pass, or legacy-readback
fixture. This keeps one plan-readiness fact in the canonical aggregate.

Non-packet compatibility remains separate. For directory reads, canonical
content is considered first and legacy content fills only a missing ID;
different content for the same ID is an error naming both paths. Explicit old
skill requests still follow their permanent redirect pointers.

`.agents/pre-mortems/` is not an executable producer or reader in the current
repository; premortem reports are written under `.agents/council/`. It is
therefore not part of the staged runtime-writer cutover contract.

Directory conflict and explicit-skill redirect fixtures are enforced by
`scripts/check-mortem-compatibility.sh`. Canonical packet behavior is enforced
directly by the root schema and Go packet/storage contract tests.
