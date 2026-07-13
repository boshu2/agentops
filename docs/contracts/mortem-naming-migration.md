# Mortem Naming Compatibility Contract

`premortem` and `postmortem` are the canonical skill slugs and Brownian
Ratchet values. Writers emit those values. The exported Go identifiers remain
`StepPreMortem` and `StepPostMortem` for source compatibility.

Legacy ratchet inputs and explicit skill requests using `pre-mortem`,
`post-mortem`, `pre_mortem`, or `post_mortem` are permanent read aliases. An
old skill request follows exactly one historical `merged-into` edge to the
canonical live tree; no executable legacy skill tree remains.

## Staged persisted-data migration

S1 through S7 kept execution-packet writers on schema v2 `pre_mortem_*` fields
and the executable compiler writer on `.agents/pre-mortem-checks/`. Schema v1 and v2 own the legacy packet keys; schema v3
owns `premortem_*`. S8 switches current writers to schema v3 and canonical
directories while retaining legacy readback. Equal old and new verdicts
are accepted only as a transition representation and normalize once. Artifact
path aliases never coexist: Draft 2020-12 cannot prove equality between arbitrary
string properties, so accepting both would let schema-only consumers certify
conflicting targets. Conflicting values, wrong-version ownership, and unknown versions fail closed
and identify the offending keys/version. Neither key remains valid when the
calling contract makes the mortem verdict optional. For directory reads, canonical
content is considered first and legacy content fills only a missing ID;
different content for the same ID is an error naming both paths.

`.agents/pre-mortems/` is not an executable producer or reader in the current
repository; premortem reports are written under `.agents/council/`. It is
therefore not part of the staged runtime-writer cutover contract.

Legacy readback, conflict, optional-absence, redirect, and both writer-era
fixtures are enforced by `scripts/check-mortem-compatibility.sh`. The current
cutover gate is `--writer=canonical-v3 --legacy-readback`; the legacy-v2 mode
remains available to prove compatibility with the preceding writer contract.
