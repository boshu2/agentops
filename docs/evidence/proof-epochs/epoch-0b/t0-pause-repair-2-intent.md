# T0 typed-pause repair invocation exact intent

Consume the immutable failed pause-metadata verdict
`b27793442f1e1067b9ab6d22dec797b263d4dcb93e080ab8b93ff2c4da6695e2`
and perform one new, bounded experiment that repairs only its two semantic
checker bypasses. All three prior failed verdicts and rejected subjects must
remain byte-immutable.

## Required criteria

- **T0PP-1 — Closed-world typed progress:** the pause ledger represents every
  tranche with an enumerated state, identifies exactly one current invocation,
  and has a closed set of fields and exact historical values. Any extra or
  contradictory progress claim is rejected regardless of its wording or
  location.
- **T0PP-2 — Complete active-transition binding:** when the live proof pointer
  advances to epoch 1, the checker verifies the transition file's exact digest,
  binds the active epoch/ref/digest to `transition.candidate`, verifies the
  candidate descriptor bytes, and binds `transition.prior` to epoch 0b. The
  previously accepted fabricated-active-pointer fixture is rejected.
- **T0PP-3 — Immutable rejected history:** all three prior FAILs and their
  rejected subject/report artifacts remain exact-byte unchanged.

## Declared exclusions

- Repairing any T1 exact-kernel behavior.
- Performing a real epoch-1 activation.
- Changing any rejected verdict, rejected subject manifest, or failed report.
- Changing bootstrap recorder behavior or proof-contract authority.
- Go CLI G0 through G2 implementation.
- Installed skill links, external systems, credentials, or release channels.
