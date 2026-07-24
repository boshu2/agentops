# T1 exact-kernel qualification intent

Judge the exact committed T1 candidate directly under active proof epoch 0b.
Do not traverse the candidate RPI dispatcher and do not activate epoch 1 while
judging it.

## Required criteria

- **T1-1 — exact intent identity:** Plan mints exact source bytes once; Plan,
  Implement, RPI, remote transport, and Validate agree on that SHA-256 digest;
  whitespace, Unicode normalization, and living-source mutation cannot compare
  equal or replace the snapshot.
- **T1-2 — bounded phase cardinality:** Plan, Implement, and fresh Validate each
  run at most once. PASS, FAIL, and NOT_PROVEN all emit `rpi-report.v2` and stop
  without retry, repair revision, campaign, or continuation state. A narrow,
  size-bounded opaque correlation object is preserved unchanged and never
  interpreted.
- **T1-3 — complete subject effects:** before/final manifests observe the
  repository root, derive additions, modifications, deletions, mode/type
  changes, and generated companions. Partial observation is NOT_PROVEN. A
  mutation outside write scope is still observed and is FAIL.
- **T1-4 — frozen acceptance:** criterion IDs are unique and exact; declared
  exclusions cannot absorb required IDs; unchecked required criteria prevent
  PASS.
- **T1-5 — frozen candidate:** candidate mutation after final-manifest freeze is
  terminal.
- **T1-6 — one linked judgment:** invocation and judgment identities are unique;
  duplicate unlinked verdicts over one exact intent/final-subject pair are
  rejected.
- **T1-7 — typed durable proof:** strict readers and one shared Python/Go golden
  corpus cover `verdict.v3`, `rpi-report.v2`, proof identity and transition,
  `subject-manifest.v2`, `scope-index.v1`, `check-receipt.v1`, and
  `effect-receipt.v1`.
- **T1-8 — external proof activation:** verdicts bind the activated proof
  identity and schema bytes. A candidate proof contract cannot activate or
  judge itself. Epoch 1 activation requires a qualifying epoch-0b verdict and a
  standalone `proof-contract-transition.v1`. For every later `N` to `N+1`
  transition, the frozen general recorder accepts only verdict.v3 PASS under
  exact prior identity, requires all candidate components and the next recorder
  in the judged subject, records content-addressed transition bytes, and updates
  the active pointer last by compare and swap.
- **T1-9 — hostile references and durable writes:** drive-qualified, UNC,
  absolute, backslash, empty, dot, parent, and trailing-separator references
  are rejected lexically before path joining. Named manifest, scope, receipt,
  report, and verdict outputs use flush, fsync, and atomic rename.

## Non-goals

- No retry, campaign, queue, tracker, Git, landing, release, or delivery state.
- No semantic PASS minted by the candidate implementation.
- No mutation of `docs/contracts/proof-contracts/active.json` during candidate
  construction or qualification.
