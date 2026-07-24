# T0 transition-schema repair invocation exact intent

Consume the immutable failed typed-pause verdict
`bc97dc05ced93855a0e2326f5ddd92dc4814db9b114158e8acc5298e63051d5b`
and perform one new, bounded experiment that repairs only its malformed
transition acceptance. All four prior failed verdicts and rejected subjects
must remain byte-immutable.

## Required criteria

- **T0PS-1 — Whole transition contract:** an advanced active pointer is accepted
  only when the exact transition bytes validate against the frozen
  `proof-contract-transition.v1` schema with format checking and duplicate-key
  rejection. Missing fields, forbidden fields, boolean epochs, malformed
  timestamps, and unsafe nested references fail closed.
- **T0PS-2 — Qualification artifact binding:** the transition's candidate
  descriptor, subject manifest, qualification corpus, qualification PASS
  verdict, validator identity, and every claimed digest/ref agree with exact
  repository-contained artifacts. A fully valid simulated epoch-1 transition
  passes; each independently mutated binding fails.
- **T0PS-3 — Immutable rejected history:** all four prior FAILs and their
  rejected subject/report artifacts remain exact-byte unchanged.

## Declared exclusions

- Repairing any T1 exact-kernel behavior.
- Performing a real epoch-1 activation.
- Changing any rejected verdict, rejected subject manifest, or failed report.
- Changing bootstrap recorder behavior or proof-contract authority.
- Go CLI G0 through G2 implementation.
- Installed skill links, external systems, credentials, or release channels.
