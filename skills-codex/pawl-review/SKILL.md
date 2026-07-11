---
name: pawl-review
description: Run one fresh, read-only, nonce-bound
---
# Pawl Review

Run a single independent reviewer execution. This skill owns the immutable
request, transport-neutral lane result, and evidence handoff. `ao pawl` owns
diversity, panel judgment, commit binding, and admission. A reviewer never fixes
the subject and the author never counts as its own reviewer.

## Constraints

- To prevent mutable review targets, bind the request to the exact head,
  contract digest, diff digest, and nonce before dispatch.
- Because independence is the point of the lane, require a fresh reviewer
  context and read-only execution with contained nonempty evidence.
- To preserve uncertainty honestly, keep transport failure separate from
  semantic CONFIRMED or REFUTED judgment.

## Route

- Use for a fresh-context or multi-model pawl lane after deterministic tests
  have produced a reviewable commit and diff.
- Use NTM when an attachable warm pane materially helps; use cold Codex/AGY
  adapters when it does not. Gas City may implement the same port inside an
  explicitly selected quest.
- Agent Mail is optional coordination for separate live actors. One local lane
  does not need identity, reservations, or acknowledgements.

## Immutable request

Construct `review-request.v1` with subject id, reviewed `head_sha`, acceptance
contract path and SHA-256, diff path and SHA-256, author context/family,
diversity mode, nonce, evidence directory, and `read_only=true`. Validate both
files against their digests immediately before dispatch. A path without a
content digest is mutable input and must not run.

## Lane execution

1. Discover the selected adapter's live capabilities before changing state.
2. Start a fresh reviewer context. Bind the request fields verbatim and permit
   writes only beneath the evidence directory.
3. Observe terminal state and collect the transcript plus a nonempty evidence
   artifact. Do not infer engagement or success from send acknowledgement.
4. Emit `review-lane-result.v1`: lane/family/context ids, echoed nonce,
   read-only attestation, evidence path, findings, and either CONFIRMED or
   REFUTED semantic judgment.
5. Validate evidence after resolving symlinks. It must be a nonempty regular
   file contained beneath the request evidence directory, and reviewer context
   must differ from author context.

Transport failure is not judgment. Provider loss, timeout, missing evidence,
or malformed markers produce a transport-class result with no semantic
disposition. Never manufacture REFUTED, lower the requested tier, or write a
panel verdict to keep the loop moving.

## Output Specification

- **Artifact directory:** the immutable request's `evidence_dir` contains the
  reviewer evidence; the adapter returns the lane result in memory and does not
  persist a result file.
- **Filename convention:** `review-evidence-<digest-prefix>.txt`, where
  `<digest-prefix>` is the first eight SHA-256 bytes, hex-encoded, of
  `subject_id + NUL + head_sha + NUL + nonce`. The in-memory lane result has no
  filesystem filename.
- **Format:** typed Go `ReviewLaneResultV1` implementing the logical
  `review-lane-result.v1` contract. It echoes lane, family, context, nonce,
  read-only attestation, evidence path, findings, and either a semantic
  disposition or a transport failure class, never both.
- **Validation command:** `go -C cli test ./internal/ports ./internal/adapters/reviewlane_worker -run '^(TestReviewRequestV1RejectsMutableOrSelfReview|TestNTMReviewLaneFreshReadOnlyNonce|TestReviewLaneResultV1SeparatesTransportFromSemanticFailure|TestReviewLaneTransportFailureIsNotRefutation)$'`
- **Downstream handoff:** pass the validated lane result and its contained
  evidence path to `ao pawl`; only the membrane writes the panel verdict.

The runtime validation boundary is
`ReviewLaneResultV1.ValidateAgainst(review-request.v1)`, which rechecks the
request digests, nonce, fresh context, read-only attestation, evidence
containment, nonempty regular-file evidence, and transport/semantic separation
before the adapter returns the result.

## Handoff

Hand the validated lane result and evidence path to `ao pawl`. Stop there. The
membrane may CONFIRM, REFUTE, HOLD, or request another lane; this skill has no
authority to choose.

See [the executable behavior contract](references/pawl-review.feature).

## Quality

- The result echoes the request nonce and comes from a context distinct from
  the author context.
- Semantic results attest read-only execution and point to contained, nonempty
  reviewer evidence; REFUTED results include at least one finding.
- Transport failures carry a reason and no semantic disposition, and every
  validated result stops at the `ao pawl` handoff boundary.
