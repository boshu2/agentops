# T1 repair qualification intent

Judge one revised T1 candidate directly under active proof epoch 0b. This is a
new experiment after the immutable FAIL verdict
`7e0e0dd26eefdc9c53660a56ed7a7840668d033aac493f6f0d5bca2930d46e60`.
Do not reinterpret or overwrite that verdict. Do not traverse the candidate
RPI dispatcher to issue the binding judgment, and do not activate epoch 1 while
judging it.

## Required criteria

- **T1R-1 — one composed exact identity:** Plan mints one exact intent snapshot
  and returns its immutable reference, digest, and byte length. RPI, a
  serialized/deserialized remote boundary, Implement, and fresh Validate
  consume that same identity packet without re-minting or re-reading the living
  source. Whitespace, Unicode normalization, source mutation, packet mutation,
  or a Plan/RPI shape mismatch is terminal.
- **T1R-2 — actual cross-language typed readers:** one shared corpus drives the
  production Python and Go strict readers for `verdict.v3`, `rpi-report.v2`,
  proof identity, `proof-contract-transition.v1`, `subject-manifest.v2`,
  `scope-index.v1`, `check-receipt.v1`, and `effect-receipt.v1`. Valid cases
  pass; duplicate keys, trailing data, unknown or missing fields, hostile
  references, digest mutation, and semantic incoherence fail for the intended
  reason. Bespoke case logic that does not call the production reader is not
  coverage.
- **T1R-3 — hostile references and durable named outputs:** artifact references
  reject raw or normalized dot, empty, parent, trailing-separator, backslash,
  drive-qualified, UNC, absolute, control-character, and symlink-alias forms
  before joining. Repository observation may use `"."` only through its
  distinct root-specific contract. Manifest, scope, check receipt, effect
  receipt, report, and verdict named outputs all use flush, fsync, atomic
  replacement, and parent-directory fsync; a callable check-receipt output is
  exercised.
- **T1R-4 — no regression of accepted kernel behavior:** all behavior previously
  accepted as T1-2 through T1-6 and T1-8 remains green: bounded phase
  cardinality and terminal reports, complete repository effects, frozen stable
  criteria, candidate mutation detection, one linked fresh judgment, and
  external non-self-certifying proof transitions with compare-and-swap and
  active-pointer-last ordering.
- **T1R-5 — exact repair lineage:** the new subject manifest covers every
  repair path and generated companion; the rejected candidate, manifest, and
  FAIL verdict remain recoverable by their exact committed identities; the
  final repair subject is unchanged before and after fresh validation; checked
  is nonempty and not_checked is empty for PASS.

## Non-goals

- No retry, campaign, queue, tracker, Git landing, release, or delivery state.
- No candidate-authored semantic PASS.
- No mutation of `docs/contracts/proof-contracts/active.json` during candidate
  construction or qualification.
- No unrelated CLI or skill-contract migration work.
