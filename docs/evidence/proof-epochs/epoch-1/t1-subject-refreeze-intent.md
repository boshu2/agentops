# T1 subject-refreeze qualification intent

Judge one new evidence-packaging invocation for the unchanged T1 repair under
active proof epoch 0b. The prior T1 FAIL
`7e0e0dd26eefdc9c53660a56ed7a7840668d033aac493f6f0d5bca2930d46e60`
and repair `NOT_PROVEN`
`b00b1fc309de43843670cf4e8f24a2de968a511b75143a5810f964dcfdd5da9b`
are terminal and immutable. Do not reinterpret or overwrite either result.
Do not traverse the candidate RPI dispatcher to issue the binding judgment,
and do not activate epoch 1 while judging it.

## Required criteria

- **T1S-1 — semantic subject refreeze:** the new subject uses the same 37
  declared roots as the rejected repair subject and differs only by explicitly
  excluding CPython-owned `**/__pycache__` directories and `**/*.pyc` files.
  Every remaining entry, digest, kind, and mode is exactly equal to the prior
  subject after removing its 17 cache entries. Recreating or mutating excluded
  bytecode cannot change the manifest; mutating any included entry is terminal.
- **T1S-2 — exact proof descriptor:** the new descriptor preserves all 25
  component refs, digests, modes, the corpus tree identity, transition
  recorder, and empty known-gap set from the repaired descriptor. Its sole
  semantic change is binding the refrozen qualification-subject digest.
- **T1S-3 — repaired kernel remains accepted:** all T1R-1 through T1R-4
  behavior remains green: one exact three-field Plan identity crosses the
  serialized remote boundary without reminting; one shared corpus drives all
  eight production Python and Go readers; hostile references and durable named
  outputs are proven; and the accepted bounded-kernel and external-transition
  behavior does not regress.
- **T1S-4 — exact lineage and stable judgment:** the prior rejected candidate,
  repair candidate, subject manifests, terminal FAIL, and terminal NOT_PROVEN
  remain recoverable by their exact committed identities. The refrozen subject
  verifies before and after fresh validation, `checked` is nonempty, and
  `not_checked` is empty for PASS.
- **T1S-5 — activation readiness:** a PASS binds the exact refrozen subject and
  is structurally suitable for the unchanged epoch-0b-to-epoch-1 activation
  recorder. The validator does not mutate the active pointer or issue a
  candidate-authored semantic verdict.

## Non-goals

- No implementation, schema, corpus, generated projection, or CLI behavior
  change.
- No broad ignored-file exclusion; only CPython bytecode caches are outside
  this evidence subject.
- No retry, campaign, queue, tracker, Git landing, release, or delivery state.
- No mutation of `docs/contracts/proof-contracts/active.json` during
  qualification.
