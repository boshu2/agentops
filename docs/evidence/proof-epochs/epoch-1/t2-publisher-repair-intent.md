# T2 transactional publisher repair intent

Date: 2026-07-24

Author context: `codex-root-t2-publisher-repair-author-20260724`

Base commit: `db9bc9e6b`

Parent intent:
`docs/evidence/proof-epochs/epoch-1/t2-intent.md`

Active proof contract:
`f6358e3858d4e6f67844966334547d6df88b58c5a2e9f7f5889ac2d1fadd2340`

## Trigger

An author-distinct adversarial review of the integrated publisher found six
blocking gaps despite the original seven tests passing:

- a declared target whose ancestor is a symlink is rejected only after owner
  generators run, allowing staged generation to escape the repository copy;
- rendered tree entries are not validated before live replacement, so a
  noncanonical POSIX filename can be published and make the durable manifest
  unreadable;
- abrupt exit immediately before the recovery journal or immediately after the
  publication manifest leaves unrecoverable or orphaned transaction state;
- a legitimate owner-map digest change bricks normal publication and pending
  recovery even when target bindings are unchanged;
- the publication lock is the replaceable owner-map inode, allowing two
  publishers to lock different inodes for the same repository; and
- check mode builds a typed receipt internally but prints only a lossy summary.

The observed `__pycache__` collision behavior is correct: unknown entries under
an owned tree stop before mutation and require explicit caller cleanup or
ownership.

## Intent

Make publication preflight incapable of escaping staging, validate every
durable byte and repository reference before live mutation, make every crash
boundary recoverable or safely ignorable, preserve legitimate owner-map
evolution, serialize on a stable state-owned lock, and expose the complete
typed check receipt without making check mode mutating.

## Acceptance

### T2P-1 — Pre-generator containment

All generator, source, and target references plus every existing ancestor are
validated before any owner command runs. A leaf or ancestor symlink that could
escape the staged repository aborts before generator execution. A hostile
target symlink probe proves that no external byte changes.

### T2P-2 — Precommit state validation

Every before, rendered, recovery, and manifest target state passes the same
closed validator before a recovery bundle or live replacement is created.
Directory entry paths reject backslashes, absolute/volume forms, dot segments,
controls, duplicate aliases, and noncanonical separators. A legal host
filename that is illegal in the publication grammar fails before mutation and
cannot create an unreadable manifest.

### T2P-3 — Total crash recovery

Recovery bundles are prepared under an incomplete name and become discoverable
only by one atomic durable commit after their exact journal validates. An
abrupt exit before that commit has not mutated live targets and is safely
discardable. Once live mutation begins, every abrupt-exit hook is recoverable.
If the publication manifest already committed, recovery verifies the exact
committed after-state, removes all old/new sidecars, finalizes a typed recovery
receipt, and never rolls back a committed publication.

### T2P-4 — Owner-map evolution

Pending recovery uses the journal-bound owner-map and target identities rather
than requiring the current map digest. Normal publication accepts a new
owner-map digest when owner/path/kind bindings remain compatible, and writes
the new digest in the next manifest. Removed, retargeted, overlapping, or
ownership-transferred paths require an explicit safe transition and otherwise
abort before mutation. A source-ref-only owner-map evolution converges
`DRIFT -> PUBLISHED -> CLEAN`.

### T2P-5 — Stable serialization

The lock is a fixed file under publication state, not the replaceable owner-map
or any generated target. Atomic replacement of the owner map while one
publisher holds the lock cannot admit a second writer. Lock acquisition,
timeout, and release are behaviorally tested.

### T2P-6 — Complete read-only check receipt

Check mode writes no repository state and emits the complete
schema-valid `publication-receipt.v1` to stdout, including every target
classification and before/rendered identity. Its concise human summary, if
retained, goes to stderr and cannot replace or corrupt machine output. The
check receipt's rendered digest is byte-identical to the following write
receipt.

### T2P-7 — Integrated recovery matrix

Fault injection covers every boundary before journal commit, after each live
target, immediately before manifest replacement, immediately after manifest
replacement, before receipt storage, and during sidecar cleanup. Each case
converges through recovery with exact bytes, modes, kinds, symlink targets,
manifest state, and no transaction or replacement leftovers. Focused tests,
schema validation, fail-fast Bats, real-map disposable publication, projection
parity, and regeneration check pass.

## Write scope

- `scripts/publish-generated-projections.py`;
- the owner map and publisher-owned publication/receipt schemas if required;
- `scripts/regen-all.sh`;
- focused Python/Bats publisher tests;
- this repair intent and later T2 evidence.

## Non-goals

- Do not weaken unowned-entry refusal or silently delete ignored cache files.
- Do not use Git for classification or recovery.
- Do not publish a partial owner set.
- Do not change skill semantics, catalog authority, CLI reader behavior, or
  the independent Go cleanup repair.
- Do not claim non-POSIX locking behavior without a separately implemented
  platform adapter.
