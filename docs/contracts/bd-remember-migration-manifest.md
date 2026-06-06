# bd remember Migration Manifest

`scripts/classify-bd-remember.py` is the W4.1 classification surface for
lineage-preserving `bd remember` migration. It reads `bd memories --json` or a
fixture supplied with `--input`, then emits a manifest without mutating the
tracker or the corpus.

The manifest dispositions are:

- `bead`: scoped to a work item or one-off PR/session context; later migration
  attaches or drops it with bead lineage.
- `pull-learning`: general and still potentially useful; later migration writes
  it as `reach=pull`, `maturity=provisional`, `source=migrated-bd-remember`.
- `discard`: empty, stale, deprecated, or invalidated memory; later migration
  records the discard rationale.

Every item preserves the original key, body, timestamps, and raw record under
`lineage`. A complete run has `classified == total` and `unclassified == 0`.
