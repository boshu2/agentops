# T0 pause-metadata repair invocation exact intent

Consume the immutable failed T0 repair verdict
`3c297141dd11978fc3c741733773373a57028b88f24e73b24df5c55fa4e932f7`
and perform one new, bounded experiment that repairs only the pause-ledger
finding. The two prior failed verdicts and both rejected subjects must remain
byte-immutable.

## Required criteria

- **T0P-1 — Self-stable pause state:** the pause ledger names every stable
  predecessor commit and failed verdict, identifies the current metadata-only
  invocation without embedding a self-referential candidate commit hash, and
  delegates exact current-subject identity to the enclosing subject manifest
  and verdict.
- **T0P-2 — Semantic fail-closed check:** the T0 evidence checker verifies the
  pause ledger's lineage, active proof authority, in-flight tranche, and known
  gaps. A hostile copy that keeps `result: PASS` while removing lineage,
  selecting rejected proof authority, or falsely claiming T1 complete is
  rejected.
- **T0P-3 — Immutable rejected history:** the initial T0 FAIL, the T0 repair
  FAIL, and their rejected subject/report artifacts remain exact-byte
  unchanged.

## Declared exclusions

- Repairing any T1 exact-kernel behavior.
- Activating epoch 1.
- Changing either rejected verdict, rejected subject manifest, or failed
  report.
- Changing bootstrap recorder behavior or proof-contract authority.
- Go CLI G0 through G2 implementation.
- Installed skill links, external systems, credentials, or release channels.
